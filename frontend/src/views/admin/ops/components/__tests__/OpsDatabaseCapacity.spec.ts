import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import OpsDatabaseCapacity from '../OpsDatabaseCapacity.vue'
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
describe('instance capacity versus application pool budgets', () => {
  it('keeps unknown capacity distinct from pool budgets and legal zero usage', () => {
    const wrapper = mount(OpsDatabaseCapacity, { props: { capacity: {
      postgresql: { state: 'Available', source: 'PostgreSQL pg_stat_activity', used: 0, limit: 300 },
      redis: { state: 'Unknown', source: 'Redis INFO unavailable', used: null, limit: null },
      postgresql_pool_limit: 20, redis_pool_limit: 10
    } } })
    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('0 / 300')
    expect(rows[0].text()).toContain('20')
    expect(rows[1].text()).toContain('admin.ops.monitoring.states.Unknown')
    expect(rows[1].text()).not.toContain('0 /')
    expect(rows[1].text()).toContain('Redis INFO unavailable')
    wrapper.unmount()
  })
  it('shows each Redis node and SQL pool without adding independent server limits together', () => {
    const wrapper = mount(OpsDatabaseCapacity, { props: { capacity: {
      postgresql: { state: 'Available', source: 'client slots', used: 8, limit: 300 },
      redis: { state: 'Available', source: 'busiest node', used: 80, limit: 100, nodes: [
        { node: 'node-a', state: 'Available', source: 'INFO', used: 80, limit: 100 },
        { node: 'node-b', state: 'Unknown', source: 'INFO failed', used: null, limit: null }
      ] },
      postgresql_pool_limit: 20, redis_pool_limit: 10,
      database_pools: [{ instance: 'pod-a', role: 'advisory_lock', active: 2, idle: 1, limit: 3, wait_count: 7, wait_seconds: 0.125 }]
    } } })
    expect(wrapper.text()).toContain('node-a')
    expect(wrapper.text()).toContain('node-b')
    expect(wrapper.text()).toContain('admin.ops.monitoring.states.Unknown')
    expect(wrapper.text()).toContain('admin.ops.monitoring.capacity.roles.advisory_lock')
    expect(wrapper.text()).toContain('2 / 1 / 3')
    expect(wrapper.text()).toContain('7 / 0.125 s')
    wrapper.unmount()
  })
})
