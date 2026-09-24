import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import UserDashboardRecentUsage from '../UserDashboardRecentUsage.vue'
import type { UsageLog } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

vi.mock('@/components/common/LoadingSpinner.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/common/EmptyState.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/icons/Icon.vue', () => ({ default: { template: '<span />' } }))

enableAutoUnmount(afterEach)

const baseLog = {
  id: 1,
  model: 'test-model',
  actual_cost: 0.12,
  input_tokens: 10,
  output_tokens: 20,
  created_at: '2026-01-01T00:00:00Z',
} as UsageLog

function render(log: Partial<UsageLog> = {}) {
  return mount(UserDashboardRecentUsage, {
    props: { data: [{ ...baseLog, ...log }], loading: false },
    global: { stubs: { RouterLink: { template: '<a><slot /></a>' } } },
  })
}

describe('UserDashboardRecentUsage', () => {
  it('renders actual and standard costs when total_cost exists', () => {
    const wrapper = render({ total_cost: 0.456789 })
    expect(wrapper.text()).toContain('$0.1200')
    expect(wrapper.text()).toContain('$0.4568')
  })

  it('hides the standard cost when total_cost is missing', () => {
    const wrapper = render()
    expect(wrapper.text()).toContain('$0.1200')
    expect(wrapper.text()).not.toContain('dashboard.standard')
    expect(wrapper.text()).not.toContain('/ $')
  })

  it.each([null, undefined, Number.NaN, Number.POSITIVE_INFINITY])('does not crash for invalid actual cost: %s', (actualCost) => {
    const wrapper = render({ actual_cost: actualCost as number })
    expect(wrapper.exists()).toBe(true)
    expect(wrapper.text()).not.toContain('NaN')
  })

  it('remounts safely after navigation away and back', () => {
    const first = render()
    first.unmount()
    const second = render({ total_cost: 0.25 })
    expect(second.text()).toContain('$0.2500')
  })
})
