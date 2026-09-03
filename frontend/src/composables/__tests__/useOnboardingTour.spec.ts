import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  destroy: vi.fn(),
  driverFactory: vi.fn(),
  store: {
    getDriverInstance: vi.fn(),
    setDriverInstance: vi.fn(),
    isDriverActive: vi.fn(() => false),
    setControlMethods: vi.fn(),
    clearControlMethods: vi.fn()
  }
}))

vi.mock('driver.js', () => ({ driver: mocks.driverFactory }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 1, role: 'admin' }, isSimpleMode: false })
}))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => mocks.store }))
vi.mock('@/components/Guide/steps', () => ({ getAdminSteps: () => [], getUserSteps: () => [] }))

import { useOnboardingTour } from '@/composables/useOnboardingTour'

describe('useOnboardingTour', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    document.body.className = 'driver-active driver-fade'
    document.body.innerHTML = '<svg class="driver-overlay"></svg>'
    mocks.store.getDriverInstance.mockReturnValue({ destroy: mocks.destroy })
  })

  afterEach(() => {
    vi.useRealTimers()
    document.body.className = ''
    document.body.innerHTML = ''
  })

  it('destroys inherited Driver state and never schedules a tour when disabled', async () => {
    const Host = defineComponent({
      setup() {
        useOnboardingTour({ enabled: false, autoStart: true })
        return () => h('div')
      }
    })

    const wrapper = mount(Host)
    await nextTick()
    vi.advanceTimersByTime(2000)

    expect(mocks.destroy).toHaveBeenCalledOnce()
    expect(mocks.store.setDriverInstance).toHaveBeenCalledWith(null)
    expect(mocks.driverFactory).not.toHaveBeenCalled()
    expect(document.querySelector('.driver-overlay')).toBeNull()
    expect(document.body.classList.contains('driver-active')).toBe(false)

    wrapper.unmount()
  })
})
