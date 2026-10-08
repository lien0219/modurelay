package middleware

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// AuthSubject is the minimal authenticated identity stored in gin context.
// Decision: {UserID int64, Concurrency int}
type AuthSubject struct {
	PrincipalType        string
	ServiceAccountID     int64
	BillingUserID        int64
	UserID               int64
	Concurrency          int
	AuthMethod           string
	AuthenticatedAt      time.Time
	MFASatisfied         bool
	MFAEnrolled          bool
	OIDCProviderID       int64
	OIDCProviderRevision int64
	OIDCWorkspaceID      int64
	OIDCAuthenticatedAt  time.Time
	OIDCValidUntil       time.Time
}

func setHumanJWTAuthentication(c *gin.Context, user *service.User, claims *service.JWTClaims) {
	c.Set(string(ContextKeyUser), AuthSubject{
		PrincipalType: service.PrincipalHuman, UserID: user.ID, Concurrency: user.Concurrency,
		AuthMethod: claims.AuthMethod, AuthenticatedAt: claims.AuthenticatedAt, MFASatisfied: claims.MFASatisfied, MFAEnrolled: user.TotpEnabled,
		OIDCProviderID: claims.OIDCProviderID, OIDCProviderRevision: claims.OIDCProviderRevision, OIDCWorkspaceID: claims.OIDCWorkspaceID, OIDCAuthenticatedAt: claims.OIDCAuthenticatedAt, OIDCValidUntil: claims.OIDCValidUntil,
	})
	ctx := service.WithSessionAuthentication(c.Request.Context(), service.SessionAuthentication{AuthMethod: claims.AuthMethod, AuthenticatedAt: claims.AuthenticatedAt, MFASatisfied: claims.MFASatisfied, MFAEnrolled: user.TotpEnabled})
	ctx = service.WithAuthenticationAssurance(ctx, service.WorkspaceAssurance{WorkspaceID: claims.OIDCWorkspaceID, ProviderID: claims.OIDCProviderID, ProviderRevision: claims.OIDCProviderRevision, AuthenticatedAt: claims.OIDCAuthenticatedAt, ValidUntil: claims.OIDCValidUntil, AuthMethod: claims.AuthMethod})
	c.Request = c.Request.WithContext(ctx)
}

// FundingUserID selects the payer for machine concurrency controls. Human keys
// retain their existing execution-user concurrency namespace and limit.
func (s AuthSubject) FundingUserID() int64 {
	if s.ServiceAccountID > 0 && s.BillingUserID > 0 {
		return s.BillingUserID
	}
	return s.UserID
}

// OwnershipID is a transient cache namespace, never a SQL user identifier.
func (s AuthSubject) OwnershipID() int64 {
	if s.ServiceAccountID > 0 {
		return -s.ServiceAccountID
	}
	return s.UserID
}

func authSubjectForAPIKey(k *service.APIKey) AuthSubject {
	p := k.ExecutionPrincipal()
	subject := AuthSubject{PrincipalType: p.Type, UserID: p.UserID, ServiceAccountID: p.ServiceAccountID, BillingUserID: k.BillingUserID()}
	concurrencyUser := k.User
	if k.ServiceAccountID != nil {
		concurrencyUser = k.BillingUser()
	}
	if concurrencyUser != nil {
		subject.Concurrency = concurrencyUser.Concurrency
	}
	return subject
}

func apiKeySubjectRole(k *service.APIKey) string {
	if k.ServiceAccountID != nil {
		return service.PrincipalTypeServiceAccount
	}
	if k.User != nil {
		return k.User.Role
	}
	return ""
}

func GetAuthSubjectFromContext(c *gin.Context) (AuthSubject, bool) {
	value, exists := c.Get(string(ContextKeyUser))
	if !exists {
		return AuthSubject{}, false
	}
	subject, ok := value.(AuthSubject)
	return subject, ok
}

func GetUserRoleFromContext(c *gin.Context) (string, bool) {
	value, exists := c.Get(string(ContextKeyUserRole))
	if !exists {
		return "", false
	}
	role, ok := value.(string)
	return role, ok
}
