import { beforeEach, describe, expect, it, vi } from 'vitest'
import { serviceAccountsAPI } from '../serviceAccounts'
import { apiClient } from '../client'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn() } }))
describe('service account API', () => {
  beforeEach(() => { vi.resetAllMocks(); for (const method of [apiClient.get, apiClient.post, apiClient.patch]) vi.mocked(method).mockResolvedValue({ data: {} }) })
  it('scopes list to both workspace and project and supports pagination', async () => {
    await serviceAccountsAPI.list(7, 9, { page: 2 })
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/projects/9/service-accounts', { params: { page: 2, page_size: 50 }, signal: undefined })
  })
  it('rotates using an idempotency key and keeps secrets out of URLs', async () => {
    await serviceAccountsAPI.rotate(7, 9, 11, 13, 'request-1')
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/projects/9/service-accounts/11/credentials/13/rotate', undefined, { headers: { 'Idempotency-Key': 'request-1' } })
  })
  it('explicitly revokes credentials and disables accounts', async () => {
    await serviceAccountsAPI.revoke(7, 9, 11, 13)
    await serviceAccountsAPI.setStatus(7, 9, 11, 'disabled')
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/projects/9/service-accounts/11/credentials/13/revoke')
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/projects/9/service-accounts/11/disable')
  })
})
