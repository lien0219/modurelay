import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SMSManagementView from '../SMSManagementView.vue'

const { adminSMS, showError, showSuccess } = vi.hoisted(() => ({
  adminSMS: {
    providers: vi.fn(),
    channels: vi.fn(),
    stats: vi.fn(),
    pricing: vi.fn(),
    updatePricing: vi.fn(),
    updateProvider: vi.fn(),
    testProvider: vi.fn(),
    syncCatalog: vi.fn(),
    catalogSyncStatus: vi.fn(),
    updateChannel: vi.fn(),
    setEnabled: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin/sms', () => ({ default: adminSMS }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('SMSManagementView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminSMS.providers.mockResolvedValue([{
      id: 1, code: '5sim', name: '5SIM', base_url: 'https://5sim.net/v1',
      health_status: 'unknown', enabled: false, credential_configured: false,
      capabilities: {},
    }])
    adminSMS.channels.mockResolvedValue([])
    adminSMS.stats.mockResolvedValue({ feature_enabled: false, orders: 0 })
    adminSMS.pricing.mockResolvedValue({
      cost_multiplier: 1.3, fixed_markup: 0, unknown_grade_multiplier: 1,
      unknown_grade_fixed_markup: 0, temporary_expiry_minutes: 10,
      self_service_cancel_after_minutes: 1,
      batch_purchase_limit: 5,
      grade_multipliers: {}, grade_fixed_markups: {},
    })
    adminSMS.updatePricing.mockImplementation(async (value: unknown) => value)
    adminSMS.updateProvider.mockResolvedValue({ updated: true })
    adminSMS.testProvider.mockResolvedValue({ healthy: true, health_status: 'healthy', latency_ms: 12 })
    adminSMS.catalogSyncStatus.mockResolvedValue({
      provider: '5sim', status: 'succeeded', duration_ms: 10,
      service_count: 1200, country_count: 80, stale: false, source: 'provider',
    })
    adminSMS.syncCatalog.mockResolvedValue({ synced: true })
  })

  it('requires a credential before the first provider save', async () => {
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    const save = wrapper.findAll('button').find((button) => button.text() === 'sms.admin.save')!
    await save.trigger('click')
    expect(showError).toHaveBeenCalledWith('sms.admin.credentialRequired')
    expect(adminSMS.updateProvider).not.toHaveBeenCalled()
  })

  it('submits a directly entered credential and clears the draft', async () => {
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    const input = wrapper.get('input[placeholder="sms.admin.credentialPlaceholder"]')
    await input.setValue('sms-provider-secret')
    await wrapper.findAll('button').find((button) => button.text() === 'sms.admin.save')!.trigger('click')
    await flushPromises()

    expect(adminSMS.updateProvider).toHaveBeenCalledWith(1, expect.objectContaining({ credential_ref: 'sms-provider-secret' }))
    expect((input.element as HTMLInputElement).value).toBe('')
  })

  it('tests supported providers without purchasing a number', async () => {
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    const testButton = wrapper.findAll('button').find((button) => button.text() === 'sms.admin.testConnection')
    expect(testButton).toBeTruthy()
    await testButton!.trigger('click')
    await flushPromises()

    expect(adminSMS.testProvider).toHaveBeenCalledWith(1)
    expect(showSuccess).toHaveBeenCalledWith('sms.admin.testSuccess (12ms)')
  })

  it('saves the administrator-configured batch purchase limit', async () => {
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    const batchLimitLabel = wrapper.findAll('label').find(label => label.text().includes('sms.admin.batchPurchaseLimit'))
    expect(batchLimitLabel).toBeDefined()
    await batchLimitLabel!.get('input').setValue('8')
    await wrapper.findAll('button').find(button => button.text() === 'sms.admin.savePricing')!.trigger('click')
    await flushPromises()

    expect(adminSMS.updatePricing).toHaveBeenCalledWith(expect.objectContaining({ batch_purchase_limit: 8 }))
  })

  it('keeps management tables scrollable instead of compressing columns on narrow screens', async () => {
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    const providerTable = wrapper.findAll('table').find(table => table.classes().includes('min-w-[1280px]'))
    const channelTable = wrapper.findAll('table').find(table => table.classes().includes('min-w-[820px]'))

    expect(providerTable).toBeDefined()
    expect(channelTable).toBeDefined()
    expect(providerTable!.get('thead').classes()).toContain('whitespace-nowrap')
    expect(channelTable!.get('thead').classes()).toContain('whitespace-nowrap')
  })

  it('shows provider-native catalog status instead of mapping configuration', async () => {
    adminSMS.providers.mockResolvedValueOnce([{
      id: 1, code: '5sim', name: '5SIM', base_url: 'https://5sim.net/v1',
      health_status: 'healthy', enabled: true, credential_configured: true,
      capabilities: {},
    }])
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true } },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('sms.admin.catalogStats')
    expect(wrapper.text()).not.toContain('sms.admin.mappings')

    const sync = wrapper.findAll('button').find((button) => button.text() === 'sms.admin.syncCatalog')!
    await sync.trigger('click')
    await flushPromises()
    expect(adminSMS.syncCatalog).toHaveBeenCalledWith('5sim')
  })
})
