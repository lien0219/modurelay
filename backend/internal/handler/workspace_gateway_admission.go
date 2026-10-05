package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/requestmodel"
	"github.com/Wei-Shaw/sub2api/internal/service"
	coderws "github.com/coder/websocket"
)

func (h *OpenAIGatewayHandler) admitWorkspaceWSTurn(ctx context.Context, key *service.APIKey, payload []byte, model string) (*service.APIKey, error) {
	latest := key
	if h.apiKeyService != nil {
		var e error
		latest, e = h.apiKeyService.TenantAdmissionSnapshot(ctx, key)
		if e != nil {
			return nil, service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "tenant access denied", e)
		}
	} else if key.Tenant != nil {
		return nil, service.NewOpenAIWSClientCloseError(coderws.StatusInternalError, "tenant admission unavailable", nil)
	}
	// A live connection is bound to its original financial and routing identity.
	// Changing those identities requires reconnecting; retained turn snapshots
	// consequently cannot be rebound to a different payer by a later turn.
	if latest.BillingUserID() != key.BillingUserID() || !sameKeyGroup(latest.GroupID, key.GroupID) {
		return nil, service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, "tenant identity changed; reconnect", nil)
	}
	candidates := requestmodel.FromBodyCandidates("", "application/json", payload)
	if model != "" {
		candidates = append(candidates, model)
	}
	for _, candidate := range candidates {
		if !latest.AllowsModel(candidate) {
			return nil, service.NewOpenAIWSClientCloseError(coderws.StatusPolicyViolation, fmt.Sprintf("Model %q is not available for this group or project", candidate), nil)
		}
	}
	return latest, nil
}
func sameKeyGroup(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// Filter only entries while preserving all catalogue envelope and provider
// metadata. Malformed restricted catalogues fail closed in the caller.
func filterTenantModelCatalog(body []byte, key *service.APIKey) ([]byte, error) {
	if key == nil || key.Tenant == nil || key.Tenant.AllowedModels == nil {
		return body, nil
	}
	var envelope map[string]json.RawMessage
	if e := json.Unmarshal(body, &envelope); e != nil {
		return nil, e
	}
	for _, field := range []string{"data", "models"} {
		raw, ok := envelope[field]
		if !ok {
			continue
		}
		var entries []json.RawMessage
		if e := json.Unmarshal(raw, &entries); e != nil {
			return nil, e
		}
		kept := []json.RawMessage{}
		for _, entry := range entries {
			var model struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Slug string `json:"slug"`
			}
			if e := json.Unmarshal(entry, &model); e != nil {
				return nil, e
			}
			id := model.ID
			if id == "" {
				id = model.Name
			}
			if id == "" {
				id = model.Slug
			}
			if id != "" && key.AllowsModel(id) {
				kept = append(kept, entry)
			}
		}
		encoded, e := json.Marshal(kept)
		if e != nil {
			return nil, e
		}
		envelope[field] = encoded
	}
	return json.Marshal(envelope)
}
