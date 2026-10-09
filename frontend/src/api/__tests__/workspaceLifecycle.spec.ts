import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { workspaceLifecycleAPI as api } from '../workspaceLifecycle'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), put: vi.fn(), post: vi.fn() } }))

describe('Workspace lifecycle API transport', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    for (const method of [apiClient.get, apiClient.put, apiClient.post]) vi.mocked(method).mockResolvedValue({ data: null })
  })

  it('uses the shared authenticated client and a POST body for a one-use download grant', async () => {
    const signal = new AbortController().signal
    const blob = new Blob(['zip'], { type: 'application/zip' })
    vi.mocked(apiClient.post).mockResolvedValueOnce({ data: { token: 'private-download-grant', expires_at: '2099-01-01T00:00:00Z' } }).mockResolvedValueOnce({ data: blob })
    const grant = await api.authorizeDownload(7, 'export-id', signal)
    expect(await api.redeemDownload(7, 'export-id', grant.token, signal)).toBe(blob)
    expect(apiClient.post).toHaveBeenNthCalledWith(1, '/workspaces/7/exports/export-id/download', undefined, { signal })
    expect(apiClient.post).toHaveBeenNthCalledWith(2, '/workspaces/7/exports/export-id/download/redeem', { token: 'private-download-grant' }, { signal, responseType: 'blob', decodeJSONBlobErrors: true })
    for (const call of vi.mocked(apiClient.post).mock.calls) expect(call[0]).not.toContain(grant.token)
    expect(apiClient.get).not.toHaveBeenCalled()
  })

  it('submits exact-name confirmation unchanged without adding a force override', async () => {
    const signal = new AbortController().signal
    await api.requestDeletion(7, { name: 'Exact Workspace ', token: 'one-use-confirmation' }, signal)
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/deletion', { name: 'Exact Workspace ', token: 'one-use-confirmation' }, { signal })
  })

  it('uses retention PUT and distinct restore commands with cancellation signals', async () => {
    const signal = new AbortController().signal
    await api.updateRetention(7, 'security', 0, signal)
    await api.restoreWorkspace(7, signal)
    await api.restoreProject(7, 20, signal)
    expect(apiClient.put).toHaveBeenCalledWith('/workspaces/7/retention/security', { retention_days: 0 }, { signal })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/restore', undefined, { signal })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/projects/20/restore', undefined, { signal })
  })

  it('preserves server pagination and separates deletion cancel from retry', async () => {
    const signal = new AbortController().signal
    await api.listExports(7, 2, 10, signal)
    await api.cancelDeletion(7, 'job-id', signal)
    await api.retryDeletion(7, 'job-id', signal)
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/exports', { params: { page: 2, page_size: 10 }, signal })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/deletion/job-id/cancel', undefined, { signal })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/deletion/job-id/retry', undefined, { signal })
  })
})
