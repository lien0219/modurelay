import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Channel } from '@/api/admin/channels'
import ChannelsView from '@/views/admin/ChannelsView.vue'

const { get, post, put, showError } = vi.hoisted(() => ({
  get: vi.fn(), post: vi.fn(), put: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/client', () => ({ apiClient: { get, post, put } }))
vi.mock('@/api/admin', async () => ({
  adminAPI: {
    channels: await import('@/api/admin/channels'),
    groups: { getAll: async () => [
      { id: 10, name: 'Anthropic group', platform: 'anthropic', rate_multiplier: 1 },
      { id: 20, name: 'OpenAI group', platform: 'openai', rate_multiplier: 1 }
    ] },
    settings: { getWebSearchEmulationConfig: async () => ({ enabled: false }) }
  }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

function channelFixture(value: unknown): Channel {
  return {
    id: 1, name: 'Mixed channel', description: 'Preserve me', status: 'active',
    billing_model_source: 'upstream', restrict_models: true,
    features_config: {
      bedrock_cc_compat: value,
      web_search_emulation: { anthropic: true },
      codex_image_generation_bridge: { openai: true },
      unrelated_feature: { nested: ['keep'] }
    },
    group_ids: [10, 20],
    model_mapping: { anthropic: { 'claude-alias': 'claude-opus-4-6' }, openai: { alias: 'gpt-5' } },
    model_pricing: [{
      platform: 'anthropic', models: ['claude-opus-4-6'], billing_mode: 'token',
      input_price: 0.000003, output_price: 0.000015, cache_write_price: null,
      cache_write_1h_price: null, cache_read_price: null, fast_multiplier: null,
      flex_multiplier: null, reasoning_effort_multipliers: null,
      image_input_price: null, image_output_price: null, per_request_price: null,
      intervals: [], time_pricing: null
    }],
    apply_pricing_to_account_stats: false, account_stats_pricing_rules: [],
    created_at: '', updated_at: ''
  }
}

const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value))
let saved: Channel | undefined
const wrappers: ReturnType<typeof mountView>[] = []
function mountView() {
  const wrapper = mount(ChannelsView, {
    global: { stubs: {
      AppLayout: defineComponent({ template: '<main><slot /></main>' }),
      TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
      BaseDialog: defineComponent({ props: ['show'], template: '<section v-if="show"><slot /><slot name="footer" /></section>' }),
      DataTable: defineComponent({ props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>' }),
      PricingEntryCard: true, Pagination: true, ConfirmDialog: true, EmptyState: true,
      Select: true, Icon: true, PlatformIcon: true
    } }
  })
  wrappers.push(wrapper)
  return wrapper
}
type View = ReturnType<typeof mountView>
async function clickText(wrapper: View, text: string) {
  const button = wrapper.findAll('button').find(b => b.text() === text)
  expect(button, text).toBeDefined()
  await button!.trigger('click')
  await flushPromises()
}
function compatToggle(wrapper: View) {
  const label = wrapper.findAll('label').find(l => l.text() === 'admin.channels.form.bedrockCCCompat')!
  return wrapper.findAll('[role="switch"]').find(el => label.element.parentElement!.parentElement!.contains(el.element))!
}
async function edit(wrapper: View) {
  await flushPromises()
  await clickText(wrapper, 'common.edit')
  await clickText(wrapper, 'admin.groups.platforms.anthropic')
}
async function save(wrapper: View) {
  await wrapper.get('#channel-form').trigger('submit')
  await flushPromises()
  expect(showError).not.toHaveBeenCalled()
}

describe('ChannelsView Bedrock CC persistence', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    saved = undefined
    get.mockImplementation(async () => ({ data: { items: saved ? [clone(saved)] : [], total: saved ? 1 : 0 } }))
    post.mockImplementation(async (_url: string, payload: Partial<Channel>) => {
      saved = clone({ ...channelFixture(false), ...payload })
      return { data: clone(saved) }
    })
    put.mockImplementation(async (_url: string, payload: Partial<Channel>) => {
      saved = clone({ ...saved!, ...payload })
      return { data: clone(saved) }
    })
  })
  afterEach(() => wrappers.splice(0).forEach(wrapper => wrapper.unmount()))

  it.each([true, false])('creates a boolean %s and keeps it after a page remount', async (enabled) => {
    const wrapper = mountView()
    await flushPromises()
    await clickText(wrapper, 'admin.channels.createChannel')
    await wrapper.get('input[placeholder="admin.channels.form.namePlaceholder"]').setValue('New channel')
    const platform = wrapper.findAll('label').find(l => l.text() === 'admin.groups.platforms.anthropic')!
    await platform.get('input').setValue(true)
    await clickText(wrapper, 'admin.groups.platforms.anthropic')
    const group = wrapper.findAll('label').find(l => l.text().includes('Anthropic group'))!
    await group.get('input').setValue(true)
    if (enabled) await compatToggle(wrapper).trigger('click')
    await save(wrapper)
    expect(saved!.features_config!.bedrock_cc_compat).toBe(enabled)
    expect(post.mock.calls[0][0]).toBe('/admin/channels')
    wrapper.unmount()
    const refreshed = mountView()
    await edit(refreshed)
    expect(compatToggle(refreshed).attributes('aria-checked')).toBe(String(enabled))
  })

  it('persists on/off/on across refresh and editing while preserving other channel settings', async () => {
    saved = channelFixture(false)
    const original = clone(saved)
    const wrapper = mountView()
    for (const enabled of [true, false, true]) {
      await wrapper.get('button[title="common.refresh"]').trigger('click')
      await edit(wrapper)
      await compatToggle(wrapper).trigger('click')
      await save(wrapper)
      expect(saved!.features_config!.bedrock_cc_compat).toBe(enabled)
      expect(saved).toEqual({ ...original, features_config: { ...original.features_config, bedrock_cc_compat: enabled } })
    }
    await edit(wrapper)
    expect(compatToggle(wrapper).attributes('aria-checked')).toBe('true')
  })

  it.each([
    [{ anthropic: true }, true], [{ anthropic: false }, false],
    [{ anthropic: 'true' }, false], [{ bedrock: true }, false],
    [{ anthropic: true, openai: true }, false], ['true', false], [null, false]
  ])('reads legacy value %j safely and saves a boolean', async (value, enabled) => {
    saved = channelFixture(value)
    const wrapper = mountView()
    await edit(wrapper)
    expect(compatToggle(wrapper).attributes('aria-checked')).toBe(String(enabled))
    await save(wrapper)
    expect(saved!.features_config!.bedrock_cc_compat).toBe(enabled)
  })

  it('clears a saved enabled flag when Anthropic is removed', async () => {
    saved = channelFixture(true)
    const wrapper = mountView()
    await edit(wrapper)
    const platform = wrapper.findAll('label').find(l => l.text() === 'admin.groups.platforms.anthropic')!
    await platform.get('input').setValue(false)
    await save(wrapper)
    expect(saved!.features_config!.bedrock_cc_compat).toBe(false)
  })
})
