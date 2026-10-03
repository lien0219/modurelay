import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import AccountTestModal from '../AccountTestModal.vue'

const { getAvailableModelsMock } = vi.hoisted(() => ({
  getAvailableModelsMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels: getAvailableModelsMock
    }
  }
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: { type: [String, Number, Boolean, null], default: '' },
    options: { type: Array, default: () => [] },
    valueKey: { type: String, default: 'value' },
    labelKey: { type: String, default: 'label' }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option
        v-for="option in options"
        :key="option[valueKey]"
        :value="option[valueKey]"
      >
        {{ option[labelKey] }}
      </option>
    </select>
  `
})

const TextAreaStub = defineComponent({
  name: 'TextArea',
  props: {
    modelValue: { type: String, default: '' }
  },
  emits: ['update:modelValue'],
  template: `
    <textarea
      v-bind="$attrs"
      :value="modelValue"
      @input="$emit('update:modelValue', $event.target.value)"
    />
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI OAuth',
    platform: 'openai',
    type: 'oauth',
    status: 'active',
    credentials: {},
    extra: {},
    concurrency: 1,
    priority: 1,
    proxy_id: null,
    auto_pause_on_expired: false
  } as any
}

describe('AccountTestModal', () => {
  const originalFetch = global.fetch

  beforeEach(() => {
    getAvailableModelsMock.mockReset()
    getAvailableModelsMock.mockResolvedValue([
      { id: 'gpt-5.4', display_name: 'GPT-5.4' }
    ])
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockResolvedValue({ done: true, value: undefined })
        })
      }
    } as any)
    localStorage.setItem('auth_token', 'test-token')
  })

  afterEach(() => {
    global.fetch = originalFetch
    localStorage.clear()
  })

  it('shows the upstream name and full API ID while submitting the original model ID', async () => {
    const id = 'doubao-seedance-1-0-pro-fast-251015'
    const name = 'Doubao-Seedance-1.0-pro-fast'
    getAvailableModelsMock.mockResolvedValue([{ id, display_name: name }])
    const wrapper = mount(AccountTestModal, {
      props: { show: false, account: { ...buildAccount(), platform: 'seedance', type: 'apikey' } },
      global: { stubs: { BaseDialog: BaseDialogStub, Select: SelectStub, TextArea: TextAreaStub, Icon: true } },
    })
    await wrapper.setProps({ show: true })
    await flushPromises()
    const option = wrapper.get(`option[value="${id}"]`)
    expect(option.text()).toContain(name)
    expect(option.text()).toContain(id)
    expect(wrapper.text()).toContain('admin.accounts.upstreamModelCatalogHint')
    await wrapper.get('select').setValue(id)
    await (wrapper.vm as any).startTest()
    await flushPromises()
    expect(JSON.parse((global.fetch as any).mock.calls[0][1].body).model_id).toBe(id)
  })

  it('searches both the friendly name and API ID in the actual model picker', async () => {
    const id = 'doubao-seedance-1-0-pro-fast-251015'
    const name = 'Doubao-Seedance-1.0-pro-fast'
    getAvailableModelsMock.mockResolvedValue([
      { id, display_name: name },
      { id: 'ep-video-test', display_name: 'Other Video Model' },
    ])
    const wrapper = mount(AccountTestModal, {
      attachTo: document.body,
      props: { show: false, account: { ...buildAccount(), platform: 'seedance', type: 'apikey' } },
      global: { stubs: { BaseDialog: BaseDialogStub, TextArea: TextAreaStub, Icon: true } },
    })
    try {
      await wrapper.setProps({ show: true })
      await flushPromises()
      expect(wrapper.get('.select-trigger').text()).toContain(name)
      expect(wrapper.get('.select-trigger').text()).toContain(id)
      await wrapper.get('.select-trigger').trigger('click')
      await flushPromises()
      const search = document.querySelector<HTMLInputElement>('.select-search-input')!
      search.value = '1.0-pro-fast'
      search.dispatchEvent(new Event('input', { bubbles: true }))
      await flushPromises()
      let options = document.querySelectorAll('[role="option"]')
      expect(options).toHaveLength(1)
      expect(options[0].textContent).toContain(name)
      expect(options[0].textContent).toContain(id)
      search.value = '251015'
      search.dispatchEvent(new Event('input', { bubbles: true }))
      await flushPromises()
      options = document.querySelectorAll('[role="option"]')
      expect(options).toHaveLength(1)
      ;(options[0] as HTMLElement).click()
      await flushPromises()
      await (wrapper.vm as any).startTest()
      expect(JSON.parse((global.fetch as any).mock.calls[0][1].body).model_id).toBe(id)
    } finally {
      wrapper.unmount()
    }
  })

  it('posts compact mode for OpenAI compact probe', async () => {
    const wrapper = mount(AccountTestModal, {
      props: {
        show: true,
        account: buildAccount()
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          TextArea: TextAreaStub,
          Icon: true
        }
      }
    })

    await flushPromises()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    ;(wrapper.vm as any).testMode = 'compact'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(global.fetch).toHaveBeenCalledTimes(1)
    const [, options] = (global.fetch as any).mock.calls[0]
    expect(JSON.parse(options.body)).toMatchObject({
      model_id: 'gpt-5.4',
      mode: 'compact'
    })
  })

  it('renders Chat Completions path status from test SSE', async () => {
    const encoder = new TextEncoder()
    const chunks = [
      encoder.encode('data: {"type":"status","text":"已通过 /v1/chat/completions 验证"}\n\n'),
      encoder.encode('data: {"type":"test_complete","success":true}\n\n')
    ]
    global.fetch = vi.fn().mockResolvedValue({
      ok: true,
      body: {
        getReader: () => ({
          read: vi.fn().mockImplementation(() => Promise.resolve(
            chunks.length > 0
              ? { done: false, value: chunks.shift() }
              : { done: true, value: undefined }
          ))
        })
      }
    } as any)

    const wrapper = mount(AccountTestModal, {
      props: {
        show: true,
        account: buildAccount()
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Select: SelectStub,
          TextArea: TextAreaStub,
          Icon: true
        }
      }
    })

    await flushPromises()
    ;(wrapper.vm as any).selectedModelId = 'gpt-5.4'
    await (wrapper.vm as any).startTest()
    await flushPromises()

    expect(wrapper.text()).toContain('已通过 /v1/chat/completions 验证')
  })
})
