<template>
  <WorkspaceFrame section="finops">
    <div class="workspace-page">
      <section v-if="store.can('budget.read')" class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><h2>{{ t('workspace.finops') }}</h2><p>{{ t('workspace.budgetDescription') }}</p></div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button>
        </div>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <template v-else-if="available">
          <div class="workspace-budget__values">
            <div><span>{{ t('workspace.spent') }}</span><strong>{{ money(budget?.spent) }}</strong></div>
            <div><span>{{ t('workspace.reserved') }}</span><strong>{{ money(budget?.reserved) }}</strong></div>
            <div><span>{{ t('workspace.remaining') }}</span><strong>{{ money(budget?.remaining) }}</strong></div>
          </div>
          <p v-if="budget?.over_budget" class="workspace-warning" role="status">{{ t('workspace.overBudget') }}</p>
          <form v-if="store.can('budget.update')" class="workspace-form" @submit.prevent="saveBudget">
            <h3>{{ t('workspace.policy') }}</h3>
            <div class="workspace-form-grid">
              <label><span>{{ t('workspace.amount') }}</span><input v-model.number="budgetForm.amount" class="input" type="number" min="0" step="0.01" required></label>
              <label><span>{{ t('workspace.timezone') }}</span><input v-model="budgetForm.timezone" class="input" maxlength="80" required></label>
              <label class="workspace-checkbox"><input v-model="budgetForm.hard_limit" type="checkbox"><span>{{ t('workspace.hardLimit') }}</span></label>
              <label class="workspace-checkbox"><input v-model="budgetForm.enabled" type="checkbox"><span>{{ t('workspace.enabled') }}</span></label>
            </div>
            <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('workspace.updateBudget') }}</button></div>
          </form>
        </template>
        <div v-else class="workspace-state">{{ t('workspace.budgetUnavailable') }}</div>
      </section>
      <section v-if="store.can('usage.read')" class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.usage') }}</h2><p>{{ t('workspace.usageDescription') }}</p></div></div>
        <p>{{ t('workspace.historicalUsageNote') }}</p>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="usage || overview" class="workspace-usage">
          <div class="workspace-budget__values">
            <div><span>{{ t('workspace.requests') }}</span><strong>{{ usageSummary.requests ?? '—' }}</strong></div>
            <div><span>{{ t('workspace.spend') }}</span><strong>{{ money(usageSummary.spend) }}</strong></div>
          </div>
          <p v-if="usageSummary.start && usageSummary.end" class="workspace-period">{{ usageSummary.start }} – {{ usageSummary.end }}</p>
          <WorkspaceDailySpend v-if="overview?.daily_spend" :rows="overview.daily_spend" :timezone="usageSummary.timezone" />
          <WorkspaceUsageBreakdowns v-if="overview" :overview="overview" include-projects />
        </div>
        <div v-else class="workspace-state">{{ t('workspace.usageUnavailable') }}</div>
      </section>
      <section v-if="store.can('finops_anomaly.read')" class="workspace-panel" aria-labelledby="finops-anomalies-title">
        <div class="workspace-panel__heading">
          <div><h2 id="finops-anomalies-title">{{ t('workspace.anomalies') }}</h2><p>{{ t('workspace.anomaliesDescription') }}</p></div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="anomalyLoading" @click="() => loadAnomalies()">{{ t('common.refresh') }}</button>
        </div>
        <div v-if="anomalyStatus" class="anomaly-summary" aria-live="polite">
          <div><span>{{ t('workspace.anomalyOpen') }}</span><strong>{{ openAnomalies }}</strong></div>
          <div><span>{{ t('workspace.anomalyHighCritical') }}</span><strong>{{ highCriticalAnomalies }}</strong></div>
          <div><span>{{ t('workspace.anomalyLastDetected') }}</span><strong>{{ formatDate(lastDetected) }}</strong></div>
          <div><span>{{ t('workspace.anomalyDetectorStatus') }}</span><strong>{{ detectorStatusLabel }}</strong></div>
        </div>
        <form class="anomaly-filters" @submit.prevent="() => loadAnomalies()">
          <label><span>{{ t('workspace.anomalyStatus') }}</span><select v-model="anomalyFilters.status" class="input"><option value="">{{ t('workspace.anomalyAllStatuses') }}</option><option value="open">{{ t('workspace.anomalyStatuses.open') }}</option><option value="acknowledged">{{ t('workspace.anomalyStatuses.acknowledged') }}</option><option value="resolved">{{ t('workspace.anomalyStatuses.resolved') }}</option></select></label>
          <label><span>{{ t('workspace.anomalySeverity') }}</span><select v-model="anomalyFilters.severity" class="input"><option value="">{{ t('workspace.anomalyAllSeverities') }}</option><option value="low">{{ t('workspace.anomalySeverities.low') }}</option><option value="medium">{{ t('workspace.anomalySeverities.medium') }}</option><option value="high">{{ t('workspace.anomalySeverities.high') }}</option><option value="critical">{{ t('workspace.anomalySeverities.critical') }}</option></select></label>
          <label><span>{{ t('workspace.anomalyDetector') }}</span><select v-model="anomalyFilters.detector_type" class="input"><option value="">{{ t('workspace.anomalyAllDetectors') }}</option><option value="spend_spike">{{ t('workspace.anomalyDetectors.spend_spike') }}</option><option value="request_spike">{{ t('workspace.anomalyDetectors.request_spike') }}</option><option value="unit_cost_spike">{{ t('workspace.anomalyDetectors.unit_cost_spike') }}</option></select></label>
          <button type="submit" class="btn btn-secondary" :disabled="anomalyLoading">{{ t('workspace.anomalyApplyFilters') }}</button>
        </form>
        <p v-if="anomalyLoading" class="workspace-state" role="status">{{ t('common.loading') }}</p>
        <p v-else-if="anomalyError" class="workspace-warning" role="alert">{{ t('workspace.anomalyLoadError') }}</p>
        <div v-else-if="anomalies.length === 0" class="workspace-state">{{ t('workspace.noAnomalies') }}</div>
        <div v-else class="anomaly-list-wrap">
          <table class="anomaly-list"><caption class="sr-only">{{ t('workspace.anomalies') }}</caption><thead><tr><th>{{ t('workspace.anomalySeverity') }}</th><th>{{ t('workspace.anomalyDetector') }}</th><th>{{ t('workspace.anomalyScope') }}</th><th>{{ t('workspace.anomalyObserved') }}</th><th>{{ t('workspace.anomalyExpected') }}</th><th>{{ t('workspace.anomalyDelta') }}</th><th>{{ t('workspace.anomalyWindow') }}</th><th>{{ t('workspace.anomalyStatus') }}</th><th>{{ t('workspace.anomalyDetected') }}</th><th><span class="sr-only">{{ t('workspace.anomalyDetails') }}</span></th></tr></thead><tbody><tr v-for="item in anomalies" :key="item.id" @click="openAnomaly(item)"><td><span class="anomaly-badge" :class="`is-${item.severity}`">{{ severityLabel(item.severity) }}</span></td><td class="anomaly-technical">{{ detectorLabel(item.detector_type) }}</td><td><span class="anomaly-technical">{{ item.dimension_type }}</span><br><span class="anomaly-value">{{ item.dimension_value }}</span></td><td class="anomaly-number">{{ metricValue(item) }}</td><td class="anomaly-number">{{ expectedValue(item) }}</td><td class="anomaly-number">{{ deltaValue(item) }}</td><td class="anomaly-window">{{ formatWindow(item.window_start, item.window_end) }}</td><td>{{ statusLabel(item.status) }}</td><td class="anomaly-window">{{ formatDate(item.last_detected_at) }}</td><td><button type="button" class="btn btn-secondary btn-sm" @click.stop="openAnomaly(item)">{{ t('workspace.anomalyDetails') }}</button></td></tr></tbody></table>
        </div>
        <div v-if="selectedAnomaly" class="anomaly-detail" aria-live="polite">
          <div class="workspace-panel__heading"><div><h3>{{ t('workspace.anomalyEvidence') }}</h3><p>{{ selectedAnomaly.dimension_type }} / <span class="anomaly-technical">{{ selectedAnomaly.dimension_value }}</span></p></div><button type="button" class="btn btn-secondary btn-sm" @click="selectedAnomaly = null">{{ t('common.close') }}</button></div>
          <dl class="anomaly-evidence"><div><dt>{{ t('workspace.anomalyObserved') }}</dt><dd>{{ metricValue(selectedAnomaly) }}</dd></div><div><dt>{{ t('workspace.anomalyExpected') }}</dt><dd>{{ expectedValue(selectedAnomaly) }}</dd></div><div><dt>{{ t('workspace.anomalyDelta') }}</dt><dd>{{ deltaValue(selectedAnomaly) }} ({{ percentValue(selectedAnomaly.relative_increase) }})</dd></div><div><dt>{{ t('workspace.anomalyBaselineSamples') }}</dt><dd>{{ selectedAnomaly.baseline_sample_count }}</dd></div><div><dt>{{ t('workspace.anomalyScore') }}</dt><dd>{{ selectedAnomaly.score.toFixed(2) }}</dd></div><div><dt>{{ t('workspace.anomalyWindow') }}</dt><dd>{{ formatWindow(selectedAnomaly.window_start, selectedAnomaly.window_end) }}</dd></div></dl>
          <p class="anomaly-explanation">{{ anomalyExplanation(selectedAnomaly) }}</p>
          <div v-if="store.can('finops_anomaly.manage') && selectedAnomaly.status !== 'resolved'" class="anomaly-actions"><button v-if="selectedAnomaly.status === 'open'" type="button" class="btn btn-secondary" :disabled="anomalySaving" @click="transitionAnomaly('acknowledged')">{{ t('workspace.anomalyAcknowledge') }}</button><label class="anomaly-reason"><span>{{ t('workspace.anomalyResolutionReason') }}</span><input v-model="resolutionReason" class="input" maxlength="1000"></label><button type="button" class="btn btn-primary" :disabled="anomalySaving || !resolutionReason.trim()" @click="transitionAnomaly('resolved')">{{ t('workspace.anomalyResolve') }}</button></div>
        </div>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import WorkspaceUsageBreakdowns from '@/components/workspace/WorkspaceUsageBreakdowns.vue'
import WorkspaceDailySpend from '@/components/workspace/WorkspaceDailySpend.vue'
import { workspaceAPI, type WorkspaceBudget, type WorkspaceOverview, type WorkspaceUsage, type WorkspaceFinopsAnomaly, type WorkspaceFinopsAnomalyDetectorStatus, type WorkspaceFinopsAnomalyFilter } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'
const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const budget = ref<WorkspaceBudget | null>(null)
const usage = ref<WorkspaceUsage | null>(null)
const overview = ref<WorkspaceOverview | null>(null)
const anomalies = ref<WorkspaceFinopsAnomaly[]>([])
const anomalyStatus = ref<WorkspaceFinopsAnomalyDetectorStatus | null>(null)
const selectedAnomaly = ref<WorkspaceFinopsAnomaly | null>(null)
const anomalyLoading = ref(false)
const anomalySaving = ref(false)
const anomalyError = ref(false)
const resolutionReason = ref('')
const anomalyFilters = reactive<WorkspaceFinopsAnomalyFilter>({ status: undefined, severity: undefined, detector_type: undefined, page: 1, page_size: 50 })
const available = computed(() => budget.value !== null)
const loading = ref(false)
const saving = ref(false)
let generation = 0
let detailRequest = 0
let controller: AbortController | null = null
const budgetForm = reactive({ amount: 0, hard_limit: false, enabled: true, timezone: 'UTC' })
function money(value?: number) { return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '—' }
function preciseMoney(value?: number) { if (typeof value !== 'number' || !Number.isFinite(value)) return '—'; if (value !== 0 && Math.abs(value) < 0.01) return `$${value.toFixed(6).replace(/0+$/, '').replace(/\.$/, '')}`; return money(value) }
function formatDate(value?: string | null) { if (!value) return '—'; const date = new Date(value); return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString() }
function formatWindow(start: string, end: string) { return `${formatDate(start)} – ${formatDate(end)}` }
function severityLabel(value: string) { return t(`workspace.anomalySeverities.${value}`) }
function statusLabel(value: string) { return t(`workspace.anomalyStatuses.${value}`) }
function detectorLabel(value: string) { return t(`workspace.anomalyDetectors.${value}`) }
function metricValue(item: WorkspaceFinopsAnomaly) { return item.detector_type === 'request_spike' ? item.observed_requests.toLocaleString() : item.detector_type === 'unit_cost_spike' ? preciseMoney(item.observed_unit_cost) : preciseMoney(item.observed_spend) }
function expectedValue(item: WorkspaceFinopsAnomaly) { return item.detector_type === 'request_spike' ? item.expected_requests.toLocaleString() : item.detector_type === 'unit_cost_spike' ? preciseMoney(item.expected_unit_cost) : preciseMoney(item.expected_spend) }
function deltaValue(item: WorkspaceFinopsAnomaly) { return item.detector_type === 'request_spike' ? (item.observed_requests - item.expected_requests).toLocaleString() : item.detector_type === 'unit_cost_spike' ? preciseMoney(item.observed_unit_cost - item.expected_unit_cost) : preciseMoney(item.spend_delta) }
function percentValue(value: number) { return Number.isFinite(value) ? `${(value * 100).toFixed(0)}%` : '—' }
function anomalyExplanation(item: WorkspaceFinopsAnomaly) { return t('workspace.anomalyExplanation', { observed: metricValue(item), expected: expectedValue(item), increase: percentValue(item.relative_increase), samples: item.baseline_sample_count, score: item.score.toFixed(2) }) }
const openAnomalies = computed(() => anomalies.value.filter(item => item.status === 'open').length)
const highCriticalAnomalies = computed(() => anomalies.value.filter(item => item.severity === 'high' || item.severity === 'critical').length)
const lastDetected = computed(() => anomalies.value.map(item => item.last_detected_at).sort().at(-1) || anomalyStatus.value?.last_successful_scan)
const detectorStatusLabel = computed(() => anomalyStatus.value?.last_failure_code ? t('workspace.anomalyStatusDegraded') : anomalyStatus.value?.last_successful_scan ? t('workspace.anomalyStatusHealthy') : t('workspace.anomalyStatusPending'))
const budgetPolicy = computed(() => budget.value?.policy || budget.value?.workspace || budget.value?.project)
const usageSummary = computed(() => overview.value?.summary || usage.value?.summary || usage.value || {})

async function load() {
  const id = store.selectedWorkspaceId
  const currentGeneration = ++generation
  controller?.abort()
  budget.value = null
  usage.value = null
  overview.value = null
  anomalies.value = []
  anomalyStatus.value = null
  selectedAnomaly.value = null
  detailRequest += 1
  anomalyError.value = false
  if (!id) { loading.value = false; anomalyLoading.value = false; return }
  controller = new AbortController()
  const signal = controller.signal
  loading.value = true
  try {
    const [budgetResult] = await Promise.allSettled([store.can('budget.read') ? workspaceAPI.getBudget(id, signal) : Promise.resolve(null)])
    if (currentGeneration !== generation) return
    budget.value = budgetResult.status === 'fulfilled' ? budgetResult.value : null
    const policy = budgetPolicy.value
    Object.assign(budgetForm, { amount: policy?.amount ?? 0, hard_limit: policy?.hard_limit ?? false, enabled: policy?.enabled ?? true, timezone: policy?.timezone || 'UTC' })
    const params = budget.value?.period_start && budget.value.period_end
      ? { start: budget.value.period_start, end: budget.value.period_end, timezone: policy?.timezone || 'UTC' }
      : {}
    const [usageResult, overviewResult] = await Promise.allSettled([
      store.can('usage.read') ? workspaceAPI.getUsage(id, params, signal) : Promise.resolve(null),
      store.can('usage.read') ? workspaceAPI.getOverview(id, signal, params) : Promise.resolve(null),
    ])
    if (currentGeneration !== generation) return
    usage.value = usageResult.status === 'fulfilled' ? usageResult.value : null
    overview.value = overviewResult.status === 'fulfilled' ? overviewResult.value : null
    await loadAnomalies(id, signal, currentGeneration)
  } finally { if (currentGeneration === generation) loading.value = false }
}

async function loadAnomalies(id = store.selectedWorkspaceId, signal = controller?.signal, currentGeneration = generation) {
  if (!id || !store.can('finops_anomaly.read') || typeof workspaceAPI.listFinopsAnomalies !== 'function') return
  anomalyLoading.value = true
  anomalyError.value = false
  try {
    const [listResult, statusResult] = await Promise.allSettled([workspaceAPI.listFinopsAnomalies(id, { ...anomalyFilters }, signal), typeof workspaceAPI.getFinopsAnomalyStatus === 'function' ? workspaceAPI.getFinopsAnomalyStatus(id, signal) : Promise.resolve(null)])
    if (currentGeneration !== generation || id !== store.selectedWorkspaceId) return
    if (listResult.status === 'fulfilled') anomalies.value = listResult.value.items || []
    else anomalyError.value = true
    if (statusResult.status === 'fulfilled') anomalyStatus.value = statusResult.value
  } catch { if (currentGeneration === generation) anomalyError.value = true }
  finally { if (currentGeneration === generation) anomalyLoading.value = false }
}

async function openAnomaly(item: WorkspaceFinopsAnomaly) {
  const id = store.selectedWorkspaceId
  if (!id) return
  const request = ++detailRequest
  selectedAnomaly.value = item
  resolutionReason.value = ''
  const currentGeneration = generation
  if (typeof workspaceAPI.getFinopsAnomaly !== 'function') return
  try {
    const detail = await workspaceAPI.getFinopsAnomaly(id, item.id, controller?.signal)
    if (request === detailRequest && currentGeneration === generation && id === store.selectedWorkspaceId) selectedAnomaly.value = detail
  } catch { /* list evidence remains usable when detail fetch is unavailable */ }
}

async function transitionAnomaly(status: 'acknowledged' | 'resolved') {
  const id = store.selectedWorkspaceId
  const item = selectedAnomaly.value
  if (!id || !item || anomalySaving.value || typeof workspaceAPI.updateFinopsAnomaly !== 'function') return
  anomalySaving.value = true
  try {
    const updated = await workspaceAPI.updateFinopsAnomaly(id, item.id, { status, resolution_reason: resolutionReason.value.trim(), expected_version: item.version })
    if (id !== store.selectedWorkspaceId) return
    selectedAnomaly.value = updated
    const index = anomalies.value.findIndex(candidate => candidate.id === updated.id)
    if (index >= 0) anomalies.value[index] = updated
    resolutionReason.value = ''
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.anomalySaveError')) }
  finally { anomalySaving.value = false }
}

async function saveBudget() {
  const id = store.selectedWorkspaceId
  if (!id || saving.value) return
  saving.value = true
  try {
    await workspaceAPI.updateBudget(id, { ...budgetForm })
    if (id !== store.selectedWorkspaceId) return
    await load()
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.saveError')) }
  finally { saving.value = false }
}

onMounted(load)
watch([() => store.selectedWorkspaceId, () => store.permissions], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-panel__heading > * { min-width: 0; }
.workspace-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.workspace-panel h3 { margin: 0 0 10px; color: var(--color-text-primary); font-size: 16px; }
.workspace-panel p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.workspace-budget__values { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.workspace-budget__values div { display: grid; gap: 5px; }
.workspace-budget__values span { color: var(--color-text-muted); font-size: 12px; }
.workspace-budget__values strong { color: var(--color-text-primary); font-size: 20px; font-variant-numeric: tabular-nums; }
.workspace-warning { color: var(--color-warning) !important; }
.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
.workspace-usage { margin-top: 18px; }
.workspace-form { display: grid; gap: 14px; margin-top: 18px; }
.workspace-form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.workspace-form label { display: grid; gap: 6px; min-width: 0; color: var(--color-text-secondary); font-size: 13px; }
.workspace-form .workspace-checkbox { display: flex; align-items: center; gap: 8px; min-height: 36px; }
.workspace-checkbox input { width: 16px; height: 16px; accent-color: var(--color-primary); }
.workspace-period { overflow-wrap: anywhere; font-size: 12px !important; }
.workspace-actions { display: flex; flex-wrap: wrap; gap: 7px; }
.anomaly-summary { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.anomaly-summary div { display: grid; min-width: 0; gap: 5px; padding: 12px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface-soft); }
.anomaly-summary span, .anomaly-evidence dt { color: var(--color-text-muted); font-size: 12px; }
.anomaly-summary strong { color: var(--color-text-primary); font-size: 18px; font-variant-numeric: tabular-nums; }
.anomaly-filters { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; margin: 18px 0; }
.anomaly-filters label, .anomaly-reason { display: grid; min-width: 150px; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }
.anomaly-filters select { min-width: 150px; }
.anomaly-list-wrap { overflow-x: auto; margin-top: 4px; }
.anomaly-list { width: 100%; min-width: 920px; border-collapse: collapse; font-size: 12px; }
.anomaly-list th, .anomaly-list td { padding: 10px 8px; border-bottom: 1px solid var(--color-border); text-align: left; vertical-align: middle; }
.anomaly-list th { color: var(--color-text-muted); font-weight: 600; white-space: nowrap; }
.anomaly-list tbody tr { cursor: pointer; }
.anomaly-list tbody tr:hover, .anomaly-list tbody tr:focus-within { background: var(--color-primary-soft); }
.anomaly-badge { display: inline-flex; max-width: 100%; align-items: center; border-radius: 999px; padding: 3px 8px; font-size: 11px; font-weight: 600; line-height: 1.25; white-space: nowrap; }
.anomaly-badge.is-low { color: var(--color-text-secondary); background: var(--color-surface-soft); }
.anomaly-badge.is-medium { color: var(--color-warning); background: color-mix(in srgb, var(--color-warning) 12%, transparent); }
.anomaly-badge.is-high, .anomaly-badge.is-critical { color: var(--color-danger); background: color-mix(in srgb, var(--color-danger) 12%, transparent); }
.anomaly-technical, .anomaly-value, .anomaly-window, .anomaly-number { overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
.anomaly-technical { color: var(--color-text-secondary); font-family: var(--font-mono, ui-monospace, monospace); font-size: 11px; }
.anomaly-number { white-space: nowrap; }
.anomaly-detail { display: grid; gap: 14px; margin-top: 18px; padding-top: 18px; border-top: 1px solid var(--color-border); }
.anomaly-evidence { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 0; }
.anomaly-evidence div { display: grid; min-width: 0; gap: 5px; }
.anomaly-evidence dd { margin: 0; color: var(--color-text-primary); font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.anomaly-explanation { margin: 0 !important; padding: 10px 12px; border-left: 3px solid var(--color-primary); background: var(--color-primary-soft); }
.anomaly-actions { display: flex; flex-wrap: wrap; align-items: end; gap: 10px; }
.anomaly-reason { flex: 1 1 240px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } .workspace-budget__values, .workspace-form-grid { grid-template-columns: 1fr; } }
@media (max-width: 760px) { .anomaly-summary, .anomaly-evidence { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 480px) { .anomaly-summary, .anomaly-evidence { grid-template-columns: 1fr; } .anomaly-filters label, .anomaly-filters select { width: 100%; } }
</style>
