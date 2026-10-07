import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { workspaceAPI } from '../workspace'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn(), put: vi.fn() } }))

describe('workspace identity API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    for (const method of [apiClient.get, apiClient.post, apiClient.patch, apiClient.delete, apiClient.put]) {
      vi.mocked(method).mockResolvedValue({ data: {} })
    }
  })

  it('reads SAML SP registration and rotates keys with revision in the tenant', async () => {
    await workspaceAPI.getSAMLServiceProvider(7, 9)
    await workspaceAPI.rotateSAMLKeys(7, 9, { revision: 4, action: 'stage' })
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/identity-providers/9/saml-sp', { signal: undefined })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/identity-providers/9/saml-keys/rotate', { revision: 4, action: 'stage' })
  })

  it('keeps domain and provider operations tenant scoped', async () => {
    await workspaceAPI.listDomains(7, { page: 2 })
    await workspaceAPI.createDomain(7, 'example.com')
    await workspaceAPI.verifyDomain(7, 3)
    await workspaceAPI.regenerateDomainToken(7, 3)
    await workspaceAPI.revokeDomain(7, 3)
    await workspaceAPI.listIdentityProviders(7)
    await workspaceAPI.disableIdentityProvider(7, 9)

    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/domains', { params: { page: 2, page_size: 50 }, signal: undefined })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/domains', { domain: 'example.com' })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/domains/3/verify')
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/domains/3/regenerate')
    expect(apiClient.delete).toHaveBeenCalledWith('/workspaces/7/domains/3')
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/identity-providers', { params: { page: 1, page_size: 50 }, signal: undefined })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/identity-providers/9/disable')
  })

  it('loads and replaces provider role and team mappings in the same tenant', async () => {
    const payload = { roles: [{ claim_value: 'engineering', role: 'developer' as const, priority: 10 }], teams: [{ claim_value: 'engineering', team_id: 4 }] }
    await workspaceAPI.getIdentityProviderMappings(7, 9)
    await workspaceAPI.updateIdentityProviderMappings(7, 9, payload)
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/identity-providers/9/mappings', { signal: undefined })
    expect(apiClient.put).toHaveBeenCalledWith('/workspaces/7/identity-providers/9/mappings', payload)
  })

  it('serializes the policy and starts SSO with a safe relative return path', async () => {
    await workspaceAPI.getIdentityPolicy(7)
    await workspaceAPI.updateIdentityPolicy(7, { require_sso: true, sso_grace_until: null })
    await workspaceAPI.startSSO(7, 9, '/workspaces/7/identity')

    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/security-policy', { signal: undefined })
    expect(apiClient.patch).toHaveBeenCalledWith('/workspaces/7/security-policy', { require_sso: true, sso_grace_until: null })
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/start', { workspace_id: 7, provider_id: 9, return_to: '/workspaces/7/identity' })
  })
})
