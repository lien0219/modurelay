package repository

import (
	"database/sql/driver"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMachineUsageInsertStoresNullHumanActorAndFrozenMachine(t *testing.T) {
	id := int64(31)
	p := prepareUsageLogInsert(&service.UsageLog{APIKeyID: 9, AccountID: 8, ServiceAccountID: &id, Model: "model", RequestID: "machine-usage"})
	require.Nil(t, p.args[0], "machines must not write a fabricated human foreign key")
	machineID, err := driver.DefaultParameterConverter.ConvertValue(p.args[len(p.args)-1])
	require.NoError(t, err)
	require.Equal(t, int64(31), machineID)
	legacy := prepareUsageLogInsert(&service.UsageLog{UserID: 22, APIKeyID: 9, Model: "model"})
	require.Equal(t, int64(22), legacy.args[0])
	legacyMachineID, err := driver.DefaultParameterConverter.ConvertValue(legacy.args[len(legacy.args)-1])
	require.NoError(t, err)
	require.Nil(t, legacyMachineID, "legacy usage must keep a NULL machine snapshot")
}
