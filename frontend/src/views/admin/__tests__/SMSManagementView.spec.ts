import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SMSManagementView from '../SMSManagementView.vue'

const { adminSMS, showError, showSuccess } = vi.hoisted(() => ({
  adminSMS: {
    providers: vi.fn(),
    channels: vi.fn(),
    stats: vi.fn(),
    updateProvider: vi.fn(),
    testProvider: vi.fn(),
    providerMappings: vi.fn(),
    updateProviderServiceMapping: vi.fn(),
    updateProviderCountryMapping: vi.fn(),
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
    adminSMS.updateProvider.mockResolvedValue({ updated: true })
    adminSMS.testProvider.mockResolvedValue({ healthy: true, health_status: 'healthy', latency_ms: 12 })
    adminSMS.providerMappings.mockResolvedValue({ items: [{ kind: 'service', target_id: 2, internal_code: 'openai', internal_name: 'OpenAI', provider_code: '', provider_name: '', temporary_supported: true, rental_supported: false, enabled: false }], total: 1, page: 1, page_size: 20, pages: 1 })
    adminSMS.updateProviderServiceMapping.mockResolvedValue({ updated: true })
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

  it('requires and saves an explicit provider service mapping', async () => {
    const wrapper = mount(SMSManagementView, {
      global: { stubs: { AppLayout: { template: '<main><slot /></main>' }, Icon: true, Pagination: true } },
    })
    await flushPromises()

    await wrapper.findAll('button').find((button) => button.text() === 'sms.admin.mappings')!.trigger('click')
    await flushPromises()
    const mappingInput = wrapper.findAll('input').find((input) => input.attributes('placeholder') === 'openai')!
    const mappingEnabled = wrapper.findAll('input[type="checkbox"]').at(-1)!
    await mappingEnabled.setValue(true)
    const mappingSave = wrapper.findAll('button').filter((button) => button.text() === 'sms.admin.save').at(-1)!
    await mappingSave.trigger('click')
    expect(showError).toHaveBeenCalledWith('sms.admin.mappingCodeRequired')
    expect(adminSMS.updateProviderServiceMapping).not.toHaveBeenCalled()

    await mappingInput.setValue('openai-provider-code')
    await mappingSave.trigger('click')
    await flushPromises()
    expect(adminSMS.updateProviderServiceMapping).toHaveBeenCalledWith(1, 2, expect.objectContaining({ provider_code: 'openai-provider-code', enabled: true }))
  })
})
