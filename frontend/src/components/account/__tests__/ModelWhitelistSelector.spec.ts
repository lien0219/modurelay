import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const {
  copyToClipboard,
  showError,
  showSuccess,
  showInfo,
  showWarning,
  syncUpstreamModels,
  syncUpstreamModelsPreview
} = vi.hoisted(() => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
  showWarning: vi.fn(),
  syncUpstreamModels: vi.fn(),
  syncUpstreamModelsPreview: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => key === 'common.copy' ? '复制' : key === 'admin.accounts.modelMappingConflict' ? `Model mapping conflict: ${params?.from} → ${params?.to}` : key
    })
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showInfo,
    showWarning
  })
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModels,
    syncUpstreamModelsPreview
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard
  })
}))

import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'
import { getModelsByPlatform, getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'

function mountSelector(props: Record<string, unknown> = {}) {
  return mount(ModelWhitelistSelector, {
    props: {
      modelValue: [],
      platform: 'openai',
      ...props,
    },
    global: {
      stubs: {
        ModelIcon: true
      }
    }
  })
}

function findModelRow(wrapper: ReturnType<typeof mountSelector>, modelId: string) {
  const row = wrapper
    .findAll('[data-testid="model-option"]')
    .find(candidate => candidate.text().includes(modelId))

  if (!row) {
    throw new Error(`Model row not found: ${modelId}`)
  }

  return row
}

describe('ModelWhitelistSelector', () => {
  beforeEach(() => {
    copyToClipboard.mockClear()
    showError.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    syncUpstreamModels.mockReset()
    syncUpstreamModelsPreview.mockReset()
  })

  it('uses Seedance video suggestions instead of the Claude fallback', async () => {
    const models = getModelsByPlatform('seedance')
    expect(models).toContain('seedance-2.0')
    expect(models.every(model => /^(doubao-)?seedance-/.test(model))).toBe(true)
    expect(getPresetMappingsByPlatform('seedance')).toEqual([])

    const wrapper = mountSelector({ platform: 'seedance' })
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.fillRelatedModels')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[models]])
  })

  it('syncs actual Seedance models with unsaved account credentials', async () => {
    const credentials = {
      platform: 'seedance',
      type: 'apikey',
      base_url: 'https://seedance.example.com/api/v3',
      api_key: 'test-seedance-key',
    }
    syncUpstreamModelsPreview.mockResolvedValue({
      models: ['ep-video-test', '48:seedance-2.0'],
      metadata: {},
    })
    const wrapper = mountSelector({ platform: 'seedance', syncCredentials: credentials })
    const syncButton = wrapper.findAll('button').find(button => button.text() === 'admin.accounts.syncUpstreamModels')

    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(syncUpstreamModelsPreview).toHaveBeenCalledWith(credentials)
    expect(syncUpstreamModels).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')).toEqual([[['ep-video-test', '48:seedance-2.0']]])
    expect(wrapper.emitted('upstream-synced')).toEqual([[]])
  })

  it('syncs Seedance models for the existing account ID', async () => {
    syncUpstreamModels.mockResolvedValue({ models: ['seedance-2.0'], metadata: {} })
    const wrapper = mountSelector({ platform: 'seedance', accountId: 73 })
    const syncButton = wrapper.findAll('button').find(button => button.text() === 'admin.accounts.syncUpstreamModels')

    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(syncUpstreamModels).toHaveBeenCalledWith(73)
    expect(syncUpstreamModelsPreview).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:modelValue')).toEqual([[['seedance-2.0']]])
  })

  it('shows synced names and full API IDs, searches names, and copies the original ID', async () => {
    const id = 'doubao-seedance-1-0-pro-fast-251015'
    const name = 'Doubao-Seedance-1.0-pro-fast'
    syncUpstreamModels.mockResolvedValue({
      models: [id, 'ep-video-test'],
      metadata: { [id]: { id, display_name: name, output_modalities: ['video'] } },
    })
    const wrapper = mountSelector({ platform: 'seedance', accountId: 73 })
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.syncUpstreamModels')!.trigger('click')
    await flushPromises()
    await wrapper.setProps({ modelValue: [id] })

    expect(wrapper.get('div.cursor-pointer').text()).toContain(name)
    expect(wrapper.get('div.cursor-pointer').text()).toContain(id)
    expect(wrapper.text()).toContain('admin.accounts.upstreamModelCatalogHint')
    await wrapper.get('div.cursor-pointer').trigger('click')
    expect(wrapper.findAll('[data-testid="model-option"]')).toHaveLength(2)
    await wrapper.get('input[placeholder="admin.accounts.searchModels"]').setValue('1.0-pro-fast')
    const rows = wrapper.findAll('[data-testid="model-option"]')
    expect(rows).toHaveLength(1)
    expect(rows[0].text()).toContain(name)
    expect(rows[0].text()).toContain(id)
    await rows[0].get('[data-testid="copy-model-id"]').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledWith(id)
    expect(wrapper.emitted('update:modelValue')).toEqual([[[id, 'ep-video-test']]])
  })

  it('filters Seedance choices by mapped output capabilities without deleting saved whitelist entries', async () => {
    const wrapper = mountSelector({
      platform: 'seedance',
      accountId: 73,
      modelValue: ['video-alias', 'seedance-lookalike', 'ep-custom', 'video-input-text'],
      modelMappings: [{ from: 'video-alias', to: 'actual-video' }],
      modelMetadata: {
        'actual-video': { id: 'actual-video', display_name: 'Video Generator', output_modalities: ['video'] },
        'seedance-lookalike': { id: 'seedance-lookalike', output_modalities: ['image'] },
        'video-input-text': { id: 'video-input-text', input_modalities: ['video'], output_modalities: ['text'] },
      },
    })
    await wrapper.get('div.cursor-pointer').trigger('click')
    const rowIDs = wrapper.findAll('[data-testid="model-option"]').map(row => row.text())
    expect(rowIDs.some(text => text.includes('video-alias') && text.includes('Video Generator'))).toBe(true)
    expect(rowIDs.some(text => text.includes('ep-custom'))).toBe(true)
    expect(rowIDs.some(text => text.includes('seedance-lookalike'))).toBe(false)
    expect(rowIDs.some(text => text.includes('video-input-text'))).toBe(false)
    expect(wrapper.get('div.cursor-pointer').text()).toContain('seedance-lookalike')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('does not reuse another account catalog after switching accounts', async () => {
    syncUpstreamModels.mockResolvedValue({ models: ['ep-old'], metadata: { 'ep-old': { id: 'ep-old', display_name: 'Old Account Video' } } })
    const wrapper = mountSelector({ platform: 'seedance', accountId: 73 })
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.syncUpstreamModels')!.trigger('click')
    await flushPromises()
    await wrapper.setProps({ accountId: 74, modelValue: ['ep-new'] })
    await wrapper.get('div.cursor-pointer').trigger('click')
    expect(wrapper.text()).not.toContain('Old Account Video')
    expect(wrapper.text()).not.toContain('ep-old')
    expect(wrapper.text()).toContain('ep-new')
  })

  it('uses wildcard mapping targets to decide video capability', async () => {
    const wrapper = mountSelector({
      platform: 'seedance',
      modelValue: ['text-v1', 'seedance-video'],
      modelMappings: [{ from: 'text-*', to: 'actual-video' }, { from: 'seedance-*', to: 'actual-text' }],
      modelMetadata: {
        'actual-video': { id: 'actual-video', display_name: 'Video Generator', output_modalities: ['video'] },
        'actual-text': { id: 'actual-text', output_modalities: ['text'] },
        'text-v1': { id: 'text-v1', output_modalities: ['text'] },
        'seedance-video': { id: 'seedance-video', output_modalities: ['video'] },
      },
    })
    await wrapper.get('div.cursor-pointer').trigger('click')
    const rows = wrapper.findAll('[data-testid="model-option"]').map(row => row.text())
    expect(rows.some(text => text.includes('text-v1') && text.includes('Video Generator'))).toBe(true)
    expect(rows.some(text => text.includes('seedance-video'))).toBe(false)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('keeps the whitelist unchanged when Seedance upstream model listing fails', async () => {
    syncUpstreamModelsPreview.mockRejectedValue(new Error('Upstream model listing is unavailable'))
    const wrapper = mountSelector({
      platform: 'seedance',
      modelValue: ['ep-existing-video'],
      syncCredentials: {
        platform: 'seedance', type: 'apikey',
        base_url: 'https://seedance.example.com/api/v3', api_key: 'test-seedance-key',
      },
    })
    const syncButton = wrapper.findAll('button').find(button => button.text() === 'admin.accounts.syncUpstreamModels')
    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.emitted('upstream-synced')).toBeUndefined()
    expect(showError).toHaveBeenCalledOnce()
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('ignores a pending sync result after switching accounts', async () => {
    let finishSync!: (value: { models: string[]; metadata: Record<string, unknown> }) => void
    syncUpstreamModels.mockReturnValue(new Promise(resolve => { finishSync = resolve }))
    const wrapper = mountSelector({ platform: 'seedance', accountId: 73 })
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.syncUpstreamModels')!.trigger('click')
    await wrapper.setProps({ accountId: 74, modelValue: ['ep-new'] })
    finishSync({ models: ['ep-old'], metadata: { 'ep-old': { id: 'ep-old', display_name: 'Old Account Video' } } })
    await flushPromises()
    await wrapper.get('div.cursor-pointer').trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(wrapper.text()).not.toContain('ep-old')
    expect(wrapper.text()).not.toContain('Old Account Video')
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('rejects a custom whitelist model that is already mapped to a different target', async () => {
    const wrapper = mountSelector({ modelMappings: [{ from: 'gpt-latest', to: 'deepseek-chat' }] })
    await wrapper.get('input[placeholder="admin.accounts.enterCustomModelName"]').setValue(' gpt-latest ')
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.addModel')!.trigger('click')

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(showInfo).toHaveBeenCalledWith(expect.stringContaining('gpt-latest → deepseek-chat'))
  })

  it('keeps the existing duplicate identity warning before checking mappings', async () => {
    const wrapper = mountSelector({ modelValue: ['gpt-latest'], modelMappings: [{ from: 'gpt-latest', to: 'deepseek-chat' }] })
    await wrapper.get('input[placeholder="admin.accounts.enterCustomModelName"]').setValue('gpt-latest')
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.addModel')!.trigger('click')
    expect(showInfo).toHaveBeenCalledWith('admin.accounts.modelExists')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('allows matching identity mapping as a whitelist model', async () => {
    const wrapper = mountSelector({ modelMappings: [{ from: 'gpt-latest', to: 'gpt-latest' }] })
    await wrapper.get('input[placeholder="admin.accounts.enterCustomModelName"]').setValue('gpt-latest')
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.addModel')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[['gpt-latest']]])
  })

  it('still allows custom models without a mapping prop', async () => {
    const wrapper = mountSelector()
    await wrapper.get('input[placeholder="admin.accounts.enterCustomModelName"]').setValue('custom-model')
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.addModel')!.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[['custom-model']]])
  })

  it('copies a model ID without selecting the model', async () => {
    const wrapper = mountSelector()
    await wrapper.get('div.cursor-pointer').trigger('click')

    const row = findModelRow(wrapper, 'gpt-5.6-sol')

    const copyButton = row.get('[data-testid="copy-model-id"]')
    expect(copyButton.attributes('aria-label')).toBe('复制 gpt-5.6-sol')

    await copyButton.trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenCalledWith('gpt-5.6-sol')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('keeps the existing model selection behavior', async () => {
    const wrapper = mountSelector()
    await wrapper.get('div.cursor-pointer').trigger('click')

    const row = findModelRow(wrapper, 'gpt-5.6-sol')
    await row.get('[data-testid="select-model"]').trigger('click')

    expect(wrapper.emitted('update:modelValue')).toEqual([[['gpt-5.6-sol']]])
    expect(copyToClipboard).not.toHaveBeenCalled()
  })

  it('warns when model IDs sync but capability metadata is incomplete', async () => {
    syncUpstreamModels.mockResolvedValue({
      models: ['x-preview-f-free'],
      warnings: [
        {
          code: 'upstream_model_metadata_incomplete',
          message: 'Model IDs were synced, but capability metadata could not be updated.'
        }
      ]
    })
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'openai',
        accountId: 46
      },
      global: {
        stubs: {
          ModelIcon: true
        }
      }
    })

    const syncButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'admin.accounts.syncUpstreamModels')
    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')).toEqual([[['x-preview-f-free']]])
    expect(showWarning).toHaveBeenCalledWith('admin.accounts.syncUpstreamModelsMetadataIncomplete')
    expect(showSuccess).not.toHaveBeenCalled()
  })

  it('shows success and a partial warning when some capabilities were saved', async () => {
    syncUpstreamModels.mockResolvedValue({
      models: ['gpt-6-astra', 'gpt-image-2'],
      warnings: [
        {
          code: 'upstream_model_metadata_partial',
          message: 'Some model capabilities were saved; remaining models are still incomplete.'
        }
      ]
    })
    const wrapper = mount(ModelWhitelistSelector, {
      props: {
        modelValue: [],
        platform: 'openai',
        accountId: 46
      },
      global: {
        stubs: {
          ModelIcon: true
        }
      }
    })

    const syncButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'admin.accounts.syncUpstreamModels')
    expect(syncButton).toBeDefined()
    await syncButton!.trigger('click')
    await flushPromises()

    expect(wrapper.emitted('update:modelValue')).toEqual([[['gpt-6-astra', 'gpt-image-2']]])
    expect(showSuccess).toHaveBeenCalledWith('admin.accounts.syncUpstreamModelsSuccess')
    expect(showWarning).toHaveBeenCalledWith('admin.accounts.syncUpstreamModelsMetadataPartial')
  })

  it('reports a successful preview so account creation can persist metadata', async () => {
    syncUpstreamModelsPreview.mockResolvedValue({
      models: ['x-preview-f-free'],
      metadata: {
        'x-preview-f-free': {
          id: 'x-preview-f-free',
          reasoning: true,
          supported_reasoning_levels: ['low', 'high', 'max'],
        },
      },
    })
    const wrapper = mountSelector({
      syncCredentials: {
        platform: 'openai',
        type: 'apikey',
        base_url: 'https://opencode.ai/zen/v1',
        api_key: 'test-key',
      },
    })
    const syncButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'admin.accounts.syncUpstreamModels')

    expect(syncButton).toBeDefined()
    await syncButton?.trigger('click')
    await flushPromises()

    expect(syncUpstreamModelsPreview).toHaveBeenCalledOnce()
    expect(wrapper.emitted('upstream-synced')).toEqual([[]])
    expect(wrapper.emitted('update:modelValue')).toEqual([[['x-preview-f-free']]])
  })

  it('shows the upstream sync button for OpenCode Go create-account credentials', () => {
    const wrapper = mountSelector({
      platform: 'opencode_go',
      syncCredentials: {
        platform: 'opencode_go',
        type: 'apikey',
        base_url: 'https://opencode.ai/zen/go/v1',
        api_key: 'sk-test',
      },
    })
    const syncButton = wrapper
      .findAll('button')
      .find(button => button.text() === 'admin.accounts.syncUpstreamModels')

    expect(syncButton).toBeDefined()
    expect(syncButton?.exists()).toBe(true)
  })
})
