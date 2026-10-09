import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export interface LifecycleRetentionPolicy {
  category: string
  retention_days: number
  minimum_days: number
  protected: boolean
}

export interface LifecycleBlocker { code: string; reason: string; count: number }

export interface LifecyclePreflight {
  eligible: boolean
  blocking_reasons: LifecycleBlocker[]
  resource_counts: Record<string, number>
  protected_records: Record<string, number>
  counts_capped: boolean
  estimated_purge_scope: string[]
  retained_scope: string[]
  earliest_purge_at: string
  protected_evidence_retained: boolean
}

export interface LifecycleDeletionJob {
  id: string
  workspace_id: number
  state: 'pending' | 'running' | 'blocked' | 'failed' | 'cancelled' | 'completed'
  previous_status: string
  phase: string
  cursor: number
  attempts: number
  progress: number
  failure_code?: string
  blocking_reasons: LifecycleBlocker[]
  earliest_purge_at: string
  created_at: string
  updated_at: string
  completed_at: string | null
  business_closed: boolean
  protected_evidence_retained: boolean
}

export interface LifecycleExportJob {
  id: string
  workspace_id: number
  state: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | 'expired'
  attempts: number
  progress: number
  created_at: string
  updated_at: string
  completed_at: string | null
  expires_at: string | null
  data_cutoff: string | null
  artifact_sha256?: string
  size_bytes: number
  failure_code?: string
}

export interface LifecycleSummary {
  retention: LifecycleRetentionPolicy[]
  deletion: LifecycleDeletionJob | null
  capabilities: { export_enabled: boolean; purge_enabled: boolean }
}

export interface LifecycleChallenge { token: string; expires_at: string; preflight: LifecyclePreflight }
export interface LifecycleDownloadGrant { token: string; expires_at: string }

const base = (workspaceId: number) => `/workspaces/${workspaceId}`
const exportPath = (workspaceId: number, exportId: string) => `${base(workspaceId)}/exports/${encodeURIComponent(exportId)}`
const deletionPath = (workspaceId: number, jobId: string) => `${base(workspaceId)}/deletion/${encodeURIComponent(jobId)}`

export const workspaceLifecycleAPI = {
  getLifecycle: async (workspaceId: number, signal?: AbortSignal): Promise<LifecycleSummary> =>
    (await apiClient.get(`${base(workspaceId)}/lifecycle`, { signal })).data,
  updateRetention: async (workspaceId: number, category: string, retentionDays: number, signal?: AbortSignal): Promise<LifecycleRetentionPolicy[]> =>
    (await apiClient.put(`${base(workspaceId)}/retention/${encodeURIComponent(category)}`, { retention_days: retentionDays }, { signal })).data,
  restoreWorkspace: async (workspaceId: number, signal?: AbortSignal): Promise<void> =>
    (await apiClient.post(`${base(workspaceId)}/restore`, undefined, { signal })).data,
  restoreProject: async (workspaceId: number, projectId: number, signal?: AbortSignal): Promise<void> =>
    (await apiClient.post(`${base(workspaceId)}/projects/${projectId}/restore`, undefined, { signal })).data,
  createExport: async (workspaceId: number, signal?: AbortSignal): Promise<LifecycleExportJob> =>
    (await apiClient.post(`${base(workspaceId)}/exports`, undefined, { signal })).data,
  listExports: async (workspaceId: number, page = 1, pageSize = 10, signal?: AbortSignal): Promise<BasePaginationResponse<LifecycleExportJob>> =>
    (await apiClient.get(`${base(workspaceId)}/exports`, { params: { page, page_size: pageSize }, signal })).data,
  getExport: async (workspaceId: number, exportId: string, signal?: AbortSignal): Promise<LifecycleExportJob> =>
    (await apiClient.get(exportPath(workspaceId, exportId), { signal })).data,
  cancelExport: async (workspaceId: number, exportId: string, signal?: AbortSignal): Promise<void> =>
    (await apiClient.post(`${exportPath(workspaceId, exportId)}/cancel`, undefined, { signal })).data,
  authorizeDownload: async (workspaceId: number, exportId: string, signal?: AbortSignal): Promise<LifecycleDownloadGrant> =>
    (await apiClient.post(`${exportPath(workspaceId, exportId)}/download`, undefined, { signal })).data,
  // The grant is sent in an authenticated POST body. Never put it in a URL.
  redeemDownload: async (workspaceId: number, exportId: string, token: string, signal?: AbortSignal): Promise<Blob> =>
    (await apiClient.post(`${exportPath(workspaceId, exportId)}/download/redeem`, { token }, { signal, responseType: 'blob', decodeJSONBlobErrors: true })).data,
  getDeletionPreflight: async (workspaceId: number, signal?: AbortSignal): Promise<LifecyclePreflight> =>
    (await apiClient.get(`${base(workspaceId)}/deletion/preflight`, { signal })).data,
  createDeletionChallenge: async (workspaceId: number, signal?: AbortSignal): Promise<LifecycleChallenge> =>
    (await apiClient.post(`${base(workspaceId)}/deletion/challenge`, undefined, { signal })).data,
  requestDeletion: async (workspaceId: number, confirmation: { name: string; token: string }, signal?: AbortSignal): Promise<LifecycleDeletionJob> =>
    (await apiClient.post(`${base(workspaceId)}/deletion`, confirmation, { signal })).data,
  getDeletion: async (workspaceId: number, signal?: AbortSignal): Promise<LifecycleDeletionJob | null> =>
    (await apiClient.get(`${base(workspaceId)}/deletion`, { signal })).data,
  cancelDeletion: async (workspaceId: number, jobId: string, signal?: AbortSignal): Promise<void> =>
    (await apiClient.post(`${deletionPath(workspaceId, jobId)}/cancel`, undefined, { signal })).data,
  retryDeletion: async (workspaceId: number, jobId: string, signal?: AbortSignal): Promise<LifecycleDeletionJob> =>
    (await apiClient.post(`${deletionPath(workspaceId, jobId)}/retry`, undefined, { signal })).data,
}
