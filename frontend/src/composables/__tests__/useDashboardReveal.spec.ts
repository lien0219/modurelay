import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, nextTick, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import { useDashboardReveal } from '../useDashboardReveal'

vi.mock('gsap', () => {
  const gsap = {
    context: (fn: () => void) => {
      fn()
      return { revert: vi.fn() }
    },
    fromTo: (_targets: unknown, _from: unknown, to: { onStart?: () => void; onComplete?: () => void }) => {
      to.onStart?.()
      to.onComplete?.()
      return { kill: vi.fn() }
    },
    set: vi.fn()
  }
  return { default: gsap, gsap }
})

describe('useDashboardReveal', () => {
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

  it('plays once per scope and skips remount replay', async () => {
    const scopeKey = `reveal-test-${Math.random()}`

    const mountOnce = () => {
      const containerRef = ref<HTMLElement | null>(null)
      let api: ReturnType<typeof useDashboardReveal> | null = null
      const Comp = defineComponent({
        setup() {
          api = useDashboardReveal(containerRef, { scopeKey })
          return { containerRef, isPending: api.isPending }
        },
        template: `
          <div ref="containerRef">
            <div data-reveal>A</div>
            <div data-reveal>B</div>
          </div>
        `
      })
      const wrapper = mount(Comp)
      return { wrapper, api: api! }
    }

    const first = mountOnce()
    await flushPromises()
    await nextTick()
    expect(first.api.hasPlayed()).toBe(true)
    first.wrapper.unmount()

    const second = mountOnce()
    await flushPromises()
    await nextTick()
    expect(second.api.hasPlayed()).toBe(true)
    expect(second.api.isPending.value).toBe(false)
    second.wrapper.unmount()
  })
})
