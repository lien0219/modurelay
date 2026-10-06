package securityaudit

import (
	"context"
	"database/sql/driver"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPromptAuditSnapshotCarriesMachineIdentity(t *testing.T) {
	request := Request{RequestID: "machine", ServiceAccountID: 31, APIKeyID: 9,
		Protocol: "openai_responses", Body: []byte("{\"input\":\"audit this prompt\"}")}
	snapshot, err := ExtractPromptSnapshot(request)
	require.NoError(t, err)
	require.EqualValues(t, 31, snapshot.ServiceAccountID)
	require.Zero(t, snapshot.UserID)
	require.Empty(t, snapshot.UsernameSnapshot)
	require.Empty(t, snapshot.UserEmailSnapshot)
	require.EqualValues(t, 31, snapshot.Redacted().ServiceAccountID)
	require.EqualValues(t, 31, request.Clone().ServiceAccountID)
	require.EqualValues(t, 31, snapshotLogFields(snapshot)["service_account_id"])
}

func TestPromptAuditPersistenceKeepsMachineIdentityAcrossJobsAndEvents(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	now := time.Now().UTC()
	snapshot := PromptSnapshot{RequestID: "machine", ServiceAccountID: 31, APIKeyID: 9, Protocol: "openai_responses"}
	jobArgs := make([]driver.Value, 22)
	for i := range jobArgs {
		jobArgs[i] = sqlmock.AnyArg()
	}
	jobArgs[0], jobArgs[1], jobArgs[2], jobArgs[3], jobArgs[4], jobArgs[5] = "machine", nil, int64(31), "", "", int64(9)
	jobColumns := strings.Split("id,request_id,user_id,service_account_id,username_snapshot,user_email_snapshot,api_key_id,api_key_name_snapshot,group_id,group_name,provider,endpoint,protocol,model,prompt_hash,redacted_preview,prompt_length,message_count,stage,execution_mode,config_version,status,attempts,max_attempts,claim_version,next_attempt_at,processing_started_at,processed_at,last_error_code,last_error_message,created_at,updated_at", ",")
	mock.ExpectQuery("(?s)INSERT INTO prompt_audit_jobs.*request_id,user_id,service_account_id").
		WithArgs(jobArgs...).WillReturnRows(sqlmock.NewRows(jobColumns).AddRow(
		1, "machine", nil, 31, "", "", 9, "", nil, "", "", "", "openai_responses", "", "", "", 0, 0, "http",
		"async_audit", 1, "queued", 0, 3, 0, now, nil, nil, "", "", now, now))
	job, err := insertJob(context.Background(), db, snapshot, ModeAsync, 1, "queued", 3)
	require.NoError(t, err)
	require.EqualValues(t, 31, job.Snapshot.ServiceAccountID)
	require.Zero(t, job.Snapshot.UserID)

	eventArgs := make([]driver.Value, 33)
	for i := range eventArgs {
		eventArgs[i] = sqlmock.AnyArg()
	}
	eventArgs[0], eventArgs[1], eventArgs[2], eventArgs[3], eventArgs[4], eventArgs[5], eventArgs[6] =
		int64(1), "machine", nil, int64(31), "", "", int64(9)
	eventColumns := strings.Split("id,job_id,request_id,user_id,service_account_id,username_snapshot,user_email_snapshot,api_key_id,api_key_name_snapshot,group_id,group_name,provider,endpoint,protocol,model,prompt_hash,redacted_preview,stage,decision,risk_level,action,categories,matched_scanners,scanner_scores,scanner_evidence,scanner_backend,scanner_version,guard_endpoint_id,policy_id,policy_version,config_version,chunk_total,latency_ms,created_at,full_prompt", ",")
	mock.ExpectQuery("(?s)INSERT INTO prompt_audit_events.*job_id,request_id,user_id,service_account_id").
		WithArgs(eventArgs...).WillReturnRows(sqlmock.NewRows(eventColumns).AddRow(
		2, 1, "machine", nil, 31, "", "", 9, "", nil, "", "", "", "openai_responses", "", "", "", "http",
		"flag", "high", "Warn", "[]", "[]", "{}", "{}", "", "", "", "", 0, 1, 1, 10, now, ""))
	event, err := insertEvent(context.Background(), db, job.ID, job.Snapshot, 1,
		&NormalizedResult{Decision: EventFlag, RiskLevel: RiskHigh, Action: ActionWarn, ChunkTotal: 1, LatencyMS: 10})
	require.NoError(t, err)
	require.EqualValues(t, 31, event.Snapshot.ServiceAccountID)
	require.Zero(t, event.Snapshot.UserID)
	require.Empty(t, event.Snapshot.UserEmailSnapshot)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPromptAuditMachineFilterChangesQueryAndDeleteConfirmationHash(t *testing.T) {
	first, second := int64(31), int64(32)
	where, args := buildEventWhere(EventFilter{ServiceAccountID: &first}, 1)
	require.Contains(t, where, "e.service_account_id=$1")
	require.Equal(t, []any{first}, args)
	require.NotEqual(t, FilterHash(EventFilter{ServiceAccountID: &first}, 9), FilterHash(EventFilter{ServiceAccountID: &second}, 9))
}
