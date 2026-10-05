import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import type { WorkspaceOverview } from '@/api/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import WorkspaceFinopsView from '../WorkspaceFinopsView.vue'

const api = vi.hoisted(() => ({ listWorkspaces: vi.fn(), listProjects: vi.fn(), getWorkspace: vi.fn(), getBudget: vi.fn(), updateBudget: vi.fn(), getUsage: vi.fn(), getOverview: vi.fn() }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-chartjs', () => ({ Line: { name: 'Line', props: ['data', 'options'], template: '<canvas />' } }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key.startsWith('workspace.') ? workspaceMessages.workspace[key.slice(10) as keyof typeof workspaceMessages.workspace] ?? key : key }),
}))

const summary = { requests: 7, spend: 14, start: '2026-10-01T00:00:00Z', end: '2026-11-01T00:00:00Z' }
const overview = (name: string): WorkspaceOverview => ({ summary, daily_spend: [{ date: '2026-10-02', requests: 7, spend: 0.000123 }], projects: [{ id: '10', name, requests: 7, spend: 14 }], platforms: [{ id: 'seedance', name: 'Seedance', requests: 5, spend: 10 }], models: [{ id: 'video-model', name: 'doubao-seedance-pro', requests: 5, spend: 10 }], api_keys: [{ id: '71', name: '71', requests: 7, spend: 14 }] })
let wrapper: VueWrapper | undefined

async function render() {
  const pinia = createPinia()
  setActivePinia(pinia)
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/finops', component: WorkspaceFinopsView }, { path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/workspaces/1/finops')
  wrapper = mount(WorkspaceFinopsView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('workspace FinOps', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.listWorkspaces.mockResolvedValue({ items: [1, 2].map(id => ({ id, name: `Workspace ${id}`, slug: `workspace-${id}`, type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: ['workspace.read', 'project.read', 'usage.read', 'budget.read', 'budget.update'] })) })
    api.listProjects.mockResolvedValue({ items: [] })
    api.getBudget.mockResolvedValue({ policy: { amount: 100, hard_limit: true, enabled: true, timezone: 'UTC' }, spent: 14, reserved: 1, remaining: 85 })
    api.getUsage.mockResolvedValue(summary)
    api.getOverview.mockImplementation(async (id: number) => overview(`Project in ${id}`))
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('shows all four server-reported FinOps dimensions with request counts and spend', async () => {
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('Project in 1')
    expect(wrapper.text()).toContain('Seedance')
    expect(wrapper.text()).toContain('doubao-seedance-pro')
    expect(wrapper.text()).toContain('71')
    expect(wrapper.text()).toContain('$10.00')
    expect(wrapper.find('input[type="number"]').element.value).toBe('100')
  })

  it('shows server-reported daily spend without inventing dates or rounding small spend to zero', async () => {
    const { wrapper } = await render()
    const dailyTable = wrapper.findAll('table').find(table => table.find('caption').text() === 'Daily spend')
    expect(dailyTable).toBeDefined()
    expect(dailyTable!.findAll('tbody tr')).toHaveLength(1)
    expect(dailyTable!.text()).toContain('2026-10-02')
    expect(dailyTable!.text()).toContain('0.000123')
    expect(wrapper.findComponent({ name: 'Line' }).props('data').datasets[0].data).toEqual([0.000123])
  })

  it('ignores the previous workspace response after switching workspace', async () => {
    let resolveOld!: (value: WorkspaceOverview) => void
    api.getOverview.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const { wrapper, router } = await render()
    await router.push('/workspaces/2/finops')
    await flushPromises()
    resolveOld(overview('Old workspace project'))
    await flushPromises()
    expect(wrapper.text()).toContain('Project in 2')
    expect(wrapper.text()).not.toContain('Old workspace project')
  })
})
