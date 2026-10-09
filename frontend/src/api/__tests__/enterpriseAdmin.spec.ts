import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { apiClient } from '../client'
import { workspaceAPI } from '../workspace'
import { enterpriseAdminAPI } from '../enterpriseAdmin'
import { totpAPI } from '../totp'

const originalAdapter = apiClient.defaults.adapter
const requests: InternalAxiosRequestConfig[] = []
beforeEach(() => {
  requests.length = 0
  localStorage.clear()
  localStorage.setItem('auth_token', 'captured-token')
  localStorage.setItem('refresh_token', 'captured-refresh')
  apiClient.defaults.adapter = async config => {
    requests.push(config)
    return { data: { code: 0, data: { items: [], pages: 4, total_capped: true } }, status: 200, statusText: 'OK', config, headers: {} }
  }
})
afterEach(() => { apiClient.defaults.adapter = originalAdapter; vi.restoreAllMocks() })

describe('Global Admin diagnostics transport', () => {
  it('preserves all admin filters and public pages with cancellation outside the query', async () => {
    const signal = new AbortController().signal
    const filters = { workspace_id: 8, owner_user_id: 2, billing_owner_user_id: 3, name_prefix: 'a_%', slug_prefix: 'b', status: 'suspended', type: 'organization', created_from: '2026-01-01T00:00:00Z', created_to: '2026-01-02T00:00:00Z', updated_from: '2026-02-01T00:00:00Z', updated_to: '2026-02-02T00:00:00Z', sort: 'updated_at' as const, direction: 'asc' as const, page: 2, page_size: 20 }
    const result = await workspaceAPI.adminList({ ...filters, signal })
    expect(requests[0].url).toBe('/admin/workspaces')
    expect(requests[0].params).toMatchObject(filters)
    expect(requests[0].params).not.toHaveProperty('signal')
    expect(requests[0].signal).toBe(signal)
    expect(result.pages).toBe(4)
    expect(result.total_capped).toBe(true)
  })

  it('uses only global diagnostic paths and the preserved inspect path', async () => {
    const signal = new AbortController().signal
    await enterpriseAdminAPI.overview(signal)
    await enterpriseAdminAPI.diagnostics(8, signal)
    await enterpriseAdminAPI.jobs(signal)
    await workspaceAPI.adminInspect(8, signal)
    expect(requests.map(request => request.url)).toEqual(['/admin/workspaces/diagnostics/overview', '/admin/workspaces/8/diagnostics', '/admin/operations/jobs', '/admin/workspaces/8'])
    expect(requests.every(request => request.signal === signal)).toBe(true)
  })

  it('sends exact guarded bodies and captured credentials without an implicit 401 replay', async () => {
    apiClient.defaults.adapter = async config => {
      requests.push(config)
      throw new AxiosError('unsafe upstream text', 'ERR_BAD_REQUEST', config, undefined, { data: { code: 'UNAUTHORIZED', message: 'secret' }, status: 401, statusText: 'Unauthorized', headers: {}, config })
    }
    localStorage.setItem('auth_token', 'replacement-token')
    const config = { signal: new AbortController().signal, preserveAuthSessionOnFailure: true, sessionProofAccessToken: 'captured-token' }
    const body = { action: 'suspend' as const, reason: 'incident', confirmation: 'suspend:8', idempotency_key: '12345678-1234-4234-8234-123456789abc', expected_updated_at: '2026-10-01T00:00:00.123456789Z' }
    await expect(workspaceAPI.adminSetStatus(8, body, config)).rejects.toMatchObject({ status: 401 })
    await expect(enterpriseAdminAPI.retryWebhook(8, 9, 10, { ...body, action: 'retry_webhook', confirmation: 'retry_webhook:10', expected_updated_at: undefined, expected_attempts: 4, expected_last_attempt_at: '' }, config)).rejects.toMatchObject({ status: 401 })
    await expect(totpAPI.stepUp('123456', undefined, config)).rejects.toMatchObject({ status: 401 })
    expect(requests).toHaveLength(3)
    expect(JSON.parse(requests[0].data)).toEqual(body)
    expect(requests[1].url).toBe('/admin/workspaces/8/webhooks/9/deliveries/10/retry')
    expect(requests.every(request => request.headers.Authorization === 'Bearer captured-token')).toBe(true)
    expect(localStorage.getItem('auth_token')).toBe('replacement-token')
  })
})
