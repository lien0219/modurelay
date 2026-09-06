/**
 * System API endpoints for admin operations
 */

import { apiClient } from '../client'
import { extractSemanticVersion } from '@/utils/version'

export interface ReleaseInfo {
  name: string
  body: string
  published_at: string
  html_url: string
}

export interface VersionInfo {
  current_version: string
  latest_version: string
  has_update: boolean
  release_info?: ReleaseInfo
  cached: boolean
  warning?: string
  build_type: string // "source" for manual builds, "release" for CI builds
  deployment_mode?: 'source' | 'binary' | 'docker'
  target_image?: string
  deploy_command?: string
}

/**
 * Get current version
 */
export async function getVersion(): Promise<{ version: string }> {
  const { data } = await apiClient.get<{ version: string }>('/admin/system/version')
  return {
    ...data,
    version: extractSemanticVersion(data.version)
  }
}

/**
 * Check for updates
 * @param force - Force refresh from GitHub API
 */
export async function checkUpdates(force = false): Promise<VersionInfo> {
  const { data } = await apiClient.get<VersionInfo>('/admin/system/check-updates', {
    params: force ? { force: 'true' } : undefined
  })
  return {
    ...data,
    current_version: extractSemanticVersion(data.current_version),
    latest_version: extractSemanticVersion(data.latest_version)
  }
}

export interface UpdateResult {
  message: string
  need_restart: boolean
}

export interface RollbackVersionInfo {
  version: string
  published_at?: string
  html_url?: string
  image?: string
  deploy_command?: string
  method?: 'binary' | 'host_command'
}

/**
 * Get versions available for rollback (up to 3 versions older than current)
 */
export async function getRollbackVersions(): Promise<{ versions: RollbackVersionInfo[] }> {
  const { data } = await apiClient.get<{ versions: RollbackVersionInfo[] }>(
    '/admin/system/rollback-versions'
  )
  return data
}

/**
 * In-place update/rollback downloads a full release binary from GitHub, which
 * can take several minutes on slow links. The global 30s axios timeout would
 * abort the request mid-download (#4504), so these calls wait as long as the
 * backend allows (15 minutes server-side).
 */
const UPDATE_REQUEST_TIMEOUT_MS = 15 * 60 * 1000

export function createSystemOperationKey(
  operation: 'update' | 'rollback' | 'restart',
  target?: string
): string {
  const requestID =
    globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
  const targetPart = target ? `-${target.replace(/[^0-9A-Za-z._-]/g, '')}` : ''
  return `system-${operation}${targetPart}-${requestID}`
}

/**
 * Perform system update
 * Downloads and applies the latest version
 */
export async function performUpdate(idempotencyKey?: string): Promise<UpdateResult> {
  const { data } = await apiClient.post<UpdateResult>('/admin/system/update', undefined, {
    timeout: UPDATE_REQUEST_TIMEOUT_MS,
    headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
  })
  return data
}

/**
 * Rollback to a previous version
 * @param version - Target version (e.g. "0.1.146"); omit to restore the local backup binary
 */
export async function rollback(version?: string, idempotencyKey?: string): Promise<UpdateResult> {
  const { data } = await apiClient.post<UpdateResult>(
    '/admin/system/rollback',
    version ? { version } : undefined,
    {
      timeout: UPDATE_REQUEST_TIMEOUT_MS,
      headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
    }
  )
  return data
}

/**
 * Restart the service
 */
export async function restartService(idempotencyKey?: string): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>('/admin/system/restart', undefined, {
    headers: idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : undefined
  })
  return data
}

export const systemAPI = {
  getVersion,
  checkUpdates,
  createSystemOperationKey,
  performUpdate,
  getRollbackVersions,
  rollback,
  restartService
}

export default systemAPI
