import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import EmailManagementView from '../EmailManagementView.vue'

const { adminEmail, showError, showSuccess } = vi.hoisted(() => ({
  adminEmail: {
    providers: vi.fn(),
    channels: vi.fn(),
    stats: vi.fn(),
    settings: vi.fn(),
    updateSettings: vi.fn(),
    updateProvider: vi.fn(),
    testProvider: vi.fn(),
    updateChannel: vi.fn(),
    setEnabled: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/admin/email', () => ({ default: adminEmail }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

describe('EmailManagementView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminEmail.providers.mockResolvedValue([{
      id: 1, code: 'emailnator', name: 'Emailnator', base_url: 'https://gmailnator.p.rapidapi.com',
      health_status: 'unknown', enabled: false, requires_credential: true, credential_configured: false, billing: {},
    }])
    adminEmail.channels.mockResolvedValue([{
      id: 2, code: 'email_channel_1', public_name: 'Public channel 1', provider_name: 'emailnator',
      enabled: false, visible: true, healthy: false, email_type: 'temporary_gmail', privacy_level: 'public_temporary',
      sale_price: 0.3, refund_policy: 'refund_if_no_message', capture_policy: 'on_target_email_received',
      order_ttl_seconds: 900, base_markup: 0, fixed_markup: 0, minimum_profit: 0,
      polling_backoff: [2, 4, 7], max_provider_requests_per_order: 60,
    }])
    adminEmail.stats.mockResolvedValue({ feature_enabled: false, orders: 0 })
    adminEmail.settings.mockResolvedValue({
      enabled: false,
      free_daily_limit: 20,
      free_active_limit: 3,
      free_generation_interval_seconds: 5,
    })
    adminEmail.updateSettings.mockImplementation(async (value: unknown) => value)
    adminEmail.updateChannel.mockResolvedValue({ updated: true })
  })

  it('protects credential entry and validates then saves polling controls', async () => {
    const wrapper = mount(EmailManagementView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Toggle: { props: ['modelValue', 'label', 'disabled'], template: '<button type="button" :disabled="disabled">{{ label }}</button>' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('input[placeholder="email.admin.credentialPlaceholder"]').attributes('type')).toBe('password')
    const backoff = wrapper.get('input[placeholder="email.admin.backoffPlaceholder"]')
    const channelSave = () => wrapper.findAll('button').filter(button => button.text() === 'email.admin.save').at(-1)!

    await backoff.setValue('2, nope')
    await channelSave().trigger('click')
    expect(showError).toHaveBeenCalledWith('email.admin.invalidBackoff')
    expect(adminEmail.updateChannel).not.toHaveBeenCalled()

    await backoff.setValue('2, 4, 9')
    await channelSave().trigger('click')
    await flushPromises()

    expect(adminEmail.updateChannel).toHaveBeenCalledWith(2, expect.objectContaining({
      order_ttl_seconds: 900,
      max_provider_requests_per_order: 60,
      polling_backoff: [2, 4, 9],
    }))
    expect(showSuccess).toHaveBeenCalledWith('email.admin.saved')
  })

  it('requires a credential before the first provider save', async () => {
    const wrapper = mount(EmailManagementView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Toggle: { props: ['modelValue', 'label', 'disabled'], template: '<button type="button" :disabled="disabled">{{ label }}</button>' },
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').filter((button) => button.text() === 'email.admin.save')[0].trigger('click')
    expect(showError).toHaveBeenCalledWith('email.admin.credentialRequired')
    expect(adminEmail.updateProvider).not.toHaveBeenCalled()

    await wrapper.get('input[placeholder="email.admin.credentialPlaceholder"]').setValue('rapid-api-secret')
    await wrapper.findAll('button').filter((button) => button.text() === 'email.admin.save')[0].trigger('click')
    await flushPromises()
    expect(adminEmail.updateProvider).toHaveBeenCalledWith(1, expect.objectContaining({ credential_ref: 'rapid-api-secret' }))
    expect((wrapper.get('input[placeholder="email.admin.credentialPlaceholder"]').element as HTMLInputElement).value).toBe('')
  })

  it('shows localized configuration guidance and two scrollable full-width tables', async () => {
    const wrapper = mount(EmailManagementView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
          Toggle: {
            props: ['modelValue', 'label', 'disabled'],
            template: '<button type="button" :disabled="disabled">{{ label }}</button>',
          },
        },
      },
    })
    await flushPromises()

    expect(wrapper.get('main > div').classes()).toContain('max-w-[1800px]')
    expect(wrapper.text()).toContain('email.admin.billingGuide')
    expect(wrapper.text()).toContain('email.admin.billingHints.costMode')
    expect(wrapper.text()).toContain('email.admin.costModes.request_based_plus_amortized')
    expect(wrapper.text()).toContain('email.admin.refundPolicies.refund_if_no_message')
    expect(wrapper.text()).toContain('email.admin.capturePolicies.on_target_email_received')
    expect(wrapper.text()).toContain('common.disabled')

    const scrollRegions = wrapper.findAll('[tabindex="0"]')
    expect(scrollRegions).toHaveLength(2)
    expect(scrollRegions.every(region => region.classes().includes('overflow-x-auto'))).toBe(true)
    expect(scrollRegions[0].get('table').classes()).toContain('w-full')
    expect(scrollRegions[0].get('table').classes()).toContain('min-w-[1400px]')
    expect(scrollRegions[1].get('table').classes()).toContain('w-full')
    expect(scrollRegions[1].get('table').classes()).toContain('min-w-[2800px]')
    expect(scrollRegions[0].get('tbody td').classes()).toContain('whitespace-nowrap')
    expect(scrollRegions[0].findAll('button').filter(button => button.text() === 'email.admin.save' || button.text() === 'email.admin.testConnection').every(button => button.classes().includes('whitespace-nowrap'))).toBe(true)
    expect(wrapper.text()).not.toContain('email.admin.tableScrollHint')
  })
})
