//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTOTPDisableFailsClosedWithoutDependencyRepository(t *testing.T) {
	user := &User{ID: 42, Role: RoleUser, TotpEnabled: true}
	require.NoError(t, user.SetPassword("correct-password"))
	svc, repo := newTotpVMService(t, user, false)
	err := svc.Disable(context.Background(), 42, "", "correct-password")
	require.ErrorIs(t, err, ErrServiceUnavailable)
	require.False(t, repo.disableCalled)
}
