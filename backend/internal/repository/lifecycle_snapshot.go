package repository

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/shopspring/decimal"
)

type lifecycleSection struct {
	name, query string
	columns     []string
	amount      string
}

// These queries are a closed field allowlist. In particular, no global user
// profile, credential value, request body or arbitrary audit metadata is read.
var lifecycleSections = []lifecycleSection{
	{name: "workspace.json", query: `SELECT id,jsonb_build_object('id',id,'type',type,'name',name,'slug',slug,'status',status,'billing_owner_user_id',billing_owner_user_id,'created_at',created_at) FROM workspaces WHERE id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "projects.json", query: `SELECT id,jsonb_build_object('id',id,'workspace_id',workspace_id,'name',name,'slug',slug,'status',status,'created_at',created_at) FROM projects WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "members.json", query: `SELECT id,jsonb_build_object('id',id,'user_id',user_id,'role',role,'status',status) FROM workspace_members WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "credentials.json", query: `SELECT k.id,jsonb_build_object('id',k.id,'project_id',k.project_id,'name',k.name,'status',k.status,'service_account_id',k.service_account_id,'created_at',k.created_at,'expires_at',k.expires_at) FROM api_keys k JOIN projects p ON p.id=k.project_id WHERE p.workspace_id=$1 AND k.id>$2 ORDER BY k.id LIMIT $3`},
	{name: "service_accounts.json", query: `SELECT id,jsonb_build_object('id',id,'project_id',project_id,'name',name,'status',status,'created_at',created_at) FROM service_accounts WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "usage.csv", query: `SELECT id,jsonb_build_object('id',id,'workspace_id',workspace_id,'project_id',project_id,'api_key_id',api_key_id,'billing_principal_user_id',billing_principal_user_id,'model',model,'resolved_platform',resolved_platform,'actual_cost',actual_cost::text,'total_cost',total_cost::text,'created_at',created_at) FROM usage_logs WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`, columns: []string{"id", "workspace_id", "project_id", "api_key_id", "billing_principal_user_id", "model", "resolved_platform", "actual_cost", "total_cost", "created_at"}, amount: "actual_cost"},
	{name: "finops.csv", query: `SELECT usage_log_id,jsonb_build_object('usage_log_id',usage_log_id,'project_id',project_id,'cost_center_id',cost_center_id,'environment',environment,'allocation_source',allocation_source,'policy_revision',policy_revision,'allocation_tags',allocation_tags,'actual_cost',actual_cost::text,'created_at',created_at) FROM usage_allocation_snapshots WHERE workspace_id=$1 AND usage_log_id>$2 ORDER BY usage_log_id LIMIT $3`, columns: []string{"usage_log_id", "project_id", "cost_center_id", "environment", "allocation_source", "policy_revision", "allocation_tags", "actual_cost", "created_at"}, amount: "actual_cost"},
	{name: "cost_allocations.json", query: `SELECT id,jsonb_build_object('id',id,'code',code,'name',name,'status',status) FROM workspace_cost_centers WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "allocation_tags.json", query: `SELECT id,jsonb_build_object('id',id,'tag_key',tag_key,'tag_value',tag_value,'status',status) FROM workspace_allocation_tags WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "project_allocations.json", query: `SELECT project_id,jsonb_build_object('project_id',project_id,'cost_center_id',cost_center_id,'environment',environment,'policy_revision',policy_revision) FROM project_cost_allocations WHERE workspace_id=$1 AND project_id>$2 ORDER BY project_id LIMIT $3`},
	{name: "anomalies.json", query: `SELECT id,jsonb_build_object('id',id,'project_id',project_id,'detector_type',detector_type,'severity',severity,'window_start',window_start,'window_end',window_end,'observed_spend',observed_spend::text,'expected_spend',expected_spend::text,'fingerprint',fingerprint) FROM finops_anomaly_snapshots WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "audit.json", query: `SELECT id,jsonb_build_object('id',id,'actor_user_id',actor_user_id,'action',action,'target_type',target_type,'target_id',target_id,'created_at',created_at) FROM workspace_audit_logs WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
	{name: "identity.json", query: `SELECT id,jsonb_build_object('id',id,'protocol',type,'status',status,'created_at',created_at) FROM workspace_identity_providers WHERE workspace_id=$1 AND id>$2 ORDER BY id LIMIT $3`},
}

func lifecycleCSVValue(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) != nil {
		s = string(raw)
	}
	// Spreadsheet tools may treat whitespace-prefixed formulae as executable.
	first := strings.TrimLeft(s, " \t\r\n")
	if first != "" && strings.ContainsRune("=+-@", rune(first[0])) {
		return "'" + s
	}
	if strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}

func (r *workspaceRepository) lifecycleExportProgress(ctx context.Context, job *service.LifecycleExportJob, n int64) error {
	result, err := r.db.ExecContext(ctx, `UPDATE workspace_export_jobs SET progress=$4,updated_at=now() WHERE workspace_id=$1 AND id=$2 AND state='running' AND lease_token=$3 AND lease_expires_at>clock_timestamp()`, job.WorkspaceID, job.ID, job.LeaseToken, n)
	return requireLifecycleAffected(result, err)
}

func (r *workspaceRepository) WriteLifecycleExportSnapshot(ctx context.Context, job *service.LifecycleExportJob, dst io.Writer) (*service.LifecycleExportResult, error) {
	if job == nil || dst == nil || job.LeaseToken == "" {
		return nil, service.ErrWorkspaceInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := r.lifecycleExportProgress(ctx, job, 0); err != nil {
		return nil, err
	}
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SET LOCAL statement_timeout='20s'; SET LOCAL idle_in_transaction_session_timeout='30s'`); err != nil {
		return nil, err
	}
	var cutoff time.Time
	var snapshot string
	if err = tx.QueryRowContext(ctx, `SELECT transaction_timestamp(),pg_current_snapshot()::text`).Scan(&cutoff, &snapshot); err != nil {
		return nil, err
	}
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	result := &service.LifecycleExportResult{DataCutoff: cutoff}
	counts := map[string]int64{}
	checksums := map[string]string{}
	totals := map[string]string{}
	names := []string{}
	var inputBytes int64
	writeSection := func(name string, data []byte) error {
		inputBytes += int64(len(data))
		if inputBytes > service.LifecycleMaxArtifactBytes {
			return service.ErrLifecycleExportLimit
		}
		sum := sha256.Sum256(data)
		checksums[name] = hex.EncodeToString(sum[:])
		names = append(names, name)
		w, e := zw.Create(name)
		if e != nil {
			return e
		}
		_, e = w.Write(data)
		return e
	}
	for _, section := range lifecycleSections {
		var data bytes.Buffer
		var cw *csv.Writer
		if len(section.columns) > 0 {
			cw = csv.NewWriter(&data)
			if err = cw.Write(section.columns); err != nil {
				return nil, err
			}
		} else {
			_ = data.WriteByte('[')
		}
		var cursor, n int64
		amount := decimal.Zero
		for {
			if err = r.lifecycleExportProgress(ctx, job, result.Records); err != nil {
				return nil, err
			}
			rows, e := tx.QueryContext(ctx, section.query, job.WorkspaceID, cursor, 500)
			if e != nil {
				return nil, e
			}
			page := int64(0)
			for rows.Next() {
				var raw []byte
				if e = rows.Scan(&cursor, &raw); e != nil {
					_ = rows.Close()
					return nil, e
				}
				page++
				n++
				result.Records++
				if result.Records > 100000 || inputBytes+int64(data.Len()+len(raw)) > service.LifecycleMaxArtifactBytes {
					_ = rows.Close()
					return nil, service.ErrLifecycleExportLimit
				}
				if cw != nil {
					fields := map[string]json.RawMessage{}
					if e = json.Unmarshal(raw, &fields); e != nil {
						_ = rows.Close()
						return nil, e
					}
					record := make([]string, len(section.columns))
					for i, key := range section.columns {
						record[i] = lifecycleCSVValue(fields[key])
					}
					if section.amount != "" {
						var value string
						if e = json.Unmarshal(fields[section.amount], &value); e != nil {
							_ = rows.Close()
							return nil, e
						}
						d, x := decimal.NewFromString(value)
						if x != nil {
							_ = rows.Close()
							return nil, x
						}
						amount = amount.Add(d)
					}
					if e = cw.Write(record); e != nil {
						_ = rows.Close()
						return nil, e
					}
					cw.Flush()
					if e = cw.Error(); e != nil {
						_ = rows.Close()
						return nil, e
					}
				} else {
					if n > 1 {
						_ = data.WriteByte(',')
					}
					_, _ = data.Write(raw)
				}
			}
			e = rows.Err()
			_ = rows.Close()
			if e != nil {
				return nil, e
			}
			if page < 500 {
				break
			}
		}
		if cw == nil {
			_ = data.WriteByte(']')
		}
		counts[section.name] = n
		if section.amount != "" {
			totals[section.name+":"+section.amount] = amount.String()
		}
		if err = writeSection(section.name, data.Bytes()); err != nil {
			return nil, err
		}
	}
	// UUID-keyed reservations use their own stable keyset, rather than an OFFSET
	// scan or lossy numeric coercion. The snapshot includes frozen allocations.
	var reservationData bytes.Buffer
	_ = reservationData.WriteByte('[')
	cursor := "00000000-0000-0000-0000-000000000000"
	n := int64(0)
	total := decimal.Zero
	for {
		if err = r.lifecycleExportProgress(ctx, job, result.Records); err != nil {
			return nil, err
		}
		rows, e := tx.QueryContext(ctx, `SELECT r.id::text,jsonb_build_object('id',r.id,'project_id',r.project_id,'api_key_id',r.api_key_id,'billing_principal_user_id',r.billing_principal_user_id,'status',r.status,'estimate',r.estimate::text,'actual',r.actual::text,'created_at',r.created_at,'finalized_at',r.finalized_at,'allocation',CASE WHEN s.reservation_id IS NULL THEN NULL ELSE jsonb_build_object('cost_center_id',s.cost_center_id,'environment',s.environment,'allocation_tags',s.allocation_tags,'policy_revision',s.policy_revision,'allocation_source',s.allocation_source) END),r.actual::text FROM budget_reservations r LEFT JOIN budget_reservation_allocation_snapshots s ON s.reservation_id=r.id AND s.workspace_id=r.workspace_id WHERE r.workspace_id=$1 AND r.id>$2::uuid ORDER BY r.id LIMIT 500`, job.WorkspaceID, cursor)
		if e != nil {
			return nil, e
		}
		page := 0
		for rows.Next() {
			var raw []byte
			var actual string
			if e = rows.Scan(&cursor, &raw, &actual); e != nil {
				_ = rows.Close()
				return nil, e
			}
			page++
			n++
			result.Records++
			if result.Records > 100000 || inputBytes+int64(reservationData.Len()+len(raw)) > service.LifecycleMaxArtifactBytes {
				_ = rows.Close()
				return nil, service.ErrLifecycleExportLimit
			}
			d, x := decimal.NewFromString(actual)
			if x != nil {
				_ = rows.Close()
				return nil, x
			}
			total = total.Add(d)
			if n > 1 {
				_ = reservationData.WriteByte(',')
			}
			_, _ = reservationData.Write(raw)
		}
		e = rows.Err()
		_ = rows.Close()
		if e != nil {
			return nil, e
		}
		if page < 500 {
			break
		}
	}
	_ = reservationData.WriteByte(']')
	counts["reservations.json"] = n
	totals["reservations.json:actual"] = total.String()
	if err = writeSection("reservations.json", reservationData.Bytes()); err != nil {
		return nil, err
	}
	manifest, err := json.Marshal(map[string]any{"schema_version": 1, "workspace_id": job.WorkspaceID, "export_id": job.ID, "export_time": time.Now().UTC(), "data_cutoff": cutoff, "consistency": "read-only PostgreSQL REPEATABLE READ snapshot; all sections share one MVCC visibility boundary; concurrent commits after the snapshot are excluded", "snapshot": snapshot, "included_sections": names, "record_counts": counts, "checksums": checksums, "amount_totals": totals, "record_total": result.Records, "exclusions": []string{"credential_values", "global_user_profiles_wallets_and_canvas", "upstream_accounts", "request_response_bodies", "arbitrary_audit_metadata", "global_payment_and_refund_records_without_durable_workspace_ownership"}})
	if err != nil {
		return nil, err
	}
	if err = writeSection("manifest.json", manifest); err != nil {
		return nil, err
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}
	if archive.Len() > int(service.LifecycleMaxArtifactBytes) {
		return nil, service.ErrLifecycleExportLimit
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if err = r.lifecycleExportProgress(ctx, job, result.Records); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(archive.Bytes())
	result.SHA256 = hex.EncodeToString(sum[:])
	result.SizeBytes = int64(archive.Len())
	written, err := dst.Write(archive.Bytes())
	if err != nil {
		return nil, err
	}
	if written != archive.Len() {
		return nil, io.ErrShortWrite
	}
	return result, nil
}
