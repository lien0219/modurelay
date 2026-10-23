import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import OpsAlertDeliveries from '../OpsAlertDeliveries.vue'
const { list, retry } = vi.hoisted(() => ({ list: vi.fn(), retry: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listAlertDeliveries: list, retryAlertDelivery: retry } }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); retry.mockResolvedValue(undefined) })
describe('notification delivery audit', () => {
  it('shows delivery failure and retries a concrete failed delivery', async () => {
    list.mockResolvedValue([{ id: 5, event_id: 9, channel: 'webhook', transition: 'resolved', target: 'https://example.org/hook', status: 'failed', attempts: 5, last_error: 'HTTP 503' }])
    const wrapper = mount(OpsAlertDeliveries, { props: { eventId: 9 } })
    await flushPromises()
    expect(list).toHaveBeenCalledWith(9)
    expect(wrapper.text()).toContain('HTTP 503')
    await wrapper.findAll('button').find(button => button.text() === 'admin.ops.monitoring.deliveries.retry')!.trigger('click')
    await flushPromises()
    expect(retry).toHaveBeenCalledWith(5)
    expect(list).toHaveBeenCalledTimes(2)
  })
  it('does not treat a failed query as an empty delivery history', async () => {
    list.mockRejectedValue(new Error('unavailable'))
    const wrapper = mount(OpsAlertDeliveries, { props: { eventId: 9 } })
    await flushPromises()
    expect(wrapper.get('[role="alert"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('admin.ops.monitoring.deliveries.empty')
  })
})
