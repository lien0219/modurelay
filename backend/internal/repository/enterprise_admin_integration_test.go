//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func enterpriseAdminExplain(t *testing.T, ctx context.Context, name, query string, args ...any) string {
	t.Helper()
	rows, e := integrationDB.QueryContext(ctx, `EXPLAIN (ANALYZE,BUFFERS) `+query, args...)
	require.NoError(t, e)
	defer rows.Close()
	var plan strings.Builder
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		plan.WriteString(line)
		plan.WriteByte('\n')
	}
	require.NoError(t, rows.Err())
	t.Log(name + "\n" + plan.String())
	return plan.String()
}

type enterpriseAdminExplainNode struct {
	NodeType       string                       `json:"Node Type"`
	Relation       string                       `json:"Relation Name"`
	Index          string                       `json:"Index Name"`
	IndexCondition string                       `json:"Index Cond"`
	Rows           float64                      `json:"Actual Rows"`
	Loops          float64                      `json:"Actual Loops"`
	Removed        float64                      `json:"Rows Removed by Filter"`
	Rechecked      float64                      `json:"Rows Removed by Index Recheck"`
	Searches       float64                      `json:"Index Searches"`
	HeapFetches    float64                      `json:"Heap Fetches"`
	Children       []enterpriseAdminExplainNode `json:"Plans"`
}

func enterpriseAdminExplainNodes(t *testing.T, ctx context.Context, name, query string, args ...any) []enterpriseAdminExplainNode {
	t.Helper()
	var raw []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) `+query, args...).Scan(&raw))
	t.Log(name + "\n" + string(raw))
	var root []struct {
		Plan enterpriseAdminExplainNode `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal(raw, &root))
	require.Len(t, root, 1)
	var nodes []enterpriseAdminExplainNode
	var visit func(enterpriseAdminExplainNode)
	visit = func(n enterpriseAdminExplainNode) {
		nodes = append(nodes, n)
		for _, child := range n.Children {
			visit(child)
		}
	}
	visit(root[0].Plan)
	return nodes
}

func enterpriseAdminAssertWebhookSourceBound(t *testing.T, nodes []enterpriseAdminExplainNode, max float64) {
	t.Helper()
	var deliveries, endpoints, searches, heapFetches float64
	for _, node := range nodes {
		work := (node.Rows + node.Removed + node.Rechecked) * node.Loops
		if node.Relation == "workspace_webhook_deliveries" {
			deliveries += work
			searches += node.Searches
			heapFetches += node.HeapFetches
		}
		if node.Relation == "workspace_webhooks" {
			endpoints += work
		}
	}
	require.LessOrEqual(t, deliveries, max, "delivery source work includes rows removed/rechecked")
	require.LessOrEqual(t, endpoints, max, "endpoint lookup work includes rows removed/rechecked")
	require.LessOrEqual(t, searches, max+2, "at most two extra empty scoped status branches")
	require.LessOrEqual(t, heapFetches, max, "index-only visibility checks must remain bounded")
}

func enterpriseAdminWebhookBulk(t *testing.T, ctx context.Context, owner, workspace, endpoint int64, prefix, status string, n int, createdAge ...time.Duration) {
	t.Helper()
	ageSeconds := float64(0)
	if len(createdAge) > 0 {
		ageSeconds = createdAge[0].Seconds()
	}
	_, e := integrationDB.ExecContext(ctx, `WITH source AS (SELECT $3::text||g AS id,now()-g*interval '1 second'-$7*interval '1 second' AS created_at FROM generate_series(1,$6::integer) g), events AS (INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,actor_user_id,subject_type,subject_id,payload) SELECT id,'workspace.updated',1,created_at,$1::bigint,$2::bigint,'workspace',($1::bigint)::text,jsonb_build_object('id',id,'type','workspace.updated','version',1,'created_at',created_at,'workspace_id',$1::bigint,'actor_user_id',$2::bigint,'subject',jsonb_build_object('type','workspace','id',($1::bigint)::text),'data',jsonb_build_object('name','fixture')) FROM source RETURNING id,workspace_id,event_type,payload,created_at) INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload,status,created_at,next_attempt_at,locked_at,finished_at) SELECT workspace_id,$4::bigint,id,event_type,payload,$5::text,created_at,now()+interval '1 day',CASE WHEN $5::text='delivering' THEN now() END,CASE WHEN $5::text IN ('succeeded','dead') THEN created_at END FROM events`, workspace, owner, prefix, endpoint, status, n, ageSeconds)
	require.NoError(t, e)
}

func enterpriseAdminAnalyzeWebhooks(t *testing.T) {
	t.Helper()
	for _, table := range []string{"workspace_webhook_deliveries", "workspace_webhooks"} {
		_, e := integrationDB.Exec(`ANALYZE ` + table)
		require.NoError(t, e)
	}
}

func TestEnterpriseAdminFix2WebhookScopedSuccessHasBoundedCompletionOrder(t *testing.T) {
	ctx, _, svc, _, owner, w := workspaceWebhookFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, w.ID)
	foreign, e := r.CreateOrganization(ctx, owner.ID, "Foreign success", "foreign-success-"+uuid.NewString())
	require.NoError(t, e)
	foreignEndpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, foreign.ID)
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, foreign.ID, foreignEndpoint.ID, "diag-foreign-success-", "succeeded", 12001)
	probe := func(t *testing.T, name string) {
		q, args := adminScopedSQL("d.finished_at", `workspace_webhook_deliveries d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`, `d.status='succeeded' AND d.finished_at IS NOT NULL`, "d.workspace_id", w.ID)
		nodes := enterpriseAdminExplainNodes(t, ctx, name, `SELECT max(success_at) FROM (`+q+` ORDER BY d.finished_at DESC LIMIT 1) last_job(success_at)`, args...)
		enterpriseAdminAssertWebhookSourceBound(t, nodes, 1)
		found := false
		for _, node := range nodes {
			if node.Index == "enterprise_admin_webhook_success_scope" {
				found = true
				require.Contains(t, node.IndexCondition, "workspace_id")
			}
		}
		require.True(t, found, "scoped success must seek workspace completion order")
	}
	enterpriseAdminAnalyzeWebhooks(t)
	t.Run("empty scoped success", func(t *testing.T) { probe(t, "empty scoped success beside12001 foreign successes") })
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "webhook" {
			require.Nil(t, worker.LastSuccessfulJobAt)
		}
	}
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, endpoint.ID, "diag-own-terminal-", "dead", 12001)
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, endpoint.ID, "diag-own-success-", "succeeded", 12001, 24*time.Hour)
	enterpriseAdminAnalyzeWebhooks(t)
	t.Run("large selected terminal history", func(t *testing.T) { probe(t, "scoped success among24002 own terminal deliveries") })
	var expected time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT finished_at FROM workspace_webhook_deliveries WHERE event_id='diag-own-success-1'`).Scan(&expected))
	d, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "webhook" {
			require.NotNil(t, worker.LastSuccessfulJobAt)
			require.WithinDuration(t, expected, *worker.LastSuccessfulJobAt, time.Microsecond)
		}
	}
}

func TestEnterpriseAdminFix2WebhookDisabledBacklogUsesBoundedNonterminalSource(t *testing.T) {
	ctx, _, svc, _, owner, w := workspaceWebhookFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	disabled := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, w.ID)
	enabled := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, w.ID)
	_, e := integrationDB.Exec(`UPDATE workspace_webhooks SET enabled=false WHERE id=$1`, disabled.ID)
	require.NoError(t, e)
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, disabled.ID, "diag-disabled-terminal-", "succeeded", 12001)
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, enabled.ID, "diag-enabled-running-", "delivering", 12001)
	enterpriseAdminAnalyzeWebhooks(t)
	for _, scope := range []int64{0, w.ID} {
		q, args := adminScopedSQL("1", `workspace_webhook_deliveries d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`, `NOT h.enabled AND d.status IN ('pending','retrying','delivering')`, "d.workspace_id", scope)
		nodes := enterpriseAdminExplainNodes(t, ctx, "disabled terminal history beside enabled nonterminal work", `SELECT count(*) FROM (`+q+` LIMIT 10001) admin_bounded`, args...)
		enterpriseAdminAssertWebhookSourceBound(t, nodes, 10001)
		inventory, inventoryArgs := adminWebhookInventorySQL(scope)
		nodes = enterpriseAdminExplainNodes(t, ctx, "combined production webhook source and classifications", inventory, inventoryArgs...)
		enterpriseAdminAssertWebhookSourceBound(t, nodes, 10001)
	}
	global, e := r.AdminOperationsJobs(ctx, admin.ID)
	require.NoError(t, e)
	detail, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, jobs := range []*service.AdminJobsDiagnostics{global, detail.Jobs} {
		for _, worker := range jobs.Workers {
			if worker.Worker == "webhook" {
				c := worker.Counts["disabled_endpoint_backlog"]
				require.True(t, c.Available)
				require.True(t, c.Capped, "zero matches in overflowing nonterminal source cannot prove no paused backlog")
				require.Zero(t, *c.Value)
			}
		}
	}
	for _, worker := range detail.Jobs.Workers {
		if worker.Worker == "webhook" {
			require.False(t, worker.OldestPending.Available)
		}
	}
	// A paused task added after the capped source window must not turn a sampled
	// zero into a claim of an exact empty backlog.
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, disabled.ID, "diag-disabled-late-", "pending", 1)
	global, e = r.AdminOperationsJobs(ctx, admin.ID)
	require.NoError(t, e)
	for _, worker := range global.Workers {
		if worker.Worker == "webhook" {
			require.True(t, worker.Counts["disabled_endpoint_backlog"].Capped)
			require.Zero(t, *worker.Counts["disabled_endpoint_backlog"].Value)
		}
	}
}

func TestEnterpriseAdminFix2WebhookGlobalPendingDoesNotScanRunningPopulation(t *testing.T) {
	ctx, _, svc, _, owner, w := workspaceWebhookFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, w.ID)
	_, e := integrationDB.Exec(`INSERT INTO workspaces(name,slug,type,owner_user_id,billing_owner_user_id) SELECT 'Running fixture','diag-delivery-workspace-'||g,'organization',$1,$1 FROM generate_series(1,12001) g`, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_members(workspace_id,user_id,role,status) SELECT id,$1,'owner','active' FROM workspaces WHERE slug LIKE 'diag-delivery-workspace-%'`, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO projects(workspace_id,name,slug,status,is_default,created_by_user_id) SELECT id,'Default','default','active',true,$1 FROM workspaces WHERE slug LIKE 'diag-delivery-workspace-%'`, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_webhooks(workspace_id,name,url,secret_current_encrypted,created_by_user_id) SELECT id,'Fixture','https://8.8.8.8/events',(SELECT secret_current_encrypted FROM workspace_webhooks WHERE id=$2),$1 FROM workspaces WHERE slug LIKE 'diag-delivery-workspace-%'`, owner.ID, endpoint.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`WITH source AS (SELECT id AS workspace_id,'diag-spread-running-'||id AS id,now()-id*interval '1 second' AS created_at FROM workspaces WHERE slug LIKE 'diag-delivery-workspace-%'), events AS (INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,actor_user_id,subject_type,subject_id,payload) SELECT id,'workspace.updated',1,created_at,workspace_id,$1::bigint,'workspace',workspace_id::text,jsonb_build_object('id',id,'type','workspace.updated','version',1,'created_at',created_at,'workspace_id',workspace_id,'actor_user_id',$1::bigint,'subject',jsonb_build_object('type','workspace','id',workspace_id::text),'data',jsonb_build_object('name','fixture')) FROM source RETURNING id,workspace_id,event_type,payload,created_at) INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload,status,created_at,next_attempt_at,locked_at) SELECT e.workspace_id,h.id,e.id,e.event_type,e.payload,'delivering',e.created_at,now()+interval '1 day',now() FROM events e JOIN workspace_webhooks h ON h.workspace_id=e.workspace_id`, owner.ID)
	require.NoError(t, e)
	enterpriseAdminAnalyzeWebhooks(t)
	q, args := adminScopedSQL("1", `workspace_webhook_deliveries d JOIN workspace_webhooks h ON h.workspace_id=d.workspace_id AND h.id=d.webhook_id`, `d.status IN ('pending','retrying')`, "d.workspace_id", 0)
	nodes := enterpriseAdminExplainNodes(t, ctx, "empty global pending beside12002 delivering rows across valid tenants", `SELECT count(*) FROM (`+q+` LIMIT 10001) admin_bounded`, args...)
	enterpriseAdminAssertWebhookSourceBound(t, nodes, 10001)
	inventory, inventoryArgs := adminWebhookInventorySQL(0)
	nodes = enterpriseAdminExplainNodes(t, ctx, "combined global production source across valid tenants", inventory, inventoryArgs...)
	enterpriseAdminAssertWebhookSourceBound(t, nodes, 10001)
	jobs, e := r.AdminOperationsJobs(ctx, admin.ID)
	require.NoError(t, e)
	for _, worker := range jobs.Workers {
		if worker.Worker == "webhook" {
			require.True(t, worker.Counts["pending"].Available)
			require.True(t, worker.Counts["pending"].Capped)
			require.Zero(t, *worker.Counts["pending"].Value)
			require.True(t, worker.Counts["running"].Available)
			require.True(t, worker.Counts["running"].Capped)
			require.EqualValues(t, 10001, *worker.Counts["running"].Value)
		}
	}
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, endpoint.ID, "diag-late-pending-", "pending", 1)
	enterpriseAdminWebhookBulk(t, ctx, owner.ID, w.ID, endpoint.ID, "diag-late-retrying-", "retrying", 1)
	var expectedOldest time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT created_at FROM workspace_webhook_deliveries WHERE event_id='diag-spread-running-'||(SELECT max(id) FROM workspaces WHERE slug LIKE 'diag-delivery-workspace-%')`).Scan(&expectedOldest))
	jobs, e = r.AdminOperationsJobs(ctx, admin.ID)
	require.NoError(t, e)
	for _, worker := range jobs.Workers {
		if worker.Worker == "webhook" {
			require.True(t, worker.Counts["pending"].Capped)
			require.Zero(t, *worker.Counts["pending"].Value)
			require.True(t, worker.OldestPending.Available, "global oldest nonterminal has its own matching creation-order index")
			require.NotNil(t, worker.OldestPending.Value)
			require.WithinDuration(t, expectedOldest, *worker.OldestPending.Value, time.Microsecond)
		}
	}
}

func TestEnterpriseAdminFixSparseOperationalQueryPlans(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	foreign, e := r.CreateOrganization(ctx, owner.ID, "Foreign query fixture", "diag-foreign-"+uuid.NewString())
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO batch_image_jobs(batch_id,user_id,workspace_id,project_id,billing_principal_user_id,provider,model,status,item_count,settled_at,finished_at) SELECT 'diag-image-'||g,$1,$2,p.id,$1,'gemini_api','image','completed',1,now(),now() FROM projects p CROSS JOIN generate_series(1,12000) g WHERE p.workspace_id=$2 AND p.is_default`, owner.ID, w.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,actor_user_id,subject_type,subject_id,payload) SELECT 'diag-event-'||g,'workspace.updated',1,now(),$1::bigint,$2::bigint,'workspace',($1::bigint)::text,jsonb_build_object('id','diag-event-'||g,'type','workspace.updated','version',1,'created_at',now(),'workspace_id',$1::bigint,'actor_user_id',$2::bigint,'subject',jsonb_build_object('type','workspace','id',($1::bigint)::text),'data',jsonb_build_object('name','fixture')) FROM generate_series(1,12000) g`, w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE workspace_id=$1 AND id LIKE 'diag-event-%'`, w.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO finops_anomaly_detection_leases(workspace_id,bucket_start,detector_version,completed_at) SELECT $1,date_trunc('hour',now())-g*interval '1 hour','diag-v1',now() FROM generate_series(1,12000) g`, w.ID)
	require.NoError(t, e)
	for _, table := range []string{"batch_image_jobs", "domain_events", "domain_event_outbox", "finops_anomaly_detection_leases"} {
		_, e = integrationDB.Exec(`ANALYZE ` + table)
		require.NoError(t, e)
	}
	queries := []struct {
		name, from, predicate, scope, index string
		workspace                           int64
	}{
		{"empty global image OR", "batch_image_jobs j", `(j.status NOT IN ('completed','failed','cancelled','output_deleted') OR j.last_error_code='SUBMIT_OUTCOME_UNKNOWN')`, "j.workspace_id", "enterprise_admin_image_attention_", 0},
		{"empty scoped image OR", "batch_image_jobs j", `(j.status NOT IN ('completed','failed','cancelled','output_deleted') OR j.last_error_code='SUBMIT_OUTCOME_UNKNOWN')`, "j.workspace_id", "enterprise_admin_image_attention_scope", w.ID},
		{"empty global outbox retry", "domain_event_outbox o JOIN domain_events e ON e.id=o.event_id", `o.delivered_at IS NULL AND o.last_error IS NOT NULL AND o.last_error<>''`, "e.workspace_id", "enterprise_admin_outbox_retry", 0},
		{"empty global outbox expired claims", "domain_event_outbox o JOIN domain_events e ON e.id=o.event_id", `o.delivered_at IS NULL AND o.locked_until<=now()`, "e.workspace_id", "enterprise_admin_outbox_claims", 0},
		{"bounded scoped outbox retry", "domain_event_outbox o JOIN domain_events e ON e.id=o.event_id", `o.delivered_at IS NULL AND o.last_error IS NOT NULL AND o.last_error<>''`, "e.workspace_id", "domain_events_scope_created", w.ID},
		{"empty scope with foreign outbox", "domain_event_outbox o JOIN domain_events e ON e.id=o.event_id", `o.delivered_at IS NULL`, "e.workspace_id", "domain_events_scope_created", foreign.ID},
		{"empty global FinOps retry", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND COALESCE(f.last_error_code,'')<>''`, "f.workspace_id", "enterprise_admin_finops_errors_", 0},
		{"empty scoped FinOps retry", "finops_anomaly_detection_leases f", `f.completed_at IS NULL AND COALESCE(f.last_error_code,'')<>''`, "f.workspace_id", "enterprise_admin_finops_", w.ID},
	}
	for _, test := range queries {
		t.Run(test.name, func(t *testing.T) {
			q, args := adminScopedSQL("1", test.from, test.predicate, test.scope, test.workspace)
			plan := enterpriseAdminExplain(t, ctx, test.name, `SELECT count(*) FROM (`+q+` LIMIT 10001) admin_bounded`, args...)
			require.Contains(t, plan, test.index)
			require.NotContains(t, plan, "Seq Scan on batch_image_jobs")
			require.NotContains(t, plan, "Seq Scan on domain_event_outbox")
			require.NotContains(t, plan, "Seq Scan on domain_events")
			require.NotContains(t, plan, "Seq Scan on finops_anomaly_detection_leases")
		})
	}
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "outbox" {
			require.True(t, worker.Counts["retry_evidence"].Capped)
			require.Zero(t, *worker.Counts["retry_evidence"].Value)
			require.False(t, worker.OldestPending.Available)
			require.Nil(t, worker.DueLagSeconds)
		}
	}
	d, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, foreign.ID)
	require.NoError(t, e)
	require.False(t, d.Counts["pending_outbox"].Capped)
}

func TestEnterpriseAdminFixAllowedOperationsRequireKnownOwnerAndPayer(t *testing.T) {
	ctx, r, _, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, []string{"suspend"}, d.AllowedOperations)
	_, e = integrationDB.Exec(`UPDATE workspaces SET status='suspended' WHERE id=$1`, w.ID)
	require.NoError(t, e)
	d, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, []string{"resume"}, d.AllowedOperations)
	require.NotNil(t, d.OwnerValid)
	require.True(t, *d.OwnerValid)
	_, e = integrationDB.Exec(`ALTER TABLE workspace_members RENAME TO diag_missing_members_for_operation`)
	require.NoError(t, e)
	d, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Empty(t, d.AllowedOperations)
	require.Nil(t, d.OwnerValid)
	require.Nil(t, d.BillingOwnerValid)
}

func TestEnterpriseAdminFixPurgeIgnoresItsOwnDeletionEvents(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	challenge, e := r.LifecycleDeletionChallenge(ctx, owner.ID, w.ID)
	require.NoError(t, e)
	_, e = r.RequestLifecycleDeletion(ctx, owner.ID, w.ID, w.Name, challenge.Token)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.NotNil(t, d.Counts["pending_outbox"].Value)
	for _, blocker := range d.Purge.Blockers {
		require.NotEqual(t, "PENDING_DOMAIN_EVENTS", blocker.Code, "preflight excludes its own deletion events")
	}
}

func enterpriseAdminActor(t *testing.T) *service.User {
	t.Helper()
	return mustCreateUser(t, testEntClient(t), &service.User{Email: "admin-diag-" + uuid.NewString() + "@example.com", Role: "admin"})
}

func TestEnterpriseAdminFixIdentityHealthBeyondRecentList(t *testing.T) {
	ctx, _, owner, w, provider, _ := enterpriseIdentityFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`UPDATE workspace_identity_providers SET last_validated_at=now()-interval '2 days',last_validation_code='DISCOVERY_FAILED' WHERE id=$1`, provider.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_identity_providers(workspace_id,provider_key,name,issuer_url,client_id,encrypted_client_secret,created_by_user_id,last_validated_at,last_validation_code) SELECT $1,'new-'||g,'Safe fixture','https://idp.example.com','client','cipher:fixture-secret',$2,now(),'SUCCESS' FROM generate_series(1,25) g`, w.ID, owner.ID)
	require.NoError(t, e)
	d, e := NewWorkspaceRepository(integrationDB).(*workspaceRepository).AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Len(t, d.Identity.Providers.Items, 20)
	require.True(t, d.Identity.Providers.Capped)
	require.Equal(t, "degraded", d.Identity.Providers.State)
	require.EqualValues(t, 1, *d.Identity.Providers.Counts["failed_validation"].Value)
	require.False(t, d.Identity.Providers.Counts["failed_validation"].Capped)
}

func TestEnterpriseAdminFixSourceSamplesCannotCertifyEmpty(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	ctx, _, a := budgetFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`INSERT INTO budget_reservations(id,request_id,actor_user_id,api_key_id,workspace_id,project_id,billing_principal_user_id,period_start,period_end,project_period_start,project_period_end,estimate,status,finalized_at) SELECT md5('diag-reservation-'||g)::uuid,'diag-reservation-'||g,$1,$2,$3,$4,$5,'2026-10-01','2026-11-01','2026-10-01','2026-11-01',0,'released',now() FROM generate_series(1,12000) g`, a.ActorUserID, a.APIKeyID, a.WorkspaceID, a.ProjectID, a.BillingPrincipalUserID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, a.WorkspaceID)
	require.NoError(t, e)
	c := d.Counts["unresolved_alerts"]
	require.True(t, c.Available)
	require.True(t, c.Capped, "empty matches within a capped reservation sample cannot prove no pending alerts")
	require.Zero(t, *c.Value)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "billing_recovery" {
			require.True(t, worker.Counts["pending_alerts"].Capped)
			require.False(t, worker.PendingAlertAge.Available)
		}
	}
	require.False(t, d.Purge.Available)
}

func TestEnterpriseAdminFixScopedHistoricalProbesAreQualifiedSamples(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,state,created_at) SELECT md5('diag-export-history-'||g)::uuid,$1,$2,'cancelled',now()-g*interval '1 second' FROM generate_series(1,12000) g`, w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,state,created_at,failure_code) VALUES($1,$2,$3,'failed',now()-interval '2 days','EXPORT_BUILD_FAILED')`, uuid.NewString(), w.ID, owner.ID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "export" {
			require.True(t, worker.Counts["failed"].Capped)
			require.Zero(t, *worker.Counts["failed"].Value)
			require.Nil(t, worker.LastFailure.Code)
			require.False(t, worker.LastFailure.Available)
			require.True(t, worker.JobsCapped)
			require.True(t, worker.Counts["pending"].Available)
			require.False(t, worker.Counts["pending"].Capped)
			require.Zero(t, *worker.Counts["pending"].Value)
		}
	}
}

func TestEnterpriseAdminFixSCIMSparseForeignErrorsAndSourceOverflow(t *testing.T) {
	ctx, _, owner, w, connector, _ := scimFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	foreign, e := r.CreateOrganization(ctx, owner.ID, "Foreign SCIM", "scim-foreign-"+uuid.NewString())
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_scim_connectors(workspace_id,public_endpoint_id,name,default_role,created_by_user_id,last_error_code,failure_count) SELECT $1,md5('foreign-connector-'||g)||substr(md5('foreign-connector-suffix-'||g),1,11),'Foreign fixture','viewer',$2,'sync_failed',3 FROM generate_series(1,12000) g`, foreign.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`ANALYZE workspace_scim_connectors`)
	require.NoError(t, e)
	q, args := adminScopedSQL("1", "workspace_scim_connectors c", `c.failure_count>0`, "c.workspace_id", w.ID)
	plan := enterpriseAdminExplain(t, ctx, "empty scoped SCIM errors with12000 foreign failures", `SELECT count(*) FROM (`+q+` LIMIT 10001) admin_bounded`, args...)
	require.NotContains(t, plan, "Seq Scan on workspace_scim_connectors")
	lastQuery, lastArgs := adminScopedSQL("COALESCE(c.last_error_code,''),c.updated_at", "workspace_scim_connectors c", `c.failure_count>0`, "c.workspace_id", w.ID)
	lastPlan := enterpriseAdminExplain(t, ctx, "scoped SCIM latest code with12000 foreign failures", lastQuery+` ORDER BY c.updated_at DESC LIMIT 1`, lastArgs...)
	require.Contains(t, lastPlan, "workspace_scim_connectors_workspace_id_id_key")
	require.NotContains(t, lastPlan, "Seq Scan on workspace_scim_connectors")
	_, e = integrationDB.Exec(`UPDATE workspace_scim_connectors SET last_error_code='sync_failed',failure_count=3,updated_at=now()-interval '2 days' WHERE id=$1`, connector.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_scim_connectors(workspace_id,public_endpoint_id,name,default_role,created_by_user_id) SELECT $1,md5('own-connector-'||g)||substr(md5('own-connector-suffix-'||g),1,11),'Scoped fixture','viewer',$2 FROM generate_series(1,12000) g`, w.ID, owner.ID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "scim" {
			require.True(t, worker.Counts["connector_errors"].Capped)
			require.Zero(t, *worker.Counts["connector_errors"].Value)
			require.Nil(t, worker.LastFailure.Code)
			require.False(t, worker.LastFailure.Available)
		}
	}
}

func TestEnterpriseAdminFixPurgeOutboxSelfEventSampleOverflowIsUnknown(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`WITH source AS (SELECT 'purge-diag-event-'||g AS id,CASE WHEN g=0 THEN 'workspace.updated' ELSE 'workspace.deletion.requested' END AS event_type,CASE WHEN g=0 THEN now()-interval '2 days' ELSE now() END AS created_at FROM generate_series(0,12000) g) INSERT INTO domain_events(id,event_type,event_version,created_at,workspace_id,actor_user_id,subject_type,subject_id,payload) SELECT id,event_type,1,created_at,$1::bigint,$2::bigint,'workspace',($1::bigint)::text,jsonb_build_object('id',id,'type',event_type,'version',1,'created_at',created_at,'workspace_id',$1::bigint,'actor_user_id',$2::bigint,'subject',jsonb_build_object('type','workspace','id',($1::bigint)::text),'data',jsonb_build_object('name','fixture')) FROM source`, w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO domain_event_outbox(event_id) SELECT id FROM domain_events WHERE workspace_id=$1 AND id LIKE 'purge-diag-event-%'`, w.ID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.True(t, d.Counts["pending_outbox"].Available)
	require.True(t, d.Counts["pending_outbox"].Capped)
	require.Zero(t, *d.Counts["pending_outbox"].Value)
	require.False(t, d.Purge.Available)
	q, args := adminScopedSQL("o.event_id", `domain_event_outbox o JOIN domain_events e ON e.id=o.event_id`, `o.delivered_at IS NULL AND e.event_type NOT LIKE 'workspace.deletion.%'`, "e.workspace_id", w.ID)
	_, e = integrationDB.Exec(`ANALYZE domain_events`)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`ANALYZE domain_event_outbox`)
	require.NoError(t, e)
	plan := enterpriseAdminExplain(t, ctx, "bounded purge self-event exclusion", `SELECT count(*) FROM (`+q+` LIMIT 10001) admin_bounded`, args...)
	require.Contains(t, plan, "domain_events_scope_created")
	require.Contains(t, plan, "domain_event_outbox_pkey")
	require.NotContains(t, plan, "Seq Scan on domain_event_outbox")
}

func TestEnterpriseAdminFixCanonicalRetention(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`INSERT INTO workspace_retention_policies(workspace_id,category,retention_days,updated_by_user_id) VALUES($1,'temporary',7,$2),($1,'operational',0,$2)`, w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE platform_retention_policies SET minimum_days=30,default_days=30 WHERE category='temporary'`)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, p := range d.Retention {
		if p.Category == "temporary" {
			require.Equal(t, 30, p.EffectiveDays)
		}
		if p.Category == "operational" {
			require.Zero(t, p.EffectiveDays)
			require.True(t, p.Protected)
		}
	}
}

func TestEnterpriseAdminFixMissingPayerSourceIsUnknown(t *testing.T) {
	ctx, r, _, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`ALTER TABLE workspace_members RENAME TO diag_missing_members`)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Nil(t, d.BillingOwnerValid)
	require.NotEqual(t, "blocked", d.State)
}

func TestEnterpriseAdminFixLatestWebhookFailureIndependentOfRecentCreatedSample(t *testing.T) {
	ctx, _, svc, _, owner, w := workspaceWebhookFixture(t)
	admin := enterpriseAdminActor(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	endpoint := createWorkspaceWebhookFixture(t, ctx, svc, owner.ID, w.ID)
	old, e := svc.Test(ctx, owner.ID, w.ID, endpoint.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='dead',created_at=now()-interval '10 days',finished_at=now()-interval '1 minute' WHERE id=$1`, old.ID)
	require.NoError(t, e)
	for i := 0; i < 25; i++ {
		tx, e := integrationDB.BeginTx(ctx, nil)
		require.NoError(t, e)
		e = insertWorkspaceMutationEvent(ctx, tx, w.ID, 0, owner.ID, "workspace_updated", "workspace", w.ID, service.DomainEventData{"name": "recent"})
		require.NoError(t, e)
		require.NoError(t, tx.Commit())
	}
	// Each newer creation corresponds to a distinct immutable domain event.
	_, e = integrationDB.Exec(`INSERT INTO workspace_webhook_deliveries(workspace_id,webhook_id,event_id,event_type,payload,status,finished_at) SELECT e.workspace_id,$2,e.id,e.event_type,e.payload,'dead',now()-interval '1 day' FROM domain_events e WHERE e.workspace_id=$1 AND e.event_type='workspace.updated' ORDER BY e.created_at DESC LIMIT 25`, w.ID, endpoint.ID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "webhook" {
			require.NotNil(t, worker.LastFailedAt)
			require.WithinDuration(t, time.Now().Add(-time.Minute), *worker.LastFailedAt, 3*time.Second)
		}
	}
	_, e = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET finished_at=now()+interval '1 day' WHERE id=$1`, old.ID)
	require.NoError(t, e)
	d, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "webhook" {
			require.Nil(t, worker.LastFailedAt)
			require.False(t, worker.LatestFailureTime.Available)
			require.Nil(t, worker.LastFailure.SourceTime)
		}
	}
}

func TestEnterpriseAdminFixIdentityStatesAndSafePurgeEvidence(t *testing.T) {
	ctx, _, owner, w, connector, _ := scimFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`UPDATE workspace_identity_providers SET last_validated_at=now()-interval '2 days',last_validation_code='DISCOVERY_FAILED' WHERE workspace_id=$1`, w.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_scim_connectors SET last_sync_at=now()-interval '2 days',last_error_code='bearer-private-material',failure_count=2 WHERE id=$1`, connector.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_deletion_jobs(id,workspace_id,requested_by_user_id,previous_status,state,available_at,earliest_purge_at,phase,progress,blocking_reasons,updated_at) VALUES($1,$2,$3,'active','blocked',now()+interval '8 days',now()+interval '8 days','credentials',12,'[{"code":"LEGAL_HOLD","reason":"private-stored-reason","count":1},{"code":"sk-private-unknown","reason":"another secret","count":1}]',now()-interval '2 days')`, uuid.NewString(), w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_lifecycle_holds(workspace_id,code,reason) VALUES($1,'LEGAL_HOLD','private-current-reason')`, w.ID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, "degraded", d.Identity.Providers.State)
	require.Equal(t, "stale", d.Identity.Providers.Items[0].Freshness)
	require.Equal(t, "DISCOVERY_FAILED", d.Identity.Providers.Items[0].LastValidationCode)
	require.Equal(t, "SYNC_FAILURE", d.Identity.SCIM.Items[0].LastErrorCode)
	require.True(t, d.Counts["members_active"].Available)
	require.EqualValues(t, 1, *d.Counts["members_active"].Value)
	require.True(t, d.Counts["projects_active"].Available)
	require.True(t, d.Counts["failed_batch_images"].Available)
	require.NotEmpty(t, d.Purge.Blockers)
	require.Equal(t, "current_bounded_preflight", d.Purge.Coverage)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "purge" {
			j := worker.Jobs[0]
			require.Equal(t, "credentials", j.Phase)
			require.NotNil(t, j.Progress)
			require.EqualValues(t, 12, *j.Progress)
			require.Len(t, j.Blockers, 2)
			require.Equal(t, "historical_job_checkpoint", j.Blockers[0].Coverage)
			require.Equal(t, "stale", j.Blockers[0].Freshness)
			require.Equal(t, "UNKNOWN_STORED_BLOCKER", j.Blockers[1].Code)
		}
	}
	raw, e := json.Marshal(d)
	require.NoError(t, e)
	for _, secret := range []string{"private-stored-reason", "private-current-reason", "bearer-private-material", "sk-private-unknown", "another secret"} {
		require.NotContains(t, string(raw), secret)
	}
	_, e = integrationDB.Exec(`UPDATE workspace_identity_providers SET last_validated_at=now()+interval '1 day',last_validation_code='SUCCESS' WHERE workspace_id=$1`, w.ID)
	require.NoError(t, e)
	d, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Equal(t, "unknown", d.Identity.Providers.Items[0].Freshness)
	require.Equal(t, "unknown", d.Identity.Providers.Items[0].State)
}

func TestEnterpriseAdminFixOldestPendingIsNotDueLag(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,created_at,available_at) VALUES($1,$2,$3,now()-interval '5 days',now()-interval '1 minute')`, uuid.NewString(), w.ID, owner.ID)
	require.NoError(t, e)
	jobs, e := r.AdminOperationsJobs(ctx, admin.ID)
	require.NoError(t, e)
	for _, worker := range jobs.Workers {
		if worker.Worker == "export" {
			require.True(t, worker.OldestPending.Available)
			require.NotNil(t, worker.OldestPending.AgeSeconds)
			require.Greater(t, *worker.OldestPending.AgeSeconds, int64(4*24*3600))
			require.Less(t, *worker.DueLagSeconds, int64(90))
			require.NotNil(t, worker.Counts["running"].Value)
		}
		if worker.Worker == "billing_recovery" {
			require.Nil(t, worker.DueLagSeconds)
		}
	}
}

func TestEnterpriseAdminPostgresCappedInventoryIsLowerBound(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`INSERT INTO projects(workspace_id,name,slug,status,created_by_user_id) SELECT $1,'inventory fixture','cap-'||g,'active',$2 FROM generate_series(1,10002) g`, w.ID, owner.ID)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	c := d.Counts["projects"]
	require.True(t, c.Available)
	require.True(t, c.Capped)
	require.EqualValues(t, 10001, *c.Value)
}

func TestEnterpriseAdminPostgresPartialFailureRecoversSnapshot(t *testing.T) {
	ctx, r, _, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`ALTER TABLE workspace_security_policies RENAME TO diag_missing_security`)
	require.NoError(t, e)
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	require.Nil(t, d.SecurityPolicy)
	require.True(t, d.Counts["members"].Available)
	require.NotEmpty(t, d.RecentAudit)
	for _, worker := range d.Jobs.Workers {
		if worker.Worker == "export" {
			require.True(t, worker.Counts["pending"].Available)
		}
	}
	raw, e := json.Marshal(d)
	require.NoError(t, e)
	require.NotContains(t, string(raw), "diag_missing_security")
	require.NotContains(t, string(raw), "does not exist")
}

func TestEnterpriseAdminPostgresFinOpsFreshnessAndRotation(t *testing.T) {
	ctx, r, _, _ := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	for _, age := range []struct{ interval, want string }{{"-1 hour", "stale"}, {"1 hour", "unknown"}} {
		_, e := integrationDB.Exec(`INSERT INTO finops_anomaly_detector_status(id,last_successful_scan,updated_at,lag_seconds) VALUES(1,now()+$1::interval,now()+$1::interval,17) ON CONFLICT(id) DO UPDATE SET last_successful_scan=EXCLUDED.last_successful_scan,updated_at=EXCLUDED.updated_at,lag_seconds=17`, age.interval)
		require.NoError(t, e)
		jobs, e := r.AdminOperationsJobs(ctx, admin.ID)
		require.NoError(t, e)
		require.Len(t, jobs.Workers, 9)
		for _, worker := range jobs.Workers {
			require.Equal(t, "unknown", worker.Liveness)
			if worker.Worker == "finops" {
				require.Equal(t, age.want, worker.Freshness)
				require.Nil(t, worker.StoredScanLagSeconds)
			}
			if worker.Worker == "async_settlement" {
				require.False(t, worker.Counts["video_pending"].Available)
				require.Nil(t, worker.Counts["video_pending"].Value)
			}
		}
		require.Equal(t, "blocked", jobs.ExportRotation.State)
		require.False(t, jobs.ExportRotation.RotationCertified)
	}
}

func TestEnterpriseAdminPostgresReadAuthorizationReloaded(t *testing.T) {
	ctx, r, _, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	for _, change := range []string{`role='user'`, `role='admin',status='inactive'`, `status='active',deleted_at=now()`} {
		_, e := integrationDB.Exec(`UPDATE users SET `+change+` WHERE id=$1`, admin.ID)
		require.NoError(t, e)
		_, e = r.AdminSearchWorkspaces(ctx, admin.ID, service.AdminWorkspaceFilter{})
		require.ErrorIs(t, e, service.ErrWorkspaceForbidden)
		_, e = r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
		require.ErrorIs(t, e, service.ErrWorkspaceForbidden)
		_, e = r.AdminOperationsJobs(ctx, admin.ID)
		require.ErrorIs(t, e, service.ErrWorkspaceForbidden)
	}
}

func TestEnterpriseAdminPostgresReceiptImmutabilityAndMigrationReplay(t *testing.T) {
	ctx, _, _, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	insert := `INSERT INTO enterprise_admin_operations(id,actor_user_id,workspace_id,idempotency_key,action,target_type,target_id,request_fingerprint,reason,previous_status,result_status,result_updated_at) VALUES($1,$2,$3,$4,'suspend','workspace',$3,$5,$6,'active','suspended',now())`
	id, key := uuid.NewString(), uuid.NewString()
	_, e := integrationDB.Exec(insert, id, admin.ID, w.ID, key, strings.Repeat("a", 64), "operator reason")
	require.NoError(t, e)
	for _, q := range []string{`UPDATE enterprise_admin_operations SET reason='changed'`, `DELETE FROM enterprise_admin_operations`, `TRUNCATE enterprise_admin_operations`} {
		_, e = integrationDB.Exec(q)
		require.Error(t, e)
	}
	_, e = integrationDB.Exec(insert, uuid.NewString(), admin.ID, w.ID, key, strings.Repeat("a", 64), "replay")
	require.Error(t, e)
	for _, reason := range []string{"", strings.Repeat("x", 501)} {
		_, e = integrationDB.Exec(insert, uuid.NewString(), admin.ID, w.ID, uuid.NewString(), strings.Repeat("a", 64), reason)
		require.Error(t, e)
	}
	_, e = integrationDB.Exec(insert, uuid.NewString(), int64(999999), w.ID, uuid.NewString(), strings.Repeat("a", 64), "foreign actor")
	require.Error(t, e)
	_, e = integrationDB.Exec(insert, uuid.NewString(), admin.ID, int64(999999), uuid.NewString(), strings.Repeat("a", 64), "foreign workspace")
	require.Error(t, e)
	require.NoError(t, ApplyMigrations(ctx, integrationDB))
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM enterprise_admin_operations WHERE id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
}

func TestEnterpriseAdminPostgresStatementTimeoutAndSavepointRecovery(t *testing.T) {
	ctx, r, _, _ := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	tx, now, e := r.adminReadTx(ctx, admin.ID)
	require.NoError(t, e)
	defer func() { _ = tx.Rollback() }()
	started := time.Now()
	e = adminSection(ctx, tx, func() error { _, err := tx.ExecContext(ctx, `SELECT pg_sleep(4)`); return err })
	require.Error(t, e)
	require.Less(t, time.Since(started), 4*time.Second)
	c := adminCount(ctx, tx, now, "workspaces", "inventory", `SELECT id FROM workspaces`)
	require.True(t, c.Available)
	require.NoError(t, tx.Commit())
}

func TestEnterpriseAdminPostgresWorkerLeaseAndFailureEvidence(t *testing.T) {
	ctx, _, webhookSvc, _, owner, w := workspaceWebhookFixture(t)
	r := NewWorkspaceRepository(integrationDB).(*workspaceRepository)
	admin := enterpriseAdminActor(t)
	endpoint := createWorkspaceWebhookFixture(t, ctx, webhookSvc, owner.ID, w.ID)
	delivery, e := webhookSvc.Test(ctx, owner.ID, w.ID, endpoint.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_webhook_deliveries SET status='dead',finished_at=now()-interval '10 minutes',error_code='stored-private-key-material' WHERE id=$1`, delivery.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE domain_event_outbox SET last_error='dispatch_timeout',available_at=now() WHERE event_id IN (SELECT id FROM domain_events WHERE workspace_id=$1)`, w.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,state,failure_code) VALUES($1,$2,$3,'failed','EXPORT_BUILD_FAILED')`, uuid.NewString(), w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO finops_anomaly_detection_leases(workspace_id,bucket_start,detector_version,last_error_code) VALUES($1,date_trunc('hour',now()),'diag-v1','timeout')`, w.ID)
	require.NoError(t, e)
	image := lifecycleCompletedImageFixture(t, owner.ID, w.ID)
	_, e = integrationDB.Exec(`UPDATE batch_image_jobs SET status='failed',last_error_code='SUBMIT_FAILED' WHERE batch_id=$1`, image)
	require.NoError(t, e)
	foreign, e := r.CreateOrganization(ctx, owner.ID, "foreign", "foreign-"+uuid.NewString())
	require.NoError(t, e)
	foreignEndpoint := createWorkspaceWebhookFixture(t, ctx, webhookSvc, owner.ID, foreign.ID)
	foreignDelivery, e := webhookSvc.Test(ctx, owner.ID, foreign.ID, foreignEndpoint.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspace_webhooks SET enabled=false WHERE id=$1`, foreignEndpoint.ID)
	require.NoError(t, e)
	_ = foreignDelivery
	for i, state := range []string{"running", "running", "pending"} {
		tenant, e := r.CreateOrganization(ctx, owner.ID, "lease fixture", "lease-"+uuid.NewString())
		require.NoError(t, e)
		lease := time.Now().Add(time.Hour)
		if i == 1 {
			lease = time.Now().Add(-time.Hour)
		}
		token := uuid.NewString()
		if state == "pending" {
			_, e = integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,state) VALUES($1,$2,$3,$4)`, uuid.NewString(), tenant.ID, owner.ID, state)
		} else {
			_, e = integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,state,lease_token,lease_expires_at) VALUES($1,$2,$3,$4,$5,$6)`, uuid.NewString(), tenant.ID, owner.ID, state, token, lease)
		}
		require.NoError(t, e)
	}
	for _, state := range []string{"pending", "blocked"} {
		tenant, e := r.CreateOrganization(ctx, owner.ID, "purge fixture", "purge-"+uuid.NewString())
		require.NoError(t, e)
		_, e = integrationDB.Exec(`INSERT INTO workspace_deletion_jobs(id,workspace_id,requested_by_user_id,previous_status,state,available_at,earliest_purge_at) VALUES($1,$2,$3,'active',$4,now()+interval '8 days',now()+interval '8 days')`, uuid.NewString(), tenant.ID, owner.ID, state)
		require.NoError(t, e)
	}
	jobs, e := r.AdminOperationsJobs(ctx, admin.ID)
	require.NoError(t, e)
	for _, worker := range jobs.Workers {
		for key, c := range worker.Counts {
			unsupportedExecution := (key == "running" || key == "failed") && (worker.Worker == "outbox" || worker.Worker == "finops" || worker.Worker == "scim" || worker.Worker == "billing_recovery")
			if worker.Worker == "notification" || strings.HasPrefix(key, "video_") || unsupportedExecution || (key == "completed" && worker.Worker == "scim") || (key == "pending" && (worker.Worker == "finops" || worker.Worker == "scim")) {
				require.False(t, c.Available)
				require.Nil(t, c.Value)
				continue
			}
			require.True(t, c.Available, "%s %s", worker.Worker, key)
		}
		switch worker.Worker {
		case "outbox":
			require.NotNil(t, worker.LastFailure.Code)
			require.Equal(t, "dispatch_timeout", *worker.LastFailure.Code)
			require.Nil(t, worker.LastFailedAt)
		case "finops":
			require.NotNil(t, worker.LastFailure.Code)
			require.Equal(t, "timeout", *worker.LastFailure.Code)
			require.Nil(t, worker.LastFailedAt)
		case "async_settlement":
			require.NotNil(t, worker.LastFailure.Code)
			require.Equal(t, "SUBMIT_FAILED", *worker.LastFailure.Code)
			require.EqualValues(t, 1, *worker.Counts["failed"].Value)
		case "export":
			require.EqualValues(t, 1, *worker.Counts["live_leases"].Value)
			require.EqualValues(t, 1, *worker.Counts["expired_leases"].Value)
			require.Equal(t, "degraded", worker.State)
			require.EqualValues(t, 2, *worker.Counts["running"].Value)
			require.EqualValues(t, 1, *worker.Counts["failed"].Value)
			require.NotNil(t, worker.LastFailure.Code)
			require.Equal(t, "EXPORT_BUILD_FAILED", *worker.LastFailure.Code)
			for _, j := range worker.Jobs {
				require.Nil(t, j.FailedAt)
			}
		case "purge":
			require.EqualValues(t, 1, *worker.Counts["cooling"].Value)
			require.EqualValues(t, 1, *worker.Counts["blocked"].Value)
		case "webhook":
			require.NotNil(t, worker.LastFailure.Code)
			require.Equal(t, "JOB_FAILURE", *worker.LastFailure.Code)
			require.EqualValues(t, 1, *worker.Counts["disabled_endpoint_backlog"].Value)
			require.Len(t, worker.Jobs, 1)
			require.Equal(t, w.ID, worker.Jobs[0].WorkspaceID)
			require.Equal(t, "JOB_FAILURE", worker.Jobs[0].FailureCode)
			require.NotNil(t, worker.Jobs[0].FailedAt)
			require.NotNil(t, worker.Jobs[0].WebhookID)
			require.Equal(t, "eligible_requires_guarded_action", worker.Jobs[0].RetryEligibility)
		}
	}
	var failures int64
	for _, bucket := range jobs.WebhookFailureTrend {
		require.True(t, bucket.Count.Available)
		failures += *bucket.Count.Value
	}
	require.EqualValues(t, 1, failures)
	raw, e := json.Marshal(jobs)
	require.NoError(t, e)
	require.NotContains(t, string(raw), "stored-private-key-material")
	d, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	for _, worker := range d.Jobs.Workers {
		for _, j := range worker.Jobs {
			require.Equal(t, w.ID, j.WorkspaceID)
		}
	}
}

func TestEnterpriseAdminPostgresFinOpsTerminalFailureIsSeparateFromPending(t *testing.T) {
	ctx, r, _, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.ExecContext(ctx, `INSERT INTO finops_anomaly_detection_leases(workspace_id,bucket_start,detector_version,attempts,last_error_code,failed_at,updated_at)
	 VALUES($1,date_trunc('hour',clock_timestamp()),'phase-i-v1',5,'retry_exhausted',clock_timestamp(),clock_timestamp())`, w.ID)
	require.NoError(t, e)
	diagnostics, e := r.AdminWorkspaceDiagnostics(ctx, admin.ID, w.ID)
	require.NoError(t, e)
	var found bool
	for _, worker := range diagnostics.Jobs.Workers {
		if worker.Worker != "finops" {
			continue
		}
		found = true
		require.True(t, worker.Counts["terminal_failed"].Available)
		require.EqualValues(t, 1, *worker.Counts["terminal_failed"].Value)
		require.True(t, worker.Counts["materialized_pending"].Available)
		require.EqualValues(t, 0, *worker.Counts["materialized_pending"].Value)
		require.True(t, worker.Counts["live_leases"].Available)
		require.EqualValues(t, 0, *worker.Counts["live_leases"].Value)
		require.True(t, worker.Counts["expired_leases"].Available)
		require.EqualValues(t, 0, *worker.Counts["expired_leases"].Value)
		require.Equal(t, "retry_exhausted", *worker.LastFailure.Code)
	}
	require.True(t, found, "FinOps diagnostics worker must be present")
}

func TestEnterpriseAdminPostgresSearchExplainAndConcurrentReplay(t *testing.T) {
	ctx, r, owner, w := lifecycleFixture(t)
	admin := enterpriseAdminActor(t)
	_, e := integrationDB.Exec(`INSERT INTO workspaces(name,slug,type,owner_user_id,billing_owner_user_id,created_at) SELECT 'bulk-'||g,'bulk-'||g,'organization',$1,$1,now()-g*interval '1 hour' FROM generate_series(1,12000) g`, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`UPDATE workspaces SET name='exact%_prefix' WHERE id=$1`, w.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`ANALYZE workspaces`)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id,state) SELECT md5('export-fixture-'||g)::uuid,$1,$2,'cancelled' FROM generate_series(1,12000) g`, w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`INSERT INTO workspace_deletion_jobs(id,workspace_id,requested_by_user_id,previous_status,state,available_at,earliest_purge_at) SELECT md5('purge-fixture-'||g)::uuid,$1,$2,'active','cancelled',now()+interval '8 days',now()+interval '8 days' FROM generate_series(1,12000) g`, w.ID, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`ANALYZE workspace_export_jobs`)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`ANALYZE workspace_deletion_jobs`)
	require.NoError(t, e)
	// Empty sparse inventory prefers the existing one-active index; a realistic
	// active estate proves the ordered global attention index avoids a full sort.
	_, e = integrationDB.Exec(`INSERT INTO workspace_deletion_jobs(id,workspace_id,requested_by_user_id,previous_status,state,available_at,earliest_purge_at) SELECT md5('purge-active-'||id)::uuid,id,$1,'active','pending',now()+interval '8 days',now()+interval '8 days' FROM workspaces WHERE slug LIKE 'bulk-%'`, owner.ID)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`ANALYZE workspace_deletion_jobs`)
	require.NoError(t, e)
	page, e := r.AdminSearchWorkspaces(ctx, admin.ID, service.AdminWorkspaceFilter{})
	require.NoError(t, e)
	require.True(t, page.TotalCapped)
	require.EqualValues(t, 10001, page.Total)
	tests := []struct {
		name, query, index string
		args               []any
	}{
		{"literal prefix", `SELECT id FROM workspaces WHERE lower(name) LIKE $1 ESCAPE '\' LIMIT 10001`, "enterprise_admin_workspace_name_prefix", []any{`exact\%\_prefix%`}},
		{"sparse status", `SELECT id FROM workspaces WHERE status='deleted' LIMIT 10001`, "enterprise_admin_workspace_status", nil},
		{"created range", `SELECT id FROM workspaces WHERE created_at>=now()-interval '2 hours' ORDER BY created_at,id LIMIT 20`, "enterprise_admin_workspace_created", nil},
		{"owner exact", `SELECT id FROM workspaces WHERE owner_user_id=$1 ORDER BY id DESC LIMIT 20`, "enterprise_admin_workspace_owner", []any{admin.ID}},
		{"sparse export attention", `SELECT id,created_at FROM workspace_export_jobs WHERE state IN ('pending','running','failed') ORDER BY created_at DESC,id DESC LIMIT 21`, "enterprise_admin_export_attention", nil},
		{"sparse purge attention", `SELECT id,created_at FROM workspace_deletion_jobs WHERE state IN ('pending','running','failed','blocked') ORDER BY created_at DESC,id DESC LIMIT 21`, "enterprise_admin_purge_attention", nil},
	}
	for _, test := range tests {
		rows, e := integrationDB.QueryContext(ctx, `EXPLAIN (ANALYZE,BUFFERS) `+test.query, test.args...)
		require.NoError(t, e)
		plan := ""
		for rows.Next() {
			var line string
			require.NoError(t, rows.Scan(&line))
			plan += line + "\n"
		}
		require.NoError(t, rows.Err())
		require.NoError(t, rows.Close())
		t.Log(test.name + "\n" + plan)
		require.Contains(t, plan, test.index)
	}
	// Replay the actual concurrent runner after losing only its newest receipt.
	_, e = integrationDB.Exec(`UPDATE workspaces SET name='duplicate-build-fixture' WHERE slug LIKE 'bulk-%'`)
	require.NoError(t, e)
	_, e = integrationDB.Exec(`DROP INDEX enterprise_admin_workspace_name_prefix`)
	require.NoError(t, e)
	// A failed concurrent UNIQUE build leaves an invalid same-name index. The
	// migration must recover it before IF NOT EXISTS can silently skip it.
	_, e = integrationDB.Exec(`CREATE UNIQUE INDEX CONCURRENTLY enterprise_admin_workspace_name_prefix ON workspaces(lower(name))`)
	require.Error(t, e)
	_, e = integrationDB.Exec(`DELETE FROM schema_migrations WHERE filename='303_enterprise_admin_diagnostics_indexes_notx.sql'`)
	require.NoError(t, e)
	require.NoError(t, ApplyMigrations(ctx, integrationDB))
	var invalid int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relname LIKE 'enterprise_admin_%' AND NOT i.indisvalid`).Scan(&invalid))
	require.Zero(t, invalid)
	b, e := migrations.FS.ReadFile("302_enterprise_admin_operations.sql")
	require.NoError(t, e)
	_, e = integrationDB.ExecContext(ctx, string(b))
	require.NoError(t, e)
}

func TestEnterpriseAdminPostgresScopedReadAndLiteralPrefix(t *testing.T) {
	ctx, r, owner, a := lifecycleFixture(t)
	admin := mustCreateUser(t, testEntClient(t), &service.User{Email: "diag-" + uuid.NewString() + "@example.com", Role: "admin"})
	b, err := r.CreateOrganization(ctx, owner.ID, "Foreign", "foreign-"+uuid.NewString())
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspaces SET name='diag%_literal' WHERE id=$1`, a.ID)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE workspaces SET name='diagZZliteral' WHERE id=$1`, b.ID)
	require.NoError(t, err)
	_, err = r.AdminDiagnosticsOverview(ctx, owner.ID)
	require.ErrorIs(t, err, service.ErrWorkspaceForbidden)
	page, err := r.AdminSearchWorkspaces(ctx, admin.ID, service.AdminWorkspaceFilter{NamePrefix: "diag%_"})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	require.Equal(t, a.ID, page.Items[0].ID)
	require.Empty(t, page.Items[0].Permissions)
	_, err = integrationDB.Exec(`INSERT INTO workspace_export_jobs(id,workspace_id,requested_by_user_id) VALUES($1,$2,$3)`, uuid.NewString(), b.ID, owner.ID)
	require.NoError(t, err)
	var before int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members WHERE user_id=$1`, admin.ID).Scan(&before))
	d, err := r.AdminWorkspaceDiagnostics(ctx, admin.ID, a.ID)
	require.NoError(t, err)
	require.Empty(t, d.Workspace.Permissions)
	for _, worker := range d.Jobs.Workers {
		for _, j := range worker.Jobs {
			require.Equal(t, a.ID, j.WorkspaceID)
		}
		require.Equal(t, "unknown", worker.Liveness)
	}
	require.NotNil(t, d.Counts["pending_exports"].Value)
	require.EqualValues(t, 0, *d.Counts["pending_exports"].Value)
	var after int
	require.NoError(t, integrationDB.QueryRow(`SELECT count(*) FROM workspace_members WHERE user_id=$1`, admin.ID).Scan(&after))
	require.Equal(t, before, after)
	raw, err := json.Marshal(d)
	require.NoError(t, err)
	for _, secret := range []string{"object_key", "lease_token", "encrypted_client_secret", "token_hash", "response_preview", "metadata"} {
		require.NotContains(t, string(raw), secret)
	}
}
