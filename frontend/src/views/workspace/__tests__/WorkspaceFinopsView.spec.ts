import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import type { WorkspaceOverview } from '@/api/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import WorkspaceFinopsView from '../WorkspaceFinopsView.vue'

const api = vi.hoisted(() => ({ listWorkspaces: vi.fn(), listProjects: vi.fn(), getWorkspace: vi.fn(), getBudget: vi.fn(), updateBudget: vi.fn(), getUsage: vi.fn(), getOverview: vi.fn(), listFinopsAnomalies: vi.fn(), getFinopsAnomalyStatus: vi.fn(), getFinopsAnomaly: vi.fn(), updateFinopsAnomaly: vi.fn(), getAllocationReport: vi.fn(), listCostCenters: vi.fn(), createCostCenter: vi.fn(), updateCostCenter: vi.fn(), archiveCostCenter: vi.fn(), listAllocationTags: vi.fn(), createAllocationTag: vi.fn(), updateAllocationTag: vi.fn(), archiveAllocationTag: vi.fn(), getProjectAllocation: vi.fn(), updateProjectAllocation: vi.fn() }))
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
    api.listFinopsAnomalies.mockResolvedValue({ items: [] })
    api.getFinopsAnomalyStatus.mockResolvedValue({ lag_seconds: 0, candidate_count: 0, finding_count: 0, scan_duration_ms: 0, last_successful_scan: null })
    api.getAllocationReport.mockResolvedValue({ workspace_id: 1, workspace_total: 0, allocated: 0, unallocated: 0, overlapping_tags: false, cost_centers: [], environments: [], tags: [] })
    api.listCostCenters.mockResolvedValue([])
    api.listAllocationTags.mockResolvedValue([])
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

  it('renders server evidence, preserves tiny money, and acknowledges a finding', async () => {
    api.listWorkspaces.mockResolvedValueOnce({ items: [{ id: 1, name: 'Workspace 1', slug: 'workspace-1', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: ['workspace.read', 'usage.read', 'budget.read', 'finops_anomaly.read', 'finops_anomaly.manage'] }] })
    const finding = { id: 8, workspace_id: 1, scope_type: 'model', scope_id: 0, dimension_type: 'model', dimension_value: 'model-x', detector_type: 'spend_spike', detector_version: 'v1', window_start: '2026-10-07T01:00:00Z', window_end: '2026-10-07T02:00:00Z', observed_spend: 0.000123, expected_spend: 0.00001, spend_delta: 0.000113, observed_requests: 30, expected_requests: 12, observed_unit_cost: 0.0000041, expected_unit_cost: 0.0000008, baseline_sample_count: 12, baseline_mad: 0, relative_increase: 11.3, score: 7.2, severity: 'high', fingerprint: 'a'.repeat(64), snapshot_id: 9, status: 'open', first_detected_at: '2026-10-07T02:05:00Z', last_detected_at: '2026-10-07T02:05:00Z', version: 1, created_at: '2026-10-07T02:05:00Z', updated_at: '2026-10-07T02:05:00Z' }
    api.listFinopsAnomalies.mockResolvedValue({ items: [finding] })
    api.getFinopsAnomaly.mockResolvedValue(finding)
    api.updateFinopsAnomaly.mockResolvedValue({ ...finding, status: 'acknowledged', version: 2 })
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('model-x')
    expect(wrapper.text()).toContain('$0.000123')
    await wrapper.findAll('button').find(button => button.text() === 'Details')?.trigger('click')
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'Acknowledge')?.trigger('click')
    await flushPromises()
    expect(api.updateFinopsAnomaly).toHaveBeenCalledWith(1, 8, expect.objectContaining({ status: 'acknowledged', expected_version: 1 }))
  })

  it('keeps the newest evidence detail when an older detail response arrives later', async () => {
    api.listWorkspaces.mockResolvedValueOnce({ items: [{ id: 1, name: 'Workspace 1', slug: 'workspace-1', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: ['workspace.read', 'usage.read', 'budget.read', 'finops_anomaly.read'] }] })
    const first = { id: 8, workspace_id: 1, scope_type: 'model', scope_id: 0, dimension_type: 'model', dimension_value: 'model-old', detector_type: 'spend_spike', detector_version: 'v1', window_start: '2026-10-07T01:00:00Z', window_end: '2026-10-07T02:00:00Z', observed_spend: 1, expected_spend: 0.1, spend_delta: 0.9, observed_requests: 30, expected_requests: 12, observed_unit_cost: 0.03, expected_unit_cost: 0.008, baseline_sample_count: 12, baseline_mad: 0, relative_increase: 9, score: 7.2, severity: 'high', fingerprint: 'a'.repeat(64), snapshot_id: 9, status: 'open', first_detected_at: '2026-10-07T02:05:00Z', last_detected_at: '2026-10-07T02:05:00Z', version: 1, created_at: '2026-10-07T02:05:00Z', updated_at: '2026-10-07T02:05:00Z' }
    const second = { ...first, id: 9, dimension_value: 'model-new', fingerprint: 'b'.repeat(64), snapshot_id: 10 }
    api.listFinopsAnomalies.mockResolvedValue({ items: [first, second] })
    let resolveFirst!: (value: typeof first) => void
    let resolveSecond!: (value: typeof second) => void
    api.getFinopsAnomaly.mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve }))
      .mockImplementationOnce(() => new Promise(resolve => { resolveSecond = resolve }))
    const { wrapper } = await render()
    await wrapper.findAll('button').find(button => button.text() === 'Details')?.trigger('click')
    await wrapper.findAll('button').find(button => button.text() === 'Details')?.trigger('click')
    resolveSecond(second)
    await flushPromises()
    resolveFirst(first)
    await flushPromises()
    expect(wrapper.find('.anomaly-detail').text()).toContain('model-new')
    expect(wrapper.find('.anomaly-detail').text()).not.toContain('model-old')
  })

  it('edits active allocation definitions through the server APIs', async () => {
    api.listWorkspaces.mockResolvedValueOnce({ items: [{ id: 1, name: 'Workspace 1', slug: 'workspace-1', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: ['workspace.read', 'workspace.update', 'project.read', 'usage.read', 'budget.read'] }] })
    const center = { id: 41, workspace_id: 1, code: 'core', name: 'Core', description: 'Primary', status: 'active' }
    const tag = { id: 51, workspace_id: 1, key: 'team', value: 'platform', description: 'Platform team', status: 'active' }
    api.listCostCenters.mockResolvedValue([center])
    api.listAllocationTags.mockResolvedValue([tag])
    api.updateCostCenter.mockResolvedValue({ ...center, code: 'core-prod', name: 'Core production' })
    api.updateAllocationTag.mockResolvedValue({ ...tag, value: 'runtime' })
    const { wrapper } = await render()

    const centerRow = wrapper.findAll('.allocation-list-row')[0]
    await centerRow.findAll('button').find(button => button.text() === 'common.edit')?.trigger('click')
    await flushPromises()
    const centerEdit = wrapper.find('form.allocation-edit-form')
    expect(centerEdit.exists()).toBe(true)
    await centerEdit.find('input').setValue('core-prod')
    await centerEdit.trigger('submit')
    await flushPromises()
    expect(api.updateCostCenter).toHaveBeenCalledWith(1, 41, expect.objectContaining({ code: 'core-prod' }))

    const tagRow = wrapper.findAll('.allocation-list-row')[1]
    await tagRow.findAll('button').find(button => button.text() === 'common.edit')?.trigger('click')
    await flushPromises()
    const tagEdit = wrapper.find('form.allocation-tag-edit-form')
    expect(tagEdit.exists()).toBe(true)
    await tagEdit.findAll('input')[1].setValue('runtime')
    await tagEdit.trigger('submit')
    await flushPromises()
    expect(api.updateAllocationTag).toHaveBeenCalledWith(1, 51, expect.objectContaining({ value: 'runtime' }))
  })
})
