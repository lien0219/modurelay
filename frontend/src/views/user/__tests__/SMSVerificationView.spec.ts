import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SMSVerificationView from '../SMSVerificationView.vue'

const { smsAPI, showError, showSuccess, refreshUser } = vi.hoisted(() => ({
  smsAPI: {
    providers: vi.fn(),
    recentSuccesses: vi.fn(),
    providerServices: vi.fn(),
    providerServicesPage: vi.fn(),
    serviceCountries: vi.fn(),
    serviceCountriesPage: vi.fn(),
    operators: vi.fn(),
    quotes: vi.fn(),
    orders: vi.fn(),
    order: vi.fn(),
    purchase: vi.fn(),
    purchaseBatch: vi.fn(),
    cancel: vi.fn(),
    finish: vi.fn(),
    resend: vi.fn(),
    ban: vi.fn(),
    refund: vi.fn(),
    extendRental: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
  refreshUser: vi.fn(),
}))

vi.mock('@/api/sms', () => ({ smsAPI }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }), useAuthStore: () => ({ refreshUser }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const messages: Record<string, string> = {
    'sms.user.statuses.pending': '等待处理',
    'sms.user.statuses.expired': '已过期',
    'sms.user.statuses.confirmingPurchase': '正在确认下单',
    'sms.user.statuses.unknown': '未知状态',
  }
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'zh-CN' },
      t: (key: string) => messages[key] || key,
    }),
  }
})

const capabilities = {
  supports_temporary: true,
  supports_rental: true,
  supports_rental_cancel: true,
  supports_webhook: false,
  supports_polling: true,
  supports_cancel: true,
  supports_refund: true,
  supports_refund_status: false,
  supports_finish: false,
  supports_ban: false,
  supports_extend: true,
  supports_resend: false,
  supports_voice: true,
  supports_voice_sms: true,
  supports_voice_caller_id: true,
  supports_voice_call: true,
  supports_operator_selection: true,
  supports_service_selection: true,
  supports_conversion_stats: true,
}

const order = (status: string, id: string) => ({
  id,
  product_type: 'temporary',
  status,
  channel_code: 'channel_1',
  channel_name: '渠道1',
  service_code: 'openai',
  country_code: 'US',
  phone_number: '+12025550123',
  price: 0.5,
  success_rate_source: 'provider',
  refund_status: 'not_requested',
  capabilities,
  created_at: '2026-09-16T08:00:00Z',
})

describe('SMSVerificationView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    refreshUser.mockResolvedValue({})
    smsAPI.recentSuccesses.mockResolvedValue({ source: 'mock', real_success_count: 0, items: [{ username: 'te***', country_code: 'US', phone: '+120****123' }] })
    smsAPI.providers.mockResolvedValue([
      { code: '5sim', name: '5SIM', beta: false, selectable: true, capabilities },
      { code: 'smspool', name: 'SMSPool', beta: true, selectable: false, capabilities },
    ])
    smsAPI.providerServices.mockResolvedValue([{ code: 'openai', name: 'OpenAI' }])
    smsAPI.providerServicesPage.mockResolvedValue({ items: [{ code: 'openai', name: 'OpenAI', stock: 10, starting_price: 0.5 }], total: 1, page: 1, page_size: 20, pages: 1, has_more: false })
    smsAPI.serviceCountries.mockResolvedValue([{ iso2: 'US', name_zh: '美国', name_en: 'United States', stock: 10, available: true }])
    smsAPI.serviceCountriesPage.mockResolvedValue({ items: [{ iso2: 'US', name_zh: '美国', name_en: 'United States', stock: 10, starting_price: 0.5, conversion_rate: 81.82, platform_30d_success_rate: 78.4, platform_30d_sample_size: 125, platform_30d_successes: 98, platform_30d_failures: 27, recommended_operator: 'virtual63', recommended_operator_stock: 8, recommended_starting_price: 0.55, available: true }], total: 1, page: 1, page_size: 20, pages: 1, has_more: false })
    smsAPI.operators.mockResolvedValue([{ code: 'any', name: 'Any / 自动选择', stock: 10, platform_30d_success_rate: 78.4, platform_30d_sample_size: 125, available: true }, { code: 'virtual63', name: 'virtual63', stock: 8, provider_rate: 81.82, platform_30d_success_rate: 80, platform_30d_sample_size: 50, available: true }])
    smsAPI.quotes.mockResolvedValue([])
    smsAPI.orders.mockResolvedValue({
      items: [order('pending', 'sms-1'), order('expired', 'sms-2')],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    smsAPI.order.mockImplementation(async (id: string) => order('pending', id))
  })

  it('loads provider-native services and countries from the selected channel', async () => {
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
          Select: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()

    expect(smsAPI.providerServicesPage).toHaveBeenCalledWith('5sim', expect.objectContaining({ product_type: 'temporary', page: 1 }))
    expect(smsAPI.serviceCountriesPage).toHaveBeenCalledWith('5sim', 'openai', expect.objectContaining({ product_type: 'temporary', page: 1 }))
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).not.toContain('5SIM')
    expect(wrapper.text()).not.toContain('SMSPool')
    expect(wrapper.text()).toContain('BETA')
    wrapper.unmount()
  })


  it('shows provider and platform 30-day delivery rates separately', async () => {
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
          Select: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()

    expect(smsAPI.serviceCountriesPage).toHaveBeenCalledWith('5sim', 'openai', expect.objectContaining({ sort: 'recommended' }))
    expect(wrapper.text()).toContain('81.82')
    expect(wrapper.text()).toContain('78.40')
    expect(wrapper.text()).toContain('125')
    wrapper.unmount()
  })

  it('shows copyable order ids with service and country icons', async () => {
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
          Select: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()

    const ordersTab = wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'sms.user.orders')
    expect(ordersTab).toBeDefined()
    await ordersTab!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[aria-label="common.copy sms-1"]').exists()).toBe(true)
    expect(wrapper.find('.fi-us').exists()).toBe(true)
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).toContain('美国')
    wrapper.unmount()
  })

  it('labels purchase reconciliation as confirming purchase', async () => {
    smsAPI.orders.mockResolvedValue({
      items: [{ ...order('reconciling', 'sms-recovery'), reconciliation_action: 'purchase' }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
          Select: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()
    const ordersTab = wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'sms.user.orders')
    await ordersTab!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('正在确认下单')
    wrapper.unmount()
  })

  it('localizes pending and expired order statuses without raw enum fallback', async () => {
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
          Select: true,
          ConfirmDialog: true,
        },
      },
    })
    await flushPromises()

    const ordersTab = wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'sms.user.orders')
    expect(ordersTab).toBeDefined()
    await ordersTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('等待处理')
    expect(wrapper.text()).toContain('已过期')
    expect(wrapper.text()).not.toContain('pending')
    expect(wrapper.text()).not.toContain('expired')
    wrapper.unmount()
  })
})
