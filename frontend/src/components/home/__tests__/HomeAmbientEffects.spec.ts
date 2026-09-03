import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'

import HomeAmbientEffects from '../HomeAmbientEffects.vue'

const { loadThreeMock } = vi.hoisted(() => ({ loadThreeMock: vi.fn() }))

vi.mock('@/utils/threeRuntime', () => ({ loadThree: loadThreeMock }))

let wrapper: VueWrapper | null = null
let getContextSpy: ReturnType<typeof vi.spyOn>
let matchMediaSpy: ReturnType<typeof vi.spyOn>

function mediaQuery(matches: boolean): MediaQueryList {
  return {
    matches,
    media: '(prefers-reduced-motion: reduce)',
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }
}

describe('HomeAmbientEffects', () => {
  beforeEach(() => {
    loadThreeMock.mockReset()
    getContextSpy = vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(() => ({} as WebGLRenderingContext))
    matchMediaSpy = vi.spyOn(window, 'matchMedia').mockReturnValue(mediaQuery(false))
    vi.stubGlobal('requestIdleCallback', vi.fn((callback: IdleRequestCallback) => {
      callback({ didTimeout: false, timeRemaining: () => 50 } as IdleDeadline)
      return 1
    }))
    vi.stubGlobal('cancelIdleCallback', vi.fn())
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    getContextSpy.mockRestore()
    matchMediaSpy.mockRestore()
    vi.unstubAllGlobals()
  })

  it('waits for the verified hero to enable the blossom renderer', async () => {
    wrapper = mount(HomeAmbientEffects)
    await flushPromises()

    expect(wrapper.attributes('data-state')).toBe('disabled')
    expect(wrapper.attributes('data-effect')).toBe('plum-blossoms')
    expect(loadThreeMock).not.toHaveBeenCalled()
  })

  it('does not create animated WebGL content when reduced motion is requested', async () => {
    matchMediaSpy.mockReturnValue(mediaQuery(true))
    wrapper = mount(HomeAmbientEffects, { props: { enabled: true } })
    await flushPromises()

    expect(wrapper.attributes('data-state')).toBe('disabled')
    expect(loadThreeMock).not.toHaveBeenCalled()
  })

  it('falls back silently when WebGL is unavailable', async () => {
    getContextSpy.mockImplementation(() => null)
    wrapper = mount(HomeAmbientEffects, { props: { enabled: true } })
    await flushPromises()

    expect(wrapper.attributes('data-state')).toBe('fallback')
    expect(loadThreeMock).not.toHaveBeenCalled()
  })

  it('falls back when the local Three.js runtime cannot be loaded', async () => {
    loadThreeMock.mockRejectedValueOnce(new Error('runtime unavailable'))
    wrapper = mount(HomeAmbientEffects, { props: { enabled: true } })
    await flushPromises()

    expect(loadThreeMock).toHaveBeenCalledOnce()
    expect(wrapper.attributes('data-state')).toBe('fallback')
  })
})
