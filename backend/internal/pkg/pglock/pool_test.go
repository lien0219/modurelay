package pglock

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPoolRegistrationOwnsOnlyDedicatedPool(t *testing.T) {
	primary, pm, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = primary.Close() }()
	locks, lm, err := sqlmock.New()
	require.NoError(t, err)
	require.Same(t, primary, PoolFor(primary))
	cleanup, err := Register(primary, locks)
	require.NoError(t, err)
	require.Same(t, locks, PoolFor(primary))
	_, err = Register(primary, locks)
	require.ErrorContains(t, err, "already registered")
	lm.ExpectClose()
	require.NoError(t, cleanup())
	require.NoError(t, cleanup())
	require.Same(t, primary, PoolFor(primary))
	require.NoError(t, pm.ExpectationsWereMet())
	require.NoError(t, lm.ExpectationsWereMet())
}

func TestPoolRegistrationRejectsBusinessPoolReuse(t *testing.T) {
	primary, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = primary.Close() }()
	_, err = Register(primary, primary)
	require.Error(t, err)
	_, err = Register(nil, primary)
	require.Error(t, err)
	require.Nil(t, PoolFor(nil))
}
