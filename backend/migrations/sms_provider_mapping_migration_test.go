package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSMSProviderMappingSeedUsesVerifiedIdentifiers(t *testing.T) {
	content, err := FS.ReadFile("247_sms_provider_mapping_seeds.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "INSERT INTO sms_provider_service_mappings")
	require.Contains(t, sql, "INSERT INTO sms_provider_country_mappings")
	require.Equal(t, 1, strings.Count(sql, "ON CONFLICT (provider_id, service_id) DO NOTHING"))
	require.Equal(t, 1, strings.Count(sql, "ON CONFLICT (provider_id, country_id) DO NOTHING"))

	for _, value := range []string{
		"('5sim', 'google', 'google', 'Google')",
		"('5sim', 'openai', 'openai', 'OpenAI')",
		"('5sim', 'telegram', 'telegram', 'Telegram')",
		"('5sim', 'discord', 'discord', 'Discord')",
		"('5sim', 'whatsapp', 'whatsapp', 'WhatsApp')",
		"('smspool', 'google', '395', 'Google/Gmail')",
		"('smspool', 'openai', '671', 'OpenAI/ChatGPT')",
		"('smspool', 'telegram', '907', 'Telegram')",
		"('smspool', 'discord', '273', 'Discord')",
		"('smspool', 'whatsapp', '1012', 'WhatsApp')",
		"('5sim', 'US', 'usa')",
		"('5sim', 'GB', 'england')",
		"('5sim', 'DE', 'germany')",
		"('smspool', 'US', '1')",
		"('smspool', 'GB', '2')",
		"('smspool', 'DE', '24')",
	} {
		require.Contains(t, sql, value)
	}

	for _, unsupported := range []string{"('5sim', 'CN'", "sms_activate", "onlinesim", "pingme"} {
		require.NotContains(t, sql, unsupported)
	}
}

func TestSMSProviderMappingSeedDoesNotEnableProvidersOrChannels(t *testing.T) {
	content, err := FS.ReadFile("240_sms_service_foundation.sql")
	require.NoError(t, err)
	foundationSQL := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, foundationSQL, "('5sim', '5SIM'")
	require.Contains(t, foundationSQL, "('smspool', 'SMSPool'")
	require.Contains(t, foundationSQL, "enabled BOOLEAN NOT NULL DEFAULT FALSE")

	seed, err := FS.ReadFile("247_sms_provider_mapping_seeds.sql")
	require.NoError(t, err)
	seedSQL := strings.Join(strings.Fields(string(seed)), " ")
	require.NotContains(t, seedSQL, "UPDATE sms_providers")
	require.NotContains(t, seedSQL, "UPDATE sms_channels")
	require.NotContains(t, seedSQL, "enabled = TRUE")
}
