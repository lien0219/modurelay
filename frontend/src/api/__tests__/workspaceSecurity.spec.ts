import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { workspaceAPI } from '../workspace'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn() } }))

describe('workspace security policy API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    for (const method of [apiClient.get, apiClient.post, apiClient.patch]) vi.mocked(method).mockResolvedValue({ data: { revision: 3 } })
  })

  it('previews all controls without mutation and preserves an explicit null age', async () => {
    const payload = { expected_revision: 2, require_sso: true, require_mfa: true, session_max_age_seconds: null, sso_grace_until: null, invitation_policy: 'verified_domains_only' as const, allow_external_members: false, workspace_jit_enabled: false, approved_identity_provider_mode: 'selected' as const, approved_identity_provider_ids: [9, 60] }
    await workspaceAPI.previewSecurityPolicy(7, payload)
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/security-policy/preview', payload, { signal: undefined })
    expect(apiClient.patch).not.toHaveBeenCalled()
  })

  it('sends the expected revision without filling omitted fields', async () => {
    await workspaceAPI.updateSecurityPolicy(7, { expected_revision: 2, session_max_age_seconds: null })
    expect(apiClient.patch).toHaveBeenCalledWith('/workspaces/7/security-policy', { expected_revision: 2, session_max_age_seconds: null })
  })
})
