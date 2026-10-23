import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import { Line } from 'vue-chartjs'
import OpsMonitoringPanel from '../OpsMonitoringPanel.vue'
import type { OpsMonitoringCapabilities, OpsMonitoringResult } from '@/api/admin/ops'

const { getCapabilities, query, replace, routeQuery } = vi.hoisted(() => ({
  getCapabilities: vi.fn(), query: vi.fn(), replace: vi.fn().mockResolvedValue(undefined), routeQuery: {} as Record<string, string>
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, te: () => false }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: routeQuery }), useRouter: () => ({ replace }) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { getMonitoringCapabilities: getCapabilities, queryMonitoring: query } }))
vi.mock('@/utils/chartColors', async () => {
  const { ref } = await import('vue')
  return { useChartThemeColors: () => ref({ primary: 'indigo', secondary: 'cyan', neutral: 'gray', warning: 'orange', text: 'black', grid: 'gray', surface: 'white' }), getChartSeriesStyle: () => ({ borderDash: [] }) }
})
enableAutoUnmount(afterEach)
const capability: OpsMonitoringCapabilities = {
  mode: 'standalone', prometheus_configured: true, kubernetes: false, history: true,
  metrics: [{ id: 'qps', unit: 'requests/s', kubernetes_only: false }, { id: 'pod_cpu_cores', unit: 'cores', kubernetes_only: true }]
}
const result: OpsMonitoringResult = {
  metric: 'qps', state: 'Available', unit: 'requests/s', step_seconds: 60,
  series: [{ labels: { service: 'relay' }, points: [{ timestamp: '2026-10-22T00:00:00Z', value: 10 }, { timestamp: '2026-10-22T00:01:00Z', value: null }, { timestamp: '2026-10-22T00:02:00Z', value: 0 }] }]
}
function mountPanel() { return shallowMount(OpsMonitoringPanel) }
beforeEach(() => {
  vi.clearAllMocks()
  Object.keys(routeQuery).forEach(key => delete routeQuery[key])
  getCapabilities.mockResolvedValue(capability)
  query.mockResolvedValue(result)
})

describe('cluster monitoring source and data semantics', () => {
  it('keeps missing samples as null, legal zero as zero, and hides Kubernetes filters in standalone mode', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    const chart = wrapper.findComponent(Line)
    expect(chart.props('data').datasets[0].data).toEqual([10, null, 0])
    expect(chart.props('data').datasets[0].spanGaps).toBe(false)
    expect(wrapper.find('[data-testid="monitor-filter-pod"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.ops.monitoring.standaloneHint')
    expect(wrapper.text()).toContain('admin.ops.monitoring.latestValues')
  })
  it('clears earlier results and shows Unknown on a source failure', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    query.mockRejectedValueOnce(new Error('source unavailable'))
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(wrapper.findComponent(Line).exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').text()).toContain('admin.ops.monitoring.queryFailed')
    expect(wrapper.find('[role="status"]').text()).toContain('admin.ops.monitoring.states.Unknown')
  })
  it('shows insufficient data without fabricating a chart', async () => {
    query.mockResolvedValue({ ...result, state: 'InsufficientData', series: [] })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('[role="status"]').text()).toContain('admin.ops.monitoring.states.InsufficientData')
    expect(wrapper.findComponent(Line).exists()).toBe(false)
  })
  it('round trips shared filters and time range without changing existing Ops query keys', async () => {
    Object.assign(routeQuery, { platform: 'openai', mon_cluster: 'west', mon_namespace: 'prod', mon_pod: 'relay-2', mon_range: '7d', mon_source: 'ingress' })
    getCapabilities.mockResolvedValue({ ...capability, mode: 'kubernetes', kubernetes: true })
    const wrapper = mountPanel()
    await flushPromises()
    expect(query).toHaveBeenCalledWith(expect.objectContaining({ cluster: 'west', namespace: 'prod', pod: 'relay-2', source: 'ingress' }), expect.anything())
    const params = query.mock.calls[0][0]
    expect(new Date(params.end).getTime() - new Date(params.start).getTime()).toBe(7 * 86400_000)
    expect(replace).toHaveBeenCalledWith({ query: expect.objectContaining({ platform: 'openai', mon_namespace: 'prod', mon_range: '7d' }) })
    expect(wrapper.get('[data-testid="monitor-filter-pod"]').element).toHaveProperty('value', 'relay-2')
  })
  it('renders long sample tables only on demand and in bounded pages', async () => {
    query.mockResolvedValue({ ...result, series: [{ labels: {}, points: Array.from({ length: 401 }, (_, index) => ({ timestamp: new Date(Date.UTC(2026, 9, 22, 0, index)).toISOString(), value: index })) }] })
    const wrapper = mountPanel()
    await flushPromises()
    const details = wrapper.get('details')
    expect(details.find('table').exists()).toBe(false)
    ;(details.element as HTMLDetailsElement).open = true
    await details.trigger('toggle')
    expect(details.findAll('tbody tr')).toHaveLength(200)
    const next = details.findAll('button').find(button => button.text() === 'admin.ops.monitoring.nextSamples')!
    await next.trigger('click')
    expect(details.findAll('tbody tr')).toHaveLength(200)
    await next.trigger('click')
    expect(details.findAll('tbody tr')).toHaveLength(1)
    expect(details.find('tbody').text()).toContain('400')
  })
  it('links different metric units using the same time scope and preserves gaps at the cursor', async () => {
    Object.assign(routeQuery, { mon_secondary: 'http_p95_ms' })
    getCapabilities.mockResolvedValue({ ...capability, metrics: [...capability.metrics, { id: 'http_p95_ms', unit: 'ms', kubernetes_only: false }] })
    query.mockImplementation(async params => params.metric === 'http_p95_ms' ? { ...result, metric: 'http_p95_ms', unit: 'ms', series: [{ labels: { service: 'relay' }, points: result.series[0].points.map((point, index) => ({ ...point, value: (index + 1) * 100 })) }] } : result)
    const wrapper = mountPanel()
    await flushPromises()
    expect(query.mock.calls[1][0]).toMatchObject({ start: query.mock.calls[0][0].start, end: query.mock.calls[0][0].end, window: '5m' })
    const charts = wrapper.findAllComponents(Line)
    expect(charts).toHaveLength(2)
    expect(charts[0].props('data').labels).toEqual(charts[1].props('data').labels)
    expect(charts[0].props('options').scales.y.title.text).toBe('requests/s')
    expect(charts[1].props('options').scales.y.title.text).toBe('ms')
    await wrapper.get('[data-testid="monitor-cursor"]').setValue('1')
    expect(wrapper.text()).toContain('admin.ops.monitoring.states.Unknown requests/s')
    expect(wrapper.text()).toContain('200 ms')
  })
  it('ignores an in-flight result after the query scope changes', async () => {
    let resolveQuery!: (value: OpsMonitoringResult) => void
    query.mockImplementationOnce(() => new Promise<OpsMonitoringResult>(resolve => { resolveQuery = resolve }))
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-testid="monitor-range"]').setValue('7d')
    resolveQuery(result)
    await flushPromises()
    expect(wrapper.findComponent(Line).exists()).toBe(false)
  })
  it('converts memory bytes to MiB consistently without filling data gaps', async () => {
    query.mockResolvedValue({ ...result, unit: 'bytes', series: [{ labels: {}, points: [{ timestamp: '2026-10-22T00:00:00Z', value: 1048576 }, { timestamp: '2026-10-22T00:01:00Z', value: null }] }] })
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.findComponent(Line).props('data').datasets[0].data).toEqual([1, null])
    expect(wrapper.get('[role="status"]').text()).toContain('MiB')
  })
  it('rejects invalid custom ranges before querying the source', async () => {
    Object.assign(routeQuery, { mon_range: 'custom', mon_start: '2026-10-22T02:00', mon_end: '2026-10-22T01:00' })
    const wrapper = mountPanel()
    await flushPromises()
    expect(query).not.toHaveBeenCalled()
    expect(wrapper.get('[role="alert"]').text()).toContain('admin.ops.monitoring.invalidRange')
  })
  it('opens a shared absolute window in local controls and preserves its UTC instants', async () => {
    Object.assign(routeQuery, { mon_range: 'custom', mon_start: '2026-10-22T02:00:00.000Z', mon_end: '2026-10-22T03:00:00.000Z' })
    const wrapper = mountPanel()
    await flushPromises()
    expect(query).toHaveBeenCalledWith(expect.objectContaining({ start: routeQuery.mon_start, end: routeQuery.mon_end }), expect.anything())
    const controls = wrapper.findAll('input[type="datetime-local"]')
    expect(new Date((controls[0].element as HTMLInputElement).value).toISOString()).toBe(routeQuery.mon_start)
    expect(replace).toHaveBeenCalledWith({ query: expect.objectContaining({ mon_start: routeQuery.mon_start, mon_end: routeQuery.mon_end }) })
  })
  it('shares edited local wall times as absolute UTC instants', async () => {
    Object.assign(routeQuery, { mon_range: 'custom', mon_start: '2026-10-22T10:00', mon_end: '2026-10-22T11:00' })
    mountPanel()
    await flushPromises()
    const params = query.mock.calls[0][0]
    expect(params.start).toBe(new Date('2026-10-22T10:00').toISOString())
    expect(replace).toHaveBeenCalledWith({ query: expect.objectContaining({ mon_start: params.start, mon_end: params.end }) })
    expect(params.start).toMatch(/Z$/)
  })
  it('queries bounded HTTP dimensions and removes status filters when switching to a histogram', async () => {
    getCapabilities.mockResolvedValue({ ...capability, metrics: [...capability.metrics, { id: 'http_p95_ms', unit: 'ms', kubernetes_only: false }] })
    Object.assign(routeQuery, { mon_protocol: 'sse', mon_route: 'gemini', mon_status_code: '502' })
    const wrapper = mountPanel()
    await flushPromises()
    expect(query.mock.calls[0][0]).toMatchObject({ protocol: 'sse', route: 'gemini', status_code: '502' })
    await wrapper.get('[data-testid="monitor-metric"]').setValue('http_p95_ms')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const params = query.mock.lastCall![0]
    expect(params).toMatchObject({ protocol: 'sse', route: 'gemini' })
    expect(params).not.toHaveProperty('status_code')
  })
  it('keeps external nginx metrics separate and does not expose unsupported dimensions', async () => {
    getCapabilities.mockResolvedValue({ ...capability, kubernetes: true, mode: 'kubernetes', sources: ['app', 'ingress', 'external_ingress'], external_ingress_adapter: 'nginx' })
    Object.assign(routeQuery, { mon_source: 'external_ingress', mon_status_code: '504', mon_service: 'relay' })
    const wrapper = mountPanel()
    await flushPromises()
    expect(query.mock.calls[0][0]).toMatchObject({ source: 'external_ingress', service: 'relay', status_code: '504' })
    expect(wrapper.find('[data-testid="monitor-filter-pod"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="monitor-filter-route"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="monitor-filter-protocol"]').exists()).toBe(false)
  })
})
