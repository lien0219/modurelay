import type { AxiosRequestConfig } from 'axios'
import { apiClient } from './client'
import type { Workspace } from './workspace'

export interface AdminWorkspaceFilter {
  workspace_id?: number; owner_user_id?: number; billing_owner_user_id?: number
  name_prefix?: string; slug_prefix?: string; status?: string; type?: string
  created_from?: string; created_to?: string; updated_from?: string; updated_to?: string
  sort?: 'id' | 'name' | 'slug' | 'created_at' | 'updated_at'; direction?: 'asc' | 'desc'
  page?: number; page_size?: number; signal?: AbortSignal
}
export interface AdminWorkspacePage { items: Workspace[]; total: number; page: number; page_size: number; pages: number; total_capped: boolean; observed_at: string }
export type AdminAction = 'suspend' | 'resume' | 'retry_webhook'
export interface AdminOperationInput {
  action: AdminAction; status?: 'active' | 'suspended'; reason: string; confirmation: string; idempotency_key: string
  expected_updated_at?: string; expected_attempts?: number; expected_last_attempt_at?: string
}
export type AdminSensitiveConfig = Pick<AxiosRequestConfig, 'signal' | 'preserveAuthSessionOnFailure' | 'sessionProofAccessToken'>
export interface AdminOperationReceipt { id: string; action: AdminAction; target_type: string; target_id: number; workspace_id: number; previous_status: string; result_status: string; result_updated_at: string; created_at: string }
export interface AdminCount { value: number | null; available: boolean; capped: boolean; observed_at: string; source: string; coverage: string; freshness: string }
export interface AdminIssue { code: string; severity: string; scope: string; observed_at: string; reason: string; recommended_action: string; runbook: string }
export interface AdminTimeEvidence { value: string | null; age_seconds: number | null; available: boolean; observed_at: string; source: string; coverage: string; freshness: string }
export interface AdminFailureEvidence { code: string | null; source_time: string | null; available: boolean; observed_at: string; source: string; coverage: string; freshness: string }
export interface AdminBlocker { code: string; reason: string; count?: AdminCount; observed_at: string; source_time: string | null; source: string; coverage: string; freshness: string; runbook: string }
export interface AdminPurgeDiagnostics { state: string; available: boolean; observed_at: string; coverage: string; blockers: AdminBlocker[]; issues: AdminIssue[] }
export interface AdminIdentityConnection { id: number; workspace_id: number; type: string; status: string; state: string; observed_at: string; source: string; freshness: string; last_validated_at: string | null; last_validation_code: string; last_sync_at: string | null; last_error_code: string; consecutive_failures: number | null; coverage: string; issues: AdminIssue[] }
export interface AdminIdentitySection { state: string; available: boolean; capped: boolean; observed_at: string; source: string; coverage: string; freshness: string; items: AdminIdentityConnection[]; counts: Record<string, AdminCount>; issues: AdminIssue[] }
export interface AdminIdentityDiagnostics { providers: AdminIdentitySection; scim: AdminIdentitySection }
export interface AdminJobSummary { id: string; workspace_id: number; state: string; created_at: string; finished_at: string | null; failed_at: string | null; failure_code: string; webhook_id?: number; delivery_id?: number; attempts?: number; last_attempt_at?: string | null; endpoint_enabled?: boolean; retry_eligibility?: string; phase?: string; progress?: number | null; blockers?: AdminBlocker[]; source_time?: string | null; observed_at: string; freshness: string }
export interface AdminWorkerDiagnostics {
  worker: string; state: string; liveness: string; observed_at: string; source: string; coverage: string; freshness: string; counts: Record<string, AdminCount>
  due_lag_seconds: number | null; stored_scan_lag_seconds?: number | null; oldest_pending: AdminTimeEvidence; pending_alert_age: AdminTimeEvidence
  last_failure: AdminFailureEvidence; latest_failure_time: AdminTimeEvidence; last_successful_job_at: string | null; last_successful_scan_at: string | null; last_failed_at: string | null
  jobs: AdminJobSummary[]; jobs_capped: boolean; issues: AdminIssue[]; runbook: string
}
export interface AdminExportRotation {
  state: string; observed_at: string; rotation_certified: boolean; format: string; single_key_no_id: boolean
  capabilities: { available: boolean; scope: string; export_enabled: boolean; purge_enabled: boolean; key_available: boolean }
  counts: Record<string, AdminCount>; coverage: string; prerequisites: string[]; issues: AdminIssue[]; runbook: string
}
export interface AdminJobsDiagnostics { state: string; observed_at: string; workers: AdminWorkerDiagnostics[]; webhook_failure_trend: { hour: string; count: AdminCount }[]; export_rotation: AdminExportRotation; issues: AdminIssue[] }
export interface AdminDiagnosticsOverview { state: string; observed_at: string; counts: Record<string, AdminCount>; jobs: AdminJobsDiagnostics | null; issues: AdminIssue[]; identity: AdminIdentityDiagnostics; worker_exceptions: AdminCount; unknown_worker_liveness: AdminCount }
export interface AdminSecuritySummary { require_sso: boolean; require_mfa: boolean; session_max_age_seconds: number | null; invitation_policy: string; allow_external_members: boolean; workspace_jit_enabled: boolean; approved_identity_provider_mode: string }
export interface AdminWorkspaceDiagnostics {
  state: string; observed_at: string; workspace: Workspace | null; counts: Record<string, AdminCount>; billing_owner_valid: boolean | null; owner_valid: boolean | null
  allowed_operations: AdminAction[]; identity: AdminIdentityDiagnostics; purge: AdminPurgeDiagnostics; security_policy: AdminSecuritySummary | null
  retention: { category: string; minimum_days: number; effective_days: number; protected: boolean }[]
  recent_audit: { id: number; action: string; target_type: string; target_id: number | null; created_at: string }[]
  jobs: AdminJobsDiagnostics | null; issues: AdminIssue[]
}

export const enterpriseAdminAPI = {
  overview: async (signal?: AbortSignal) => (await apiClient.get<AdminDiagnosticsOverview>('/admin/workspaces/diagnostics/overview', { signal })).data,
  diagnostics: async (id: number, signal?: AbortSignal) => (await apiClient.get<AdminWorkspaceDiagnostics>(`/admin/workspaces/${id}/diagnostics`, { signal })).data,
  jobs: async (signal?: AbortSignal) => (await apiClient.get<AdminJobsDiagnostics>('/admin/operations/jobs', { signal })).data,
  retryWebhook: async (workspaceId: number, webhookId: number, deliveryId: number, body: AdminOperationInput, config: AdminSensitiveConfig) => (await apiClient.post<AdminOperationReceipt>(`/admin/workspaces/${workspaceId}/webhooks/${webhookId}/deliveries/${deliveryId}/retry`, body, config)).data,
}
