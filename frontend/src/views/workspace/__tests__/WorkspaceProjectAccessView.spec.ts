import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import WorkspaceProjectAccessView from '../WorkspaceProjectAccessView.vue'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(), listProjects: vi.fn(), getProject: vi.fn(), listProjectAccessGrants: vi.fn(), listTeams: vi.fn(), listMembers: vi.fn(),
  updateProjectAccessMode: vi.fn(), createProjectAccessGrant: vi.fn(), updateProjectAccessGrant: vi.fn(), deleteProjectAccessGrant: vi.fn(),
}))

vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key.startsWith('workspace.') ? workspaceMessages.workspace[key.slice(10) as keyof typeof workspaceMessages.workspace] ?? key : key }),
}))

const permissions = ['workspace.read', 'workspace.project_access.update', 'project_access.read', 'project_access.update', 'team.read', 'member.read']
const workspace = { id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', project_access_mode: 'all_projects', owner_user_id: 7, billing_owner_user_id: 7, permissions }
const project = { id: 9, workspace_id: 1, name: 'Production API', slug: 'production-api', description: '', status: 'active', is_default: false }
const team = { id: 11, workspace_id: 1, name: 'Backend', slug: 'backend', description: '', status: 'active' }
const member = { id: 13, workspace_id: 1, user_id: 21, role: 'developer', status: 'active', username: 'Alice', email: 'alice@example.com' }
const grant = { id: 15, workspace_id: 1, project_id: 9, subject_type: 'team', subject_id: 11, role: 'developer', created_by_user_id: 7 }

let wrapper: VueWrapper | undefined

async function render() {
  const pinia = createPinia()
  setActivePinia(pinia)
  api.listWorkspaces.mockResolvedValue({ items: [workspace] })
  api.listProjects.mockResolvedValue({ items: [project] })
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/projects/:projectId/access', component: WorkspaceProjectAccessView }, { path: '/workspaces/:workspaceId/projects/:projectId', component: { template: '<main />' } }] })
  await router.push('/workspaces/1/projects/9/access')
  wrapper = mount(WorkspaceProjectAccessView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return wrapper
}

describe('WorkspaceProjectAccessView', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.getProject.mockResolvedValue(project)
    api.listProjectAccessGrants.mockResolvedValue({ items: [grant] })
    api.listTeams.mockResolvedValue({ items: [team] })
    api.listMembers.mockResolvedValue({ items: [member] })
    api.updateProjectAccessMode.mockResolvedValue({ ...workspace, project_access_mode: 'assigned_projects' })
    api.createProjectAccessGrant.mockResolvedValue(grant)
  })

  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('switches to assigned projects and creates a project grant with a tenant subject', async () => {
    const view = await render()
    await view.find('[data-testid="project-access-mode"]').setValue('assigned_projects')
    await flushPromises()
    expect(api.updateProjectAccessMode).toHaveBeenCalledWith(1, 'assigned_projects')

    await view.find('[data-testid="project-access-form"] select[name="subject-type"]').setValue('member')
    await view.find('[data-testid="project-access-form"] select[name="subject-id"]').setValue('13')
    await view.find('[data-testid="project-access-form"] select[name="role"]').setValue('viewer')
    await view.find('[data-testid="project-access-form"]').trigger('submit')
    await flushPromises()
    expect(api.createProjectAccessGrant).toHaveBeenCalledWith(1, 9, { subject_type: 'member', subject_id: 13, role: 'viewer' })
  })
})
