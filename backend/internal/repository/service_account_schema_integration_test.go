//go:build integration

package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type saSchemaFixture struct{ workspace, project, payer, account, sa, key int64 }

func serviceAccountSchemaFixture(t *testing.T) saSchemaFixture {
	t.Helper()
	ctx, workspaces, payer, workspace := workspaceFixture(t)
	p, err := workspaces.CreateProject(ctx, payer.ID, workspace.ID, service.ProjectInput{Name: "Machines", Slug: "machines"})
	require.NoError(t, err)
	a := mustCreateAccount(t, testEntClient(t), &service.Account{Name: "sa-schema-" + uuid.NewString()})
	f := saSchemaFixture{workspace: workspace.ID, project: p.ID, payer: payer.ID, account: a.ID}
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO service_accounts(workspace_id,project_id,name,slug,created_by_user_id) VALUES($1,$2,'Backend','backend',$3) RETURNING id`, f.workspace, f.project, f.payer).Scan(&f.sa))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(project_id,service_account_id,key,key_suffix,name) VALUES($1,$2,'sha256:'||encode(sha256($3::bytea),'hex'),'abcd','Machine') RETURNING id`, f.project, f.sa, []byte(uuid.NewString())).Scan(&f.key))
	return f
}

func (f saSchemaFixture) reservation(t *testing.T, request string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := integrationDB.Exec(`INSERT INTO budget_reservations(id,request_id,service_account_id,api_key_id,workspace_id,project_id,billing_principal_user_id,period_start,period_end,project_period_start,project_period_end,estimate,status) VALUES($1,$2,$3,$4,$5,$6,$7,'2026-10-01','2026-11-01','2026-10-01','2026-11-01',1,'pending')`, id, request, f.sa, f.key, f.workspace, f.project, f.payer)
	require.NoError(t, err)
	return id
}

func TestServiceAccountSchemaTenantAndPrincipalConstraints(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	other := serviceAccountSchemaFixture(t)
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO service_accounts(workspace_id,project_id,name,slug) VALUES($1,$2,'Cross','cross')`, []any{f.workspace, other.project}},
		{`INSERT INTO api_keys(project_id,service_account_id,key,key_suffix,name) VALUES($1,$2,'sha256:'||repeat('a',64),'abcd','Cross')`, []any{other.project, f.sa}},
	} {
		_, err := integrationDB.Exec(statement.query, statement.args...)
		require.Error(t, err, "cross-tenant binding rejected")
		var dbError *pq.Error
		require.ErrorAs(t, err, &dbError)
		require.Equal(t, pq.ErrorCode("23503"), dbError.Code)
	}
	_, err := integrationDB.Exec(`INSERT INTO service_accounts(workspace_id,project_id,name,slug) VALUES($1,$2,'Duplicate','backend')`, f.workspace, f.project)
	require.Error(t, err, "slug unique per project")
	for _, query := range []string{
		`UPDATE service_accounts SET workspace_id=$2 WHERE id=$1`,
		`UPDATE service_accounts SET project_id=$2 WHERE id=$1`,
	} {
		_, err = integrationDB.Exec(query, f.sa, other.workspace)
		require.Error(t, err)
	}
	for _, query := range []string{
		`UPDATE api_keys SET service_account_id=$2 WHERE id=$1`,
		`UPDATE api_keys SET user_id=$2 WHERE id=$1`,
		`UPDATE api_keys SET project_id=$2 WHERE id=$1`,
	} {
		_, err = integrationDB.Exec(query, f.key, other.sa)
		require.Error(t, err, "principal binding immutable")
	}
	_, err = integrationDB.Exec(`INSERT INTO api_keys(user_id,project_id,service_account_id,key,name) VALUES($1,$2,$3,'sha256:'||repeat('b',64),'Both')`, f.payer, f.project, f.sa)
	require.Error(t, err, "exclusive user or machine")
	_, err = integrationDB.Exec(`INSERT INTO api_keys(project_id,service_account_id,key,name) VALUES($1,$2,'raw-machine-secret','Raw')`, f.project, f.sa)
	require.Error(t, err, "machine secrets stored only as digest")
}

func TestServiceAccountSchemaMachineBatchReservationIdentity(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	other := serviceAccountSchemaFixture(t)
	batch := "machine-batch-" + uuid.NewString()
	r := f.reservation(t, "batch_image_hold:"+batch)
	query := `INSERT INTO batch_image_jobs(batch_id,user_id,api_key_id,service_account_id,workspace_id,project_id,billing_principal_user_id,budget_reservation_id,provider,model,item_count,estimated_cost) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'gemini_api','image',1,1)`
	for _, bad := range []struct{ payer, key, sa int64 }{
		{other.payer, f.key, f.sa}, {f.payer, other.key, f.sa}, {f.payer, f.key, other.sa},
	} {
		_, err := integrationDB.Exec(query, batch, bad.payer, bad.key, bad.sa, f.workspace, f.project, f.payer, r)
		require.Error(t, err, "machine batch must preserve admitted SA, key and payer")
	}
	_, err := integrationDB.Exec(query, batch, f.payer, f.key, f.sa, f.workspace, f.project, f.payer, r)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE batch_image_jobs SET service_account_id=NULL WHERE batch_id=$1`, batch)
	require.Error(t, err)
	_, err = integrationDB.Exec(`UPDATE service_accounts SET status='disabled' WHERE id=$1`, f.sa)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE batch_image_jobs SET status='settling' WHERE batch_id=$1`, batch)
	require.NoError(t, err, "accepted job survives disable")
}

func TestServiceAccountMigrationsPreservePreexistingRows(t *testing.T) {
	ctx := context.Background()
	container, err := tcpostgres.Run(ctx, selectDockerImage(ctx, postgresImageTag), tcpostgres.WithDatabase("service_account_history"), tcpostgres.WithUsername("postgres"), tcpostgres.WithPassword("postgres"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := openSQLWithRetry(ctx, dsn, 30*time.Second)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	old := fstest.MapFS{}
	names, err := fs.Glob(migrations.FS, "*.sql")
	require.NoError(t, err)
	for _, name := range names {
		if name >= "284" {
			continue
		}
		b, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		old[name] = &fstest.MapFile{Data: b}
	}
	require.NoError(t, applyMigrationsFS(ctx, db, old))
	var user, key, account, usage int64
	require.NoError(t, db.QueryRow(`INSERT INTO users(email,password_hash) VALUES('historic@example.com','hash') RETURNING id`).Scan(&user))
	require.NoError(t, db.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,'sk-historical','Legacy') RETURNING id`, user).Scan(&key))
	require.NoError(t, db.QueryRow(`INSERT INTO accounts(name,platform,type) VALUES('Historical','openai','apikey') RETURNING id`).Scan(&account))
	require.NoError(t, db.QueryRow(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model,actual_cost) VALUES($1,$2,$3,'historic','historic-model',2.5) RETURNING id`, user, key, account).Scan(&usage))
	var beforeKey, beforeUsage string
	require.NoError(t, db.QueryRow(`SELECT to_jsonb(k)::text FROM api_keys k WHERE id=$1`, key).Scan(&beforeKey))
	require.NoError(t, db.QueryRow(`SELECT to_jsonb(u)::text FROM usage_logs u WHERE id=$1`, usage).Scan(&beforeUsage))
	var job int64
	require.NoError(t, db.QueryRow(`INSERT INTO prompt_audit_jobs(user_id,username_snapshot,user_email_snapshot) VALUES($1,'historic','historic@example.com') RETURNING id`, user).Scan(&job))
	for _, query := range []string{
		`INSERT INTO content_moderation_logs(user_id,user_email) VALUES($1,'historic@example.com')`,
		`INSERT INTO prompt_audit_events(user_id,job_id,username_snapshot,user_email_snapshot) VALUES($1,$2,'historic','historic@example.com')`,
		`INSERT INTO ops_error_logs(user_id,error_phase,error_type) VALUES($1,'upstream','http')`,
		`INSERT INTO ops_system_logs(user_id,level,message) VALUES($1,'warn','historic')`,
	} {
		args := []any{user}
		if strings.Contains(query, "job_id") {
			args = append(args, job)
		}
		_, err := db.Exec(query, args...)
		require.NoError(t, err)
	}
	auditBefore := map[string]string{}
	for _, table := range []string{"content_moderation_logs", "prompt_audit_jobs", "prompt_audit_events", "ops_error_logs", "ops_system_logs"} {
		var snapshot string
		require.NoError(t, db.QueryRow(`SELECT to_jsonb(l)::text FROM `+table+` l WHERE user_id=$1`, user).Scan(&snapshot))
		auditBefore[table] = snapshot
	}
	// Fault injection: a failed concurrent build leaves an invalid catalog entry
	// with the next migration's index name. The runner must replace it.
	_, err = db.Exec(`INSERT INTO ops_system_logs(user_id,level,message) VALUES($1,'warn','second')`, user)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE UNIQUE INDEX CONCURRENTLY idx_ops_system_logs_service_account_created ON ops_system_logs(user_id)`)
	require.Error(t, err)
	var interruptedValid bool
	require.NoError(t, db.QueryRow(`SELECT indisvalid FROM pg_index WHERE indexrelid='idx_ops_system_logs_service_account_created'::regclass`).Scan(&interruptedValid))
	require.False(t, interruptedValid)
	require.NoError(t, ApplyMigrations(ctx, db))
	require.NoError(t, ApplyMigrations(ctx, db), "runner reapply preserves checksums and rows")
	var afterKey, afterUsage string
	require.NoError(t, db.QueryRow(`SELECT (to_jsonb(k)-'service_account_id'-'key_suffix')::text FROM api_keys k WHERE id=$1`, key).Scan(&afterKey))
	require.NoError(t, db.QueryRow(`SELECT (to_jsonb(u)-'service_account_id')::text FROM usage_logs u WHERE id=$1`, usage).Scan(&afterUsage))
	require.Equal(t, beforeKey, afterKey)
	require.Equal(t, beforeUsage, afterUsage)
	for table, before := range auditBefore {
		var after string
		require.NoError(t, db.QueryRow(`SELECT (to_jsonb(l)-'service_account_id')::text FROM `+table+` l WHERE user_id=$1 ORDER BY id LIMIT 1`, user).Scan(&after))
		require.Equal(t, before, after, table+" historical human evidence must be unchanged")
	}
	var count int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM usage_service_account_hourly_rollups`).Scan(&count))
	require.Zero(t, count)
	for _, table := range []string{"api_keys", "usage_logs", "budget_reservations", "batch_image_jobs", "content_moderation_logs", "prompt_audit_jobs", "prompt_audit_events", "ops_error_logs", "ops_system_logs"} {
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM `+table+` WHERE service_account_id IS NOT NULL`).Scan(&count))
		require.Zero(t, count)
	}
	var valid bool
	require.NoError(t, db.QueryRow(`SELECT bool_and(indisvalid) FROM pg_index WHERE indexrelid::regclass::text = ANY($1)`, pq.Array(strings.Fields("api_keys_service_account_id usage_logs_service_account_created budget_reservations_service_account_pending batch_image_jobs_service_account_created"))).Scan(&valid))
	require.True(t, valid)
	indexes := strings.Fields("idx_content_moderation_logs_service_account_created idx_prompt_audit_jobs_service_account_created idx_prompt_audit_events_service_account_created idx_ops_error_logs_service_account_created idx_ops_system_logs_service_account_created")
	var indexCount int
	require.NoError(t, db.QueryRow(`SELECT count(*),bool_and(indisvalid AND NOT indisunique) FROM pg_index WHERE indexrelid::regclass::text = ANY($1)`, pq.Array(indexes)).Scan(&indexCount, &valid))
	require.Equal(t, len(indexes), indexCount)
	require.True(t, valid, "audit indexes, including the interrupted one, are valid nonunique indexes")
}

func TestServiceAccountSchemaAuditMachineEvidence(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	t.Cleanup(func() {
		for _, table := range []string{"prompt_audit_events", "prompt_audit_jobs", "content_moderation_logs", "ops_error_logs", "ops_system_logs"} {
			_, err := integrationDB.Exec(`DELETE FROM `+table+` WHERE service_account_id=$1`, f.sa)
			require.NoError(t, err)
		}
	})
	var job int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO prompt_audit_jobs(service_account_id,api_key_id) VALUES($1,$2) RETURNING id`, f.sa, f.key).Scan(&job))
	for _, table := range []string{"content_moderation_logs", "prompt_audit_jobs", "prompt_audit_events", "ops_error_logs", "ops_system_logs"} {
		t.Run(table, func(t *testing.T) {
			columns, values := "service_account_id,api_key_id", "$1,$2"
			args := []any{f.sa, f.key}
			switch table {
			case "prompt_audit_events":
				columns += ",job_id"
				values += ",$3"
				args = append(args, job)
			case "ops_error_logs":
				columns += ",error_phase,error_type"
				values += ",'upstream','http'"
			case "ops_system_logs":
				columns += ",level,message"
				values += ",'warn','machine'"
			}
			insert := `INSERT INTO ` + table + `(` + columns + `) VALUES(` + values + `) RETURNING id`
			var id int64
			require.NoError(t, integrationDB.QueryRow(insert, args...).Scan(&id))
			_, err := integrationDB.Exec(`UPDATE `+table+` SET user_id=$2 WHERE id=$1`, id, f.payer)
			require.Error(t, err, "machine evidence cannot carry a human actor")
			var dbError *pq.Error
			require.ErrorAs(t, err, &dbError)
			require.Equal(t, pq.ErrorCode("23514"), dbError.Code)
			for _, field := range map[string][]string{
				"content_moderation_logs": {"user_email"},
				"prompt_audit_jobs":       {"username_snapshot", "user_email_snapshot"},
				"prompt_audit_events":     {"username_snapshot", "user_email_snapshot"},
			}[table] {
				_, err = integrationDB.Exec(`UPDATE `+table+` SET `+field+`='creator' WHERE id=$1`, id)
				require.Error(t, err, "creator snapshots forbidden on machine evidence")
			}
			_, err = integrationDB.Exec(`UPDATE `+table+` SET service_account_id=9223372036854775807 WHERE id=$1`, id)
			require.Error(t, err, "NOT VALID FK still enforces new writes")
			_, err = integrationDB.Exec(`UPDATE service_accounts SET status='disabled' WHERE id=$1`, f.sa)
			require.NoError(t, err)
			var actor sql.NullInt64
			var snapshot int64
			require.NoError(t, integrationDB.QueryRow(`SELECT user_id,service_account_id FROM `+table+` WHERE id=$1`, id).Scan(&actor, &snapshot))
			require.False(t, actor.Valid)
			require.Equal(t, f.sa, snapshot, "disabling an identity retains historical evidence")
		})
	}
}

func TestServiceAccountSchemaUsageIdentityAndRollupParity(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	other := serviceAccountSchemaFixture(t)
	r := f.reservation(t, "machine-request")
	insert := `INSERT INTO usage_logs(user_id,service_account_id,api_key_id,account_id,request_id,model,workspace_id,project_id,billing_principal_user_id,budget_reservation_id,resolved_platform,actual_cost,input_tokens,output_tokens,created_at) VALUES(NULL,$1,$2,$3,$4,'model',$5,$6,$7,$8,'openai',1.25,10,20,'2026-10-05 10:12:00Z') RETURNING id`
	for _, bad := range []struct {
		sa, key, payer int64
		reservation    uuid.UUID
	}{
		{other.sa, f.key, f.payer, r}, {f.sa, other.key, f.payer, r}, {f.sa, f.key, other.payer, r}, {f.sa, f.key, f.payer, uuid.New()},
	} {
		var id int64
		err := integrationDB.QueryRow(insert, bad.sa, bad.key, f.account, uuid.NewString(), f.workspace, f.project, bad.payer, bad.reservation).Scan(&id)
		require.Error(t, err, "NULL actor must not bypass full reservation identity")
	}
	var id int64
	require.NoError(t, integrationDB.QueryRow(insert, f.sa, f.key, f.account, "machine-request", f.workspace, f.project, f.payer, r).Scan(&id))
	for _, column := range []string{"service_account_id", "user_id"} {
		_, err := integrationDB.Exec(`UPDATE usage_logs SET `+column+`=$2 WHERE id=$1`, id, other.sa)
		require.Error(t, err, "machine snapshot immutable")
	}
	_, err := integrationDB.Exec(`UPDATE budget_reservations SET service_account_id=$2 WHERE id=$1`, r, other.sa)
	require.Error(t, err, "budget machine snapshot immutable")
	assert := func(requests int64, cost float64, input, output int64) {
		t.Helper()
		var gotRequests, gotInput, gotOutput int64
		var gotCost float64
		require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(SUM(request_count),0),COALESCE(SUM(actual_cost),0),COALESCE(SUM(input_tokens),0),COALESCE(SUM(output_tokens),0) FROM usage_service_account_hourly_rollups WHERE service_account_id=$1`, f.sa).Scan(&gotRequests, &gotCost, &gotInput, &gotOutput))
		require.Equal(t, requests, gotRequests)
		require.Equal(t, cost, gotCost)
		require.Equal(t, input, gotInput)
		require.Equal(t, output, gotOutput)
		var tenantCount int64
		var tenantCost float64
		require.NoError(t, integrationDB.QueryRow(`SELECT COALESCE(SUM(request_count),0),COALESCE(SUM(actual_cost),0) FROM usage_tenant_hourly_rollups WHERE workspace_id=$1`, f.workspace).Scan(&tenantCount, &tenantCost))
		require.Equal(t, requests, tenantCount)
		require.Equal(t, cost, tenantCost)
	}
	assert(1, 1.25, 10, 20)
	tx := testTx(t)
	_, err = tx.Exec(`UPDATE usage_logs SET actual_cost=2,input_tokens=30 WHERE id=$1`, id)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	assert(1, 1.25, 10, 20)
	_, err = integrationDB.Exec(`UPDATE usage_logs SET actual_cost=2,input_tokens=30,output_tokens=40,model='corrected' WHERE id=$1`, id)
	require.NoError(t, err)
	assert(1, 2, 30, 40)
	_, err = integrationDB.Exec(`UPDATE service_accounts SET status='disabled',disabled_at=now() WHERE id=$1`, f.sa)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`UPDATE budget_reservations SET status='finalized',actual=2,finalized_at=now() WHERE id=$1`, r)
	require.NoError(t, err)
	_, err = integrationDB.Exec(`DELETE FROM usage_logs WHERE id=$1`, id)
	require.Error(t, err, "machine Usage must retain its original financial evidence")
	assert(1, 2, 30, 40)
}

func TestServiceAccountSchemaHistoricalSnapshotsAndCreatorIndependence(t *testing.T) {
	isolateWorkspaceTestFixtures(t)
	f := serviceAccountSchemaFixture(t)
	var key, usage int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO api_keys(user_id,key,name) VALUES($1,$2,'Legacy') RETURNING id`, f.payer, "legacy-"+uuid.NewString()).Scan(&key))
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO usage_logs(user_id,api_key_id,account_id,request_id,model) VALUES($1,$2,$3,$4,'legacy') RETURNING id`, f.payer, key, f.account, uuid.NewString()).Scan(&usage))
	var sa sql.NullInt64
	require.NoError(t, integrationDB.QueryRow(`SELECT service_account_id FROM usage_logs WHERE id=$1`, usage).Scan(&sa))
	require.False(t, sa.Valid)
	_, err := integrationDB.Exec(`UPDATE usage_logs SET service_account_id=$2 WHERE id=$1`, usage, f.sa)
	require.Error(t, err, "historical NULL machine attribution must never be backfilled")
	_, err = integrationDB.Exec(`UPDATE api_keys SET service_account_id=$2,user_id=NULL,project_id=$3,key='sha256:'||repeat('c',64) WHERE id=$1`, key, f.sa, f.project)
	require.Error(t, err, "historical key cannot become a machine")
	var creator int64
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users(email,password_hash,role,status) VALUES($1,'fixture','user','active') RETURNING id`, "sa-creator-"+uuid.NewString()+"@example.com").Scan(&creator))
	_, err = integrationDB.Exec(`UPDATE service_accounts SET created_by_user_id=$2 WHERE id=$1`, f.sa, creator)
	require.NoError(t, err)
	deletePersonalWorkspaceFixture(t, creator)
	_, err = integrationDB.Exec(`DELETE FROM users WHERE id=$1`, creator)
	require.NoError(t, err)
	require.NoError(t, integrationDB.QueryRow(`SELECT created_by_user_id FROM service_accounts WHERE id=$1`, f.sa).Scan(&sa))
	require.False(t, sa.Valid)
	var status string
	require.NoError(t, integrationDB.QueryRow(`SELECT status FROM service_accounts WHERE id=$1`, f.sa).Scan(&status))
	require.Equal(t, "active", status)
}
