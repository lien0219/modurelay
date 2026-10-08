import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useWorkspaceStore } from '@/stores/workspace'
import type { Workspace } from '@/api/workspace'
import enWorkspace from '@/i18n/locales/en/workspace'
import zhWorkspace from '@/i18n/locales/zh/workspace'
import WorkspaceFinopsView from '../WorkspaceFinopsView.vue'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(), listProjects: vi.fn(), getBudget: vi.fn(), getUsage: vi.fn(), getOverview: vi.fn(),
  listFinopsAnomalies: vi.fn(), getFinopsAnomalyStatus: vi.fn(), getAllocationReport: vi.fn(), listCostCenters: vi.fn(),
  listAllocationTags: vi.fn(), updateCostCenter: vi.fn(), updateAllocationTag: vi.fn(),
}))
vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/workspace/WorkspaceUsageBreakdowns.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/workspace/WorkspaceDailySpend.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const workspace = (id: number, permissions: string[]): Workspace => ({ id, name: `Workspace ${id}`, slug: `workspace-${id}`, type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions })
const report = (id: number, key: string) => ({ workspace_id: id, workspace_total: 1, allocated: 1, unallocated: 0, overlapping_tags: false, cost_centers: [{ key, cost: 1, request_count: 1 }], environments: [{ key: 'production', cost: 1, request_count: 1 }], tags: [] })
let wrapper: VueWrapper | undefined

async function render(permissions = ['workspace.read', 'project.read', 'usage.read', 'budget.read'], allocationResolver?: (workspaceId: number) => unknown) {
  localStorage.clear()
  api.listWorkspaces.mockResolvedValue({ items: [workspace(1, permissions), workspace(2, permissions)] })
  api.listProjects.mockResolvedValue({ items: [] })
  api.getBudget.mockResolvedValue({ policy: { amount: 10, hard_limit: true, enabled: true, timezone: 'UTC' }, spent: 1, reserved: 0, remaining: 9 })
  api.getUsage.mockResolvedValue({ summary: { requests: 1, spend: 1 } })
  api.getOverview.mockResolvedValue({ summary: { requests: 1, spend: 1 } })
  api.listFinopsAnomalies.mockResolvedValue({ items: [] })
  api.getFinopsAnomalyStatus.mockResolvedValue({ lag_seconds: 0, candidate_count: 0, finding_count: 0, scan_duration_ms: 0 })
  api.listCostCenters.mockResolvedValue([])
  api.listAllocationTags.mockResolvedValue([])
  if (allocationResolver) api.getAllocationReport.mockImplementation(allocationResolver)
  else api.getAllocationReport.mockResolvedValue(report(1, 'core'))
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = useWorkspaceStore()
  await store.loadWorkspaces()
  wrapper = mount(WorkspaceFinopsView, { global: { plugins: [pinia] } })
  await flushPromises()
  return { wrapper, store }
}

describe('workspace allocation controls', () => {
  beforeEach(() => vi.resetAllMocks())
  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('uses typed report and control-plane API shapes for the selected workspace', async () => {
    await render()
    expect(api.getAllocationReport).toHaveBeenCalledWith(1, expect.objectContaining({ timezone: 'UTC' }), expect.anything())
    expect(api.listCostCenters).toHaveBeenCalledWith(1, false, expect.anything())
    expect(api.listAllocationTags).toHaveBeenCalledWith(1, false, expect.anything())
  })

  it('hides mutation controls without workspace.update permission', async () => {
    api.listCostCenters.mockResolvedValue([{ id: 41, workspace_id: 1, code: 'core', name: 'Core', description: '', status: 'active' }])
    const { wrapper } = await render()
    expect(wrapper!.find('.allocation-inline-form').exists()).toBe(false)
    expect(wrapper!.findAll('.allocation-list-row button').some(button => button.text() === 'common.edit')).toBe(false)
  })

  it('keeps tag inputs bounded and required', async () => {
    const { wrapper } = await render(['workspace.read', 'workspace.update', 'project.read', 'usage.read', 'budget.read'])
    const tagForm = wrapper!.findAll('form.allocation-inline-form')[1]
    const inputs = tagForm.findAll('input')
    expect(inputs[0].attributes('required')).toBeDefined()
    expect(inputs[0].attributes('maxlength')).toBe('63')
    expect(inputs[1].attributes('required')).toBeDefined()
    expect(inputs[1].attributes('maxlength')).toBe('255')
  })

  it('ignores an older allocation report after switching workspaces', async () => {
    let resolveOld!: (value: ReturnType<typeof report>) => void
    const { wrapper, store } = await render(undefined, (id: number) => id === 1 ? new Promise(resolve => { resolveOld = resolve }) : Promise.resolve(report(2, 'new-center')))
    await store.selectWorkspace(2)
    await flushPromises()
    expect(wrapper!.text()).toContain('new-center')
    resolveOld(report(1, 'old-center'))
    await flushPromises()
    expect(wrapper!.text()).not.toContain('old-center')
  })

  it('keeps allocation copy and mobile table containment labels in both locales', async () => {
    const { wrapper } = await render()
    expect(wrapper!.findAll('.allocation-table-wrap')).toHaveLength(3)
    expect(enWorkspace.workspace.allocationBreakdown).toBe('Cost allocation breakdown')
    expect(zhWorkspace.workspace.allocationBreakdown).toBe('成本归因明细')
  })
})
