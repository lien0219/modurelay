package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

type scimExpiryProbe struct {
	SCIMRepository
	batch int
	calls int
}

func (r *scimExpiryProbe) NotifyExpiringTokens(_ context.Context, batch int) error {
	r.batch = batch
	r.calls++
	return nil
}
func TestSCIMExpiryMonitorUsesBoundedBatchAndStops(t *testing.T) {
	r := &scimExpiryProbe{}
	m := NewSCIMTokenExpiryMonitor(r)
	require.NoError(t, m.runBatch(context.Background()))
	require.Equal(t, 100, r.batch)
	require.Equal(t, 1, r.calls)
	m.Start()
	m.Stop()
	m.Stop()
}
