import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { workspaceAPI } from '../workspace'

vi.mock('../client', () => ({ apiClient: { get: vi.fn(), post: vi.fn(), patch: vi.fn(), delete: vi.fn() } }))

describe('workspace governance API', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    for (const method of [apiClient.get, apiClient.post, apiClient.patch, apiClient.delete]) {
      vi.mocked(method).mockResolvedValue({ data: {} })
    }
  })

  it('keeps team management tenant scoped and paginated', async () => {
    await workspaceAPI.listTeams(7, { page: 2 })
    await workspaceAPI.createTeam(7, { name: 'Backend', slug: 'backend', description: 'API' })
    await workspaceAPI.addTeamMember(7, 11, 13)
    await workspaceAPI.removeTeamMember(7, 11, 13)
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/teams', { params: { page: 2, page_size: 50 }, signal: undefined })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/teams', { name: 'Backend', slug: 'backend', description: 'API' })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/teams/11/members', { workspace_member_id: 13 })
    expect(apiClient.delete).toHaveBeenCalledWith('/workspaces/7/teams/11/members/13')
  })

  it('uses explicit project access mode and grant payloads', async () => {
    await workspaceAPI.updateProjectAccessMode(7, 'assigned_projects')
    await workspaceAPI.listProjectAccessGrants(7, 9)
    await workspaceAPI.createProjectAccessGrant(7, 9, { subject_type: 'team', subject_id: 11, role: 'developer' })
    await workspaceAPI.updateProjectAccessGrant(7, 9, 15, { subject_type: 'member', subject_id: 13, role: 'viewer' })
    await workspaceAPI.deleteProjectAccessGrant(7, 9, 15)
    expect(apiClient.patch).toHaveBeenCalledWith('/workspaces/7/project-access-mode', { project_access_mode: 'assigned_projects' })
    expect(apiClient.get).toHaveBeenCalledWith('/workspaces/7/projects/9/access-grants', { params: { page: 1, page_size: 50 }, signal: undefined })
    expect(apiClient.post).toHaveBeenCalledWith('/workspaces/7/projects/9/access-grants', { subject_type: 'team', subject_id: 11, role: 'developer' })
    expect(apiClient.patch).toHaveBeenCalledWith('/workspaces/7/projects/9/access-grants/15', { subject_type: 'member', subject_id: 13, role: 'viewer' })
    expect(apiClient.delete).toHaveBeenCalledWith('/workspaces/7/projects/9/access-grants/15')
  })
})
