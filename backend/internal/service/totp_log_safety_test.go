//go:build unit

package service

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/require"
)

type totpLogSafetyCache struct{ TotpCache }

func (totpLogSafetyCache) GetSetupSession(context.Context, int64) (*TotpSetupSession, error) {
	return &TotpSetupSession{Secret: "JBSWY3DPEHPK3PXP", SetupToken: "private-setup-token"}, nil
}
func (totpLogSafetyCache) DeleteSetupSession(context.Context, int64) error { return nil }

type totpLogSafetyUsers struct{ UserRepository }

func (totpLogSafetyUsers) UpdateTotpSecret(context.Context, int64, *string) error { return nil }
func (totpLogSafetyUsers) EnableTotp(context.Context, int64) error                { return nil }

type totpLogSafetyEncryptor struct{}

func (totpLogSafetyEncryptor) Encrypt(value string) (string, error) { return "cipher:" + value, nil }
func (totpLogSafetyEncryptor) Decrypt(value string) (string, error) {
	return value[len("cipher:"):], nil
}

func TestTOTPSetupLogsDoNotDiscloseFactorMaterial(t *testing.T) {
	var output bytes.Buffer
	original := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(original) })
	settings := NewSettingService(&totpVMSettingRepoStub{values: map[string]string{SettingKeyTotpEnabled: "true"}}, nil)
	svc := NewTotpService(totpLogSafetyUsers{}, totpLogSafetyEncryptor{}, totpLogSafetyCache{}, settings, nil, nil)
	code, err := totp.GenerateCode("JBSWY3DPEHPK3PXP", time.Now())
	require.NoError(t, err)
	require.NoError(t, svc.CompleteSetup(context.Background(), 42, code, "private-setup-token"))
	require.NotContains(t, output.String(), "JBSW")
	require.NotContains(t, output.String(), "private-setup-token")
	require.NotContains(t, output.String(), code)
}
