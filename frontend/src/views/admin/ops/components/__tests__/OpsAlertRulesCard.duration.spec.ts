import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
enableAutoUnmount(afterEach)


import OpsAlertRulesCard from '../OpsAlertRulesCard.vue'
import { ref } from 'vue'
const { createAlertRule, getMonitoringCapabilities } = vi.hoisted(() => ({ createAlertRule: vi.fn(), getMonitoringCapabilities: vi.fn() }))
vi.mock('@vueuse/core', () => ({ useMediaQuery: () => ref(true) }))
vi.mock('@/api', () => ({ adminAPI: { groups: { getAll: vi.fn().mockResolvedValue([]) } } }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { listAlertRules: vi.fn().mockResolvedValue([]), createAlertRule, getMonitoringCapabilities } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))

async function openRule() {
  const wrapper = shallowMount(OpsAlertRulesCard, {
    global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' } } },
  })
  await flushPromises()
  await wrapper.findAll('button').find(b => b.text().includes('admin.ops.alertRules.create'))!.trigger('click')
  await wrapper.find('input[type="text"]').setValue('Error rate')
  return wrapper
}
describe('alert rule integer durations', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getMonitoringCapabilities.mockResolvedValue({ external_ingress_adapter: 'nginx' })
  })
  it.each(['1.5', '0.5'])('rejects fractional duration %s before submitting', async value => {
    const wrapper = await openRule()
    const input = wrapper.find(value === '1.5' ? 'input[min="1"][max="1440"]' : 'input[min="0"][max="1440"]')
    await input.setValue(value)
    const save = wrapper.findAll('button').find(b => b.text() === 'common.save')!
    expect(wrapper.text()).toContain(value === '1.5' ? 'admin.ops.alertRules.validation.sustainedRange' : 'admin.ops.alertRules.validation.cooldownRange')
    await save.trigger('click')
    expect(createAlertRule).not.toHaveBeenCalled()
  })
  it('allows whole-minute boundaries and a fractional metric threshold', async () => {
    const wrapper = await openRule()
    await wrapper.get('input[min="1"][max="1440"]').setValue('1440')
    await wrapper.get('input[min="0"][max="1440"]').setValue('0')
    await wrapper.get('input[type="number"]:not([min])').setValue('1.25')
    const save = wrapper.findAll('button').find(b => b.text() === 'common.save')!
    expect(save.attributes('disabled')).toBeUndefined()
    await save.trigger('click')
    await flushPromises()
    expect(createAlertRule).toHaveBeenCalledWith(expect.objectContaining({ sustained_minutes: 1440, cooldown_minutes: 0, min_requests: 0, threshold: 1.25 }))
  })
  it('disables minimum requests for non-rate metrics and saves zero', async () => {
    const wrapper = await openRule()
    const input = wrapper.get('input[min="0"][max="1000000"]')
    expect(input.attributes('disabled')).toBeUndefined()
    const draft = (wrapper.vm as unknown as { draft: { metric_type: string; min_requests: number } }).draft
    draft.metric_type = 'cpu_usage_percent'
    draft.min_requests = 100
    await flushPromises()
    expect(wrapper.get('input[min="0"][max="1000000"]').attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(createAlertRule).toHaveBeenCalledWith(expect.objectContaining({ metric_type: 'cpu_usage_percent', min_requests: 0 }))
  })
  it('preserves request gates, resource filters and webhook targets for HTTP latency rules', async () => {
    const wrapper = await openRule()
    const draft = (wrapper.vm as unknown as { draft: { metric_type: string; min_requests: number; window_minutes: number } }).draft
    draft.metric_type = 'http_p99_ms'
    draft.min_requests = 20
    draft.window_minutes = 15
    await flushPromises()
    expect(wrapper.get('input[min="0"][max="1000000"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('[data-testid="rule-filter-namespace"]').setValue('production')
    await wrapper.get('[data-testid="rule-filter-service"]').setValue('relay')
    await wrapper.get('#rule-webhook-urls').setValue('https://example.org/notify')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(createAlertRule).toHaveBeenCalledWith(expect.objectContaining({
      metric_type: 'http_p99_ms', min_requests: 20, window_minutes: 15,
      filters: expect.objectContaining({ namespace: 'production', service: 'relay', webhook_urls: ['https://example.org/notify'] })
    }))
  })
  it('removes incompatible filters when switching an HTTP rule to HPA', async () => {
    const wrapper = await openRule()
    const draft = (wrapper.vm as unknown as { draft: { metric_type: string; filters: Record<string, unknown> } }).draft
    draft.metric_type = 'hpa_at_max'
    draft.filters = { cluster: 'west', namespace: 'production', service: 'relay', pod: 'relay-1', source: 'app', exclude_429: true, min_errors: 10 }
    await flushPromises()
    expect(wrapper.find('[data-testid="rule-filter-pod"]').exists()).toBe(false)
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(createAlertRule).toHaveBeenCalledWith(expect.objectContaining({ metric_type: 'hpa_at_max', filters: { cluster: 'west', namespace: 'production' } }))
  })
  it.each(['normalized', 'nginx'])('removes Pod filters when switching an app rule to %s external ingress', async adapter => {
    getMonitoringCapabilities.mockResolvedValue({ external_ingress_adapter: adapter })
    const wrapper = await openRule()
    const draft = (wrapper.vm as unknown as { draft: { metric_type: string; filters: Record<string, unknown> } }).draft
    draft.metric_type = 'http_5xx_count'
    draft.filters = {
      cluster: 'west', namespace: 'production', service: 'relay', source: 'app',
      deployment: 'relay', pod: 'relay-1', uid: 'pod-uid',
      protocol: 'sse', route: 'responses', status_code: '502',
    }
    await flushPromises()
    expect(wrapper.find('[data-testid="rule-filter-pod"]').exists()).toBe(true)
    await wrapper.get('[data-testid="rule-filter-source"]').setValue('external_ingress')
    for (const key of ['pod', 'uid', 'deployment']) {
      expect(wrapper.find(`[data-testid="rule-filter-${key}"]`).exists()).toBe(false)
    }
    for (const key of ['protocol', 'route']) {
      expect(wrapper.find(`[data-testid="rule-filter-${key}"]`).exists()).toBe(adapter === 'normalized')
    }
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(createAlertRule).toHaveBeenCalledWith(expect.objectContaining({
      metric_type: 'http_5xx_count',
      filters: {
        cluster: 'west', namespace: 'production', service: 'relay', source: 'external_ingress', status_code: '502',
        ...(adapter === 'normalized' ? { protocol: 'sse', route: 'responses' } : {}),
      },
    }))
  })
  it('rejects invalid webhook schemes without losing the draft', async () => {
    const wrapper = await openRule()
    await wrapper.get('#rule-webhook-urls').setValue('http://example.org/notify')
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    expect(createAlertRule).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.ops.monitoring.webhookInvalid')
  })
  it('describes application pool usage and saves it without a request gate', async () => {
    const wrapper = await openRule()
    const draft = (wrapper.vm as unknown as { draft: { metric_type: string; min_requests: number } }).draft
    draft.metric_type = 'db_pool_usage_percent'
    draft.min_requests = 5
    await flushPromises()
    expect(wrapper.text()).toContain('admin.ops.alertRules.metricDescriptions.dbPool')
    expect(wrapper.get('input[min="0"][max="1000000"]').attributes('disabled')).toBeDefined()
    await wrapper.findAll('button').find(b => b.text() === 'common.save')!.trigger('click')
    await flushPromises()
    expect(createAlertRule).toHaveBeenCalledWith(expect.objectContaining({ metric_type: 'db_pool_usage_percent', min_requests: 0 }))
  })
})
