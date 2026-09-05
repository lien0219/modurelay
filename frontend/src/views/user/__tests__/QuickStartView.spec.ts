import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { useAppStore } from '@/stores/app'
import type { PublicSettings } from '@/types'
import QuickStartView from '../QuickStartView.vue'

const translate = vi.hoisted(() => (key: string, params?: Record<string, unknown>) => {
  const values: Record<string, string> = {
    'quickStart.title': 'Quick start',
    'quickStart.stepsTitle': 'Connect in four steps',
    'quickStart.steps.apiKey.title': 'Create an API key',
    'quickStart.steps.connection.title': 'Keep the URL and key together',
    'quickStart.steps.codex.title': 'Configure Codex',
    'quickStart.steps.ccswitch.title': 'Import into CC Switch',
    'quickStart.lookup.submit': 'Query models',
    'quickStart.lookup.success': '{count} models returned',
    'quickStart.lookup.modelCopied': 'Model ID copied',
    'quickStart.guides.codex.configPathUnix': 'macOS / Linux: ~/.codex/config.toml',
    'quickStart.guides.codex.configPathWindows': 'Windows: %USERPROFILE%\\.codex\\config.toml',
    'quickStart.guides.ccswitch.configFileValue': 'No file needs manual editing. CC Switch manages the provider inside the application.',
  }
  const value = values[key] || key
  return params ? value.replace('{count}', String(params.count ?? '')) : value
})

vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({ t: translate }),
}))

vi.mock('@/composables/useClipboard', async () => {
  const { ref } = await vi.importActual<typeof import('vue')>('vue')
  return { useClipboard: () => ({ copied: ref(false), copyToClipboard: vi.fn().mockResolvedValue(true) }) }
})

function mountView(apiBaseUrl = '') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const appStore = useAppStore(pinia)
  vi.spyOn(appStore, 'fetchPublicSettings').mockResolvedValue(
    apiBaseUrl ? ({ api_base_url: apiBaseUrl } as PublicSettings) : null
  )

  return mount(QuickStartView, {
    global: {
      plugins: [pinia],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        RouterLink: { template: '<a><slot /></a>' },
        Icon: { template: '<span />' },
      },
    },
  })
}

describe('QuickStartView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('renders the user-facing connection flow without channel access or completion checkboxes', () => {
    const wrapper = mountView()

    expect(wrapper.findAll('article')).toHaveLength(6)
    expect(wrapper.text()).toContain('Create an API key')
    expect(wrapper.text()).toContain('Configure Codex')
    expect(wrapper.text()).toContain('Import into CC Switch')
    expect(wrapper.findAll('input[type="checkbox"]')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('Available channels')
    expect(wrapper.findAll('img')).toHaveLength(0)
    expect(wrapper.text()).toContain('~/.codex/config.toml')
    expect(wrapper.text()).toContain('%USERPROFILE%\\.codex\\config.toml')
    expect(wrapper.text()).toContain('No file needs manual editing')
    expect(wrapper.text()).toContain('[model_providers.modurelay]')
    expect(wrapper.text()).toContain('env_key = "MODURELAY_API_KEY"')
  })

  it('prefers the configured production API URL and adds /v1 once', async () => {
    const wrapper = mountView('https://api.production.example.com/')

    await flushPromises()

    expect((wrapper.get('#quick-start-api-url').element as HTMLInputElement).value)
      .toBe('https://api.production.example.com/v1')
    expect(wrapper.text()).toContain('base_url = "https://api.production.example.com/v1"')
    expect(wrapper.text()).not.toContain('192.168.1.2:3001')
  })

  it('queries model IDs with the entered URL and API key', async () => {
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ data: [{ id: 'gpt-5.5' }, { id: 'claude-sonnet' }] }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const wrapper = mountView()

    await wrapper.find('#quick-start-api-url').setValue('https://gateway.example.com')
    await wrapper.find('#quick-start-api-key').setValue('sk-user-test')
    await wrapper.find('form').trigger('submit')

    expect(fetchMock).toHaveBeenCalledWith(
      'https://gateway.example.com/v1/models',
      expect.objectContaining({
        headers: {
          Accept: 'application/json',
          Authorization: 'Bearer sk-user-test',
        },
      })
    )
    expect(wrapper.text()).toContain('2 models returned')
    expect(wrapper.text()).toContain('gpt-5.5')
    expect(wrapper.text()).toContain('claude-sonnet')
  })
})
