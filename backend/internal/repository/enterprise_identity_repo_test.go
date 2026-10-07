package repository

import (
	"context"
	"database/sql"
	"encoding/hex"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type enterpriseTestEncryptor struct{}

func (enterpriseTestEncryptor) Encrypt(value string) (string, error) { return "cipher:" + value, nil }
func (enterpriseTestEncryptor) Decrypt(value string) (string, error) {
	return value[len("cipher:"):], nil
}

func TestEnterpriseIdentityNullableEmailAtLink(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery("FROM workspace_user_identities").WithArgs(int64(8), int64(9), "stable-subject").WillReturnRows(
		sqlmock.NewRows([]string{"id", "workspace_id", "provider_id", "user_id", "subject", "email_at_link", "email_verified", "last_seen_at"}).
			AddRow(10, 8, 9, 11, "stable-subject", nil, false, time.Now().UTC()))
	item, err := (&enterpriseIdentityRepository{db: db}).FindIdentity(context.Background(), 8, 9, "stable-subject")
	require.NoError(t, err)
	require.EqualValues(t, 11, item.UserID)
	require.Empty(t, item.EmailAtLink)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseOIDCStateReturnsNonceAndRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	hash := make([]byte, 32)
	now := time.Now().UTC()
	mock.ExpectQuery("UPDATE workspace_identity_auth_states.*nonce_ciphertext").WithArgs(hash, now, make([]byte, 32)).WillReturnRows(
		sqlmock.NewRows([]string{"workspace_id", "provider_id", "provider_revision", "browser_session_hash", "nonce_hash", "pkce_verifier_ciphertext", "nonce_ciphertext", "return_to", "intent", "link_user_id", "expires_at", "created_at"}).
			AddRow(8, 9, 3, make([]byte, 32), make([]byte, 32), "cipher:pkce", "cipher:nonce", "/workspace", "login", nil, now.Add(time.Minute), now))
	item, err := (&enterpriseIdentityRepository{db: db, encryptor: enterpriseTestEncryptor{}}).ConsumeOIDCState(context.Background(), hex.EncodeToString(hash), make([]byte, 32), now)
	require.NoError(t, err)
	require.Equal(t, "pkce", item.PKCEVerifier)
	require.Equal(t, "nonce", item.Nonce)
	require.EqualValues(t, 3, item.ProviderRevision)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterprisePolicyReadDoesNotWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery("SELECT.*FROM workspaces.*workspace_security_policies").WithArgs(int64(8)).WillReturnRows(
		sqlmock.NewRows([]string{"workspace_id", "require_sso", "sso_grace_until", "revision", "updated_by_user_id", "updated_at"}).
			AddRow(8, false, nil, 1, nil, time.Now().UTC()))
	policy, err := (&enterpriseIdentityRepository{db: db}).GetPolicy(context.Background(), 8, 0)
	require.NoError(t, err)
	require.False(t, policy.RequireSSO)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEnterpriseUnknownIdentityIsNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery("FROM workspace_user_identities").WillReturnError(sql.ErrNoRows)
	_, err = (&enterpriseIdentityRepository{db: db}).FindIdentity(context.Background(), 8, 9, "missing")
	require.ErrorIs(t, err, service.ErrWorkspaceNotFound)
	_ = mock.ExpectationsWereMet()
}
