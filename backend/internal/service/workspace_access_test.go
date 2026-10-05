package service

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestWorkspacePermissions(t *testing.T) {
	roles := map[string][]string{
		"owner":     {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "workspace.update", "workspace.archive", "member.read", "invitation.read", "member.invite", "member.update", "member.remove", "owner.manage", "billing.owner.update", "project.read", "project.create", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "billing.read", "budget.read", "budget.update", "audit.read", "webhook.read", "webhook.create", "webhook.update", "webhook.delete", "webhook.secret.rotate", "webhook.test", "webhook.delivery.read", "webhook.delivery.retry"},
		"admin":     {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "workspace.update", "member.read", "invitation.read", "member.invite", "member.update", "member.remove", "project.read", "project.create", "project.update", "project.archive", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "budget.read", "budget.update", "audit.read", "webhook.read", "webhook.create", "webhook.update", "webhook.delete", "webhook.secret.rotate", "webhook.test", "webhook.delivery.read", "webhook.delivery.retry"},
		"developer": {"service_account.read", "service_account.create", "service_account.update", "service_account.disable", "service_account.credential.read", "service_account.credential.create", "service_account.credential.update", "service_account.credential.revoke", "service_account.credential.rotate", "workspace.read", "member.read", "project.read", "key.read", "key.create", "key.update", "key.revoke", "usage.read", "budget.read", "webhook.read", "webhook.delivery.read"},
		"billing":   {"service_account.read", "workspace.read", "project.read", "usage.read", "billing.read", "budget.read", "budget.update", "webhook.read"},
		"viewer":    {"service_account.read", "workspace.read", "project.read", "usage.read", "budget.read"},
	}
	for role, expected := range roles {
		t.Run(role, func(t *testing.T) {
			require.ElementsMatch(t, expected, WorkspacePermissions(role))
			for _, p := range roles["owner"] {
				require.Equal(t, containsPermission(expected, p), HasWorkspacePermission(role, p), p)
			}
			require.False(t, HasWorkspacePermission(role, "unknown"))
		})
	}
	require.Empty(t, WorkspacePermissions("global-admin"))
	require.False(t, HasWorkspacePermission("", "workspace.read"))
}
func containsPermission(v []string, p string) bool {
	for _, s := range v {
		if s == p {
			return true
		}
	}
	return false
}

func TestWorkspaceValidation(t *testing.T) {
	for _, name := range []string{"", " ", strings.Repeat("a", 101)} {
		require.Error(t, ValidateWorkspaceNameSlug(name, "valid"))
	}
	for _, slug := range []string{"", "Bad", "-bad", "has space", strings.Repeat("x", 64)} {
		require.Error(t, ValidateWorkspaceNameSlug("Valid", slug))
	}
	require.NoError(t, ValidateWorkspaceNameSlug("工作空间", "acme-1"))
	for _, role := range []string{"owner", "admin", "developer", "billing", "viewer"} {
		require.True(t, ValidWorkspaceRole(role))
	}
	require.False(t, ValidWorkspaceRole("superadmin"))
}

func TestAPIKeyBillingPrincipalFallback(t *testing.T) {
	actor := &User{ID: 11}
	payer := &User{ID: 22}
	k := &APIKey{UserID: 11, User: actor}
	require.Same(t, actor, k.BillingUser())
	require.EqualValues(t, 11, k.BillingUserID())
	k.BillingPrincipal = payer
	require.Same(t, payer, k.BillingUser())
	require.EqualValues(t, 22, k.BillingUserID())
	require.Same(t, actor, k.User)
	require.EqualValues(t, 11, (&APIKey{UserID: 11}).BillingUserID())
}
