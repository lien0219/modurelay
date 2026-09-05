package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

type downstreamBillingProbeSettingRepo struct {
	SettingRepository
	values map[string]string
	getErr error
	setErr error
	gets   int
}

func (r *downstreamBillingProbeSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.gets++
	if r.getErr != nil {
		return "", r.getErr
	}
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *downstreamBillingProbeSettingRepo) Set(_ context.Context, key, value string) error {
	if r.setErr != nil {
		return r.setErr
	}
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[key] = value
	return nil
}

func newDownstreamBillingProbeSettingService(repo SettingRepository) *SettingService {
	return NewSettingService(repo, &config.Config{})
}

func TestDownstreamBillingProbeSettingsDefaultAndUpdate(t *testing.T) {
	repo := &downstreamBillingProbeSettingRepo{}
	svc := newDownstreamBillingProbeSettingService(repo)

	settings, err := svc.GetDownstreamBillingProbeSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.Enabled)

	require.NoError(t, svc.SetDownstreamBillingProbeSettings(
		context.Background(),
		&DownstreamBillingProbeSettings{Enabled: false},
	))
	require.Equal(t, "false", repo.values[SettingKeyDownstreamBillingProbeEnabled])
	require.False(t, svc.IsDownstreamBillingProbeEnabled(context.Background()))
}

func TestDownstreamBillingProbeRuntimeReadIsCached(t *testing.T) {
	repo := &downstreamBillingProbeSettingRepo{values: map[string]string{
		SettingKeyDownstreamBillingProbeEnabled: "true",
	}}
	svc := newDownstreamBillingProbeSettingService(repo)

	for range 3 {
		require.True(t, svc.IsDownstreamBillingProbeEnabled(context.Background()))
	}
	require.Equal(t, 1, repo.gets)
}

func TestDownstreamBillingProbeRuntimeReadFailsClosed(t *testing.T) {
	repo := &downstreamBillingProbeSettingRepo{getErr: errors.New("database unavailable")}
	svc := newDownstreamBillingProbeSettingService(repo)

	require.False(t, svc.IsDownstreamBillingProbeEnabled(context.Background()))
	require.Equal(t, 1, repo.gets)
}

func TestDownstreamBillingProbeSettingsRejectInvalidStoredValue(t *testing.T) {
	repo := &downstreamBillingProbeSettingRepo{values: map[string]string{
		SettingKeyDownstreamBillingProbeEnabled: "enabled",
	}}
	svc := newDownstreamBillingProbeSettingService(repo)

	settings, err := svc.GetDownstreamBillingProbeSettings(context.Background())
	require.ErrorContains(t, err, "invalid downstream billing probe setting")
	require.Nil(t, settings)
}
