//go:build integration

package service

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestPersistSMSMessagePostgres(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("docker is required for integration tests: %v", err)
		}
		t.Skip("docker is unavailable")
	}

	container, err := tcpostgres.Run(
		ctx,
		"postgres:18.1-alpine3.23",
		tcpostgres.WithDatabase("sms_persistence_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(context.Background()) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))

	_, err = db.ExecContext(ctx, `
		CREATE TABLE sms_messages (
			id BIGSERIAL PRIMARY KEY,
			order_id BIGINT NOT NULL,
			message_text TEXT NOT NULL DEFAULT '',
			verification_code TEXT NOT NULL DEFAULT '',
			received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
			sender TEXT NOT NULL DEFAULT '',
			provider_received_at TIMESTAMPTZ,
			message_type TEXT NOT NULL DEFAULT '',
			service_code TEXT NOT NULL DEFAULT '',
			other_sms BOOLEAN NOT NULL DEFAULT FALSE,
			dedupe_hash TEXT NOT NULL,
			UNIQUE(order_id, dedupe_hash)
		)`)
	require.NoError(t, err)

	receivedAt := time.Date(2026, 9, 27, 2, 29, 23, 0, time.UTC)
	metadata := map[string]any{
		"messages": []map[string]any{{
			"verification_code":    "823937",
			"sender":               "5sim",
			"provider_received_at": receivedAt,
			"message_type":         "sms",
			"service_code":         "openai",
			"other_sms":            true,
		}},
	}
	svc := &SMSService{db: db}
	for range 2 {
		require.NoError(t, svc.persistSMSMessage(ctx, 54, "Your code is 823937", metadata, 0))
	}

	var orderID int64
	var code, sender, messageType, serviceCode, dedupeHash string
	var storedReceivedAt time.Time
	var otherSMS bool
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT order_id,verification_code,sender,provider_received_at,message_type,service_code,other_sms,dedupe_hash
		FROM sms_messages`).Scan(&orderID, &code, &sender, &storedReceivedAt, &messageType, &serviceCode, &otherSMS, &dedupeHash))
	require.Equal(t, int64(54), orderID)
	require.Equal(t, "823937", code)
	require.Equal(t, "5sim", sender)
	require.WithinDuration(t, receivedAt, storedReceivedAt, time.Microsecond)
	require.Equal(t, "sms", messageType)
	require.Equal(t, "openai", serviceCode)
	require.True(t, otherSMS)
	require.NotEmpty(t, dedupeHash)

	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sms_messages`).Scan(&count))
	require.Equal(t, 1, count)
}
