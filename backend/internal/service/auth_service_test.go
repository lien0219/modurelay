package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

type missingLoginUserRepository struct{ UserRepository }

func (missingLoginUserRepository) GetByEmail(context.Context, string) (*User, error) {
	return nil, ErrUserNotFound
}

func TestDummyLoginPasswordHashIsValid(t *testing.T) {
	err := bcrypt.CompareHashAndPassword(dummyLoginPasswordHash, []byte("probe-password"))
	require.ErrorIs(t, err, bcrypt.ErrMismatchedHashAndPassword)
}

func TestUnknownAccountDummyPasswordComparisonIsClassifiedAsMismatch(t *testing.T) {
	_, _, err := (&AuthService{userRepo: missingLoginUserRepository{}}).Login(context.Background(), "missing@example.com", "probe-password")
	require.True(t, IsPasswordMismatch(err))
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestIsReservedEmail_DingTalkDomain(t *testing.T) {
	require.True(t, isReservedEmail("dingtalk-123@dingtalk-connect.invalid"))
	require.True(t, isReservedEmail("DINGTALK-456@DINGTALK-CONNECT.INVALID")) // case-insensitive
	require.False(t, isReservedEmail("real@dingtalk.com"))
}
