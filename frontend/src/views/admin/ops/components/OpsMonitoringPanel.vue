<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { Chart as ChartJS, CategoryScale, Legend, LineElement, LinearScale, PointElement, Tooltip, type Plugin } from 'chart.js'
import { Line } from 'vue-chartjs'
import { opsAPI, type OpsMonitoringCapabilities, type OpsMonitoringQuery, type OpsMonitoringResult } from '@/api/admin/ops'
import { getChartSeriesStyle, useChartThemeColors } from '@/utils/chartColors'
import OpsKubernetesEvents from './OpsKubernetesEvents.vue'
import OpsBillingHealth from './OpsBillingHealth.vue'
import { httpDimensionKeys, httpDimensionsFor, monitoringProtocols, monitoringRoutes, isHTTPMonitoringMetric } from '../monitoringFilters'

ChartJS.register(CategoryScale, Legend, LineElement, LinearScale, PointElement, Tooltip)
const props = defineProps<{ refreshToken?: number }>()
const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const colors = useChartThemeColors()
const capabilities = ref<OpsMonitoringCapabilities | null>(null)
const result = ref<OpsMonitoringResult | null>(null)
const comparison = ref<OpsMonitoringResult | null>(null)
const secondaryResult = ref<OpsMonitoringResult | null>(null)
const cursorIndex = ref(-1)
const showSamples = ref(false)
const samplePage = ref(0)
const samplePageSize = 200
const primaryChart = ref<{ chart: ChartJS<'line'> | null } | null>(null)
const secondaryChart = ref<{ chart: ChartJS<'line'> | null } | null>(null)
const appliedQuery = ref<OpsMonitoringQuery | null>(null)
const loading = ref(false)
const error = ref('')
const resourceKeys = ['cluster', 'namespace', 'service', 'deployment', 'pod', 'uid'] as const
const allowedRanges = ['15m', '1h', '6h', '24h', '7d', '30d', 'custom'] as const
type Range = typeof allowedRanges[number]
const routeText = (key: string, fallback = '') => typeof route.query[`mon_${key}`] === 'string' ? String(route.query[`mon_${key}`]) : fallback
const metric = ref(routeText('metric', 'qps'))
const secondaryMetric = ref(routeText('secondary'))
const range = ref<Range>(allowedRanges.includes(routeText('range') as Range) ? routeText('range') as Range : '1h')
const start = ref(sharedTimeInput(routeText('start')))
const end = ref(sharedTimeInput(routeText('end')))
const source = ref(['app', 'ingress', 'external_ingress'].includes(routeText('source')) ? routeText('source') : 'app')
const windowSize = ref<NonNullable<OpsMonitoringQuery['window']>>(['1m', '5m', '15m', '60m'].includes(routeText('window')) ? routeText('window') as NonNullable<OpsMonitoringQuery['window']> : '5m')
const step = ref(0)
const minRequests = ref(20)
const compare = ref(false)
const threshold = ref<number | ''>('')
const filters = reactive(Object.fromEntries(resourceKeys.map(key => [key, routeText(key)])) as Record<typeof resourceKeys[number], string>)
const httpFilters = reactive(Object.fromEntries(httpDimensionKeys.map(key => [key, routeText(key)])) as Record<typeof httpDimensionKeys[number], string>)
const visibleHTTPDimensions = computed(() => httpDimensionsFor(metric.value, source.value, capabilities.value?.external_ingress_adapter))
const sourceOptions = computed(() => (capabilities.value?.sources ?? ['app', 'ingress']).filter(item => item !== 'external_ingress' || (isHTTPMonitoringMetric(metric.value) && !metric.value.endsWith('_started') && !metric.value.startsWith('ttft_'))))
const resourceOptions = ref<Record<string, string[]>>({})
let controller: AbortController | null = null
let sequence = 0
let selectingFallback = false
const resourceKeysFor = (id: string) => source.value === 'external_ingress' ? resourceKeys.filter(key => ['cluster', 'namespace', 'service'].includes(key)) : id.startsWith('db_instance_') || id.startsWith('redis_instance_') ? resourceKeys.slice(0, 1) : id.startsWith('hpa_') ? resourceKeys.slice(0, 2) : id.startsWith('deployment_') ? resourceKeys.filter(key => ['cluster', 'namespace', 'deployment'].includes(key)) : resourceKeys
const visibleResourceKeys = computed(() => resourceKeysFor(metric.value))
const appOnlyMetric = (id: string) => id.endsWith('_started') || id.startsWith('db_instance_') || id.startsWith('redis_instance_') || id.startsWith('db_pool_') || id.startsWith('redis_pool_')
const secondaryOptions = computed(() => (capabilities.value?.metrics ?? []).filter(item => item.id !== metric.value && !(source.value !== 'app' && appOnlyMetric(item.id)) && !(source.value === 'external_ingress' && (!isHTTPMonitoringMetric(item.id) || item.id.startsWith('ttft_'))) && httpDimensionKeys.every(key => !httpFilters[key] || httpDimensionsFor(item.id, source.value, capabilities.value?.external_ingress_adapter).includes(key)) && resourceKeys.every(key => !filters[key] || !visibleResourceKeys.value.includes(key) || resourceKeysFor(item.id).includes(key))))
const secondaryUnit = computed(() => secondaryResult.value?.unit === 'bytes' ? 'MiB' : secondaryResult.value?.unit ?? '')
const displayUnit = computed(() => result.value?.unit === 'bytes' ? 'MiB' : result.value?.unit ?? selectedMetric.value?.unit ?? '')
const plotValue = (value: number | null | undefined) => value == null ? null : result.value?.unit === 'bytes' ? value / 1048576 : value
const selectedMetric = computed(() => capabilities.value?.metrics.find(item => item.id === metric.value))
const kubernetes = computed(() => capabilities.value?.kubernetes === true)
const labelFor = (id: string) => te(`admin.ops.monitoring.metrics.${id}`) ? t(`admin.ops.monitoring.metrics.${id}`) : id
const sourceSelectable = computed(() => !selectedMetric.value?.kubernetes_only && !appOnlyMetric(metric.value))
const state = computed(() => error.value ? 'Unknown' : result.value?.state ?? 'Unknown')
const seriesName = (labels: Record<string, string>) => Object.entries(labels).map(([key, value]) => `${key}=${value}`).join(' · ') || t('admin.ops.monitoring.aggregate')
const rangeSeconds: Record<Exclude<Range, 'custom'>, number> = { '15m': 900, '1h': 3600, '6h': 21600, '24h': 86400, '7d': 604800, '30d': 2592000 }
const shortSeriesName = (labels: Record<string, string>, index: number) => `${index + 1}. ${labels.pod ?? labels.service ?? labels.namespace ?? t('admin.ops.monitoring.aggregate')}`
const dateText = (timestamp: string) => new Date(timestamp).toLocaleString()
function formatValue(value: number | null | undefined) {
  if (value == null || !Number.isFinite(value)) return t('admin.ops.monitoring.states.Unknown')
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 }).format(plotValue(value)!)
}
function toLocalInput(date: Date) {
  return new Date(date.getTime() - date.getTimezoneOffset() * 60_000).toISOString().slice(0, 19)
}
function sharedTimeInput(value: string) {
  // Shared URLs contain an absolute instant. datetime-local only accepts local
  // wall time; convert at this boundary, never reinterpret UTC as wall time.
  if (!value) return ''
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? toLocalInput(date) : value
}
function queryBounds() {
  const until = range.value === 'custom' ? new Date(end.value) : new Date()
  const since = range.value === 'custom' ? new Date(start.value) : new Date(until.getTime() - rangeSeconds[range.value] * 1000)
  const duration = until.getTime() - since.getTime()
  if (!Number.isFinite(duration) || duration <= 0 || duration > (capabilities.value?.max_range_seconds ?? 31 * 86400) * 1000) throw new Error(t('admin.ops.monitoring.invalidRange'))
  return { since, until, duration }
}
async function syncQuery(since: Date, until: Date) {
  const next = { ...route.query }
  const values: Record<string, string> = { metric: metric.value, secondary: secondaryMetric.value, range: range.value, start: range.value === 'custom' ? since.toISOString() : '', end: range.value === 'custom' ? until.toISOString() : '', source: source.value, window: windowSize.value, ...filters, ...httpFilters }
  for (const [key, value] of Object.entries(values)) {
    if (value) next[`mon_${key}`] = value
    else delete next[`mon_${key}`]
  }
  await router.replace({ query: next })
}
async function load() {
  const request = ++sequence
  controller?.abort()
  controller = new AbortController()
  const signal = controller.signal
  loading.value = true
  error.value = ''
  // Never leave an earlier resource's values under newly selected filters.
  result.value = null
  appliedQuery.value = null
  comparison.value = null
  secondaryResult.value = null
  cursorIndex.value = -1
  showSamples.value = false
  samplePage.value = 0
  try {
    if (!capabilities.value) capabilities.value = await opsAPI.getMonitoringCapabilities({ signal })
    if (!capabilities.value.metrics.some(item => item.id === metric.value)) {
      selectingFallback = true
      metric.value = capabilities.value.metrics[0]?.id ?? 'qps'
      selectingFallback = false
    }
    if (secondaryMetric.value && !secondaryOptions.value.some(item => item.id === secondaryMetric.value)) { selectingFallback = true; secondaryMetric.value = ''; selectingFallback = false }
    const { since, until, duration } = queryBounds()
    const params: OpsMonitoringQuery = {
      metric: metric.value, source: sourceSelectable.value ? source.value : 'app', window: windowSize.value,
      start: since.toISOString(), end: until.toISOString(), min_requests: minRequests.value,
      ...(step.value > 0 ? { step: step.value } : {}),
      ...Object.fromEntries(visibleHTTPDimensions.value.filter(key => httpFilters[key]).map(key => [key, httpFilters[key]])),
      ...Object.fromEntries(Object.entries(filters).filter(([key, value]) => value.trim() && visibleResourceKeys.value.includes(key as typeof resourceKeys[number])).map(([key, value]) => [key, value.trim()]))
    }
    const responses = await Promise.all([
      opsAPI.queryMonitoring(params, { signal }),
      compare.value ? opsAPI.queryMonitoring({ ...params, start: new Date(since.getTime() - duration).toISOString(), end: since.toISOString() }, { signal }) : Promise.resolve(null),
      secondaryMetric.value ? opsAPI.queryMonitoring({ ...params, metric: secondaryMetric.value }, { signal }) : Promise.resolve(null)
    ])
    if (request !== sequence) return
    appliedQuery.value = params
    result.value = responses[0]
    comparison.value = responses[1]
    secondaryResult.value = responses[2]
    await syncQuery(since, until)
  } catch (err) {
    if (request !== sequence || signal.aborted) return
    error.value = err instanceof Error && err.message === t('admin.ops.monitoring.invalidRange') ? err.message : t('admin.ops.monitoring.queryFailed')
  } finally {
    if (request === sequence) loading.value = false
  }
}
async function discoverResources() {
  if (!kubernetes.value) return
  try {
    const data = await opsAPI.queryMonitoring({ metric: 'pod_info' })
    resourceOptions.value = Object.fromEntries(resourceKeys.map(key => [key, [...new Set(data.series.map(series => series.labels[key]).filter(Boolean))].sort()]))
  } catch { resourceOptions.value = {} }
}
function clearChildren(key: string) {
  const hierarchy = ['cluster', 'namespace', 'deployment', 'service', 'pod', 'uid']
  const index = hierarchy.indexOf(key)
  for (const child of hierarchy.slice(index + 1)) filters[child as typeof resourceKeys[number]] = ''
}
function drillDown(labels: Record<string, string>) {
  for (const key of resourceKeys) filters[key] = labels[key] ?? ''
  void load()
}
function zoom() {
  try {
    const { since, until } = queryBounds()
    range.value = 'custom'
    start.value = toLocalInput(new Date((since.getTime() + until.getTime()) / 2))
    end.value = toLocalInput(until)
    void load()
  } catch { error.value = t('admin.ops.monitoring.invalidRange') }
}
const timestamps = computed(() => [...new Set([...(result.value?.series ?? []), ...(secondaryResult.value?.series ?? [])].flatMap(series => series.points.map(point => point.timestamp)))].sort())
const samplePageCount = computed(() => Math.ceil(timestamps.value.length / samplePageSize))
const sampleRows = computed(() => timestamps.value.slice(samplePage.value * samplePageSize, (samplePage.value + 1) * samplePageSize).map((timestamp, index) => ({ timestamp, index: samplePage.value * samplePageSize + index })))
const displayedSeries = computed(() => (result.value?.series ?? []).slice(0, 6))
const pointStateMaps = computed(() => displayedSeries.value.map(series => new Map(series.points.map(point => [point.timestamp, point.state ?? 'Unknown']))))
const secondaryPointStateMaps = computed(() => (secondaryResult.value?.series ?? []).slice(0, 6).map(series => new Map(series.points.map(point => [point.timestamp, point.state ?? 'Unknown']))))
const chartData = computed(() => {
  const datasets = displayedSeries.value.map((series, index) => {
    const points = new Map(series.points.map(point => [point.timestamp, point.value]))
    return {
      label: shortSeriesName(series.labels, index), data: timestamps.value.map(timestamp => plotValue(points.get(timestamp))),
      borderColor: index % 2 === 0 ? colors.value.primary : colors.value.secondary,
      ...getChartSeriesStyle(index), pointRadius: 0, pointHitRadius: 8, borderWidth: 2, spanGaps: false
    }
  })
  if (comparison.value) {
    const previous = comparison.value.series.slice(0, 6)
    for (const [index, series] of previous.entries()) {
      datasets.push({ label: `${t('admin.ops.monitoring.previousPeriod')} · ${shortSeriesName(series.labels, index)}`,
        data: timestamps.value.map((_, i) => plotValue(series.points[i]?.value)), borderColor: colors.value.neutral,
        ...getChartSeriesStyle(index), borderDash: [5, 5], pointRadius: 0, pointHitRadius: 8, borderWidth: 1, spanGaps: false })
    }
  }
  if (threshold.value !== '' && Number.isFinite(threshold.value)) datasets.push({
    label: t('admin.ops.monitoring.threshold'), data: timestamps.value.map(() => Number(threshold.value)), borderColor: colors.value.warning,
    ...getChartSeriesStyle(0), borderDash: [3, 3], pointRadius: 0, pointHitRadius: 0, borderWidth: 1, spanGaps: false
  })
  return { labels: timestamps.value.map(dateText), datasets }
})
const secondaryChartData = computed(() => ({
  labels: timestamps.value.map(dateText),
  datasets: (secondaryResult.value?.series ?? []).slice(0, 6).map((series, index) => {
    const points = new Map(series.points.map(point => [point.timestamp, point.value]))
    return { label: shortSeriesName(series.labels, index), data: timestamps.value.map(timestamp => {
      const value = points.get(timestamp)
      return value == null ? null : secondaryResult.value?.unit === 'bytes' ? value / 1048576 : value
    }), borderColor: index % 2 === 0 ? colors.value.secondary : colors.value.primary, ...getChartSeriesStyle(index),
    pointRadius: 0, pointHitRadius: 8, borderWidth: 2, spanGaps: false }
  })
}))
const secondaryHasValues = computed(() => secondaryResult.value?.series.some(series => series.points.some(point => point.value != null)))
const linkedCursor: Plugin<'line'> = {
  id: 'ops-linked-cursor',
  afterEvent(chart, args) {
    if (args.event.type === 'mouseout') { cursorIndex.value = -1; return }
    if (args.event.type !== 'mousemove' || args.event.x == null || !args.inChartArea) return
    const index = Number(chart.scales.x.getValueForPixel(args.event.x))
    if (Number.isFinite(index)) cursorIndex.value = Math.max(0, Math.min(timestamps.value.length - 1, Math.round(index)))
  },
  afterDraw(chart) {
    if (cursorIndex.value < 0) return
    const x = chart.scales.x.getPixelForValue(cursorIndex.value)
    const { ctx, chartArea } = chart
    ctx.save(); ctx.beginPath(); ctx.setLineDash([3, 3]); ctx.strokeStyle = colors.value.neutral
    ctx.moveTo(x, chartArea.top); ctx.lineTo(x, chartArea.bottom); ctx.stroke(); ctx.restore()
  }
}
watch(cursorIndex, index => {
  for (const component of [primaryChart.value, secondaryChart.value]) {
    const chart = component?.chart
    if (!chart) continue
    const active = index < 0 ? [] : chart.data.datasets.flatMap((dataset, datasetIndex) => dataset.data[index] == null ? [] : [{ datasetIndex, index }])
    chart.setActiveElements(active)
    const point = active.length ? chart.getDatasetMeta(active[0].datasetIndex).data[index] : null
    chart.tooltip?.setActiveElements(active, point ? { x: point.x, y: point.y } : { x: 0, y: 0 })
    chart.update('none')
  }
}, { flush: 'post' })
const chartOptions = computed(() => ({
  responsive: true, maintainAspectRatio: false, animation: false as const,
  interaction: { intersect: false, mode: 'index' as const },
  plugins: { legend: { labels: { color: colors.value.text, boxWidth: 18 } }, tooltip: { backgroundColor: colors.value.surface, titleColor: colors.value.text, bodyColor: colors.value.text } },
  scales: { x: { ticks: { color: colors.value.text, maxTicksLimit: 4, maxRotation: 0, callback: (_value: unknown, index: number) => timestamps.value[index] ? new Date(timestamps.value[index]).toLocaleString(undefined, { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' }) : '' }, grid: { display: false } }, y: { title: { display: true, text: displayUnit.value }, ticks: { color: colors.value.text }, grid: { color: colors.value.grid } } }
}))
const secondaryChartOptions = computed(() => ({ ...chartOptions.value, scales: { ...chartOptions.value.scales, y: { ...chartOptions.value.scales.y, title: { display: true, text: secondaryUnit.value } } } }))
const hasValues = computed(() => displayedSeries.value.some(series => series.points.some(point => point.value != null)))
watch(metric, value => {
  for (const key of resourceKeys) if (!resourceKeysFor(value).includes(key)) filters[key] = ''
  if (appOnlyMetric(value)) source.value = 'app'
}, { flush: 'sync' })
watch([metric, source], () => {
  for (const key of httpDimensionKeys) if (!visibleHTTPDimensions.value.includes(key)) httpFilters[key] = ''
  for (const key of resourceKeys) if (!visibleResourceKeys.value.includes(key)) filters[key] = ''
  if (!sourceOptions.value.includes(source.value)) source.value = 'app'
}, { flush: 'sync' })
watch(() => props.refreshToken, () => { if (!loading.value) void load() })
watch([metric, secondaryMetric, range, start, end, source, windowSize, minRequests, step, filters, httpFilters], () => {
  if (selectingFallback) return
  sequence++
  controller?.abort()
  loading.value = false
  result.value = null
  appliedQuery.value = null
  comparison.value = null
  secondaryResult.value = null
  cursorIndex.value = -1
}, { deep: true, flush: 'sync' })
onMounted(async () => { await load(); void discoverResources() })
onBeforeUnmount(() => { sequence++; controller?.abort() })
</script>

<template>
  <section class="monitor-panel" aria-labelledby="monitor-title">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="min-w-0">
        <h2 id="monitor-title" class="text-lg font-semibold">{{ t('admin.ops.monitoring.title') }}</h2>
        <p class="mt-1 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.description') }}</p>
      </div>
      <span class="rounded-lg bg-[var(--color-surface-soft)] px-3 py-2 text-sm">{{ capabilities ? t(`admin.ops.monitoring.modes.${capabilities.mode}`) : t('admin.ops.monitoring.states.Unknown') }}</span>
    </div>

    <form class="mt-4 space-y-4" @submit.prevent="load">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <label class="monitor-label">{{ t('admin.ops.monitoring.metric') }}
          <select v-model="metric" class="input" data-testid="monitor-metric">
            <option v-for="item in capabilities?.metrics ?? []" :key="item.id" :value="item.id">{{ labelFor(item.id) }} ({{ item.unit }}){{ item.kubernetes_only && !kubernetes ? ` · ${t('admin.ops.monitoring.states.NotApplicable')}` : '' }}</option>
          </select>
        </label>
        <label class="monitor-label">{{ t('admin.ops.monitoring.range') }}
          <select v-model="range" class="input" data-testid="monitor-range"><option v-for="item in allowedRanges" :key="item" :value="item">{{ item === 'custom' ? t('admin.ops.monitoring.custom') : item }}</option></select>
        </label>
        <label class="monitor-label">{{ t('admin.ops.monitoring.window') }}
          <select v-model="windowSize" class="input"><option v-for="item in ['1m', '5m', '15m', '60m']" :key="item">{{ item }}</option></select>
        </label>
        <label class="monitor-label">{{ t('admin.ops.monitoring.filters.source') }}
          <select v-model="source" class="input" :disabled="!sourceSelectable" data-testid="monitor-source"><option v-for="item in sourceOptions" :key="item" :value="item">{{ t(`admin.ops.monitoring.${item === 'app' ? 'appSource' : item === 'ingress' ? 'ingressSource' : 'externalIngressSource'}`) }}</option></select>
        </label>
      </div>
      <p v-if="!sourceSelectable" class="text-xs text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.fixedSource') }}</p>
      <label class="monitor-label">{{ t('admin.ops.monitoring.secondaryMetric') }}
        <select v-model="secondaryMetric" class="input" data-testid="monitor-secondary"><option value="">{{ t('admin.ops.monitoring.noSecondary') }}</option><option v-for="item in secondaryOptions" :key="item.id" :value="item.id">{{ labelFor(item.id) }} ({{ item.unit }})</option></select>
      </label>
      <div v-if="range === 'custom'" class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <label class="monitor-label">{{ t('admin.ops.monitoring.start') }}<input v-model="start" class="input" type="datetime-local" required /></label>
        <label class="monitor-label">{{ t('admin.ops.monitoring.end') }}<input v-model="end" class="input" type="datetime-local" required /></label>
      </div>
      <fieldset v-if="kubernetes" class="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <legend class="mb-2 text-sm font-semibold">{{ t('admin.ops.monitoring.resourceFilters') }}</legend>
        <label v-for="key in visibleResourceKeys" :key="key" class="monitor-label">{{ t(`admin.ops.monitoring.filters.${key}`) }}
          <input v-model="filters[key]" :list="`monitor-resources-${key}`" class="input" :data-testid="`monitor-filter-${key}`" @change="clearChildren(key)" />
          <datalist :id="`monitor-resources-${key}`"><option v-for="value in resourceOptions[key] ?? []" :key="value" :value="value" /></datalist>
        </label>
      </fieldset>
      <p v-else-if="capabilities" class="text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.standaloneHint') }}</p>
      <fieldset v-if="visibleHTTPDimensions.length" class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <legend class="mb-2 text-sm font-semibold">{{ t('admin.ops.monitoring.httpFilters') }}</legend>
        <label v-for="key in visibleHTTPDimensions" :key="key" class="monitor-label">{{ t(`admin.ops.monitoring.filters.${key}`) }}
          <select v-if="key !== 'status_code'" v-model="httpFilters[key]" class="input" :data-testid="`monitor-filter-${key}`">
            <option value="">{{ t('admin.ops.monitoring.allValues') }}</option>
            <option v-for="item in key === 'protocol' ? monitoringProtocols : monitoringRoutes" :key="item" :value="item">{{ item }}</option>
          </select>
          <input v-else v-model="httpFilters[key]" class="input" inputmode="numeric" pattern="[1-5][0-9]{2}" maxlength="3" placeholder="502" :data-testid="`monitor-filter-${key}`" />
        </label>
      </fieldset>
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <label class="monitor-label">{{ t('admin.ops.monitoring.step') }}<input v-model.number="step" class="input" type="number" min="0" step="1" /></label>
        <label class="monitor-label">{{ t('admin.ops.alertRules.form.minRequests') }}<input v-model.number="minRequests" class="input" type="number" min="0" max="1000000" step="1" /></label>
        <label class="monitor-label">{{ t('admin.ops.monitoring.threshold') }} ({{ displayUnit }})<input v-model.number="threshold" class="input" type="number" step="any" /></label>
      </div>
      <div class="flex flex-wrap items-center gap-3">
        <button type="submit" class="btn btn-primary" :disabled="loading">{{ loading ? t('common.loading') : t('admin.ops.monitoring.query') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="loading || !result" @click="zoom">{{ t('admin.ops.monitoring.zoom') }}</button>
        <label class="flex items-center gap-2 text-sm"><input v-model="compare" type="checkbox" />{{ t('admin.ops.monitoring.compare') }}</label>
      </div>
    </form>

    <div class="mt-4 flex flex-wrap items-center gap-2 text-sm" role="status" aria-live="polite">
      <strong>{{ loading ? t('common.loading') : t(`admin.ops.monitoring.states.${state}`) }}</strong>
      <span v-if="displayUnit">· {{ displayUnit }}</span>
      <span v-if="result?.step_seconds">· {{ t('admin.ops.monitoring.actualStep', { value: result.step_seconds }) }}</span>
      <span>· Prometheus{{ sourceSelectable ? ` / ${source}` : '' }}</span>
    </div>
    <p v-if="error" role="alert" class="mt-2 text-sm text-[var(--color-danger)]">{{ error }}</p>
    <p v-if="result?.reason" class="mt-2 break-words text-sm text-[var(--color-text-secondary)]">{{ result.reason }}</p>
    <p v-if="capabilities && !capabilities.prometheus_configured" class="mt-2 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.noPrometheus') }}</p>
    <p v-if="selectedMetric?.kubernetes_only && !kubernetes" class="mt-2 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.standaloneHint') }}</p>
    <p v-if="result?.series.some(series => series.labels.limit_source === 'regular_container_sum') || secondaryResult?.series.some(series => series.labels.limit_source === 'regular_container_sum')" class="mt-2 text-sm text-[var(--color-warning)]">{{ t('admin.ops.monitoring.limitFallback') }}</p>
    <label v-if="secondaryResult && timestamps.length" class="monitor-label mt-4">{{ t('admin.ops.monitoring.linkedCursor') }}: {{ cursorIndex >= 0 ? dateText(timestamps[cursorIndex]) : '—' }}
      <input v-model.number="cursorIndex" type="range" min="0" :max="timestamps.length - 1" step="1" class="w-full" data-testid="monitor-cursor" />
    </label>
    <div class="mt-4 grid min-w-0 grid-cols-1 gap-5" :class="secondaryResult ? 'xl:grid-cols-2' : ''">
      <div v-if="hasValues" class="min-w-0"><h3 class="mb-2 text-sm font-semibold">{{ labelFor(metric) }}</h3><div class="h-80"><Line ref="primaryChart" :data="chartData" :options="chartOptions" :plugins="[linkedCursor]" :aria-label="labelFor(metric)" /></div>
        <p v-if="cursorIndex >= 0" class="mt-2 break-words text-xs">{{ displayedSeries.map((series, index) => `${shortSeriesName(series.labels, index)}: ${chartData.datasets[index].data[cursorIndex] == null ? t(`admin.ops.monitoring.states.${pointStateMaps[index].get(timestamps[cursorIndex]) ?? 'Unknown'}`) : Number(chartData.datasets[index].data[cursorIndex]).toLocaleString(undefined, { maximumFractionDigits: 3 })} ${displayUnit}`).join(' · ') }}</p>
      </div>
      <div v-if="secondaryResult" class="min-w-0"><h3 class="mb-2 text-sm font-semibold">{{ labelFor(secondaryMetric) }} · {{ t(`admin.ops.monitoring.states.${secondaryResult.state}`) }}</h3>
        <div v-if="secondaryHasValues" class="h-80"><Line ref="secondaryChart" :data="secondaryChartData" :options="secondaryChartOptions" :plugins="[linkedCursor]" :aria-label="labelFor(secondaryMetric)" /></div>
        <p v-else class="py-8 text-sm">{{ t('admin.ops.monitoring.noValues') }}</p>
        <p v-if="cursorIndex >= 0" class="mt-2 break-words text-xs">{{ secondaryChartData.datasets.map((series, index) => `${series.label}: ${series.data[cursorIndex] == null ? t(`admin.ops.monitoring.states.${secondaryPointStateMaps[index].get(timestamps[cursorIndex]) ?? 'Unknown'}`) : Number(series.data[cursorIndex]).toLocaleString(undefined, { maximumFractionDigits: 3 })} ${secondaryUnit}`).join(' · ') }}</p>
        <details class="mt-3 text-sm"><summary class="cursor-pointer py-2">{{ t('admin.ops.monitoring.latestValues') }}</summary><ul class="space-y-2"><li v-for="(series, index) in secondaryResult.series" :key="index" class="break-words font-mono text-xs">{{ index + 1 }}. {{ seriesName(series.labels) }} · {{ series.points.at(-1)?.value ?? t('admin.ops.monitoring.states.Unknown') }} {{ secondaryResult.unit }}</li></ul></details>
      </div>
    </div>
    <p v-if="!loading && !hasValues" class="py-8 text-center text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.noValues') }}</p>
    <p v-if="(result?.series.length ?? 0) > 6" class="mt-2 text-xs text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.seriesLimit') }}</p>
    <p v-if="comparison && comparison.state !== 'Available'" class="mt-2 text-sm">{{ t('admin.ops.monitoring.previousPeriod') }}: {{ t(`admin.ops.monitoring.states.${comparison.state}`) }}</p>

    <div v-if="result?.series.length" class="mt-4 overflow-x-auto rounded-xl border border-[var(--color-border)]">
      <table class="w-full text-left text-sm">
        <caption class="p-3 text-left font-semibold">{{ t('admin.ops.monitoring.latestValues') }}</caption>
        <thead class="bg-[var(--color-surface-soft)]"><tr><th class="p-3">{{ t('admin.ops.monitoring.resource') }}</th><th class="p-3">{{ t('admin.ops.monitoring.timestamp') }}</th><th class="p-3">{{ t('admin.ops.monitoring.value') }}</th><th v-if="kubernetes" class="p-3">{{ t('admin.ops.monitoring.drillDown') }}</th></tr></thead>
        <tbody><tr v-for="(series, index) in result.series" :key="index" class="border-t border-[var(--color-border)]">
          <td class="max-w-lg break-words p-3 font-mono text-xs">{{ index + 1 }}. {{ seriesName(series.labels) }}</td><td class="whitespace-nowrap p-3">{{ series.points.at(-1) ? dateText(series.points.at(-1)!.timestamp) : '—' }}</td><td class="whitespace-nowrap p-3 font-mono">{{ series.points.at(-1)?.value == null ? t(`admin.ops.monitoring.states.${series.points.at(-1)?.state ?? 'Unknown'}`) : formatValue(series.points.at(-1)?.value) }} {{ displayUnit }}</td>
          <td v-if="kubernetes" class="p-3"><button class="btn btn-sm btn-secondary" @click="drillDown(series.labels)">{{ t('admin.ops.monitoring.drillDown') }}</button></td>
        </tr></tbody>
      </table>
    </div>
    <details v-if="hasValues" class="mt-4 text-sm" @toggle="showSamples = ($event.target as HTMLDetailsElement).open"><summary class="cursor-pointer rounded-lg py-2 font-semibold focus-visible:outline focus-visible:outline-2 focus-visible:outline-[var(--color-primary)]">{{ t('admin.ops.monitoring.samples') }}</summary>
      <div v-if="showSamples" class="max-h-80 overflow-auto"><table class="w-full text-left"><thead><tr><th class="p-2">{{ t('admin.ops.monitoring.timestamp') }}</th><th v-for="(series, i) in displayedSeries" :key="i" class="p-2">{{ seriesName(series.labels) }}</th></tr></thead><tbody><tr v-for="row in sampleRows" :key="row.timestamp"><td class="whitespace-nowrap p-2">{{ dateText(row.timestamp) }}</td><td v-for="(_, j) in displayedSeries" :key="j" class="p-2 font-mono">{{ chartData.datasets[j].data[row.index] == null ? t(`admin.ops.monitoring.states.${pointStateMaps[j].get(row.timestamp) ?? 'Unknown'}`) : Number(chartData.datasets[j].data[row.index]).toLocaleString(undefined, { maximumFractionDigits: 3 }) }}</td></tr></tbody></table></div>
      <div v-if="showSamples && samplePageCount > 1" class="mt-3 flex flex-wrap items-center gap-3"><button class="btn btn-sm btn-secondary" :disabled="samplePage === 0" @click="samplePage--">{{ t('admin.ops.monitoring.previousSamples') }}</button><span>{{ samplePage + 1 }} / {{ samplePageCount }}</span><button class="btn btn-sm btn-secondary" :disabled="samplePage + 1 >= samplePageCount" @click="samplePage++">{{ t('admin.ops.monitoring.nextSamples') }}</button></div>
    </details>
    <p class="mt-4 border-t border-[var(--color-border)] pt-4 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.logsHint') }}</p>
    <OpsKubernetesEvents v-if="capabilities?.kubernetes && appliedQuery" :query="appliedQuery" :enabled="capabilities.kubernetes_events === true" />
    <OpsBillingHealth />
  </section>
</template>

<style scoped>
.monitor-panel { border: 1px solid var(--color-border); border-radius: 16px; background: var(--color-surface); padding: 20px; color: var(--color-text-primary); min-width: 0; }
.monitor-label { display: flex; min-width: 0; flex-direction: column; gap: 6px; font-size: 14px; }
.monitor-panel input, .monitor-panel select { min-width: 0; }
@media (max-width: 640px) { .monitor-panel { padding: 16px; } .monitor-panel button, .monitor-panel select { min-height: 44px; } }
</style>
