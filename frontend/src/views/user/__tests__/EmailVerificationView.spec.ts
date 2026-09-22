import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import EmailVerificationView from '../EmailVerificationView.vue'

const { emailAPI, showError, showSuccess } = vi.hoisted(() => ({
  emailAPI: {
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
    emailAPI.quotes.mockResolvedValue([quote])
    emailAPI.orders.mockResolvedValue([])
    emailAPI.purchase.mockResolvedValue({ id: 'order-1' })
  })

  it('renders provider mailbox types through i18n keys', async () => {
    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const privateTab = wrapper.findAll('button').find(button => button.text() === 'email.user.privateInbox')
    expect(privateTab).toBeDefined()
    await privateTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('email.user.gmailReal')
    expect(wrapper.text()).toContain('email.user.gmailAlias')
    expect(wrapper.text()).toContain('email.user.outlookReal')
    expect(wrapper.text()).toContain('email.user.outlookAlias')
    expect(wrapper.text()).not.toContain('Gmail Real')
    expect(wrapper.text()).not.toContain('Outlook Alias')
    wrapper.unmount()
  })

  it('uses provider-native mailbox flow without requiring a verification platform', async () => {
    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).not.toContain('email.user.publicPrivacyWarning')
    expect(wrapper.text()).not.toContain('email.user.privatePrivacyHint')
    expect(wrapper.text()).not.toContain('email.user.selectService')

    const quoteButton = wrapper.findAll('button').find(button => button.text() === 'email.user.getQuote')
    expect(quoteButton).toBeDefined()
    await quoteButton!.trigger('click')
    await flushPromises()

    expect(emailAPI.quotes).toHaveBeenCalledWith({ address_type: 'gmail' })

    const purchase = wrapper.findAll('button').find(button => button.text() === 'email.user.generateFree')
    expect(purchase).toBeDefined()
    await purchase!.trigger('click')
    await flushPromises()

    expect(emailAPI.purchase).toHaveBeenCalledWith({
      channel_code: 'email_channel_1',
      address_type: 'gmail',
      expected_price: 0.3,
      quote_id: 'quote-1',
    }, expect.stringMatching(/^email-quote-1-/))
    expect(wrapper.get('div.mx-auto').classes()).toContain('max-w-[1800px]')
    wrapper.unmount()
  })

  it('does not expose provider verification URLs in the user inbox', async () => {
    emailAPI.purchase.mockResolvedValue({
      id: 'order-1',
      channel_code: 'email_channel_1',
      channel_name: 'Channel 1',
      status: 'waiting_email',
      email_address: 'demo@gmail.com',
      expires_at: new Date(Date.now() + 60_000).toISOString(),
      messages: [{
        id: 'message-1',
        from_name: 'OpenAI',
        from_address: 'noreply@example.com',
        subject: 'Verification code',
        received_at: new Date().toISOString(),
        verification_code: '123456',
        verification_url: 'https://example.com/verify?token=secret',
        text_body: 'Your verification code is 123456',
      }],
    })

    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const quoteButton = wrapper.findAll('button').find(button => button.text() === 'email.user.getQuote')
    await quoteButton!.trigger('click')
    await flushPromises()
    const purchaseButton = wrapper.findAll('button').find(button => button.text() === 'email.user.generateFree')
    await purchaseButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('123456')
    expect(wrapper.text()).toContain('email.user.viewMessage')
    expect(wrapper.text()).not.toContain('https://example.com/verify?token=secret')
    expect(wrapper.text()).not.toContain('email.user.safeOpen')
    wrapper.unmount()
  })


  it('shows the newest verification code by received_at even when messages are unsorted', async () => {
    const older = new Date(Date.now() - 120_000).toISOString()
    const newer = new Date(Date.now() - 30_000).toISOString()
    emailAPI.purchase.mockResolvedValue({
      id: 'order-1',
      channel_code: 'email_channel_1',
      channel_name: 'Channel 1',
      status: 'waiting_email',
      email_address: 'demo@gmail.com',
      expires_at: new Date(Date.now() + 60_000).toISOString(),
      messages: [
        {
          id: 'message-newer',
          from_name: 'OpenAI',
          from_address: 'noreply@example.com',
          subject: 'New code',
          received_at: newer,
          verification_code: '222222',
          text_body: 'New code 222222',
        },
        {
          id: 'message-older',
          from_name: 'OpenAI',
          from_address: 'noreply@example.com',
          subject: 'Old code',
          received_at: older,
          verification_code: '111111',
          text_body: 'Old code 111111',
        },
      ],
    })

    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const quoteButton = wrapper.findAll('button').find(button => button.text() === 'email.user.getQuote')
    await quoteButton!.trigger('click')
    await flushPromises()
    const purchaseButton = wrapper.findAll('button').find(button => button.text() === 'email.user.generateFree')
    await purchaseButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('222222')
    wrapper.unmount()
  })

  it('shows the newest verification code in the orders table even when message order is reversed', async () => {
    const older = new Date(Date.now() - 120_000).toISOString()
    const newer = new Date(Date.now() - 30_000).toISOString()
    emailAPI.orders.mockResolvedValue({
      items: [{
        id: 'order-1',
        order_no: 'EML-1',
        service_code: 'openai',
        channel_code: 'email_channel_1',
        channel_name: 'Channel 1',
        email_address: 'demo@gmail.com',
        address_type: 'gmail',
        price: 0,
        status: 'verification_extracted',
        refund_policy: 'refund_if_no_message',
        capture_policy: 'on_verification_extracted',
        created_at: older,
        refund_status: 'not_applicable',
        messages: [
          { id: 'm-new', from_address: 'noreply@example.com', from_name: 'OpenAI', to_address: 'demo@gmail.com', subject: 'New code', text_body: '222222', received_at: newer, verification_code: '222222' },
          { id: 'm-old', from_address: 'noreply@example.com', from_name: 'OpenAI', to_address: 'demo@gmail.com', subject: 'Old code', text_body: '111111', received_at: older, verification_code: '111111' },
        ],
      }],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const ordersTab = wrapper.findAll('button').find(button => button.text() === 'email.user.orders')
    await ordersTab!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('222222')
    wrapper.unmount()
  })


  it('keeps the orders table mounted during background refresh', async () => {
    const order = {
      id: 'order-1',
      order_no: 'EML-1',
      service_code: 'openai',
      channel_code: 'email_channel_1',
      channel_name: 'Channel 1',
      email_address: 'demo@gmail.com',
      address_type: 'gmail',
      price: 0,
      status: 'verification_extracted',
      refund_policy: 'refund_if_no_message',
      capture_policy: 'on_verification_extracted',
      created_at: new Date().toISOString(),
      refund_status: 'not_applicable',
      messages: [],
    }
    let resolveRefresh: ((value: unknown) => void) | undefined
    emailAPI.orders
      .mockResolvedValueOnce({ items: [order], total: 1, page: 1, page_size: 20, pages: 1 })
      .mockImplementationOnce(() => new Promise(resolve => { resolveRefresh = resolve }))

    vi.useFakeTimers()
    const wrapper = mount(EmailVerificationView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })
    await flushPromises()

    const ordersTab = wrapper.findAll('button').find(button => button.text() === 'email.user.orders')
    await ordersTab!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('demo@gmail.com')

    await vi.advanceTimersByTimeAsync(10_000)
    await flushPromises()

    expect(wrapper.text()).toContain('demo@gmail.com')
    expect(wrapper.text()).not.toContain('email.user.loading')

    resolveRefresh?.({ items: [order], total: 1, page: 1, page_size: 20, pages: 1 })
    await flushPromises()
    wrapper.unmount()
    vi.useRealTimers()
  })

})
