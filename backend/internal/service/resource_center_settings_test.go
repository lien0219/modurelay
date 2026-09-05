package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type resourceCenterSettingRepoStub struct {
	setMultipleErr error
	updates        map[string]string
}

func (s *resourceCenterSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *resourceCenterSettingRepoStub) GetValue(context.Context, string) (string, error) {
	panic("unexpected GetValue call")
}

func (s *resourceCenterSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (s *resourceCenterSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (s *resourceCenterSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	s.updates = settings
	return s.setMultipleErr
}

func (s *resourceCenterSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *resourceCenterSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestResourceCenterServiceUpdateConfigNotifiesAfterPersistence(t *testing.T) {
	repo := &resourceCenterSettingRepoStub{}
	service := &ResourceCenterService{settingRepo: repo}
	notifications := 0
	service.SetOnUpdateCallback(func() { notifications++ })

	require.NoError(t, service.UpdateConfig(context.Background(), ResourceCenterConfig{Enabled: true, ForbidURLs: true, BannedWords: []string{"spam"}}))
	require.Equal(t, "true", repo.updates[SettingKeyResourceCenterEnabled])
	require.Equal(t, 1, notifications)
}

func TestResourceCenterServiceUpdateConfigDoesNotNotifyWhenPersistenceFails(t *testing.T) {
	repo := &resourceCenterSettingRepoStub{setMultipleErr: errors.New("settings unavailable")}
	service := &ResourceCenterService{settingRepo: repo}
	notifications := 0
	service.SetOnUpdateCallback(func() { notifications++ })

	require.Error(t, service.UpdateConfig(context.Background(), ResourceCenterConfig{Enabled: false}))
	require.Equal(t, 0, notifications)
}
