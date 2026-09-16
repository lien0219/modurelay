import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import EmailVerificationView from '../EmailVerificationView.vue'

const { emailAPI, showError, showSuccess } = vi.hoisted(() => ({
  emailAPI: {
    services: vi.fn(),
    quotes: vi.fn(),
    orders: vi.fn(),
    order: vi.fn(),
    purchase: vi.fn(),
    cancel: vi.fn(),
    requestRefund: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/email', () => ({ emailAPI }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const quote = {
  channel_code: 'email_channel_1',
  public_name: 'Public channel 1',
  email_type: 'temporary_gmail',
  privacy_level: 'public_temporary',
  sale_price: 0.3,
  success_rate: 0.96,
  success_rate_grade: 'S',
  success_rate_sample_count: 30,
  estimated_delivery_seconds: 8,
  retention_description: '7 days',
  refund_policy_description: 'refund_if_no_message',
  quote_id: 'quote-1',
  quote_expires_at: new Date(Date.now() + 30_000).toISOString(),
  capabilities: {},
}

describe('EmailVerificationView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    emailAPI.services.mockResolvedValue([{ code: 'google', name: 'Google' }])
    emailAPI.quotes.mockResolvedValue([quote])
    emailAPI.orders.mockResolvedValue([])
    emailAPI.purchase.mockResolvedValue({ id: 'order-1' })
  })

  it('shows the public-inbox warning and purchases the signed quote idempotently', async () => {
    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('email.user.privacyWarning')
    expect(wrapper.text()).toContain('email.user.refundPolicies.refund_if_no_message')

    const purchase = wrapper.findAll('button').find(button => button.text() === 'email.user.purchase')
    expect(purchase).toBeDefined()
    await purchase!.trigger('click')
    await flushPromises()

    expect(emailAPI.purchase).toHaveBeenCalledWith({
      channel_code: 'email_channel_1',
      service_code: 'google',
      address_type: 'gmail',
      expected_price: 0.3,
      quote_id: 'quote-1',
    }, 'email-quote-1')
    expect(emailAPI.orders).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: expect.any(Number) }))
    expect(wrapper.get('div.mx-auto').classes()).toContain('max-w-[1800px]')
    wrapper.unmount()
  })
})
