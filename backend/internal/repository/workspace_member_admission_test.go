package repository

import (
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceMemberAdmissionSCIMErrorContract(t *testing.T) {
	for _, denial := range []error{service.ErrExternalMemberNotAllowed, service.ErrWorkspaceJITDisabled, service.ErrIdentityProviderNotApproved, service.ErrOIDCProviderDisabled, service.ErrOIDCAccountLinkRequired, service.ErrOIDCStateSessionMismatch} {
		t.Run(denial.Error(), func(t *testing.T) {
			var typed *service.SCIMError
			require.True(t, errors.As(scimAdmissionError(denial), &typed))
			require.Equal(t, 409, typed.Status)
			require.Equal(t, "invalidValue", typed.Type)
		})
	}
	var unavailable *service.SCIMError
	require.True(t, errors.As(scimAdmissionError(service.ErrSecurityPolicyUnavailable), &unavailable))
	require.Equal(t, 503, unavailable.Status)
}
