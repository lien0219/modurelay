package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageTaskMemoryStore struct {
	task    *ImageTaskRecord
	ttl     time.Duration
	saveErr error
	getErr  error
}

func (s *imageTaskMemoryStore) Save(_ context.Context, task *ImageTaskRecord, ttl time.Duration) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	copy := *task
	s.task = &copy
	s.ttl = ttl
	return nil
}

func (s *imageTaskMemoryStore) Get(_ context.Context, _ string) (*ImageTaskRecord, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	if s.task == nil {
		return nil, ErrImageTaskNotFound
	}
	copy := *s.task
	return &copy, nil
}

func TestImageTaskServiceLifecycleAndOwnership(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, 10*time.Minute)
	owner := ImageTaskOwner{UserID: 7, APIKeyID: 9}

	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusProcessing, created.Status)
	require.Equal(t, created.ID, created.TaskID)
	require.Equal(t, "image.generation.task", created.Object)
	require.Equal(t, time.Hour, store.ttl)
	require.Equal(t, owner.UserID, store.task.UserID)
	require.Equal(t, owner.APIKeyID, store.task.APIKeyID)

	_, err = svc.Get(context.Background(), ImageTaskOwner{UserID: 7, APIKeyID: 10}, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)

	result := json.RawMessage(`{"created":123,"data":[{"url":"https://example.test/image.png"}]}`)
	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, result))

	completed, err := svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusCompleted, completed.Status)
	require.Equal(t, http.StatusOK, completed.HTTPStatus)
	require.Equal(t, "https://example.test/image.png", completed.ImageURL)
	require.JSONEq(t, string(result), string(completed.Result))
	require.NotNil(t, completed.CompletedAt)
}

func TestImageTaskPersistsPrivateTenantSnapshot(t *testing.T) {
	var owner ImageTaskOwner
	require.NoError(t, json.Unmarshal([]byte(`{"UserID":7,"APIKeyID":9,"workspace_id":1,"project_id":2,"billing_principal_user_id":3,"budget_reservation_id":"private-reservation","policy_quota_reservation_id":"policy-reservation","policy_quota_estimated_tokens":17}`), &owner))
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	public, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	privateJSON, err := json.Marshal(store.task)
	require.NoError(t, err)
	require.Contains(t, string(privateJSON), `"workspace_id":1`)
	require.Contains(t, string(privateJSON), `"project_id":2`)
	require.Contains(t, string(privateJSON), `"billing_principal_user_id":3`)
	require.Contains(t, string(privateJSON), `"budget_reservation_id":"private-reservation"`)
	require.Contains(t, string(privateJSON), `"policy_quota_reservation_id":"policy-reservation"`)
	require.Contains(t, string(privateJSON), `"policy_quota_estimated_tokens":17`)
	publicJSON, err := json.Marshal(public)
	require.NoError(t, err)
	require.NotContains(t, string(publicJSON), "private-reservation")
	require.NotContains(t, string(publicJSON), "billing_principal")
}

func TestImageTaskRejectsIncompleteTenantSnapshot(t *testing.T) {
	var owner ImageTaskOwner
	require.NoError(t, json.Unmarshal([]byte(`{"UserID":7,"APIKeyID":9,"workspace_id":1}`), &owner))
	store := &imageTaskMemoryStore{}
	_, err := NewImageTaskService(store).Create(context.Background(), owner)
	require.Error(t, err)
	require.Nil(t, store.task)
}

func TestImageTaskMachineIdentitySnapshotAndOwnership(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskService(store)
	owner := ImageTaskOwner{ServiceAccountID: 31, APIKeyID: 9, WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, BudgetReservationID: "machine-reservation"}

	created, err := svc.Create(context.Background(), owner)
	require.NoError(t, err)
	require.Zero(t, store.task.UserID)
	require.Equal(t, int64(31), store.task.ServiceAccountID)
	require.Equal(t, owner.BillingPrincipalUserID, store.task.BillingPrincipalUserID)

	_, err = svc.Get(context.Background(), owner, created.ID)
	require.NoError(t, err)
	otherCredential := owner
	otherCredential.APIKeyID++
	_, err = svc.Get(context.Background(), otherCredential, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
	otherServiceAccount := owner
	otherServiceAccount.ServiceAccountID++
	_, err = svc.Get(context.Background(), otherServiceAccount, created.ID)
	require.ErrorIs(t, err, ErrImageTaskNotFound)
}

func TestImageTaskRejectsMixedHumanAndMachineOwner(t *testing.T) {
	store := &imageTaskMemoryStore{}
	_, err := NewImageTaskService(store).Create(context.Background(), ImageTaskOwner{
		UserID: 7, ServiceAccountID: 31, APIKeyID: 9,
		WorkspaceID: 1, ProjectID: 2, BillingPrincipalUserID: 3, BudgetReservationID: "reservation",
	})
	require.Error(t, err)
	require.Nil(t, store.task)
}

func TestImageTaskServiceInvalidResultBecomesFailed(t *testing.T) {
	store := &imageTaskMemoryStore{}
	svc := NewImageTaskServiceWithOptions(store, time.Hour, time.Minute)
	created, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.NoError(t, err)

	require.NoError(t, svc.Complete(context.Background(), created.ID, http.StatusOK, json.RawMessage(`not-json`)))
	got, err := svc.Get(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2}, created.ID)
	require.NoError(t, err)
	require.Equal(t, ImageTaskStatusFailed, got.Status)
	require.Equal(t, http.StatusBadGateway, got.HTTPStatus)
	require.Contains(t, string(got.Error), "non-JSON")
}

func TestImageTaskServiceMapsStoreFailures(t *testing.T) {
	store := &imageTaskMemoryStore{saveErr: errors.New("redis down")}
	svc := NewImageTaskService(store)

	_, err := svc.Create(context.Background(), ImageTaskOwner{UserID: 1, APIKeyID: 2})
	require.ErrorIs(t, err, ErrImageTaskUnavailable)
}
