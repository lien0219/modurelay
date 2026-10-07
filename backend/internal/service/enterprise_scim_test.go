package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type scimTokenTestRepository struct {
	SCIMRepository
	hash   []byte
	prefix string
	calls  int
}

func (r *scimTokenTestRepository) CreateToken(_ context.Context, _, _, connector int64, prefix string, hash []byte, expiry *time.Time) (*SCIMToken, error) {
	r.calls++
	r.hash = append([]byte(nil), hash...)
	r.prefix = prefix
	return &SCIMToken{ID: 1, ConnectorID: connector, Prefix: prefix, Status: "active", ExpiresAt: expiry}, nil
}
func TestSCIMTokenIsShownOnceAndRepositoryOnlyReceivesHash(t *testing.T) {
	r := &scimTokenTestRepository{}
	s := NewEnterpriseSCIMService(r)
	result, err := s.CreateToken(context.Background(), 7, 42, 9, SCIMTokenInput{})
	require.NoError(t, err)
	require.Len(t, result.Secret, 52)
	require.Equal(t, HashEnterpriseToken(result.Secret), r.hash)
	require.NotEqual(t, result.Secret, r.prefix)
	raw, err := json.Marshal(result.Token)
	require.NoError(t, err)
	require.NotContains(t, string(raw), result.Secret)
	expired := time.Now().Add(-time.Hour)
	_, err = s.CreateToken(context.Background(), 7, 42, 9, SCIMTokenInput{ExpiresAt: &expired})
	require.Error(t, err)
	require.Equal(t, 1, r.calls)
}
func TestSCIMErrorHasTypedSafeProtocolDetails(t *testing.T) {
	var typed *SCIMError
	require.True(t, errors.As(NewSCIMError(409, "uniqueness", "resource already exists"), &typed))
	require.Equal(t, 409, typed.Status)
	require.Equal(t, "uniqueness", typed.Type)
}

func TestSCIMOpaqueResourceIDAlphabetAndLength(t *testing.T) {
	for _, char := range "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789_-" {
		require.True(t, ValidSCIMResourceID(strings.Repeat(string(char), 43)))
	}
	for _, id := range []string{strings.Repeat("a", 42), strings.Repeat("a", 44), strings.Repeat("a", 42) + "/", strings.Repeat("a", 42) + "+", strings.Repeat("a", 42) + "=", strings.Repeat("a", 41) + "?"} {
		require.False(t, ValidSCIMResourceID(id))
	}
}
