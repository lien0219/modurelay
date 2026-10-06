import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { policyAPI } from '../policies'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), patch: vi.fn() } }))

describe('policy API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(apiClient.get).mockResolvedValue({ data: {} })
    vi.mocked(apiClient.patch).mockResolvedValue({ data: {} })
  })

  it('keeps workspace and project policy paths tenant-scoped', async () => {
    await policyAPI.getWorkspace(7)
    await policyAPI.getProject(7, 9)
    expect(apiClient.get).toHaveBeenNthCalledWith(1, '/workspaces/7/policy', { signal: undefined })
    expect(apiClient.get).toHaveBeenNthCalledWith(2, '/workspaces/7/projects/9/policy', { signal: undefined })
  })

  it('updates with the expected revision and preserves null versus empty allowlists', async () => {
    await policyAPI.updateWorkspace(7, {
      expected_revision: 3,
      allowed_models: null,
      allowed_platforms: [],
      rpm_limit: null,
    })
    expect(apiClient.patch).toHaveBeenCalledWith('/workspaces/7/policy', {
      expected_revision: 3,
      allowed_models: null,
      allowed_platforms: [],
      rpm_limit: null,
    })
  })

  it('loads service-account policy and effective policy without accepting client scope metadata', async () => {
    await policyAPI.getServiceAccount(7, 9, 11)
    await policyAPI.getEffective(7, 9, 11)
    expect(apiClient.get).toHaveBeenNthCalledWith(1, '/workspaces/7/projects/9/service-accounts/11/policy', { signal: undefined })
    expect(apiClient.get).toHaveBeenNthCalledWith(2, '/workspaces/7/projects/9/service-accounts/11/effective-policy', { signal: undefined })
  })
})
