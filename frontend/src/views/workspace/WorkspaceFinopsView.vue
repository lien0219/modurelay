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
      <section v-if="store.can('usage.read') && allocationSupported" class="workspace-panel" aria-labelledby="allocation-report-title">
        <div class="workspace-panel__heading">
          <div><h2 id="allocation-report-title">{{ t('workspace.allocationBreakdown') }}</h2><p>{{ t('workspace.allocationDescription') }}</p></div>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="allocationLoading" @click="() => loadAllocation()">{{ t('common.refresh') }}</button>
        </div>
        <form class="allocation-filters" @submit.prevent="() => loadAllocation()">
          <label><span>{{ t('workspace.environment') }}</span><select v-model="allocationFilters.environment" class="input"><option value="">{{ t('workspace.allEnvironments') }}</option><option value="production">{{ t('workspace.environments.production') }}</option><option value="staging">{{ t('workspace.environments.staging') }}</option><option value="development">{{ t('workspace.environments.development') }}</option><option value="testing">{{ t('workspace.environments.testing') }}</option><option value="unallocated">{{ t('workspace.unallocated') }}</option></select></label>
          <label><span>{{ t('workspace.costCenter') }}</span><select v-model="allocationFilters.cost_center_id" class="input"><option :value="undefined">{{ t('workspace.allCostCenters') }}</option><option v-for="center in costCenters" :key="center.id" :value="center.id">{{ center.code }} · {{ center.name }}</option></select></label>
          <label><span>{{ t('workspace.tagKey') }}</span><input v-model="allocationFilters.tag_key" class="input" maxlength="63" autocomplete="off"></label>
          <label><span>{{ t('workspace.tagValue') }}</span><input v-model="allocationFilters.tag_value" class="input" maxlength="255" autocomplete="off"></label>
          <button type="submit" class="btn btn-secondary" :disabled="allocationLoading">{{ t('workspace.applyAllocationFilters') }}</button>
        </form>
        <p v-if="allocationError" class="workspace-warning" role="alert">{{ t('workspace.allocationLoadError') }}</p>
        <div v-if="allocationLoading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <template v-else-if="allocationReport">
          <div class="workspace-budget__values allocation-totals">
            <div><span>{{ t('workspace.workspaceTotal') }}</span><strong>{{ money(allocationReport.workspace_total) }}</strong></div>
            <div><span>{{ t('workspace.allocatedSpend') }}</span><strong>{{ money(allocationReport.allocated) }}</strong></div>
            <div><span>{{ t('workspace.unallocated') }}</span><strong>{{ money(allocationReport.unallocated) }}</strong></div>
          </div>
          <p class="workspace-period">{{ allocationReport.overlapping_tags ? t('workspace.overlappingTagsNote') : '' }}</p>
          <div class="allocation-groups">
            <div class="allocation-group"><h3>{{ t('workspace.costCenters') }}</h3><div class="allocation-table-wrap"><table class="allocation-table"><caption class="sr-only">{{ t('workspace.costCenters') }}</caption><thead><tr><th>{{ t('workspace.group') }}</th><th>{{ t('workspace.spend') }}</th><th>{{ t('workspace.requests') }}</th></tr></thead><tbody><tr v-for="item in allocationReport.cost_centers || []" :key="item.key"><td>{{ allocationCenterLabel(item.key) }}</td><td>{{ money(item.cost) }}</td><td>{{ item.request_count }}</td></tr><tr v-if="!(allocationReport.cost_centers || []).length"><td colspan="3">{{ t('workspace.noAllocationData') }}</td></tr></tbody></table></div></div>
            <div class="allocation-group"><h3>{{ t('workspace.environment') }}</h3><div class="allocation-table-wrap"><table class="allocation-table"><caption class="sr-only">{{ t('workspace.environment') }}</caption><thead><tr><th>{{ t('workspace.group') }}</th><th>{{ t('workspace.spend') }}</th><th>{{ t('workspace.requests') }}</th></tr></thead><tbody><tr v-for="item in allocationReport.environments || []" :key="item.key"><td>{{ environmentLabel(item.key) }}</td><td>{{ money(item.cost) }}</td><td>{{ item.request_count }}</td></tr><tr v-if="!(allocationReport.environments || []).length"><td colspan="3">{{ t('workspace.noAllocationData') }}</td></tr></tbody></table></div></div>
            <div class="allocation-group"><h3>{{ t('workspace.tags') }}</h3><div class="allocation-table-wrap"><table class="allocation-table"><caption class="sr-only">{{ t('workspace.tags') }}</caption><thead><tr><th>{{ t('workspace.group') }}</th><th>{{ t('workspace.spend') }}</th><th>{{ t('workspace.requests') }}</th></tr></thead><tbody><tr v-for="item in allocationReport.tags || []" :key="item.key"><td class="allocation-technical">{{ item.key }}</td><td>{{ money(item.cost) }}</td><td>{{ item.request_count }}</td></tr><tr v-if="!(allocationReport.tags || []).length"><td colspan="3">{{ t('workspace.noAllocationData') }}</td></tr></tbody></table></div></div>
          </div>
        </template>
        <div v-else class="workspace-state">{{ t('workspace.allocationUnavailable') }}</div>
      </section>
      <section v-if="store.can('workspace.read') && allocationSupported" class="workspace-panel" aria-labelledby="allocation-config-title">
        <div class="workspace-panel__heading"><div><h2 id="allocation-config-title">{{ t('workspace.allocationConfiguration') }}</h2><p>{{ t('workspace.allocationConfigurationDescription') }}</p></div></div>
        <div class="allocation-config-grid">
          <div class="allocation-config-column">
            <h3>{{ t('workspace.costCenters') }}</h3>
            <form v-if="store.can('workspace.update')" class="allocation-inline-form" @submit.prevent="createCenter">
              <label><span>{{ t('workspace.code') }}</span><input v-model="centerForm.code" class="input" maxlength="63" required></label>
              <label><span>{{ t('workspace.name') }}</span><input v-model="centerForm.name" class="input" maxlength="120" required></label>
              <label><span>{{ t('workspace.descriptionLabel') }}</span><input v-model="centerForm.description" class="input" maxlength="2000"></label>
              <button type="submit" class="btn btn-primary" :disabled="allocationSaving">{{ t('workspace.createCostCenter') }}</button>
            </form>
            <div v-if="costCenters.length" class="allocation-list"><div v-for="center in costCenters" :key="center.id" class="allocation-list-row">
              <form v-if="editingCenterId === center.id" class="allocation-edit-form" @submit.prevent="saveCenter(center)">
                <label><span>{{ t('workspace.code') }}</span><input v-model="centerEditForm.code" class="input" maxlength="63" required></label>
                <label><span>{{ t('workspace.name') }}</span><input v-model="centerEditForm.name" class="input" maxlength="120" required></label>
                <label><span>{{ t('workspace.descriptionLabel') }}</span><input v-model="centerEditForm.description" class="input" maxlength="2000"></label>
                <div class="allocation-edit-actions"><button type="submit" class="btn btn-primary btn-sm" :disabled="allocationSaving">{{ allocationSaving ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="allocationSaving" @click="cancelCenterEdit">{{ t('common.cancel') }}</button></div>
              </form>
              <template v-else><div><strong>{{ center.code }}</strong><span>{{ center.name }}</span></div><div class="allocation-row-actions"><button v-if="store.can('workspace.update')" type="button" class="btn btn-secondary btn-sm" :disabled="allocationSaving" @click="beginCenterEdit(center)">{{ t('common.edit') }}</button><button v-if="store.can('workspace.update')" type="button" class="btn btn-secondary btn-sm" :disabled="allocationSaving" @click="archiveCenter(center)">{{ t('workspace.archive') }}</button></div></template>
            </div></div>
            <p v-else class="workspace-state allocation-empty">{{ t('workspace.noCostCenters') }}</p>
          </div>
          <div class="allocation-config-column">
            <h3>{{ t('workspace.tags') }}</h3>
            <form v-if="store.can('workspace.update')" class="allocation-inline-form" @submit.prevent="createTag">
              <label><span>{{ t('workspace.tagKey') }}</span><input v-model="tagForm.key" class="input" maxlength="63" required></label>
              <label><span>{{ t('workspace.tagValue') }}</span><input v-model="tagForm.value" class="input" maxlength="255" required></label>
              <label><span>{{ t('workspace.descriptionLabel') }}</span><input v-model="tagForm.description" class="input" maxlength="500"></label>
              <button type="submit" class="btn btn-primary" :disabled="allocationSaving">{{ t('workspace.createTag') }}</button>
            </form>
            <div v-if="allocationTags.length" class="allocation-list"><div v-for="tag in allocationTags" :key="tag.id" class="allocation-list-row">
              <form v-if="editingTagId === tag.id" class="allocation-edit-form allocation-tag-edit-form" @submit.prevent="saveTag(tag)">
                <label><span>{{ t('workspace.tagKey') }}</span><input v-model="tagEditForm.key" class="input" maxlength="63" required></label>
                <label><span>{{ t('workspace.tagValue') }}</span><input v-model="tagEditForm.value" class="input" maxlength="255" required></label>
                <label><span>{{ t('workspace.descriptionLabel') }}</span><input v-model="tagEditForm.description" class="input" maxlength="500"></label>
                <div class="allocation-edit-actions"><button type="submit" class="btn btn-primary btn-sm" :disabled="allocationSaving">{{ allocationSaving ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="allocationSaving" @click="cancelTagEdit">{{ t('common.cancel') }}</button></div>
              </form>
              <template v-else><div><strong class="allocation-technical">{{ tag.key }}={{ tag.value }}</strong><span>{{ tag.description }}</span></div><div class="allocation-row-actions"><button v-if="store.can('workspace.update')" type="button" class="btn btn-secondary btn-sm" :disabled="allocationSaving" @click="beginTagEdit(tag)">{{ t('common.edit') }}</button><button v-if="store.can('workspace.update')" type="button" class="btn btn-secondary btn-sm" :disabled="allocationSaving" @click="archiveTag(tag)">{{ t('workspace.archive') }}</button></div></template>
            </div></div>
            <p v-else class="workspace-state allocation-empty">{{ t('workspace.noTags') }}</p>
          </div>
        </div>
        <div v-if="projects.length" class="project-allocation-editor">
          <h3>{{ t('workspace.projectDefaultAllocation') }}</h3>
          <label class="project-allocation-select"><span>{{ t('workspace.project') }}</span><select v-model="selectedProjectId" class="input"><option v-for="project in projects" :key="project.id" :value="project.id">{{ project.name }}</option></select></label>
          <form v-if="projectAllocationForm" class="workspace-form" @submit.prevent="saveProjectAllocation">
            <div class="workspace-form-grid"><label><span>{{ t('workspace.environment') }}</span><select v-model="projectAllocationForm.environment" class="input"><option value="production">{{ t('workspace.environments.production') }}</option><option value="staging">{{ t('workspace.environments.staging') }}</option><option value="development">{{ t('workspace.environments.development') }}</option><option value="testing">{{ t('workspace.environments.testing') }}</option></select></label><label><span>{{ t('workspace.costCenter') }}</span><select v-model="projectAllocationForm.cost_center_id" class="input"><option :value="null">{{ t('workspace.unallocated') }}</option><option v-for="center in costCenters" :key="center.id" :value="center.id">{{ center.code }} · {{ center.name }}</option></select></label></div>
            <fieldset class="allocation-tag-picker"><legend>{{ t('workspace.tags') }}</legend><label v-for="tag in allocationTags" :key="tag.id" class="allocation-tag-option"><input type="checkbox" :checked="projectAllocationForm.tags[tag.key] === tag.value" @change="toggleProjectTag(tag)"><span class="allocation-technical">{{ tag.key }}={{ tag.value }}</span></label><p v-if="!allocationTags.length" class="workspace-state allocation-empty">{{ t('workspace.noTags') }}</p></fieldset>
            <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="allocationSaving || !store.can('project.update')">{{ allocationSaving ? t('common.saving') : t('workspace.saveAllocation') }}</button><span class="allocation-revision">{{ t('workspace.policyRevision', { revision: projectAllocationForm.policy_revision }) }}</span></div>
          </form>
        </div>
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
import { workspaceAPI, type WorkspaceBudget, type WorkspaceOverview, type WorkspaceUsage, type WorkspaceFinopsAnomaly, type WorkspaceFinopsAnomalyDetectorStatus, type WorkspaceFinopsAnomalyFilter, type WorkspaceAllocationReport, type WorkspaceAllocationFilter, type WorkspaceCostCenter, type WorkspaceAllocationTag, type WorkspaceProjectAllocation, type Project } from '@/api/workspace'
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
const allocationReport = ref<WorkspaceAllocationReport | null>(null)
const allocationLoading = ref(false)
const allocationError = ref(false)
const allocationSaving = ref(false)
const costCenters = ref<WorkspaceCostCenter[]>([])
const allocationTags = ref<WorkspaceAllocationTag[]>([])
const projects = ref<Project[]>([])
const selectedProjectId = ref<number | undefined>(undefined)
const projectAllocation = ref<WorkspaceProjectAllocation | null>(null)
const editingCenterId = ref<number | null>(null)
const editingTagId = ref<number | null>(null)
const allocationFilters = reactive<WorkspaceAllocationFilter>({ timezone: 'UTC' })
const centerForm = reactive({ code: '', name: '', description: '' })
const tagForm = reactive({ key: '', value: '', description: '' })
const centerEditForm = reactive({ code: '', name: '', description: '' })
const tagEditForm = reactive({ key: '', value: '', description: '' })
const projectAllocationForm = reactive<{ cost_center_id: number | null; environment: 'production' | 'staging' | 'development' | 'testing'; tags: Record<string, string>; policy_revision: number }>({ cost_center_id: null, environment: 'development', tags: {}, policy_revision: 1 })
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
const allocationSupported = computed(() => typeof workspaceAPI.getAllocationReport === 'function')
function allocationCenterLabel(key: string) { if (key === '0' || key === '') return t('workspace.unallocated'); return costCenters.value.find(center => String(center.id) === key)?.code || key }
function environmentLabel(key: string) { return key === 'unallocated' ? t('workspace.unallocated') : t(`workspace.environments.${key}`) }

function syncProjectAllocation(value: WorkspaceProjectAllocation | null) {
  projectAllocation.value = value
  if (!value) {
    Object.assign(projectAllocationForm, { cost_center_id: null, environment: 'development', tags: {}, policy_revision: 1 })
    return
  }
  const tags = { ...(value.allocation.tags || value.allocation.allocation_tags || {}) }
  Object.assign(projectAllocationForm, { cost_center_id: value.allocation.cost_center_id ?? null, environment: value.allocation.environment as typeof projectAllocationForm.environment, tags, policy_revision: value.allocation.policy_revision || 1 })
}

async function loadProjectAllocation(id: number, currentGeneration = generation) {
  if (!id || typeof workspaceAPI.getProjectAllocation !== 'function') return
  try {
    const result = await workspaceAPI.getProjectAllocation(idWorkspace(), id)
    if (currentGeneration === generation && id === selectedProjectId.value && idWorkspace() === store.selectedWorkspaceId) syncProjectAllocation(result)
  } catch { if (currentGeneration === generation) syncProjectAllocation(null) }
}

function idWorkspace() { return store.selectedWorkspaceId || 0 }

async function loadAllocation(id = store.selectedWorkspaceId, signal = controller?.signal, currentGeneration = generation) {
  if (!id || !allocationSupported.value) return
  allocationLoading.value = true
  allocationError.value = false
  const reportPromise = workspaceAPI.getAllocationReport(id, { ...allocationFilters }, signal)
  const centersPromise = typeof workspaceAPI.listCostCenters === 'function' ? workspaceAPI.listCostCenters(id, false, signal) : Promise.resolve([])
  const tagsPromise = typeof workspaceAPI.listAllocationTags === 'function' ? workspaceAPI.listAllocationTags(id, false, signal) : Promise.resolve([])
  const projectsPromise = typeof workspaceAPI.listProjects === 'function' ? workspaceAPI.listProjects(id, { page: 1, page_size: 100, signal }) : Promise.resolve({ items: [] })
  try {
    const [reportResult, centersResult, tagsResult, projectsResult] = await Promise.allSettled([reportPromise, centersPromise, tagsPromise, projectsPromise])
    if (currentGeneration !== generation || id !== store.selectedWorkspaceId) return
    if (reportResult.status === 'fulfilled') allocationReport.value = reportResult.value
    else allocationError.value = true
    if (centersResult.status === 'fulfilled') costCenters.value = Array.isArray(centersResult.value) ? centersResult.value : ((centersResult.value as { items?: WorkspaceCostCenter[] }).items || [])
    if (tagsResult.status === 'fulfilled') allocationTags.value = Array.isArray(tagsResult.value) ? tagsResult.value : ((tagsResult.value as { items?: WorkspaceAllocationTag[] }).items || [])
    if (projectsResult.status === 'fulfilled') {
      const page = projectsResult.value as { items?: Project[] }
      projects.value = Array.isArray(page) ? page as unknown as Project[] : (page.items || [])
      if (!selectedProjectId.value || !projects.value.some(project => project.id === selectedProjectId.value)) selectedProjectId.value = projects.value[0]?.id
      if (selectedProjectId.value) await loadProjectAllocation(selectedProjectId.value, currentGeneration)
    }
  } catch { if (currentGeneration === generation) allocationError.value = true }
  finally { if (currentGeneration === generation) allocationLoading.value = false }
}

function toggleProjectTag(tag: WorkspaceAllocationTag) {
  const next = { ...projectAllocationForm.tags }
  if (next[tag.key] === tag.value) delete next[tag.key]
  else next[tag.key] = tag.value
  projectAllocationForm.tags = next
}

async function createCenter() {
  const id = store.selectedWorkspaceId
  if (!id || allocationSaving.value || typeof workspaceAPI.createCostCenter !== 'function') return
  allocationSaving.value = true
  try { await workspaceAPI.createCostCenter(id, { ...centerForm }); Object.assign(centerForm, { code: '', name: '', description: '' }); await loadAllocation() } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}
function beginCenterEdit(center: WorkspaceCostCenter) {
  editingCenterId.value = center.id
  Object.assign(centerEditForm, { code: center.code, name: center.name, description: center.description || '' })
}
function cancelCenterEdit() {
  editingCenterId.value = null
  Object.assign(centerEditForm, { code: '', name: '', description: '' })
}
async function saveCenter(center: WorkspaceCostCenter) {
  const id = store.selectedWorkspaceId
  if (!id || allocationSaving.value || typeof workspaceAPI.updateCostCenter !== 'function') return
  allocationSaving.value = true
  try {
    await workspaceAPI.updateCostCenter(id, center.id, { code: centerEditForm.code, name: centerEditForm.name, description: centerEditForm.description })
    cancelCenterEdit()
    await loadAllocation()
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}
async function archiveCenter(center: WorkspaceCostCenter) {
  const id = store.selectedWorkspaceId
  if (!id || allocationSaving.value || typeof workspaceAPI.archiveCostCenter !== 'function') return
  allocationSaving.value = true
  try { await workspaceAPI.archiveCostCenter(id, center.id); await loadAllocation() } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}
async function createTag() {
  const id = store.selectedWorkspaceId
  if (!id || allocationSaving.value || typeof workspaceAPI.createAllocationTag !== 'function') return
  allocationSaving.value = true
  try { await workspaceAPI.createAllocationTag(id, { ...tagForm }); Object.assign(tagForm, { key: '', value: '', description: '' }); await loadAllocation() } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}
function beginTagEdit(tag: WorkspaceAllocationTag) {
  editingTagId.value = tag.id
  Object.assign(tagEditForm, { key: tag.key, value: tag.value, description: tag.description || '' })
}
function cancelTagEdit() {
  editingTagId.value = null
  Object.assign(tagEditForm, { key: '', value: '', description: '' })
}
async function saveTag(tag: WorkspaceAllocationTag) {
  const id = store.selectedWorkspaceId
  if (!id || allocationSaving.value || typeof workspaceAPI.updateAllocationTag !== 'function') return
  allocationSaving.value = true
  try {
    await workspaceAPI.updateAllocationTag(id, tag.id, { key: tagEditForm.key, value: tagEditForm.value, description: tagEditForm.description })
    cancelTagEdit()
    await loadAllocation()
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}
async function archiveTag(tag: WorkspaceAllocationTag) {
  const id = store.selectedWorkspaceId
  if (!id || allocationSaving.value || typeof workspaceAPI.archiveAllocationTag !== 'function') return
  allocationSaving.value = true
  try { await workspaceAPI.archiveAllocationTag(id, tag.id); await loadAllocation() } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}
async function saveProjectAllocation() {
  const workspaceId = store.selectedWorkspaceId
  const projectId = selectedProjectId.value
  if (!workspaceId || !projectId || allocationSaving.value || typeof workspaceAPI.updateProjectAllocation !== 'function') return
  allocationSaving.value = true
  try { const result = await workspaceAPI.updateProjectAllocation(workspaceId, projectId, { cost_center_id: projectAllocationForm.cost_center_id, environment: projectAllocationForm.environment, tags: { ...projectAllocationForm.tags }, policy_revision: projectAllocationForm.policy_revision }); syncProjectAllocation(result); app.showSuccess(t('common.saved')) } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.allocationSaveError')) } finally { allocationSaving.value = false }
}

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
  allocationReport.value = null
  costCenters.value = []
  allocationTags.value = []
  cancelCenterEdit()
  cancelTagEdit()
  projects.value = []
  selectedProjectId.value = undefined
  syncProjectAllocation(null)
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
    await loadAllocation(id, signal, currentGeneration)
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
watch(selectedProjectId, value => { if (value) void loadProjectAllocation(value) })
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
.allocation-filters { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; margin: 18px 0; }
.allocation-filters label, .allocation-inline-form label, .project-allocation-select { display: grid; min-width: 150px; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }
.allocation-filters input, .allocation-filters select { min-width: 150px; }
.allocation-totals { margin-top: 8px; }
.allocation-groups { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; margin-top: 18px; }
.allocation-group { min-width: 0; overflow: hidden; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface-soft); }
.allocation-group h3 { margin: 0; padding: 12px 14px; border-bottom: 1px solid var(--color-border); }
.allocation-table-wrap { max-width: 100%; overflow-x: auto; }
.allocation-table { width: 100%; min-width: 320px; border-collapse: collapse; font-size: 12px; }
.allocation-table th, .allocation-table td { padding: 9px 10px; border-bottom: 1px solid var(--color-border); text-align: left; vertical-align: middle; }
.allocation-table th { color: var(--color-text-muted); font-weight: 600; white-space: nowrap; }
.allocation-table tr:last-child td { border-bottom: 0; }
.allocation-table td:nth-child(2), .allocation-table td:nth-child(3) { white-space: nowrap; font-variant-numeric: tabular-nums; }
.allocation-technical { overflow-wrap: anywhere; font-family: var(--font-mono, ui-monospace, monospace); font-size: 11px; }
.allocation-config-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 18px; margin-top: 18px; }
.allocation-config-column { min-width: 0; }
.allocation-config-column h3, .project-allocation-editor h3 { margin: 0 0 12px; color: var(--color-text-primary); font-size: 16px; }
.allocation-inline-form { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: end; gap: 10px; margin-bottom: 14px; }
.allocation-inline-form label:last-of-type { grid-column: 1 / -1; }
.allocation-list { display: grid; gap: 7px; }
.allocation-list-row { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 12px; padding: 10px 12px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface-soft); }
.allocation-list-row > div { display: grid; min-width: 0; gap: 3px; }
.allocation-list-row strong, .allocation-list-row span { overflow-wrap: anywhere; }
.allocation-list-row span { color: var(--color-text-secondary); font-size: 12px; }
.allocation-row-actions, .allocation-edit-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: 7px; }
.allocation-edit-form { display: grid; width: 100%; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: end; gap: 10px; }
.allocation-edit-form label { display: grid; min-width: 0; gap: 6px; color: var(--color-text-secondary); font-size: 12px; }
.allocation-edit-actions { grid-column: 1 / -1; }
.allocation-empty { padding: 14px 0; text-align: left; }
.project-allocation-editor { margin-top: 24px; padding-top: 20px; border-top: 1px solid var(--color-border); }
.project-allocation-select { max-width: 360px; }
.allocation-tag-picker { display: flex; flex-wrap: wrap; gap: 8px 14px; min-width: 0; margin: 0; padding: 12px; border: 1px solid var(--color-border); border-radius: 8px; }
.allocation-tag-picker legend { padding: 0 4px; color: var(--color-text-secondary); font-size: 13px; }
.allocation-tag-option { display: inline-flex; min-height: 36px; align-items: center; gap: 7px; }
.allocation-tag-option input { width: 16px; height: 16px; accent-color: var(--color-primary); }
.allocation-revision { align-self: center; color: var(--color-text-muted); font-size: 12px; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } .workspace-budget__values, .workspace-form-grid { grid-template-columns: 1fr; } }
@media (max-width: 760px) { .anomaly-summary, .anomaly-evidence { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
@media (max-width: 900px) { .allocation-groups, .allocation-config-grid { grid-template-columns: 1fr; } }
@media (max-width: 760px) { .allocation-edit-form { grid-template-columns: repeat(2, minmax(0, 1fr)); } .allocation-edit-actions { grid-column: 1 / -1; } }
@media (max-width: 480px) { .anomaly-summary, .anomaly-evidence { grid-template-columns: 1fr; } .anomaly-filters label, .anomaly-filters select, .allocation-filters label, .allocation-filters input, .allocation-filters select { width: 100%; } .allocation-inline-form, .allocation-edit-form { grid-template-columns: 1fr; } .allocation-inline-form label:last-of-type { grid-column: auto; } .allocation-edit-actions { grid-column: auto; justify-content: flex-start; } .allocation-row-actions { justify-content: flex-start; } }
</style>
