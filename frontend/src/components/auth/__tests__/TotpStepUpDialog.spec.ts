import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import en from '@/i18n/locales/en'
import { createTestI18n } from '@/__tests__/utils/i18n'
import TotpStepUpDialog from '../TotpStepUpDialog.vue'

const mocks = vi.hoisted(() => ({ showError: vi.fn(), stepUp: vi.fn() }))
vi.mock('@/stores', () => ({ useAppStore: () => ({ showError: mocks.showError }) }))
vi.mock('@/api', () => ({ totpAPI: { stepUp: mocks.stepUp } }))

function controller() {
  return { visible: ref(true), onVerified: vi.fn(), onCancel: vi.fn() }
}

describe('TotpStepUpDialog guarded proof flow', () => {
  beforeEach(() => { vi.resetAllMocks() })
  afterEach(() => { document.body.innerHTML = '' })

  it('redacts opt-in proof failures instead of displaying upstream text', async () => {
    const current = controller()
    const onVerify = vi.fn().mockRejectedValue(new Error('raw provider secret'))
    const wrapper = mount(TotpStepUpDialog, {
      attachTo: document.body,
      props: { controller: current, onVerify, safeErrors: true },
      global: { plugins: [createTestI18n({ en })] }
    })
    await wrapper.get('input[aria-hidden="true"]').setValue('123456'); await flushPromises()
    expect(onVerify).toHaveBeenCalledTimes(1)
    expect(mocks.showError).toHaveBeenCalledWith('Verification failed, please try again')
    expect(mocks.showError).not.toHaveBeenCalledWith('raw provider secret')
    expect(current.onVerified).not.toHaveBeenCalled()
  })

  it('aborts a cancelled proof and ignores its late result', async () => {
    let resolveProof!: () => void
    const proof = new Promise<void>(resolve => { resolveProof = resolve })
    const current = controller()
    const onVerify = vi.fn().mockReturnValue(proof)
    const wrapper = mount(TotpStepUpDialog, {
      attachTo: document.body,
      props: { controller: current, onVerify, safeErrors: true },
      global: { plugins: [createTestI18n({ en })] }
    })
    await wrapper.get('input[aria-hidden="true"]').setValue('654321'); await flushPromises()
    const signal = onVerify.mock.calls[0][1] as AbortSignal
    const backdrop = wrapper.findAll('div').find(node => node.classes().includes('bg-black/50'))
    expect(backdrop).toBeDefined()
    await backdrop!.trigger('click')
    expect(signal.aborted).toBe(true)
    resolveProof(); await flushPromises()
    expect(current.onCancel).toHaveBeenCalledTimes(1)
    expect(current.onVerified).not.toHaveBeenCalled()
    expect(mocks.showError).not.toHaveBeenCalled()
  })
})
