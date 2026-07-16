import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { useGsapCounter } from '../useGsapCounter'

vi.mock('gsap', () => {
  const tweens: Array<{ kill: ReturnType<typeof vi.fn>; vars: any; proxy: any }> = []
  const gsap = {
    to: (proxy: any, vars: any) => {
      const tween = {
        kill: vi.fn(),
        vars,
        proxy
      }
      tweens.push(tween)
      // Simulate immediate completion for tests
      proxy.value = vars.value
      vars.onUpdate?.()
      vars.onComplete?.()
      return tween
    },
    __tweens: tweens
  }
  return { default: gsap, gsap }
})

function mountCounter(
  source: ReturnType<typeof ref<number | null | undefined>>,
  options: Parameters<typeof useGsapCounter>[1] = {}
) {
  let api: ReturnType<typeof useGsapCounter> | null = null
  const Comp = defineComponent({
    setup() {
      api = useGsapCounter(source, options)
      return () => null
    }
  })
  const wrapper = mount(Comp)
  return { wrapper, api: api! }
}

describe('useGsapCounter', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn()
      }))
    )
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows fallback for invalid values', async () => {
    const source = ref<number | null>(null)
    const { api, wrapper } = mountCounter(source, { fallback: 'N/A' })
    expect(api.displayText.value).toBe('N/A')
    source.value = Number.NaN
    await nextTick()
    expect(api.displayText.value).toBe('N/A')
    wrapper.unmount()
  })

  it('formats integers with thousand separators', async () => {
    const source = ref(1234)
    const { api, wrapper } = mountCounter(source, { format: 'integer' })
    await flushPromises()
    expect(api.displayText.value).toBe('1,234')
    wrapper.unmount()
  })

  it('formats currency with prefix and decimals', async () => {
    const source = ref(12.5)
    const { api, wrapper } = mountCounter(source, {
      format: 'currency',
      decimals: 2,
      prefix: '$'
    })
    await flushPromises()
    expect(api.displayText.value).toBe('$12.50')
    wrapper.unmount()
  })

  it('formats percent with suffix', async () => {
    const source = ref(85.2)
    const { api, wrapper } = mountCounter(source, {
      format: 'percent',
      decimals: 1,
      suffix: '%'
    })
    await flushPromises()
    expect(api.displayText.value).toBe('85.2%')
    wrapper.unmount()
  })

  it('skips tween when prefers-reduced-motion', async () => {
    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockImplementation((query: string) => ({
        matches: query.includes('prefers-reduced-motion'),
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn()
      }))
    )
    const source = ref(100)
    const { api, wrapper } = mountCounter(source, { format: 'integer' })
    await flushPromises()
    expect(api.displayText.value).toBe('100')
    source.value = 200
    await nextTick()
    expect(api.displayText.value).toBe('200')
    wrapper.unmount()
  })
})
