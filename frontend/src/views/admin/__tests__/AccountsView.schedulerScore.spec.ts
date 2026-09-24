import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getUpstreamBillingProbeSettings: vi.fn().mockResolvedValue({ enabled: true, interval_minutes: 30 }),
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

// Render the scheduler-score cell slot for every row so the fallback logic is observable.
const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <div v-for="row in data" :key="row.id" :data-test="'scheduler-score-' + row.id">
        <slot name="cell-scheduler_score" :row="row" />
      </div>
      <div v-for="row in data" :key="'health-' + row.id" :data-test="'health-score-' + row.id">
        <slot name="cell-health_score" :row="row" />
      </div>
    </div>
  `
}

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        HelpTooltip: true,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountHealthLiquidGauge: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

const baseAccount = {
  platform: 'openai',
  type: 'apikey',
  status: 'active',
  schedulable: true,
  concurrency: 1,
  priority: 0,
  error_message: null,
  last_used_at: null,
  expires_at: null,
  auto_pause_on_expired: false,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z'
}

describe('admin AccountsView scheduler score column', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [
        {
          ...baseAccount,
          id: 1,
          name: 'ungrouped-openai',
          // 未分组账号：后端只返回基础分（scheduler_score），无分组维度分数
          scheduler_score: {
            base_score: 1.234567,
            sticky_score: 0,
            sticky_weighted_enabled: false
          },
          health: {
            score: 92,
            state: 'healthy',
            sample_count: 24,
            error_rate_ewma: 0.02,
            latency_ewma_ms: 180,
            consecutive_failures: 0,
            open_count: 0,
            updated_at_unix: 1767225600
          }
        },
        {
          ...baseAccount,
          id: 2,
          name: 'grouped-openai',
          scheduler_score: {
            base_score: 2,
            sticky_score: 3,
            sticky_weighted_enabled: true
          },
          scheduler_scores: [
            {
              group_id: 5,
              group_name: 'group-five',
              base_score: 2,
              sticky_score: 3,
              sticky_weighted_enabled: true
            }
          ]
        },
        {
          ...baseAccount,
          id: 3,
          name: 'no-score',
          platform: 'anthropic'
        },
        {
          ...baseAccount,
          id: 4,
          name: 'paused-openai',
          schedulable: false,
          health: {
            score: 100,
            state: 'warming',
            sample_count: 0,
            error_rate_ewma: 0,
            latency_ewma_ms: 0,
            consecutive_failures: 0,
            open_count: 0,
            updated_at_unix: 1767225600
          }
        }
      ],
      total: 4,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('falls back to the base score for ungrouped accounts instead of showing a dash', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_scheduler_score: '0'
    }))

    const ungroupedCell = wrapper.find('[data-test="scheduler-score-1"]')
    expect(ungroupedCell.exists()).toBe(true)
    expect(ungroupedCell.text()).toContain('1.234567')
    expect(ungroupedCell.text()).toContain('admin.accounts.schedulerScore.ungrouped')
    expect(ungroupedCell.text()).not.toBe('-')
  })

  it('requests and renders the unified health score by default', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_health_score: '1'
    }))
    const healthCell = wrapper.find('[data-test="health-score-1"]')
    expect(healthCell.exists()).toBe(true)
    expect(healthCell.text()).toContain('92')
    expect(healthCell.text()).toContain('admin.accounts.healthScore.states.healthy')
  })

  it('marks health sampling as paused and grays it out when scheduling is disabled', async () => {
    const wrapper = mountView()
    await flushPromises()

    const healthCell = wrapper.find('[data-test="health-score-4"]')
    expect(healthCell.exists()).toBe(true)
    expect(healthCell.text()).toContain('100')
    expect(healthCell.text()).toContain('admin.accounts.healthScore.samplingPaused')
    expect(healthCell.text()).not.toContain('admin.accounts.healthScore.states.warming')

    const state = healthCell.find('[data-testid="account-health-state"]')
    expect(state.classes()).toContain('bg-gray-100')
    expect(state.classes()).toContain('text-gray-500')
  })

  it('omits the health lookup when the health column is hidden', async () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['health_score']))
    localStorage.setItem('account-hidden-columns-version', 'scheduler-score-hidden-by-default')

    mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_health_score: '0'
    }))
  })

  it('renders per-group scores for grouped accounts', async () => {
    const wrapper = mountView()
    await flushPromises()

    const groupedCell = wrapper.find('[data-test="scheduler-score-2"]')
    expect(groupedCell.exists()).toBe(true)
    expect(groupedCell.text()).toContain('group-five')
    expect(groupedCell.text()).toContain('2')
  })

  it('keeps scheduler score hidden for old saved column settings until the admin opts in again', async () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['today_stats']))

    mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_scheduler_score: '0'
    }))
    expect(JSON.parse(localStorage.getItem('account-hidden-columns') || '[]')).toContain('scheduler_score')
  })

  it('requests scheduler scores when the migrated column settings explicitly show the column', async () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['today_stats']))
    localStorage.setItem('account-hidden-columns-version', 'scheduler-score-hidden-by-default')

    mountView()
    await flushPromises()

    expect(listAccounts.mock.calls[0]?.[2]).toEqual(expect.objectContaining({
      include_scheduler_score: '1'
    }))
  })

  it('still shows a dash when no scheduler score is available', async () => {
    const wrapper = mountView()
    await flushPromises()

    const emptyCell = wrapper.find('[data-test="scheduler-score-3"]')
    expect(emptyCell.exists()).toBe(true)
    expect(emptyCell.text()).toBe('-')
  })
})
