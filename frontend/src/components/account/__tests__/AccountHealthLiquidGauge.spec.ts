import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const { add, revert, set, to, kill } = vi.hoisted(() => ({
  add: vi.fn(),
  revert: vi.fn(),
  set: vi.fn(),
  to: vi.fn(),
  kill: vi.fn()
}))

vi.mock('gsap', () => ({
  default: {
    matchMedia: () => ({ add, revert }),
    set,
    to
  }
}))

import AccountHealthLiquidGauge from '../AccountHealthLiquidGauge.vue'

describe('AccountHealthLiquidGauge', () => {
  beforeEach(() => {
    add.mockReset()
    revert.mockReset()
    set.mockReset()
    to.mockReset()
    kill.mockReset()
    add.mockImplementation((_query: string, setup: () => (() => void) | void) => setup())
    to.mockReturnValue({ kill })
  })

  it('clamps the liquid level to the 0-100 score range', async () => {
    const wrapper = mount(AccountHealthLiquidGauge, {
      props: { score: 140, state: 'healthy' }
    })

    expect(wrapper.get('[data-testid="account-health-liquid-fill"]').attributes('style')).toContain('height: 100%')

    await wrapper.setProps({ score: -5 })
    expect(wrapper.get('[data-testid="account-health-liquid-fill"]').attributes('style')).toContain('height: 0%')
  })

  it.each([
    ['warming', 'account-health-liquid-gauge--warming'],
    ['healthy', 'account-health-liquid-gauge--healthy'],
    ['degraded', 'account-health-liquid-gauge--degraded'],
    ['open', 'account-health-liquid-gauge--open'],
    ['half_open', 'account-health-liquid-gauge--half_open']
  ] as const)('maps the %s state to a semantic liquid stage', (state, expectedClass) => {
    const wrapper = mount(AccountHealthLiquidGauge, {
      props: { score: 60, state }
    })

    expect(wrapper.get('[data-testid="account-health-liquid-gauge"]').classes()).toContain(expectedClass)
  })

  it('keeps the decorative gauge out of the accessibility tree', () => {
    const wrapper = mount(AccountHealthLiquidGauge, {
      props: { score: 92, state: 'healthy' }
    })

    expect(wrapper.get('[data-testid="account-health-liquid-gauge"]').attributes('aria-hidden')).toBe('true')
  })

  it('scopes the wave tween to element refs and reverts it on unmount', () => {
    const wrapper = mount(AccountHealthLiquidGauge, {
      props: { score: 92, state: 'healthy' }
    })

    expect(add).toHaveBeenCalledWith('(prefers-reduced-motion: no-preference)', expect.any(Function))
    expect(set).toHaveBeenCalledOnce()
    expect(to).toHaveBeenCalledOnce()

    wrapper.unmount()
    expect(revert).toHaveBeenCalledOnce()
  })
})
