//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

type workspaceProtectedUserRepo struct{ userRepoStub }

func (r *workspaceProtectedUserRepo) GuardWorkspaceUserDeletion(context.Context, int64) error {
	return ErrWorkspaceConflict
}
func TestAdminDeleteChecksWorkspaceBeforeKeyEnumeration(t *testing.T) {
	users := &workspaceProtectedUserRepo{userRepoStub: userRepoStub{user: &User{ID: 7, Role: RoleUser}}}
	keys := &apiKeyRepoStub{allowListByUserID: true}
	svc := &adminServiceImpl{userRepo: users, apiKeyRepo: keys}
	require.ErrorIs(t, svc.DeleteUser(context.Background(), 7), ErrWorkspaceConflict)
	require.Empty(t, keys.listByUserIDCalls)
	require.Empty(t, users.deletedIDs)
}
