import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import WorkspaceProjectDetailView from '../WorkspaceProjectDetailView.vue'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(), listProjects: vi.fn(), getWorkspace: vi.fn(),
  getProject: vi.fn(), listKeys: vi.fn(), listAvailableGroups: vi.fn(),
  getProjectBudget: vi.fn(), getProjectUsage: vi.fn(), getProjectOverview: vi.fn(),
  updateProjectBudget: vi.fn(), createKey: vi.fn(), updateKey: vi.fn(), revokeKey: vi.fn(),
  archiveProject: vi.fn(),
}))
const lifecycle = vi.hoisted(() => ({ restoreProject: vi.fn() }))

vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/api/workspaceLifecycle', () => ({ workspaceLifecycleAPI: lifecycle }))
vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<canvas />' } }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key.startsWith('workspace.') ? workspaceMessages.workspace[key.slice(10) as keyof typeof workspaceMessages.workspace] ?? key : key }),
}))

const allPermissions = ['workspace.read', 'project.read', 'key.read', 'key.create', 'key.update', 'key.revoke', 'budget.read', 'budget.update', 'usage.read']
const project = (id: number) => ({ id, workspace_id: 1, name: id === 10 ? 'Default' : 'Production', slug: `project-${id}`, description: '', status: 'active', is_default: id === 10 })
const budget = { workspace_id: 1, project_id: 20, policy: { amount: 100, hard_limit: true, enabled: true, timezone: 'UTC' }, spent: 12.5, reserved: 2, remaining: 85.5, over_budget: false }
const summary = { workspace_id: 1, project_id: 20, requests: 5, spend: 12.5, reserved: 2, members: 1, projects: 2, api_keys: 1, start: '2026-10-01T00:00:00Z', end: '2026-11-01T00:00:00Z', timezone: 'UTC' }

let wrapper: VueWrapper | undefined

async function render(permissions = allPermissions) {
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = useWorkspaceStore()
  api.listWorkspaces.mockResolvedValue({ items: [{ id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions }] })
  await store.loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/projects/:projectId', component: WorkspaceProjectDetailView }, { path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/workspaces/1/projects/20')
  await router.isReady()
  wrapper = mount(WorkspaceProjectDetailView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { store, router, wrapper }
}

describe('project workspace context and FinOps', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.listProjects.mockResolvedValue({ items: [project(10), project(20)] })
    api.getProject.mockImplementation(async (_workspaceId: number, projectId: number) => project(projectId))
    api.listKeys.mockResolvedValue({ items: [] })
    api.listAvailableGroups.mockResolvedValue([])
    api.getProjectBudget.mockResolvedValue(budget)
    api.getProjectUsage.mockResolvedValue(summary)
    api.getProjectOverview.mockResolvedValue({ summary, daily_spend: [{ date: '2026-10-03', requests: 5, spend: 12.5 }], projects: [], platforms: [{ id: 'seedance', name: 'Seedance', requests: 3, spend: 9 }], models: [{ id: 'model-1', name: 'doubao-seedance-pro', requests: 3, spend: 9 }], api_keys: [{ id: '71', name: '71', requests: 5, spend: 12.5 }] })
  })

  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('selects the route project after the default project was loaded', async () => {
    const { wrapper, store } = await render()
    expect(wrapper.findAll('select')[1].element.value).toBe('20')
    expect(store.selectedProject?.name).toBe('Production')
  })

  it('loads the new project when the same view navigates between project routes', async () => {
    const { wrapper, router } = await render()
    await router.push('/workspaces/1/projects/10')
    await flushPromises()
    expect(wrapper.find('h2').text()).toBe('Default')
    expect(wrapper.findAll('select')[1].element.value).toBe('10')
  })

  it('navigates project details when the project selector changes', async () => {
    const { wrapper, router } = await render()
    await wrapper.findAll('select')[1].setValue('10')
    await flushPromises()
    expect(router.currentRoute.value.params.projectId).toBe('10')
    expect(wrapper.find('h2').text()).toBe('Default')
  })

  it('keeps permitted budget and usage visible to a viewer without key access', async () => {
    api.listKeys.mockRejectedValue({ status: 403 })
    const { wrapper } = await render(['workspace.read', 'project.read', 'usage.read', 'budget.read'])
    expect(wrapper.text()).toContain('$12.50')
    expect(wrapper.text()).toContain('Requests')
    expect(wrapper.text()).not.toContain('Project keys')
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('renders platform, model, and API key request counts and spend from the project overview', async () => {
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('Seedance')
    expect(wrapper.text()).toContain('doubao-seedance-pro')
    expect(wrapper.text()).toContain('71')
    expect(wrapper.text()).toContain('$9.00')
  })

  it('renders server-aggregated machine usage and filters both usage requests by service account', async () => {
    api.getProjectOverview.mockResolvedValue({ summary, service_accounts: [
      { id: '31', name: 'backend-api', requests: 3, spend: 9, tokens: 123, models: ['machine-model'] },
      { id: 'legacy', name: 'Human / Legacy', requests: 2, spend: 3.5, tokens: 0, models: [] },
    ] })
    const { wrapper } = await render()
    const rows = wrapper.find('[data-testid="service-account-breakdown"]')
    expect(rows.text()).toContain('backend-api')
    expect(rows.text()).toContain('123')
    expect(rows.text()).toContain('machine-model')
    expect(rows.text()).toContain('$3.50')
    await wrapper.find('[data-testid="service-account-usage-filter"]').setValue('31')
    await flushPromises()
    expect(api.getProjectUsage).toHaveBeenLastCalledWith(1, 20, expect.objectContaining({ service_account_id: 31 }), expect.any(AbortSignal))
    expect(api.getProjectOverview).toHaveBeenLastCalledWith(1, 20, expect.any(AbortSignal), expect.objectContaining({ service_account_id: 31 }))
  })

  it('clears the machine usage filter when navigating between projects', async () => {
    api.getProjectOverview.mockResolvedValue({ summary, service_accounts: [{ id: '31', name: 'backend-api', requests: 3, spend: 9, tokens: 123, models: [] }] })
    const { wrapper, router } = await render()
    await wrapper.find('[data-testid="service-account-usage-filter"]').setValue('31')
    await flushPromises()
    await router.push('/workspaces/1/projects/10')
    await flushPromises()
    expect(wrapper.find('[data-testid="service-account-usage-filter"]').element.value).toBe('')
    expect(api.getProjectOverview.mock.lastCall?.[3]).not.toHaveProperty('service_account_id')
  })

  it('shows project daily spend with its server-reported timezone', async () => {
    const { wrapper } = await render(['workspace.read', 'project.read', 'usage.read', 'budget.read'])
    const dailyTable = wrapper.findAll('table').find(table => table.find('caption').text() === 'Daily spend')
    expect(dailyTable).toBeDefined()
    expect(dailyTable!.text()).toContain('2026-10-03')
    expect(dailyTable!.text()).toContain('$12.50')
    expect(wrapper.text()).toContain('Timezone: UTC')
  })

  it('ignores a late project response after navigating to another project', async () => {
    let resolveOld!: (value: ReturnType<typeof project>) => void
    api.getProject.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const { wrapper, router } = await render()
    await router.push('/workspaces/1/projects/10')
    await flushPromises()
    resolveOld(project(20))
    await flushPromises()
    expect(wrapper.find('h2').text()).toBe('Default')
  })

  it('offers project archive only with permission and confirms before using the existing archive API', async () => {
    const { wrapper } = await render([...allPermissions, 'project.archive'])
    expect(wrapper.find('[data-testid="archive-project"]').exists()).toBe(true)
    await wrapper.get('[data-testid="archive-project"]').trigger('click')
    await flushPromises()
    expect(api.archiveProject).not.toHaveBeenCalled()
    const dialog = wrapper.findComponent({ name: 'ConfirmDialog' })
    expect(dialog.exists()).toBe(true)
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(api.archiveProject).toHaveBeenCalledWith(1, 20)
  })

  it('offers restore for an archived project in an active parent Workspace', async () => {
    api.getProject.mockImplementation(async (_wid: number, pid: number) => ({ ...project(pid), status: 'archived' }))
    const { wrapper } = await render(['workspace.read', 'project.read', 'project.restore'])
    expect(wrapper.find('[data-testid="restore-project"]').exists()).toBe(true)
    await wrapper.get('[data-testid="restore-project"]').trigger('click')
    await flushPromises()
    wrapper.findComponent({ name: 'ConfirmDialog' }).vm.$emit('confirm')
    await flushPromises()
    expect(lifecycle.restoreProject).toHaveBeenCalledWith(1, 20, expect.any(AbortSignal))
  })

  it('hides project restore when its parent is archived even if stale permissions include restore', async () => {
    api.getProject.mockImplementation(async (_wid: number, pid: number) => ({ ...project(pid), status: 'archived' }))
    const { wrapper, store } = await render(['workspace.read', 'project.read', 'project.restore'])
    store.workspaces[0].status = 'archived'
    await flushPromises()
    expect(wrapper.find('[data-testid="restore-project"]').exists()).toBe(false)
  })

  it('ignores a late restore failure after changing the project route', async () => {
    let rejectRestore!: (error: unknown) => void
    api.getProject.mockImplementation(async (_wid: number, pid: number) => ({ ...project(pid), status: 'archived' }))
    lifecycle.restoreProject.mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectRestore = reject }))
    const { wrapper, router } = await render(['workspace.read', 'project.read', 'project.restore'])
    await wrapper.get('[data-testid="restore-project"]').trigger('click')
    await flushPromises()
    wrapper.findComponent({ name: 'ConfirmDialog' }).vm.$emit('confirm')
    await flushPromises()
    await router.push('/workspaces/1/projects/10')
    await flushPromises()
    rejectRestore({ message: 'OLD_PROJECT_FAILURE' })
    await flushPromises()
    expect(wrapper.text()).not.toContain('OLD_PROJECT_FAILURE')
    expect(wrapper.find('h2').text()).toBe('Default')
    expect(wrapper.get('[data-testid="restore-project"]').attributes('disabled')).toBeUndefined()
  })
})
