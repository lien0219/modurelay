package service

import (
	"net/mail"
	"regexp"
	"strings"
)

var enterpriseClaimPath = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*(\.[A-Za-z_][A-Za-z0-9_-]*){0,7}$`)

// Claim paths are deliberately limited to object properties. The signed sub,
// issuer, audience and nonce can never be remapped by a workspace administrator.
func validateOIDCClaimMapping(mapping map[string]any) error {
	if len(mapping) > 4 {
		return ErrEnterpriseIdentityInvalid
	}
	for key, value := range mapping {
		if key != "email" && key != "name" && key != "groups" && key != "email_verified" {
			return ErrEnterpriseIdentityInvalid
		}
		path, ok := value.(string)
		if !ok || len(path) > 200 || !enterpriseClaimPath.MatchString(path) {
			return ErrEnterpriseIdentityInvalid
		}
	}
	return nil
}

func enterpriseClaim(raw map[string]any, path string) (any, bool) {
	var current any = raw
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func ApplyOIDCClaimMapping(claims *OIDCClaims, mapping map[string]any) error {
	if claims == nil || validateOIDCClaimMapping(mapping) != nil {
		return ErrEnterpriseIdentityInvalid
	}
	path := func(key string) string {
		if configured, ok := mapping[key].(string); ok {
			return configured
		}
		return key
	}
	claims.Email, claims.Name, claims.EmailVerified = "", "", false
	if value, ok := enterpriseClaim(claims.Raw, path("email")); ok {
		text, valid := value.(string)
		if !valid || len(text) > 320 {
			return ErrEnterpriseIdentityInvalid
		}
		text = strings.ToLower(strings.TrimSpace(text))
		address, err := mail.ParseAddress(text)
		if err != nil || address.Address != text {
			return ErrEnterpriseIdentityInvalid
		}
		at := strings.LastIndex(text, "@")
		domain, err := NormalizeEnterpriseDomain(text[at+1:])
		if err != nil {
			return ErrEnterpriseIdentityInvalid
		}
		claims.Email = text[:at+1] + domain
	}
	if value, ok := enterpriseClaim(claims.Raw, path("email_verified")); ok {
		claims.EmailVerified, _ = value.(bool)
	}
	if value, ok := enterpriseClaim(claims.Raw, path("name")); ok {
		text, valid := value.(string)
		if !valid || len(text) > 256 {
			return ErrEnterpriseIdentityInvalid
		}
		claims.Name = strings.TrimSpace(text)
	}
	value, present := enterpriseClaim(claims.Raw, path("groups"))
	claims.Groups, claims.GroupsPresent, claims.GroupsComplete = nil, present, present
	if overage, _ := claims.Raw["hasgroups"].(bool); overage {
		claims.GroupsComplete = false
	}
	if names, ok := claims.Raw["_claim_names"].(map[string]any); ok {
		if _, distributed := names[path("groups")]; distributed {
			claims.GroupsComplete = false
		}
		if _, distributed := names["groups"]; distributed {
			claims.GroupsComplete = false
		}
	}
	if !present || !claims.GroupsComplete {
		return nil
	}
	var values []any
	switch groups := value.(type) {
	case []any:
		values = groups
	case []string:
		for _, group := range groups {
			values = append(values, group)
		}
	case string:
		values = []any{groups}
	default:
		return ErrEnterpriseIdentityInvalid
	}
	if len(values) > 200 {
		return ErrEnterpriseIdentityInvalid
	}
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		group, ok := value.(string)
		if !ok || len(group) > 512 || strings.ContainsAny(group, "\x00\r\n") {
			return ErrEnterpriseIdentityInvalid
		}
		group = strings.TrimSpace(group)
		if group != "" && !seen[group] {
			claims.Groups = append(claims.Groups, group)
			seen[group] = true
		}
	}
	return nil
}
