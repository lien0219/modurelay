package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDatabaseReadReplicaDefaults(t *testing.T) {
	resetViperWithJWTSecret(t)
	cfg, err := Load()
	require.NoError(t, err)
	require.False(t, cfg.Database.ReadReplica.Enabled)
	require.Equal(t, 64, cfg.Database.ReadReplica.MaxOpenConns)
	require.Equal(t, 16, cfg.Database.ReadReplica.MaxIdleConns)
}

func TestLoadDatabaseReadReplicaEnvironment(t *testing.T) {
	resetViperWithJWTSecret(t)
	t.Setenv("DATABASE_READ_REPLICA_ENABLED", "true")
	t.Setenv("DATABASE_READ_REPLICA_HOST", "postgres-replicas")
	t.Setenv("DATABASE_READ_REPLICA_PORT", "5433")
	t.Setenv("DATABASE_READ_REPLICA_USER", "reporting")
	t.Setenv("DATABASE_READ_REPLICA_PASSWORD", "reader-secret")
	t.Setenv("DATABASE_READ_REPLICA_DBNAME", "reporting-db")
	t.Setenv("DATABASE_READ_REPLICA_SSLMODE", "require")
	t.Setenv("DATABASE_READ_REPLICA_MAX_OPEN_CONNS", "32")
	t.Setenv("DATABASE_READ_REPLICA_MAX_IDLE_CONNS", "8")
	t.Setenv("DATABASE_READ_REPLICA_CONNECT_TIMEOUT_SECONDS", "2")
	cfg, err := Load()
	require.NoError(t, err)
	require.True(t, cfg.Database.ReadReplica.Enabled)
	replica := cfg.Database.ReadReplicaDatabase()
	require.Equal(t, "postgres-replicas", replica.Host)
	require.Equal(t, 5433, replica.Port)
	require.Equal(t, "reporting", replica.User)
	require.Equal(t, "reader-secret", replica.Password)
	require.Equal(t, "reporting-db", replica.DBName)
	require.Equal(t, "require", replica.SSLMode)
	require.Equal(t, 32, replica.MaxOpenConns)
	require.Equal(t, 8, replica.MaxIdleConns)
	require.Contains(t, cfg.Database.ReadReplicaDSNWithTimezone("UTC"), "default_transaction_read_only=on connect_timeout=2")
	require.NotEqual(t, replica.Host, cfg.Database.Host)
}

func TestDatabaseReadReplicaInheritsConnectionSettings(t *testing.T) {
	primary := DatabaseConfig{
		Host: "primary", Port: 5432, User: "app", Password: "secret", DBName: "app", SSLMode: "require",
		MaxOpenConns: 256, MaxIdleConns: 128, ConnMaxLifetimeMinutes: 30, ConnMaxIdleTimeMinutes: 5,
		ReadReplica: DatabaseReadReplicaConfig{Enabled: true, Host: "replica", MaxOpenConns: 64, MaxIdleConns: 16},
	}
	replica := primary.ReadReplicaDatabase()
	require.Equal(t, primary.Port, replica.Port)
	require.Equal(t, primary.User, replica.User)
	require.Equal(t, primary.Password, replica.Password)
	require.Equal(t, primary.DBName, replica.DBName)
	require.Equal(t, primary.SSLMode, replica.SSLMode)
	require.Equal(t, primary.ConnMaxLifetimeMinutes, replica.ConnMaxLifetimeMinutes)
	require.Equal(t, "primary", primary.Host)
}

func TestLoadDatabaseReadReplicaValidation(t *testing.T) {
	for _, tc := range []struct{ name, key, value string }{
		{"missing host", "DATABASE_READ_REPLICA_HOST", ""},
		{"invalid port", "DATABASE_READ_REPLICA_PORT", "65536"},
		{"no connections", "DATABASE_READ_REPLICA_MAX_OPEN_CONNS", "0"},
		{"too many idle connections", "DATABASE_READ_REPLICA_MAX_IDLE_CONNS", "1000"},
		{"no timeout", "DATABASE_READ_REPLICA_CONNECT_TIMEOUT_SECONDS", "0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetViperWithJWTSecret(t)
			t.Setenv("DATABASE_READ_REPLICA_ENABLED", "true")
			t.Setenv("DATABASE_READ_REPLICA_HOST", "replica")
			t.Setenv(tc.key, tc.value)
			_, err := Load()
			require.ErrorContains(t, err, "database.read_replica")
		})
	}
}
