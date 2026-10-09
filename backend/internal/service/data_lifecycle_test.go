package service

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLifecycleArtifactTenantBindingAndAuthentication(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := bytes.Repeat([]byte("financial evidence\n"), 10000)
	var sealed bytes.Buffer
	require.NoError(t, EncryptLifecycleArtifact(&sealed, bytes.NewReader(plain), key, "42/job/attempt"))
	require.NotContains(t, sealed.String(), "financial evidence")
	var decoded bytes.Buffer
	require.NoError(t, DecryptLifecycleArtifact(&decoded, bytes.NewReader(sealed.Bytes()), key, "42/job/attempt"))
	require.Equal(t, plain, decoded.Bytes())
	for _, scope := range []string{"43/job/attempt", "42/other/attempt"} {
		require.Error(t, DecryptLifecycleArtifact(&bytes.Buffer{}, bytes.NewReader(sealed.Bytes()), key, scope))
	}
	corrupt := append([]byte{}, sealed.Bytes()...)
	corrupt[len(corrupt)-4] ^= 1
	require.Error(t, DecryptLifecycleArtifact(&bytes.Buffer{}, bytes.NewReader(corrupt), key, "42/job/attempt"))
	require.Error(t, DecryptLifecycleArtifact(&bytes.Buffer{}, bytes.NewReader(sealed.Bytes()[:sealed.Len()-20]), key, "42/job/attempt"))
	require.Error(t, DecryptLifecycleArtifact(&bytes.Buffer{}, bytes.NewReader(append(sealed.Bytes(), 1)), key, "42/job/attempt"))
}

func TestLifecycleAuthenticationFailureReleasesNoPlaintext(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 32)
	plain := bytes.Repeat([]byte("retained financial evidence\n"), 10000)
	var sealed bytes.Buffer
	require.NoError(t, EncryptLifecycleArtifact(&sealed, bytes.NewReader(plain), key, "42/job/attempt"))
	corrupt := append([]byte{}, sealed.Bytes()...)
	corrupt[len(corrupt)-1] ^= 1 // Mandatory terminal authentication fails after valid data chunks.
	var output bytes.Buffer
	require.Error(t, DecryptLifecycleArtifact(&output, bytes.NewReader(corrupt), key, "42/job/attempt"))
	require.Empty(t, output.Bytes(), "whole artifact authentication must precede releasing plaintext")
}

func TestLifecycleObjectKeyRejectsForeignScopeAndTraversal(t *testing.T) {
	job := "d8df15e1-f98b-4781-98b4-2b716edc4fbf"
	attempt := "86b2f358-214a-4095-9d0c-ce4063f23410"
	key, err := LifecycleObjectKey(42, job, attempt)
	require.NoError(t, err)
	require.True(t, ValidLifecycleObjectKey(key, 42, job))
	require.False(t, ValidLifecycleObjectKey(key, 43, job))
	require.False(t, ValidLifecycleObjectKey(key+"/../other", 42, job))
	_, err = LifecycleObjectKey(42, "../foreign", attempt)
	require.Error(t, err)
}

func TestLifecycleArchivedOwnerPermissions(t *testing.T) {
	ac := &WorkspaceAccess{Workspace: &Workspace{Type: "organization", Status: "archived"}, Member: &WorkspaceMember{Role: "owner", Status: "active"}}
	require.NoError(t, CheckWorkspacePermission(ac, "workspace.restore"))
	require.NoError(t, CheckWorkspacePermission(ac, "export.create"))
	require.Error(t, CheckWorkspacePermission(ac, "workspace.update"))
	ac.Member.Role = "admin"
	require.Error(t, CheckWorkspacePermission(ac, "workspace.restore"))
	require.Error(t, CheckWorkspacePermission(ac, "export.create"))
	ac.Member.Role = "owner"
	ac.Workspace.Status = "purging"
	require.Error(t, CheckWorkspacePermission(ac, "workspace.restore"))
}

func TestLifecycleStrongAuthenticationNeverUsesEnrollmentAsProof(t *testing.T) {
	now := time.Now().UTC()
	base := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now, MFAEnrolled: true})
	require.Error(t, RequireLifecycleStrongAuthentication(base, now))
	require.Error(t, RequireLifecycleStrongAuthentication(WithRecentAuthentication(base, now, false), now))
	require.NoError(t, RequireLifecycleStrongAuthentication(WithRecentAuthentication(base, now, true), now))
	stale := WithSessionAuthentication(context.Background(), SessionAuthentication{AuthMethod: "password", AuthenticatedAt: now.Add(-time.Hour)})
	require.Error(t, RequireLifecycleStrongAuthentication(stale, now))
}
