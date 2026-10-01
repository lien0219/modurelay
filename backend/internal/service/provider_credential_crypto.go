package service

import apperrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"

// ErrProviderCredentialEncryptionKeyNotConfigured prevents an administrator
// from storing a credential with the process-local, auto-generated encryption
// key. Such a value would be unreadable after the next process restart.
var ErrProviderCredentialEncryptionKeyNotConfigured = apperrors.BadRequest(
	"PROVIDER_CREDENTIAL_ENCRYPTION_KEY_NOT_CONFIGURED",
	"set a fixed TOTP_ENCRYPTION_KEY before saving provider credentials",
)
