package handler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	rate "github.com/Wei-Shaw/sub2api/internal/middleware"
	ippkg "github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

const scimMediaType = "application/scim+json"

type scimRateLimiter interface {
	Allow(context.Context, string, int, time.Duration) (rate.AllowResult, error)
}

type EnterpriseSCIMHandler struct {
	scim     *service.EnterpriseSCIMService
	limiter  scimRateLimiter
	callback string
}

func NewEnterpriseSCIMHandler(scim *service.EnterpriseSCIMService, limiter scimRateLimiter, callback string) *EnterpriseSCIMHandler {
	return &EnterpriseSCIMHandler{scim: scim, limiter: limiter, callback: callback}
}

// SCIMBaseURL uses only retained operator configuration, never a request Host.
func SCIMBaseURL(publicID, callback string) (string, error) {
	u, err := url.Parse(callback)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || !service.ValidSCIMResourceID(publicID) {
		return "", service.NewSCIMError(503, "", "provisioning endpoint origin is not configured")
	}
	return u.Scheme + "://" + u.Host + "/scim/v2/" + publicID, nil
}

func (h *EnterpriseSCIMHandler) Register(r *gin.Engine) {
	// The embedded frontend skips this namespace. Keep unknown protocol paths
	// and methods in the SCIM error contract as well.
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/scim/v2/") {
			writeSCIMError(c, service.NewSCIMError(404, "", "provisioning endpoint not found"))
			return
		}
		c.String(http.StatusNotFound, "404 page not found")
	})
	g := r.Group("/scim/v2/:connector_public_id", h.authenticate)
	g.GET("/ServiceProviderConfig", h.configuration)
	g.GET("/ResourceTypes", h.resourceTypes)
	g.GET("/ResourceTypes/:type", h.resourceTypes)
	g.GET("/Schemas", h.schemas)
	g.GET("/Schemas/:schema", h.schemas)
	for _, resource := range []string{"Users", "Groups"} {
		g.GET("/"+resource, h.resources(resource, "list"))
		g.POST("/"+resource, h.resources(resource, "create"))
		g.GET("/"+resource+"/:resource_id", h.resources(resource, "get"))
		g.PUT("/"+resource+"/:resource_id", h.resources(resource, "replace"))
		g.PATCH("/"+resource+"/:resource_id", h.resources(resource, "patch"))
		g.DELETE("/"+resource+"/:resource_id", h.resources(resource, "delete"))
		g.POST("/"+resource+"/.search", h.unsupported)
	}
	g.Any("/Bulk", h.unsupported)
}

func writeSCIM(c *gin.Context, status int, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		writeSCIMError(c, service.NewSCIMError(500, "", "provisioning operation failed"))
		return
	}
	c.Data(status, scimMediaType, raw)
}
func writeSCIMError(c *gin.Context, err error) {
	var e *service.SCIMError
	if !errors.As(err, &e) {
		e = &service.SCIMError{Status: 500, Detail: "provisioning operation failed"}
	}
	c.Set("scim_outcome_error", e)
	c.Header("Cache-Control", "no-store")
	if e.Status == 401 {
		c.Header("WWW-Authenticate", `Bearer realm="SCIM"`)
	}
	data := gin.H{"schemas": []string{service.SCIMErrorSchema}, "status": strconv.Itoa(e.Status), "detail": e.Detail}
	if e.Type != "" {
		data["scimType"] = e.Type
	}
	writeSCIM(c, e.Status, data)
	c.Abort()
}
func (h *EnterpriseSCIMHandler) allow(c *gin.Context, key string, limit int) bool {
	if h.limiter == nil {
		writeSCIMError(c, service.NewSCIMError(503, "", "provisioning rate limiter unavailable"))
		return false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	out, err := h.limiter.Allow(ctx, key, limit, time.Minute)
	if err != nil {
		writeSCIMError(c, service.NewSCIMError(503, "", "provisioning rate limiter unavailable"))
		return false
	}
	if !out.Allowed {
		seconds := int64((out.RetryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.FormatInt(seconds, 10))
		writeSCIMError(c, service.NewSCIMError(429, "", "provisioning rate limit exceeded"))
		return false
	}
	return true
}
func (h *EnterpriseSCIMHandler) authenticate(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	ip := ippkg.GetSecurityClientIP(c, false)
	if ip == "" {
		ip = c.ClientIP()
	}
	hash := sha256.Sum256([]byte(ip))
	if !h.allow(c, "scim:ip:"+hex.EncodeToString(hash[:16]), 600) {
		return
	}
	if h.scim == nil {
		writeSCIMError(c, service.NewSCIMError(503, "", "provisioning unavailable"))
		return
	}
	headers := c.Request.Header.Values("Authorization")
	if len(headers) != 1 || !strings.HasPrefix(strings.ToLower(headers[0]), "bearer ") || strings.ContainsAny(headers[0][7:], " \t\r\n,") {
		writeSCIMError(c, service.NewSCIMError(401, "", "invalid provisioning credential"))
		return
	}
	p, err := h.scim.Authenticate(c.Request.Context(), c.Param("connector_public_id"), headers[0][7:])
	if err != nil {
		writeSCIMError(c, err)
		return
	}
	if p == nil || p.WorkspaceID <= 0 || p.ConnectorID <= 0 || p.TokenID <= 0 || p.ConnectorRevision <= 0 || !service.ValidSCIMResourceID(p.PublicID) {
		writeSCIMError(c, service.NewSCIMError(401, "", "invalid provisioning credential"))
		return
	}
	if !h.allow(c, fmt.Sprintf("scim:connector:%d", p.ConnectorID), 300) || !h.allow(c, fmt.Sprintf("scim:token:%d", p.TokenID), 150) {
		return
	}
	switch c.Request.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		defer func() {
			success := c.Writer.Status() < 400
			code := ""
			if !success {
				code = "sync_failed"
				if value, exists := c.Get("scim_outcome_error"); exists {
					if failure, ok := value.(error); ok {
						code = scimHTTPOutcomeCode(failure)
					}
				}
			}
			// Exactly one attempt covers validation and unsupported writes. An
			// observer failure must never change the committed HTTP result.
			outcomeCtx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
			defer cancel()
			_ = h.scim.RecordSyncOutcome(outcomeCtx, p, success, code)
		}()
	}
	base, err := SCIMBaseURL(p.PublicID, h.callback)
	if err != nil {
		writeSCIMError(c, err)
		return
	}
	c.Set("scim_principal", p)
	c.Set("scim_base", base)
	c.Next()
}

func readSCIMBody(c *gin.Context, target any) error {
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != scimMediaType && media != "application/json" {
		return service.NewSCIMError(415, "", "JSON media type required")
	}
	if c.Request.ContentLength > service.SCIMMaxPayloadBytes {
		return service.NewSCIMError(400, "invalidValue", "payload too large")
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, service.SCIMMaxPayloadBytes))
	if err != nil {
		return service.NewSCIMError(400, "invalidValue", "invalid payload size")
	}
	if err = service.ValidateSCIMJSON(body); err != nil {
		return err
	}
	d := json.NewDecoder(strings.NewReader(string(body)))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return service.NewSCIMError(400, "invalidValue", "invalid resource payload")
	}
	return nil
}

func (h *EnterpriseSCIMHandler) unsupported(c *gin.Context) {
	writeSCIMError(c, service.NewSCIMError(501, "", "operation not supported"))
}
func scimListResponse(resources any, total, start, count int) gin.H {
	return gin.H{"schemas": []string{service.SCIMListSchema}, "totalResults": total, "startIndex": start, "itemsPerPage": count, "Resources": resources}
}
func scimHTTPOutcomeCode(err error) string {
	var e *service.SCIMError
	if !errors.As(err, &e) {
		return "sync_failed"
	}
	switch e.Status {
	case 409:
		if e.Type == "uniqueness" {
			if e.Detail == "identity cannot be provisioned" {
				return "security_conflict"
			}
			return "uniqueness"
		}
		return "security_conflict"
	case 412:
		return "precondition"
	case 404:
		return "not_found"
	case 400:
		return "invalidValue"
	default:
		return "sync_failed"
	}
}

func (h *EnterpriseSCIMHandler) resources(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, _ := c.Get("scim_principal")
		p, ok := value.(*service.SCIMPrincipal)
		if !ok || p == nil || p.WorkspaceID <= 0 || p.ConnectorID <= 0 || p.TokenID <= 0 || p.ConnectorRevision <= 0 || !service.ValidSCIMResourceID(p.PublicID) {
			writeSCIMError(c, service.NewSCIMError(401, "", "invalid provisioning credential"))
			return
		}
		if h.scim == nil {
			writeSCIMError(c, service.NewSCIMError(503, "", "provisioning unavailable"))
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
		defer cancel()
		base := c.GetString("scim_base")
		id := c.Param("resource_id")
		if action != "list" && action != "create" && !service.ValidSCIMResourceID(id) {
			writeSCIMError(c, service.NewSCIMError(404, "", "resource not found"))
			return
		}
		if action == "list" {
			singular := "User"
			if resource == "Groups" {
				singular = "Group"
			}
			q, err := service.ParseSCIMListQuery(c.Request.URL.Query(), singular)
			if err != nil {
				writeSCIMError(c, err)
				return
			}
			if resource == "Users" {
				rows, total, e := h.scim.ListUsers(ctx, p, q)
				if e != nil {
					writeSCIMError(c, e)
					return
				}
				if rows == nil {
					rows = []service.SCIMUser{}
				}
				for i := range rows {
					decorateSCIMUser(&rows[i], base)
				}
				writeSCIM(c, 200, scimListResponse(rows, total, q.StartIndex, len(rows)))
			} else {
				rows, total, e := h.scim.ListGroups(ctx, p, q)
				if e != nil {
					writeSCIMError(c, e)
					return
				}
				if rows == nil {
					rows = []service.SCIMGroup{}
				}
				for i := range rows {
					decorateSCIMGroup(&rows[i], base)
				}
				writeSCIM(c, 200, scimListResponse(rows, total, q.StartIndex, len(rows)))
			}
			return
		}
		var user *service.SCIMUser
		var group *service.SCIMGroup
		var err error
		if action == "get" {
			if resource == "Users" {
				user, err = h.scim.GetUser(ctx, p, id)
			} else {
				group, err = h.scim.GetGroup(ctx, p, id)
			}
		} else {
			versions := c.Request.Header.Values("If-Match")
			if len(versions) > 1 {
				writeSCIMError(c, service.NewSCIMError(400, "invalidValue", "ambiguous version precondition"))
				return
			}
			version, e := service.ParseSCIMIfMatch(c.GetHeader("If-Match"))
			if e != nil {
				writeSCIMError(c, e)
				return
			}
			var patch *service.SCIMPatchRequest
			var userInput *service.SCIMUserInput
			var groupInput *service.SCIMGroupInput
			switch action {
			case "patch":
				patch = &service.SCIMPatchRequest{}
				err = readSCIMBody(c, patch)
			case "create", "replace":
				if resource == "Users" {
					userInput = &service.SCIMUserInput{}
					err = readSCIMBody(c, userInput)
					if err == nil {
						_, err = service.ValidateSCIMUserInput(userInput)
					}
					if err == nil && userInput.ID != "" && (action == "create" || userInput.ID != id) {
						err = service.NewSCIMError(400, "mutability", "id is server assigned and immutable")
					}
				}
				if resource == "Groups" {
					groupInput = &service.SCIMGroupInput{}
					err = readSCIMBody(c, groupInput)
					if err == nil {
						err = service.ValidateSCIMGroupInput(groupInput)
					}
					if err == nil && groupInput.ID != "" && (action == "create" || groupInput.ID != id) {
						err = service.NewSCIMError(400, "mutability", "id is server assigned and immutable")
					}
				}
			}
			if err != nil {
				writeSCIMError(c, err)
				return
			}
			if resource == "Users" {
				user, err = h.scim.MutateUser(ctx, p, id, service.SCIMUserMutation{Action: action, User: userInput, Patch: patch, IfMatch: version})
			} else {
				group, err = h.scim.MutateGroup(ctx, p, id, service.SCIMGroupMutation{Action: action, Group: groupInput, Patch: patch, IfMatch: version})
			}
		}
		if err != nil {
			writeSCIMError(c, err)
			return
		}
		if action == "delete" {
			c.Status(http.StatusNoContent)
			return
		}
		status := 200
		if action == "create" {
			status = 201
		}
		if user != nil {
			decorateSCIMUser(user, base)
			c.Header("ETag", user.Meta.Version)
			c.Header("Location", user.Meta.Location)
			writeSCIM(c, status, user)
			return
		}
		if group != nil {
			decorateSCIMGroup(group, base)
			c.Header("ETag", group.Meta.Version)
			c.Header("Location", group.Meta.Location)
			writeSCIM(c, status, group)
			return
		}
		writeSCIMError(c, service.NewSCIMError(500, "", "provisioning operation failed"))
	}
}
func decorateSCIMUser(user *service.SCIMUser, base string) {
	user.Meta.ResourceType = "User"
	user.Meta.Location = base + "/Users/" + user.ID
	user.Meta.Version = fmt.Sprintf(`W/"%d"`, user.Revision)
}
func decorateSCIMGroup(group *service.SCIMGroup, base string) {
	group.Meta.ResourceType = "Group"
	group.Meta.Location = base + "/Groups/" + group.ID
	group.Meta.Version = fmt.Sprintf(`W/"%d"`, group.Revision)
	for i := range group.Members {
		group.Members[i].Ref = base + "/Users/" + group.Members[i].Value
	}
}

func (h *EnterpriseSCIMHandler) configuration(c *gin.Context) {
	writeSCIM(c, 200, gin.H{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"}, "patch": gin.H{"supported": true}, "bulk": gin.H{"supported": false, "maxOperations": 0, "maxPayloadSize": 0}, "filter": gin.H{"supported": true, "maxResults": 100}, "changePassword": gin.H{"supported": false}, "sort": gin.H{"supported": false}, "etag": gin.H{"supported": true}, "authenticationSchemes": []gin.H{{"type": "oauthbearertoken", "name": "Provisioning bearer token", "description": "Connector scoped provisioning credential", "primary": true}}})
}
func (h *EnterpriseSCIMHandler) resourceTypes(c *gin.Context) {
	rows := []gin.H{}
	for _, spec := range []struct{ name, schema string }{{"User", service.SCIMUserSchema}, {"Group", service.SCIMGroupSchema}} {
		row := gin.H{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:ResourceType"}, "id": spec.name, "name": spec.name, "endpoint": "/" + spec.name + "s", "schema": spec.schema, "schemaExtensions": []any{}, "meta": gin.H{"resourceType": "ResourceType", "location": c.GetString("scim_base") + "/ResourceTypes/" + spec.name}}
		if c.Param("type") == spec.name {
			writeSCIM(c, 200, row)
			return
		}
		rows = append(rows, row)
	}
	if c.Param("type") != "" {
		writeSCIMError(c, service.NewSCIMError(404, "", "resource type not found"))
		return
	}
	writeSCIM(c, 200, scimListResponse(rows, len(rows), 1, len(rows)))
}

func scimSchemaAttribute(name, typ string, multi, required bool, mutability, uniqueness string, sub []gin.H) gin.H {
	row := gin.H{"name": name, "type": typ, "multiValued": multi, "required": required, "caseExact": false, "mutability": mutability, "returned": "default", "uniqueness": uniqueness}
	if sub != nil {
		row["subAttributes"] = sub
	}
	return row
}
func (h *EnterpriseSCIMHandler) schemas(c *gin.Context) {
	attr := func(name, typ string, multi, required bool) gin.H {
		return scimSchemaAttribute(name, typ, multi, required, "readWrite", "none", nil)
	}
	common := []gin.H{scimSchemaAttribute("id", "string", false, true, "readOnly", "server", nil), attr("externalId", "string", false, false)}
	userAttrs := append(append([]gin.H{}, common...), scimSchemaAttribute("userName", "string", false, true, "readWrite", "server", nil), attr("active", "boolean", false, false), attr("displayName", "string", false, false), scimSchemaAttribute("name", "complex", false, false, "readWrite", "none", []gin.H{attr("givenName", "string", false, false), attr("familyName", "string", false, false)}), scimSchemaAttribute("emails", "complex", true, true, "readWrite", "none", []gin.H{attr("value", "string", false, true), attr("type", "string", false, false), attr("primary", "boolean", false, false), attr("display", "string", false, false)}))
	groupAttrs := append(append([]gin.H{}, common...), attr("displayName", "string", false, true), scimSchemaAttribute("members", "complex", true, false, "readWrite", "none", []gin.H{scimSchemaAttribute("value", "string", false, true, "immutable", "none", nil), scimSchemaAttribute("$ref", "reference", false, false, "readOnly", "none", nil), attr("display", "string", false, false)}))
	rows := []gin.H{}
	for _, spec := range []struct {
		id, name   string
		attributes []gin.H
	}{{service.SCIMUserSchema, "User", userAttrs}, {service.SCIMGroupSchema, "Group", groupAttrs}} {
		row := gin.H{"schemas": []string{"urn:ietf:params:scim:schemas:core:2.0:Schema"}, "id": spec.id, "name": spec.name, "attributes": spec.attributes, "meta": gin.H{"resourceType": "Schema", "location": c.GetString("scim_base") + "/Schemas/" + spec.id}}
		if c.Param("schema") == spec.id {
			writeSCIM(c, 200, row)
			return
		}
		rows = append(rows, row)
	}
	if c.Param("schema") != "" {
		writeSCIMError(c, service.NewSCIMError(404, "", "schema not found"))
		return
	}
	writeSCIM(c, 200, scimListResponse(rows, len(rows), 1, len(rows)))
}
