//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsMonitoringKubernetesEventsPaginationUIDAndRedaction(t *testing.T) {
	pages := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pages++
		require.Equal(t, "Bearer test-kube-token", r.Header.Get("Authorization"))
		require.Empty(t, r.URL.Query().Get("fieldSelector"))
		require.Equal(t, "500", r.URL.Query().Get("limit"))
		if pages == 1 {
			fmt.Fprint(w, `{"metadata":{"continue":"next/page token"},"items":[{"metadata":{"uid":"event-old","namespace":"ns","creationTimestamp":"2026-10-21T00:00:00Z"},"involvedObject":{"kind":"Pod","name":"api-0","uid":"old-uid"},"type":"Warning","reason":"BackOff","message":"api_key=secret-value password=hunter2 Authorization=Bearer sensitive-bearer https://user:url-password@registry CrashLoopBackOff","firstTimestamp":"2026-10-21T00:00:00Z","lastTimestamp":"2026-10-21T00:01:00Z","count":3,"source":{"component":"kubelet"}},{"metadata":{"uid":"ignore-no-uid"},"involvedObject":{"kind":"Pod"}}]}`)
			return
		}
		require.Equal(t, "next/page token", r.URL.Query().Get("continue"))
		fmt.Fprint(w, `{"metadata":{},"items":[{"metadata":{"uid":"event-new","namespace":"ns","creationTimestamp":"2026-10-21T00:02:00Z"},"involvedObject":{"kind":"Pod","name":"api-0","uid":"new-uid"},"type":"Normal","reason":"Started","message":"Started container","eventTime":"2026-10-21T00:02:00Z","series":{"count":5,"lastObservedTime":"2026-10-21T00:03:00Z"},"reportingComponent":"kubelet"}]}`)
	}))
	defer server.Close()
	events, err := fetchKubernetesMonitoringEvents(context.Background(), server.Client(), server.URL, "test-kube-token", "cluster")
	require.NoError(t, err)
	require.Equal(t, 2, pages)
	require.Len(t, events, 2)
	require.Equal(t, "old-uid", events[0].PodUID)
	require.Equal(t, "new-uid", events[1].PodUID)
	require.Equal(t, int64(5), events[1].Count)
	require.NotContains(t, events[0].Message, "secret-value")
	require.NotContains(t, events[0].Message, "hunter2")
	require.NotContains(t, events[0].Message, "sensitive-bearer")
	require.NotContains(t, events[0].Message, "url-password")
	require.Contains(t, events[0].Message, "CrashLoopBackOff")
	require.Equal(t, "2026-10-21T00:03:00Z", events[1].LastSeenAt.Format(time.RFC3339))
}

func TestOpsMonitoringKubernetesEventsPermissionError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "private bearer token details")
	}))
	defer server.Close()
	events, err := fetchKubernetesMonitoringEvents(context.Background(), server.Client(), server.URL, "secret", "cluster")
	require.Error(t, err)
	require.Nil(t, events)
	require.NotContains(t, err.Error(), "private")
}

type monitoringEventsTestRepo struct {
	OpsRepository
	events     []OpsKubernetesEvent
	heartbeats []*OpsJobHeartbeat
	filter     OpsMonitoringEventsFilter
}

func (r *monitoringEventsTestRepo) UpsertMonitoringEvents(context.Context, []OpsKubernetesEvent) error {
	return nil
}
func (r *monitoringEventsTestRepo) DeleteExpiredMonitoringEvents(context.Context, string, time.Time) error {
	return nil
}
func (r *monitoringEventsTestRepo) ListMonitoringEvents(_ context.Context, f OpsMonitoringEventsFilter) ([]OpsKubernetesEvent, error) {
	r.filter = f
	return r.events, nil
}
func (r *monitoringEventsTestRepo) ListJobHeartbeats(context.Context) ([]*OpsJobHeartbeat, error) {
	return r.heartbeats, nil
}

func TestOpsMonitoringEventsArchivedDataAndCollectorState(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-3 * time.Minute)
	repo := &monitoringEventsTestRepo{events: []OpsKubernetesEvent{{EventUID: "e1", PodUID: "deleted-pod-uid"}}}
	s := &OpsService{cfg: &config.Config{Ops: config.OpsConfig{Enabled: true, Monitoring: config.OpsMonitoringConfig{Mode: "kubernetes", ClusterName: "cluster", KubernetesEventsEnabled: true}}}, opsRepo: repo}
	cfg := s.monitoringConfig()
	repo.heartbeats = []*OpsJobHeartbeat{{JobName: monitoringEventsJobName(cfg), LastSuccessAt: &old}}
	result, err := s.ListMonitoringEvents(context.Background(), OpsMonitoringEventsFilter{UID: "deleted-pod-uid"})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringUnknown, result.State)
	require.Len(t, result.Events, 1)
	require.Equal(t, "deleted-pod-uid", repo.filter.UID)
	require.Equal(t, "cluster", repo.filter.Cluster)
	repo.heartbeats[0].LastSuccessAt = &now
	result, err = s.ListMonitoringEvents(context.Background(), OpsMonitoringEventsFilter{})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringAvailable, result.State)
	s.cfg.Ops.Monitoring.KubernetesEventsEnabled = false
	result, err = s.ListMonitoringEvents(context.Background(), OpsMonitoringEventsFilter{})
	require.NoError(t, err)
	require.Equal(t, OpsMonitoringNotApplicable, result.State)
}

func TestOpsMonitoringKubernetesScalingEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"items":[{"metadata":{"uid":"e-hpa","namespace":"n","creationTimestamp":"2026-10-23T00:00:00Z"},"involvedObject":{"kind":"HorizontalPodAutoscaler","name":"api","uid":"hpa-uid"},"reason":"SuccessfulRescale","message":"New size: 3","count":1},{"metadata":{"uid":"e-deployment","namespace":"n","creationTimestamp":"2026-10-23T00:00:00Z"},"involvedObject":{"kind":"Deployment","name":"api","uid":"deployment-uid"},"reason":"ScalingReplicaSet","count":2},{"metadata":{"uid":"e-node","namespace":"n","creationTimestamp":"2026-10-23T00:00:00Z"},"involvedObject":{"kind":"Node","name":"node","uid":"node-uid"}}]}`)
	}))
	defer server.Close()
	events, err := fetchKubernetesMonitoringEvents(context.Background(), server.Client(), server.URL, "token", "cluster")
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, "HorizontalPodAutoscaler", events[0].ObjectKind)
	require.Equal(t, "api", events[0].ObjectName)
	require.Equal(t, "hpa-uid", events[0].ObjectUID)
	require.Empty(t, events[0].Pod)
	require.Empty(t, events[0].PodUID)
	require.Equal(t, "Deployment", events[1].ObjectKind)
}
