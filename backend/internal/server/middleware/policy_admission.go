package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/googleapi"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PolicyAdmissionMiddleware applies the persisted hierarchy after API-key
// authentication. It is intentionally independent from billing and scheduler
// admission so every protocol route can share the same model/platform gate.
func PolicyAdmissionMiddleware(resolver *domain.EffectivePolicyResolver, quotaServices ...*service.PolicyQuotaService) gin.HandlerFunc {
	var quotaService *service.PolicyQuotaService
	if len(quotaServices) > 0 {
		quotaService = quotaServices[0]
	}
	return func(c *gin.Context) {
		if resolver == nil {
			c.Next()
			return
		}
		key, ok := GetAPIKeyFromContext(c)
		if !ok || key == nil {
			c.Next()
			return
		}
		requestCtx := service.WithEffectivePolicyResolver(c.Request.Context(), resolver)
		if quotaService != nil {
			requestCtx = service.WithPolicyQuotaService(requestCtx, quotaService)
		}
		c.Request = c.Request.WithContext(requestCtx)

		policy, cachedOK := policyFromContext(c.Request.Context())
		if !cachedOK {
			var err error
			var legacy policyAdmissionLegacyLayers
			policy, legacy, err = policyForAPIKey(c, resolver, key)
			if err != nil {
				policyAdmissionError(c, err)
				return
			}
			ctx := contextWithPolicy(c.Request.Context(), policy)
			ctx = contextWithLegacyPolicyLayers(ctx, legacy)
			c.Request = c.Request.WithContext(ctx)
		}
		admissionPolicy := policyForAdmission(policy, legacyPolicyLayersFromContext(c.Request.Context()))

		models, err := policyRequestModels(c)
		if err != nil {
			writePolicyBodyReadError(c, err)
			return
		}
		if requested, ok := service.RequestedPublicModelFromContext(c.Request.Context()); ok && requested != "" {
			models = append(models, requested)
		}
		if upstream, ok := service.ResolvedUpstreamModelFromContext(c.Request.Context()); ok && upstream != "" {
			models = append(models, upstream)
		}
		platform := policyPlatform(c, key)
		if platform != "" && !admissionPolicy.AllowsPlatform(platform) {
			policyDenied(c, http.StatusForbidden, "POLICY_PLATFORM_DENIED", "platform is not allowed by the effective policy", policyPlatformScope(admissionPolicy, platform))
			return
		}
		if len(models) == 0 {
			c.Next()
			return
		}

		// The first pass runs before composite routing and checks the public
		// model. The second pass uses the resolver's context values to check the
		// mapped model and concrete provider selected by the scheduler.
		for _, model := range models {
			if !admissionPolicy.AllowsModel(model) {
				policyDenied(c, http.StatusForbidden, "POLICY_MODEL_DENIED", "model is not allowed by the effective policy", policyModelScope(admissionPolicy, model))
				return
			}
		}

		// Composite routes are resolved between the two policy middleware passes.
		// Delay quota admission until the concrete target is known so a denied
		// candidate never consumes a request reservation.
		if quotaService != nil && policyQuotaAdmissionEligible(c, key) && service.PolicyQuotaReservationFromContext(c.Request.Context()) == nil {
			if key.Tenant != nil && key.Tenant.WorkspaceID > 0 && key.Tenant.ProjectID > 0 {
				requestID := policyQuotaRequestID(c)
				estimatedTokens := policyQuotaTokenEstimate(c)
				requestUnits := policyQuotaRequestUnits(c)
				handle, err := quotaService.Admit(c.Request.Context(), policy, service.PolicyQuotaAttribution{
					PolicyContext: domain.PolicyContext{
						WorkspaceID:      key.Tenant.WorkspaceID,
						ProjectID:        key.Tenant.ProjectID,
						ServiceAccountID: valueOrZero(key.ServiceAccountID),
					},
					APIKeyID:     key.ID,
					RequestUnits: requestUnits,
				}, requestID, estimatedTokens)
				if err != nil {
					policyQuotaAdmissionError(c, err)
					return
				}
				if handle != nil {
					ctx := service.WithPolicyQuotaReservation(c.Request.Context(), handle)
					ctx = service.WithPolicyQuotaService(ctx, quotaService)
					c.Request = c.Request.WithContext(ctx)
					defer cleanupPolicyQuotaReservation(ctx, handle)
				}
			}
		}
		c.Next()
	}
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func policyQuotaAdmissionEligible(c *gin.Context, key *service.APIKey) bool {
	if c == nil || c.Request == nil || key == nil {
		return false
	}
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch:
	default:
		return false
	}
	if key.Group != nil && key.Group.Platform == service.PlatformComposite {
		if _, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); !ok {
			return false
		}
	}
	return true
}

func policyQuotaRequestID(c *gin.Context) string {
	if c != nil && c.Request != nil {
		if requestID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string); strings.TrimSpace(requestID) != "" {
			return strings.TrimSpace(requestID)
		}
		if requestID := strings.TrimSpace(c.GetHeader("X-Client-Request-ID")); requestID != "" {
			return requestID
		}
	}
	return uuid.NewString()
}

func policyQuotaTokenEstimate(c *gin.Context) int64 {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return 1
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		return 1
	}
	requestmodel.ResetRequestBody(c.Request, body)
	if len(body) == 0 {
		return 1
	}
	// This deliberately stays a protocol-neutral fallback. Provider usage is
	// authoritative at finalize; the estimate only prevents a hard token quota
	// from being bypassed before an output length is known.
	estimate := int64((len(body) + 3) / 4)
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		for _, field := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens"} {
			if value, ok := payload[field].(float64); ok && value > 0 && value < 1e9 {
				estimate += int64(value)
				break
			}
		}
	}
	if estimate <= 0 {
		return 1
	}
	return estimate
}

// policyQuotaRequestUnits counts logical requests represented by one gateway
// call. Image batches must consume one request-quota unit per item so the
// batch endpoint cannot turn a large request into a single quota event.
func policyQuotaRequestUnits(c *gin.Context) int64 {
	if c == nil || c.Request == nil {
		return 1
	}
	path := strings.TrimRight(strings.TrimSpace(c.FullPath()), "/")
	if path == "" {
		path = strings.TrimRight(strings.TrimSpace(c.Request.URL.Path), "/")
	}
	if !strings.HasSuffix(path, "/images/batches") {
		return 1
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		return 1
	}
	requestmodel.ResetRequestBody(c.Request, body)
	var payload struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || len(payload.Items) == 0 {
		return 1
	}
	return int64(len(payload.Items))
}

func cleanupPolicyQuotaReservation(ctx context.Context, handle *service.PolicyQuotaReservationHandle) {
	if handle == nil || handle.Finalized() {
		return
	}
	// An accepted asynchronous task transfers ownership to its detached
	// execution. The task must decide release/finalize/preserve after the
	// provider boundary instead of request middleware releasing it on return.
	if handle.Durable() {
		return
	}
	cleanupCtx := context.Background()
	if ctx != nil {
		cleanupCtx = context.WithoutCancel(ctx)
	}
	if handle.ProviderStarted() {
		if handle.ProviderRejected() {
			if err := handle.FinalizeRequestOnly(cleanupCtx); err != nil {
				// Keep the pending reservation durable so a later retry can
				// settle the request-only outcome after a transient DB failure.
				return
			}
			return
		}
		handle.Preserve()
		return
	}
	if err := handle.Release(cleanupCtx); err != nil {
		// Admission already succeeded, so a release failure must be visible to
		// operations. The reservation remains pending for recovery instead of
		// being treated as successfully released.
		return
	}
}

func policyQuotaAdmissionError(c *gin.Context, err error) {
	if errors.Is(err, service.ErrPolicyQuotaExceeded) {
		c.Header("Retry-After", strconv.Itoa(60))
		policyDenied(c, http.StatusTooManyRequests, "POLICY_QUOTA_EXCEEDED", "policy quota exhausted", "")
		return
	}
	// A configured durable quota cannot fail open. Keep the response generic so
	// SQL details and tenant state are never disclosed to gateway callers.
	policyAdmissionError(c, err)
}

type policyAdmissionLegacyLayers struct {
	group      bool
	credential bool
}

type policyAdmissionLegacyLayersContextKey struct{}

func contextWithLegacyPolicyLayers(ctx context.Context, layers policyAdmissionLegacyLayers) context.Context {
	return context.WithValue(ctx, policyAdmissionLegacyLayersContextKey{}, layers)
}

func legacyPolicyLayersFromContext(ctx context.Context) policyAdmissionLegacyLayers {
	if ctx == nil {
		return policyAdmissionLegacyLayers{}
	}
	layers, _ := ctx.Value(policyAdmissionLegacyLayersContextKey{}).(policyAdmissionLegacyLayers)
	return layers
}

// policyForAdmission removes only the model/platform predicates projected from
// legacy Group/API-key fields. Those predicates continue to be enforced by the
// existing GroupModelAllowlist middleware with its established 404 contract;
// the full projected policy remains in context for provenance and numeric RPM.
func policyForAdmission(policy domain.EffectivePolicy, legacy policyAdmissionLegacyLayers) domain.EffectivePolicy {
	if !legacy.group && !legacy.credential {
		return policy
	}
	out := policy
	if legacy.group && out.Layers.Group != nil {
		group := *out.Layers.Group
		group.AllowedModels = nil
		group.AllowedPlatforms = nil
		out.Layers.Group = &group
	}
	if legacy.credential && out.Layers.Credential != nil {
		credential := *out.Layers.Credential
		credential.AllowedModels = nil
		credential.AllowedPlatforms = nil
		out.Layers.Credential = &credential
	}
	return out
}

func policyForAPIKey(c *gin.Context, resolver *domain.EffectivePolicyResolver, key *service.APIKey) (domain.EffectivePolicy, policyAdmissionLegacyLayers, error) {
	// Group and credential restrictions are supplied by the existing runtime
	// objects below. The SQL repository intentionally persists only the new
	// tenant scopes, so do not ask it for unsupported legacy rows.
	return resolvePolicyForAPIKey(c.Request.Context(), resolver, key)
}

// ResolvePolicyForAPIKey re-evaluates the hierarchy from the authenticated
// API-key snapshot. It is used by long-lived transports between logical turns;
// callers must pass the server-resolved key and never client tenant IDs.
func ResolvePolicyForAPIKey(ctx context.Context, resolver *domain.EffectivePolicyResolver, key *service.APIKey) (domain.EffectivePolicy, error) {
	policy, _, err := resolvePolicyForAPIKey(ctx, resolver, key)
	return policy, err
}

func resolvePolicyForAPIKey(ctx context.Context, resolver *domain.EffectivePolicyResolver, key *service.APIKey) (domain.EffectivePolicy, policyAdmissionLegacyLayers, error) {
	if key == nil {
		return domain.EffectivePolicy{}, policyAdmissionLegacyLayers{}, fmt.Errorf("%w: API key is required", domain.ErrInvalidPolicy)
	}
	identity := domain.PolicyContext{APIKeyID: key.ID, CredentialID: key.ID}
	if key.GroupID != nil {
		identity.GroupID = *key.GroupID
	} else if key.Group != nil {
		identity.GroupID = key.Group.ID
	}
	if key.Tenant != nil {
		identity.WorkspaceID = key.Tenant.WorkspaceID
		identity.ProjectID = key.Tenant.ProjectID
	}
	if key.ServiceAccountID != nil {
		identity.ServiceAccountID = *key.ServiceAccountID
	}
	policy, err := resolver.Resolve(ctx, identity)
	if err != nil {
		return domain.EffectivePolicy{}, policyAdmissionLegacyLayers{}, err
	}
	return projectLegacyPolicyLayers(policy, key)
}

// projectLegacyPolicyLayers keeps legacy Group/API-key model and platform
// restrictions in the effective-policy snapshot used by gateway admission.
// Legacy Group RPM remains on its existing per-user counter path.
func projectLegacyPolicyLayers(policy domain.EffectivePolicy, key *service.APIKey) (domain.EffectivePolicy, policyAdmissionLegacyLayers, error) {
	legacy := policyAdmissionLegacyLayers{}
	if key == nil {
		return policy, legacy, nil
	}
	if policy.Layers.Group == nil && key.Group != nil {
		group := key.Group
		projectedGroup := &domain.Policy{Scope: domain.PolicyScopeGroup, ScopeID: group.ID}
		if group.ModelAllowlist.Enabled {
			projectedGroup.AllowedModels = append([]string(nil), group.ModelAllowlist.Models...)
			if group.ModelAllowlist.Models != nil && len(group.ModelAllowlist.Models) == 0 {
				projectedGroup.AllowedModels = []string{}
			}
		}
		if group.Platform != "" && group.Platform != service.PlatformComposite {
			projectedGroup.AllowedPlatforms = []string{group.Platform}
		}
		if projectedGroup.AllowedModels != nil || projectedGroup.AllowedPlatforms != nil {
			policy.Layers.Group = projectedGroup
			legacy = policyAdmissionLegacyLayers{group: true}
		}
	}
	if policy.Layers.Credential == nil && key.Tenant != nil && key.Tenant.AllowedModels != nil {
		policy.Layers.Credential = &domain.Policy{
			Scope:         domain.PolicyScopeCredential,
			ScopeID:       key.ID,
			AllowedModels: append([]string(nil), key.Tenant.AllowedModels...),
		}
		legacy.credential = true
	}
	if projected, err := domain.ResolveEffectivePolicy(policy.Layers); err != nil {
		return domain.EffectivePolicy{}, legacy, err
	} else {
		return projected, legacy, nil
	}
}

func policyRequestModels(c *gin.Context) ([]string, error) {
	if c == nil || c.Request == nil {
		return nil, nil
	}
	if model := strings.TrimSpace(c.Param("model")); model != "" {
		return []string{model}, nil
	}
	if action := strings.Trim(strings.TrimSpace(c.Param("modelAction")), "/"); action != "" {
		if idx := strings.LastIndex(action, ":"); idx >= 0 {
			return []string{strings.TrimSpace(action[:idx])}, nil
		}
		return []string{action}, nil
	}
	if model := strings.TrimSpace(c.Query("model")); model != "" {
		return []string{model}, nil
	}
	if c.Request.Method != http.MethodPost && c.Request.Method != http.MethodPut && c.Request.Method != http.MethodPatch {
		return nil, nil
	}
	body, err := httputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		return nil, err
	}
	requestmodel.ResetRequestBody(c.Request, body)
	models := requestmodel.FromBodyCandidates(c.FullPath(), c.GetHeader("Content-Type"), body)
	return models, nil
}

func policyPlatform(c *gin.Context, key *service.APIKey) string {
	if platform, ok := c.Request.Context().Value(ctxkey.ForcePlatform).(string); ok && strings.TrimSpace(platform) != "" {
		return strings.TrimSpace(platform)
	}
	if platform, ok := service.ResolvedTargetPlatformFromContext(c.Request.Context()); ok {
		return platform
	}
	if key != nil && key.Group != nil && key.Group.Platform != service.PlatformComposite {
		return key.Group.Platform
	}
	return ""
}

func contextWithPolicy(ctx context.Context, policy domain.EffectivePolicy) context.Context {
	return service.WithEffectivePolicy(ctx, policy)
}

func policyFromContext(ctx context.Context) (domain.EffectivePolicy, bool) {
	return service.EffectivePolicyFromContext(ctx)
}

func DenyPolicyPlatform(c *gin.Context, platform string) {
	policy, _ := policyFromContext(c.Request.Context())
	policyDenied(c, http.StatusForbidden, "POLICY_PLATFORM_DENIED", "platform is not allowed by the effective policy", policyPlatformScope(policy, platform))
}

func policyModelScope(policy domain.EffectivePolicy, model string) domain.PolicyScope {
	for _, entry := range []struct {
		scope  domain.PolicyScope
		policy *domain.Policy
	}{{domain.PolicyScopeGroup, policy.Layers.Group}, {domain.PolicyScopeWorkspace, policy.Layers.Workspace}, {domain.PolicyScopeProject, policy.Layers.Project}, {domain.PolicyScopeServiceAccount, policy.Layers.ServiceAccount}, {domain.PolicyScopeCredential, policy.Layers.Credential}} {
		if entry.policy != nil && !entry.policy.AllowsModel(model) {
			return entry.scope
		}
	}
	return ""
}

func policyPlatformScope(policy domain.EffectivePolicy, platform string) domain.PolicyScope {
	for _, entry := range []struct {
		scope  domain.PolicyScope
		policy *domain.Policy
	}{{domain.PolicyScopeGroup, policy.Layers.Group}, {domain.PolicyScopeWorkspace, policy.Layers.Workspace}, {domain.PolicyScopeProject, policy.Layers.Project}, {domain.PolicyScopeServiceAccount, policy.Layers.ServiceAccount}, {domain.PolicyScopeCredential, policy.Layers.Credential}} {
		if entry.policy != nil && !entry.policy.AllowsPlatform(platform) {
			return entry.scope
		}
	}
	return ""
}

func policyDenied(c *gin.Context, status int, code, message string, scope domain.PolicyScope) {
	writePolicyGatewayError(c, status, code, message, scope)
	c.Abort()
}

func writePolicyGatewayError(c *gin.Context, status int, code, message string, scope domain.PolicyScope) {
	if scope != "" {
		message = fmt.Sprintf("%s (scope=%s)", message, scope)
	}
	path := c.Request.URL.Path
	if strings.HasPrefix(path, "/v1beta") || strings.HasPrefix(path, "/antigravity/v1beta") {
		errorBody := gin.H{"code": status, "message": message, "status": googleapi.HTTPStatusToGoogleStatus(status)}
		errorBody["details"] = []gin.H{{
			"@type":    "type.googleapis.com/google.rpc.ErrorInfo",
			"reason":   code,
			"metadata": gin.H{"scope": string(scope)},
		}}
		c.JSON(status, gin.H{"error": errorBody})
	} else if strings.Contains(path, "/messages") {
		errorType := "api_error"
		if status == http.StatusForbidden {
			errorType = "permission_error"
		}
		c.JSON(status, gin.H{"type": "error", "error": gin.H{"type": errorType, "message": message, "code": code}})
	} else {
		apiCode := code
		errorBody := gin.H{"message": message, "type": "invalid_request_error"}
		if code == "POLICY_MODEL_DENIED" {
			apiCode = "model_not_found"
			errorBody["policy_code"] = code
		}
		errorBody["code"] = apiCode
		c.JSON(status, gin.H{"error": errorBody})
	}
}

func policyAdmissionError(c *gin.Context, _ error) {
	writePolicyGatewayError(c, http.StatusServiceUnavailable, "POLICY_UNAVAILABLE", "Policy admission is temporarily unavailable", "")
	c.Abort()
}

func writePolicyBodyReadError(c *gin.Context, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writePolicyGatewayError(c, http.StatusRequestEntityTooLarge, "REQUEST_BODY_TOO_LARGE", "Request body is too large", "")
		c.Abort()
		return
	}
	writePolicyGatewayError(c, http.StatusBadRequest, "INVALID_REQUEST_BODY", "Request body could not be read", "")
	c.Abort()
}
