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
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import WorkspaceUsageBreakdowns from '@/components/workspace/WorkspaceUsageBreakdowns.vue'
import WorkspaceDailySpend from '@/components/workspace/WorkspaceDailySpend.vue'
import { workspaceAPI, type WorkspaceBudget, type WorkspaceOverview, type WorkspaceUsage } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'
const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const budget = ref<WorkspaceBudget | null>(null)
const usage = ref<WorkspaceUsage | null>(null)
const overview = ref<WorkspaceOverview | null>(null)
const available = computed(() => budget.value !== null)
const loading = ref(false)
const saving = ref(false)
let generation = 0
let controller: AbortController | null = null
const budgetForm = reactive({ amount: 0, hard_limit: false, enabled: true, timezone: 'UTC' })
function money(value?: number) { return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '—' }
const budgetPolicy = computed(() => budget.value?.policy || budget.value?.workspace || budget.value?.project)
const usageSummary = computed(() => overview.value?.summary || usage.value?.summary || usage.value || {})

async function load() {
  const id = store.selectedWorkspaceId
  const currentGeneration = ++generation
  controller?.abort()
  budget.value = null
  usage.value = null
  overview.value = null
  if (!id) { loading.value = false; return }
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
  } finally { if (currentGeneration === generation) loading.value = false }
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
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } .workspace-budget__values, .workspace-form-grid { grid-template-columns: 1fr; } }
</style>
