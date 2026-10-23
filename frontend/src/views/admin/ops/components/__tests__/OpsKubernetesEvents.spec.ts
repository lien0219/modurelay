import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import OpsKubernetesEvents from '../OpsKubernetesEvents.vue'
const { getEvents } = vi.hoisted(() => ({ getEvents: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { getKubernetesEvents: getEvents } }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.resetAllMocks() })
describe('Pod event history', () => {
  it('does not widen an unsupported Service scope to all namespace events', async () => {
    const wrapper = mount(OpsKubernetesEvents, { props: { enabled: true, query: { metric: 'qps', service: 'relay', namespace: 'prod' } } })
    await flushPromises()
    expect(getEvents).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.ops.monitoring.events.drillDown')
  })
  it('queries a selected Pod UID and keeps archived history visibly stale', async () => {
    getEvents.mockResolvedValue({ state: 'Unknown', events: [{ cluster: 'west', namespace: 'prod', event_uid: 'event-1', pod: 'relay', pod_uid: 'uid-before-recreation', reason: 'OOMKilled', type: 'Warning', message: 'Memory limit exceeded', first_seen_at: '2026-10-22T01:00:00Z', last_seen_at: '2026-10-22T01:01:00Z', count: 1, source: 'kubelet' }] })
    const wrapper = mount(OpsKubernetesEvents, { props: { enabled: true, query: { metric: 'qps', service: 'relay-service', pod: 'relay', uid: 'uid-before-recreation', start: '2026-10-22T00:00:00Z', end: '2026-10-22T02:00:00Z' } } })
    await flushPromises()
    expect(getEvents.mock.calls[0][0]).not.toHaveProperty('service')
    expect(getEvents.mock.calls[0][0]).toMatchObject({ uid: 'uid-before-recreation', start: '2026-10-22T00:00:00Z', end: '2026-10-22T02:00:00Z' })
    expect(wrapper.text()).toContain('admin.ops.monitoring.events.stale')
    expect(wrapper.text()).toContain('uid-before-recreation')
    expect(wrapper.text()).toContain('OOMKilled')
  })
  it('shows disabled collection as not applicable without querying', async () => {
    const wrapper = mount(OpsKubernetesEvents, { props: { enabled: false, query: { metric: 'pod_restarts' } } })
    await flushPromises()
    expect(getEvents).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.ops.monitoring.events.disabled')
  })
  it('queries Deployment events by object identity without widening the resource scope', async () => {
    getEvents.mockResolvedValue({ state: 'Available', events: [{ cluster: 'west', namespace: 'prod', event_uid: 'event-scale', object_kind: 'Deployment', object_name: 'relay', object_uid: 'deployment-uid', reason: 'ScalingReplicaSet', type: 'Normal', message: 'Scaled to 3', count: 1, last_seen_at: '2026-10-22T01:01:00Z' }] })
    const wrapper = mount(OpsKubernetesEvents, { props: { enabled: true, query: { metric: 'deployment_ready_replicas', deployment: 'relay', namespace: 'prod' } } })
    await flushPromises()
    expect(getEvents.mock.calls[0][0]).toMatchObject({ object_kind: 'Deployment', object_name: 'relay', namespace: 'prod' })
    expect(wrapper.text()).toContain('Deployment / relay')
    expect(wrapper.text()).toContain('deployment-uid')
  })
})
