import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'

import HomeHeroScene from '../HomeHeroScene.vue'

let wrapper: VueWrapper | null = null
let getContextSpy: ReturnType<typeof vi.spyOn>

describe('HomeHeroScene', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    getContextSpy = vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockImplementation(() => ({} as any))
  })

  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
    getContextSpy.mockRestore()
    vi.useRealTimers()
  })

  it('loads the verified same-origin Sylva source through the scoped host', () => {
    wrapper = mount(HomeHeroScene)

    const frame = wrapper.get('iframe')
    expect(frame.attributes('src')).toBe('/threeui/sylva/inner-green-3d.html')
    expect(frame.attributes('sandbox')).toBe('allow-same-origin allow-scripts')
    expect(frame.attributes('aria-hidden')).toBe('true')
  })

  it('keeps scene exposure and framing stable at every page progress', async () => {
    wrapper = mount(HomeHeroScene, { props: { progress: 0 } })
    expect(wrapper.attributes('style')).toContain('--sylva-opacity: 1')
    expect(wrapper.attributes('style')).toContain('--sylva-scale: 1')
    expect(wrapper.attributes('style')).toContain('--sylva-shift: 0vh')

    await wrapper.setProps({ progress: 0.95 })

    expect(wrapper.attributes('style')).toContain('--sylva-opacity: 1')
    expect(wrapper.attributes('style')).toContain('--sylva-scale: 1')
    expect(wrapper.attributes('style')).toContain('--sylva-shift: 0vh')
  })

  it('isolates the authored renderer canvas and reports readiness', async () => {
    wrapper = mount(HomeHeroScene)
    const frame = wrapper.get('iframe').element as HTMLIFrameElement
    const frameDocument = document.implementation.createHTMLDocument('Sylva')
    const frameWindow = new EventTarget() as Window & { __ready?: boolean }

    Object.defineProperties(frame, {
      contentDocument: { configurable: true, value: frameDocument },
      contentWindow: { configurable: true, value: frameWindow },
    })

    frameDocument.body.innerHTML = '<main id="hero"><canvas id="scene"></canvas></main>'
    frameWindow.__ready = true

    await wrapper.get('iframe').trigger('load')

    const presentationStyle = frameDocument.getElementById('modurelay-sylva-scene-presentation')
    expect(presentationStyle?.textContent).toContain('#scene')
    expect(presentationStyle?.textContent).toContain('background: #4a4d44')
    expect(frameDocument.documentElement.dataset.modurelayPresentation).toBe('scene')
    expect(wrapper.classes()).toContain('sylva-scene-ready')
    expect(wrapper.emitted('ready')).toHaveLength(1)
  })

  it('uses the static scene fallback when WebGL is unavailable', async () => {
    getContextSpy.mockImplementation(() => null)
    wrapper = mount(HomeHeroScene)

    await vi.runAllTimersAsync()

    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(wrapper.get('.sylva-scene-fallback').exists()).toBe(true)
    expect(wrapper.find('.sylva-scene-fallback img').exists()).toBe(false)
    expect(wrapper.emitted('ready')).toHaveLength(1)
  })
})
