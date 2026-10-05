import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(),
  listProjects: vi.fn(),
  getWorkspace: vi.fn(),
}))

vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))

const workspace = (id: number, permissions = ['workspace.read', 'project.read']): any => ({
  id,
  name: id === 1 ? 'Personal' : `Workspace ${id}`,
  slug: id === 1 ? 'personal-1' : `workspace-${id}`,
  type: id === 1 ? 'personal' : 'organization',
  status: 'active',
  owner_user_id: 7,
  billing_owner_user_id: 7,
  permissions,
})

const project = (id: number, workspaceId: number, isDefault = false): any => ({
  id,
  workspace_id: workspaceId,
  name: isDefault ? 'Default' : `Project ${id}`,
  slug: isDefault ? 'default' : `project-${id}`,
  description: '',
  status: 'active',
  is_default: isDefault,
})

describe('workspace store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
    api.listWorkspaces.mockResolvedValue({ items: [workspace(1), workspace(2, ['workspace.read'])] })
    api.listProjects.mockImplementation(async (id: number) => ({ items: [project(id * 10, id, true)] }))
    api.getWorkspace.mockImplementation(async (id: number) => workspace(id))
  })

  it('selects the personal workspace and its default project on first load', async () => {
    const store = useWorkspaceStore()
    await store.loadWorkspaces()

    expect(store.selectedWorkspace?.id).toBe(1)
    expect(store.selectedProject?.is_default).toBe(true)
    expect(store.can('workspace.read')).toBe(true)
    expect(store.can('workspace.update')).toBe(false)
  })

  it('switches workspace and ignores an older project response', async () => {
    let resolveFirst!: (value: any) => void
    const firstProjects = new Promise(resolve => { resolveFirst = resolve })
    api.listProjects.mockResolvedValueOnce({ items: [project(10, 1, true)] })
      .mockImplementationOnce(() => firstProjects)
      .mockResolvedValueOnce({ items: [project(30, 3, true)] })
    api.listWorkspaces.mockResolvedValue({ items: [workspace(1), workspace(3)] })
    const store = useWorkspaceStore()
    await store.loadWorkspaces()
    const firstSwitch = store.selectWorkspace(1)
    await store.selectWorkspace(3)
    resolveFirst({ items: [project(10, 1, true)] })
    await firstSwitch

    expect(store.selectedWorkspace?.id).toBe(3)
    expect(store.selectedProject?.workspace_id).toBe(3)
  })

  it('clears tenant state on reset', async () => {
    const store = useWorkspaceStore()
    await store.loadWorkspaces()
    store.reset()

    expect(store.workspaces).toEqual([])
    expect(store.selectedWorkspace).toBeNull()
    expect(store.selectedProject).toBeNull()
    expect(store.permissions).toEqual([])
  })

  it('keeps the latest project reload for the same workspace', async () => {
    const store = useWorkspaceStore()
    await store.loadWorkspaces()
    let resolveOld!: (value: { items: ReturnType<typeof project>[] }) => void
    api.listProjects.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      .mockResolvedValueOnce({ items: [project(11, 1, true)] })
    const oldLoad = store.loadProjects(1)
    await store.loadProjects(1)
    resolveOld({ items: [project(10, 1, true)] })
    await oldLoad
    expect(store.selectedProjectId).toBe(11)
    expect(store.projects.map(item => item.id)).toEqual([11])
  })

  it('removes an inaccessible workspace before selecting a fallback', async () => {
    const store = useWorkspaceStore()
    await store.loadWorkspaces()
    api.getWorkspace.mockRejectedValue({ status: 403 })
    await store.refreshWorkspace(1)
    expect(store.workspaces.map(item => item.id)).toEqual([2])
    expect(store.selectedWorkspaceId).toBe(2)
    expect(store.can('workspace.update')).toBe(false)
  })
})
