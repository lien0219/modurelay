import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import WorkspaceTeamsView from '../WorkspaceTeamsView.vue'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(), listProjects: vi.fn(), listTeams: vi.fn(), listMembers: vi.fn(), listTeamMembers: vi.fn(),
  createTeam: vi.fn(), updateTeam: vi.fn(), archiveTeam: vi.fn(), addTeamMember: vi.fn(), removeTeamMember: vi.fn(),
}))

vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key.startsWith('workspace.') ? workspaceMessages.workspace[key.slice(10) as keyof typeof workspaceMessages.workspace] ?? key : key }),
}))

const permissions = ['workspace.read', 'team.read', 'team.create', 'team.update', 'team.archive', 'team.member.update', 'member.read']
const workspace = { id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions }
const team = { id: 11, workspace_id: 1, name: 'Backend', slug: 'backend', description: 'API', status: 'active' }
const member = { id: 13, workspace_id: 1, user_id: 21, role: 'developer', status: 'active', username: 'Alice', email: 'alice@example.com' }

let wrapper: VueWrapper | undefined

async function render() {
  const pinia = createPinia()
  setActivePinia(pinia)
  api.listWorkspaces.mockResolvedValue({ items: [workspace] })
  api.listProjects.mockResolvedValue({ items: [] })
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/teams', component: WorkspaceTeamsView }] })
  await router.push('/workspaces/1/teams')
  wrapper = mount(WorkspaceTeamsView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return wrapper
}

describe('WorkspaceTeamsView', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.listTeams.mockResolvedValue({ items: [team] })
    api.listMembers.mockResolvedValue({ items: [member] })
    api.listTeamMembers.mockResolvedValue({ items: [] })
    api.createTeam.mockResolvedValue(team)
    api.addTeamMember.mockResolvedValue({})
  })

  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('creates a tenant-scoped team and adds a workspace member to the selected team', async () => {
    const view = await render()
    await view.find('button').trigger('click')
    await view.find('[data-testid="team-create-form"] input[name="name"]').setValue('Data')
    await view.find('[data-testid="team-create-form"] input[name="slug"]').setValue('data')
    await view.find('[data-testid="team-create-form"] textarea[name="description"]').setValue('Analytics')
    await view.find('[data-testid="team-create-form"]').trigger('submit')
    await flushPromises()
    expect(api.createTeam).toHaveBeenCalledWith(1, { name: 'Data', slug: 'data', description: 'Analytics' })

    await view.find('[data-testid="team-member-form"] select[name="member-id"]').setValue('13')
    await view.find('[data-testid="team-member-form"]').trigger('submit')
    await flushPromises()
    expect(api.addTeamMember).toHaveBeenCalledWith(1, 11, 13)
  })
})
