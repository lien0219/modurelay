import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { workspaceAPI } from '../workspace'
vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn(), put: vi.fn() } }))
describe('workspace SCIM API', () => {
  beforeEach(() => { vi.resetAllMocks(); for (const method of [apiClient.get, apiClient.post, apiClient.patch, apiClient.delete, apiClient.put]) vi.mocked(method).mockResolvedValue({ data: [] }) })
  it('scopes connector read/create/edit/disable to the workspace and sends revisions', async () => {
    const signal = new AbortController().signal
    await workspaceAPI.listSCIMConnectors(7, signal)
    await workspaceAPI.createSCIMConnector(7, { name: 'Entra', default_role: 'viewer' })
    await workspaceAPI.updateSCIMConnector(7, 4, { name: 'Renamed', default_role: 'developer', revision: 3 })
    await workspaceAPI.disableSCIMConnector(7, 4, 4)
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/scim-connectors', { signal })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/scim-connectors', { name: 'Entra', default_role: 'viewer' })
    expect(apiClient.patch).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4', { name: 'Renamed', default_role: 'developer', revision: 3 })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/disable', { revision: 4 })
  })
  it('creates a replacement token without revoking the old token or adding an expiry implicitly', async () => {
    await workspaceAPI.listSCIMTokens(7, 4)
    await workspaceAPI.createSCIMToken(7, 4, {})
    await workspaceAPI.createSCIMToken(7, 4, { expires_at: '2027-01-01T00:00:00.000Z' })
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/tokens', { signal: undefined })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/tokens', {})
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/tokens', { expires_at: '2027-01-01T00:00:00.000Z' })
    expect(apiClient.delete).not.toHaveBeenCalled()
    await workspaceAPI.revokeSCIMToken(7, 4, 9)
    expect(apiClient.delete).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/tokens/9')
  })
  it('lists directory groups and explicitly binds/unbinds an encoded group ID with revision', async () => {
    await workspaceAPI.listSCIMGroups(7, 4)
    await workspaceAPI.bindSCIMGroup(7, 4, 'group/id', { revision: 2, team_id: 5 })
    await workspaceAPI.bindSCIMGroup(7, 4, 'group/id', { revision: 3, team_id: null })
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/groups', { signal: undefined })
    expect(apiClient.put).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/groups/group%2Fid/team', { revision: 2, team_id: 5 })
    expect(apiClient.put).toHaveBeenCalledWith('/workspaces/7/scim-connectors/4/groups/group%2Fid/team', { revision: 3, team_id: null })
  })
})
