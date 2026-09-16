import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SMSVerificationView from '../SMSVerificationView.vue'

const { smsAPI, showError, showSuccess } = vi.hoisted(() => ({
  smsAPI: {
    services: vi.fn(),
    countries: vi.fn(),
    quotes: vi.fn(),
    orders: vi.fn(),
    order: vi.fn(),
    purchase: vi.fn(),
    cancel: vi.fn(),
    refund: vi.fn(),
    extendRental: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/sms', () => ({ smsAPI }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const messages: Record<string, string> = {
    'sms.user.statuses.pending': '等待处理',
    'sms.user.statuses.expired': '已过期',
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

const order = (status: string, id: string) => ({
  id,
  product_type: 'temporary',
  status,
  channel_code: 'sms-channel',
  channel_name: 'SMS channel',
  service_code: 'openai',
  country_code: 'US',
  phone_number: '+12025550123',
  price: 0.5,
  success_rate_source: 'provider',
  refund_status: 'not_requested',
  created_at: '2026-09-16T08:00:00Z',
})

describe('SMSVerificationView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    smsAPI.services.mockResolvedValue([{ code: 'openai', name: 'OpenAI' }])
    smsAPI.countries.mockResolvedValue([{ iso2: 'US', name_zh: '美国', name_en: 'United States' }])
    smsAPI.quotes.mockResolvedValue([])
    smsAPI.orders.mockResolvedValue({
      items: [order('pending', 'sms-1'), order('expired', 'sms-2')],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })
  })

  it('localizes pending and expired order statuses without raw enum fallback', async () => {
    const wrapper = mount(SMSVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Pagination: true,
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
