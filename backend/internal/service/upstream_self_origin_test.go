package service

import (
	"errors"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/stretchr/testify/require"
)

func TestUpstreamValidatorsRejectDeploymentOrigin(t *testing.T) {
	for _, allowlistEnabled := range []bool{false, true} {
		name := "allowlist_disabled"
		if allowlistEnabled {
			name = "allowlist_enabled"
		}
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Server.FrontendURL = "https://app.example.test/admin"
			cfg.Security.URLAllowlist.Enabled = allowlistEnabled
			cfg.Security.URLAllowlist.UpstreamHosts = []string{"app.example.test"}

			validators := []struct {
				name     string
				validate func(string) (string, error)
			}{
				{name: "anthropic_gateway", validate: (&GatewayService{cfg: cfg}).validateUpstreamBaseURL},
				{name: "openai_gateway", validate: (&OpenAIGatewayService{cfg: cfg}).validateUpstreamBaseURL},
				{name: "gemini_gateway", validate: (&GeminiMessagesCompatService{cfg: cfg}).validateUpstreamBaseURL},
				{name: "account_test", validate: (&AccountTestService{cfg: cfg}).validateUpstreamBaseURL},
				{name: "cn_probe", validate: func(raw string) (string, error) { return cnValidateProbeURL(cfg, raw) }},
			}

			for _, validator := range validators {
				t.Run(validator.name, func(t *testing.T) {
					_, err := validator.validate("https://token:secret@app.example.test:443/v1")
					require.ErrorIs(t, err, urlvalidator.ErrSameHTTPOrigin)
					require.NotContains(t, err.Error(), "token")
					require.NotContains(t, err.Error(), "secret")
				})
			}
		})
	}
}

func TestGrokUpstreamValidatorRejectsDeploymentOriginWithoutLeakingURL(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.FrontendURL = "https://app.example.test"
	cfg.Security.URLAllowlist.Enabled = false
	account := &Account{
		Platform: PlatformGrok,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"base_url": "https://token:secret@app.example.test/v1",
		},
	}

	_, err := buildGrokResponsesURL(account, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "base URL rejected by URL security policy")
	require.False(t, errors.Is(err, urlvalidator.ErrSameHTTPOrigin), "redacted validator must not expose the underlying policy error")
	require.False(t, strings.Contains(err.Error(), "token") || strings.Contains(err.Error(), "secret"))
}

func TestUpstreamValidatorsAllowDifferentOrigin(t *testing.T) {
	cfg := &config.Config{}
	cfg.Server.FrontendURL = "https://app.example.test"
	cfg.Security.URLAllowlist.Enabled = true
	cfg.Security.URLAllowlist.UpstreamHosts = []string{"upstream.example.test", "app.example.test"}

	normalized, err := (&OpenAIGatewayService{cfg: cfg}).validateUpstreamBaseURL("https://upstream.example.test/v1/")
	require.NoError(t, err)
	require.Equal(t, "https://upstream.example.test/v1", normalized)

	normalized, err = (&OpenAIGatewayService{cfg: cfg}).validateUpstreamBaseURL("https://app.example.test:8443/v1")
	require.NoError(t, err)
	require.Equal(t, "https://app.example.test:8443/v1", normalized)
}
