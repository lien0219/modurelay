package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
)

const (
	PrincipalTypeUser           = "user"
	PrincipalTypeServiceAccount = "service_account"
)

var ErrExecutionPrincipalInvalid = errors.New("invalid execution principal")

// ExecutionPrincipal identifies the caller. It never identifies the payer or
// the human who created a machine credential.
type ExecutionPrincipal struct {
	Type             string `json:"type"`
	UserID           int64  `json:"user_id,omitempty"`
	ServiceAccountID int64  `json:"service_account_id,omitempty"`
}

func (p ExecutionPrincipal) Validate() error {
	if (p.Type == PrincipalTypeUser && p.UserID > 0 && p.ServiceAccountID == 0) ||
		(p.Type == PrincipalTypeServiceAccount && p.UserID == 0 && p.ServiceAccountID > 0) {
		return nil
	}
	return ErrExecutionPrincipalInvalid
}

func (k *APIKey) ExecutionPrincipal() ExecutionPrincipal {
	if k == nil {
		return ExecutionPrincipal{}
	}
	if k.ServiceAccountID != nil {
		// Service-account credentials are machine identities. Their user_id is
		// SQL NULL; management provenance lives on the service account and must
		// never enter execution attribution.
		return ExecutionPrincipal{Type: PrincipalTypeServiceAccount, ServiceAccountID: *k.ServiceAccountID}
	}
	return ExecutionPrincipal{Type: PrincipalTypeUser, UserID: k.UserID}
}

type executionPrincipalContextKey struct{}

func WithExecutionPrincipal(ctx context.Context, p ExecutionPrincipal) context.Context {
	return context.WithValue(ctx, executionPrincipalContextKey{}, p)
}
func ExecutionPrincipalFromContext(ctx context.Context) ExecutionPrincipal {
	if ctx == nil {
		return ExecutionPrincipal{}
	}
	p, _ := ctx.Value(executionPrincipalContextKey{}).(ExecutionPrincipal)
	return p
}

// HashServiceAccountCredential reuses the auth-cache SHA-256 lookup algorithm.
// Only new machine credentials store this digest; legacy credentials retain
// their original storage and authentication contract.
func HashServiceAccountCredential(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// CredentialAuthCacheDigest permits eviction using a stored digest without
// retaining the raw credential. Custom human keys cannot contain ':' so this
// storage marker cannot be confused with a valid legacy custom key.
func CredentialAuthCacheDigest(key string) string {
	if strings.HasPrefix(key, "sha256:") && len(key) == 71 {
		if _, err := hex.DecodeString(key[7:]); err == nil {
			return key[7:]
		}
	}
	return HashServiceAccountCredential(key)[7:]
}
