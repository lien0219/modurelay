import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SMSVerificationView from '../SMSVerificationView.vue'

const { smsAPI, showError, showSuccess, refreshUser } = vi.hoisted(() => ({
  smsAPI: {
    settings: vi.fn(),
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
    'sms.user.capabilities.cancelRefund': '未收到验证码之前允许取消订单并自动退款',
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
    smsAPI.settings.mockResolvedValue({ batch_purchase_limit: 5 })
    smsAPI.recentSuccesses.mockResolvedValue({ source: 'mock', real_success_count: 0, items: [{ username: 'te***', country_code: 'US', phone: '+120****123' }] })
    smsAPI.providers.mockResolvedValue([
      { code: 'channel_1', name: 'Channel 1', beta: false, selectable: true, capabilities },
      { code: 'channel_2', name: 'Channel 2', beta: true, selectable: false, capabilities },
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

    expect(smsAPI.providerServicesPage).toHaveBeenCalledWith('channel_1', expect.objectContaining({ product_type: 'temporary', page: 1 }))
    expect(smsAPI.serviceCountriesPage).toHaveBeenCalledWith('channel_1', 'openai', expect.objectContaining({ product_type: 'temporary', page: 1 }))
    expect(wrapper.text()).toContain('OpenAI')
    expect(wrapper.text()).not.toContain('5sim')
    expect(wrapper.text()).not.toContain('smspool')
    expect(wrapper.text()).toContain('BETA')
    wrapper.unmount()
  })

  it('uses the backend service icon in the selected-service preview', async () => {
    const icon = '/api/v1/sms/service-icons/custom-service'
    smsAPI.providerServicesPage.mockResolvedValueOnce({
      items: [{ code: 'custom-service', name: 'Custom service', icon, stock: 10, starting_price: 0.5 }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      has_more: false,
    })
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
          Select: true,
          ConfirmDialog: true,
          SMSServiceLogo: {
            props: ['icon', 'label'],
            template: '<span data-test="sms-service-logo" :data-icon="icon" :data-label="label" />',
          },
        },
      },
    })
    await flushPromises()

    const logos = wrapper.findAll('[data-test="sms-service-logo"]')
    expect(logos).toHaveLength(2)
    expect(logos[0].attributes('data-icon')).toBe(icon)
    expect(logos[1].attributes('data-icon')).toBe(icon)
    wrapper.unmount()
  })

  it('disables channel 1 while the long-term number tab is selected', async () => {
    smsAPI.providers.mockResolvedValueOnce([
      { code: 'channel_1', name: 'Channel 1', beta: false, selectable: true, capabilities: { ...capabilities, supports_rental: false } },
      { code: 'channel_2', name: 'Channel 2', beta: false, selectable: true, capabilities },
    ])
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

    const providerButtons = () => wrapper.findAll('button').filter(button => button.text().includes('sms.user.channel'))
    await providerButtons()[1].trigger('click')
    await flushPromises()

    const rentalTab = wrapper.findAll('[role="tab"]').find(tab => tab.text() === 'sms.user.rental')
    expect(rentalTab).toBeDefined()
    await rentalTab!.trigger('click')
    await flushPromises()

    const channelOne = providerButtons()[0]
    expect(channelOne.attributes('disabled')).toBeDefined()
    expect(channelOne.text()).toContain('sms.user.rentalUnavailable')
    await channelOne.trigger('click')
    expect(providerButtons()[1].classes()).toContain('border-primary-500')
    wrapper.unmount()
  })

  it('uses the administrator-configured batch purchase limit', async () => {
    smsAPI.settings.mockResolvedValueOnce({ batch_purchase_limit: 3 })
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

    const quantityLabel = wrapper.findAll('label').find(label => label.text().includes('sms.user.quantity'))
    expect(quantityLabel).toBeDefined()
    expect(quantityLabel!.get('input').attributes('max')).toBe('3')
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

    expect(smsAPI.serviceCountriesPage).toHaveBeenCalledWith('channel_1', 'openai', expect.objectContaining({ sort: 'recommended' }))
    expect(wrapper.text()).toContain('81.82')
    expect(wrapper.text()).toContain('78.40')
    expect(wrapper.text()).toContain('125')
    wrapper.unmount()
  })

  it('shows the automatic refund policy when temporary quotes support cancellation and refund', async () => {
    smsAPI.quotes.mockResolvedValue([{
      channel_code: 'channel_1',
      public_name: 'Channel 1',
      channel_role: 'primary',
      sale_price: 0.5,
      stock: 10,
      success_rate_source: 'provider',
      estimated_delivery_seconds: 30,
      capabilities,
      quote_id: 'quote-1',
      quote_expires_at: '2099-01-01T00:00:00Z',
    }])
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
    const countryButton = wrapper.findAll('button').find(button => button.text().includes('美国'))
    expect(countryButton).toBeDefined()
    await countryButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('未收到验证码之前允许取消订单并自动退款')
    expect(wrapper.text()).not.toContain('sms.user.capabilities.cancel')
    expect(wrapper.text()).not.toContain('sms.user.capabilities.refund')
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

  it('keeps order columns readable inside a keyboard-scrollable wide table', async () => {
    const messageText = 'Your OpenAI verification code is 757119. Do not share it with anyone.'
    smsAPI.orders.mockResolvedValue({
      items: [{
        ...order('completed', 'sms-wide-table'),
        refund_status: 'approved',
        messages: [{ id: 'message-1', verification_code: '757119', message_text: messageText }],
      }],
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

    const scrollRegion = wrapper.get('.sms-orders-scroll')
    const table = wrapper.get('.sms-orders-table')
    expect(scrollRegion.classes()).toContain('overflow-x-auto')
    expect(scrollRegion.attributes('tabindex')).toBe('0')
    expect(table.classes()).toContain('min-w-[2200px]')
    expect(table.classes()).toContain('table-fixed')
    expect(table.get('thead').classes()).toContain('whitespace-nowrap')
    expect(table.findAll('tbody td')).toHaveLength(11)
    expect(table.findAll('tbody td').every(cell => cell.classes().includes('whitespace-nowrap'))).toBe(true)
    expect(table.get('.sms-order-statuses').classes()).toContain('min-w-max')
    expect(table.get('.sms-order-message-text').classes()).toContain('truncate')
    expect(table.get('.sms-order-message-text').attributes('title')).toBe(messageText)
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

  it('keeps cancellation as the only user refund entry point', async () => {
    smsAPI.orders.mockResolvedValue({
      items: [{ ...order('active', 'sms-active'), capabilities: { ...capabilities, supports_refund: true } }],
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

    expect(wrapper.text()).toContain('sms.user.cancel')
    expect(wrapper.text()).not.toContain('sms.user.requestRefund')
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

  it('renders the cancellation action but disables it during the safety window', async () => {
    smsAPI.orders.mockResolvedValue({
      items: [{ ...order('active', 'sms-safe-window'), cancel_remaining_seconds: 42, can_cancel: false }],
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

    const cancelButton = wrapper.findAll('button').find(button => button.text() === 'sms.user.cancel')
    expect(cancelButton).toBeDefined()
    expect(cancelButton!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })

  it('enables cancellation only when the backend marks the safety window elapsed', async () => {
    smsAPI.orders.mockResolvedValue({
      items: [{ ...order('active', 'sms-cancel-ready'), cancel_remaining_seconds: 0, can_cancel: true }],
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

    const cancelButton = wrapper.findAll('button').find(button => button.text() === 'sms.user.cancel')
    expect(cancelButton).toBeDefined()
    expect(cancelButton!.attributes('disabled')).toBeUndefined()
    wrapper.unmount()
  })

  it('stops the expiry countdown and labels cancelled/refunded orders as ended', async () => {
    smsAPI.orders.mockResolvedValue({
      items: [{ ...order('cancelled', 'sms-ended'), refund_status: 'approved', expires_at: '2099-01-01T00:00:00Z' }],
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

    expect(wrapper.text()).toContain('sms.user.ended')
    wrapper.unmount()
  })

  it('stops the expiry countdown while cancellation/refund reconciliation is pending', async () => {
    smsAPI.orders.mockResolvedValue({
      items: [{ ...order('reconciling', 'sms-refund-reconciling'), reconciliation_action: 'refund', refund_status: 'pending', expires_at: '2099-01-01T00:00:00Z' }],
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

    expect(wrapper.text()).toContain('sms.user.ended')
    wrapper.unmount()
  })
})
