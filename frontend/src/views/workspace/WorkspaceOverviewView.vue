<template>
  <WorkspaceFrame section="overview">
    <div class="workspace-page">
      <div class="workspace-kpis" aria-live="polite">
        <article v-for="item in kpis" :key="item.label" class="workspace-kpi">
          <span>{{ item.label }}</span>
          <strong>{{ item.value }}</strong>
        </article>
      </div>

      <section class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><h2>{{ t('workspace.projects') }}</h2><p>{{ t('workspace.projectsDescription') }}</p></div>
          <div class="workspace-actions">
            <button type="button" class="btn btn-secondary btn-sm" @click="showCreateWorkspace = true">{{ t('workspace.createWorkspace') }}</button>
            <RouterLink v-if="store.can('project.create')" class="btn btn-primary btn-sm" :to="projectsPath">{{ t('workspace.createProject') }}</RouterLink>
          </div>
        </div>
        <div v-if="store.projects.length" class="workspace-project-grid">
          <RouterLink v-for="project in store.projects" :key="project.id" class="workspace-project" :to="`${projectsPath}/${project.id}`">
            <span class="workspace-project__title">{{ project.name }}</span>
            <code>{{ project.slug }}</code>
            <span class="workspace-project__meta"><span>{{ project.status }}</span><span v-if="project.is_default">{{ t('workspace.default') }}</span></span>
          </RouterLink>
        </div>
        <div v-else class="workspace-empty">{{ t('workspace.noProjects') }}</div>
      </section>

      <section v-if="showCreateWorkspace" class="workspace-panel workspace-form-panel">
        <h2>{{ t('workspace.createWorkspace') }}</h2>
        <form class="workspace-form" @submit.prevent="createWorkspace">
          <label><span>{{ t('common.name') }}</span><input v-model="workspaceForm.name" class="input" required maxlength="100"></label>
          <label><span>{{ t('workspace.slug') }}</span><input v-model="workspaceForm.slug" class="input" required pattern="[a-z0-9][a-z0-9-]{0,62}"></label>
          <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="workspaceSaving">{{ workspaceSaving ? t('common.saving') : t('common.create') }}</button><button type="button" class="btn btn-secondary" @click="showCreateWorkspace = false">{{ t('common.cancel') }}</button></div>
        </form>
      </section>

      <section v-if="store.can('workspace.update') && members.length" class="workspace-panel workspace-form-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.settings') }}</h2><p>{{ t('workspace.settingsDescription') }}</p></div></div>
        <form class="workspace-form" @submit.prevent="saveWorkspaceDetails">
          <label><span>{{ t('common.name') }}</span><input v-model="settingsForm.name" class="input" required maxlength="100"></label>
          <label><span>{{ t('workspace.slug') }}</span><input v-model="settingsForm.slug" class="input" required pattern="[a-z0-9][a-z0-9-]{0,62}"></label>
          <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="settingsSaving">{{ settingsSaving ? t('common.saving') : t('common.save') }}</button></div>
        </form>
        <form v-if="store.can('billing.owner.update')" class="workspace-form" @submit.prevent="saveBillingOwner">
          <label><span>{{ t('workspace.billingOwner') }}</span><select v-model.number="settingsForm.billing_owner_user_id" class="input"><option v-for="member in members" :key="member.user_id" :value="member.user_id">{{ member.username || member.email || `#${member.user_id}` }}</option></select></label>
          <div class="workspace-actions"><button type="submit" class="btn btn-secondary" :disabled="settingsSaving">{{ settingsSaving ? t('common.saving') : t('common.save') }}</button></div>
        </form>
      </section>

      <section v-if="budgetAvailable" class="workspace-panel workspace-budget">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.budget') }}</h2><p>{{ t('workspace.budgetDescription') }}</p></div><RouterLink class="btn btn-secondary btn-sm" :to="finopsPath">{{ t('workspace.viewFinops') }}</RouterLink></div>
        <div class="workspace-budget__values"><div><span>{{ t('workspace.spent') }}</span><strong>{{ formatMoney(budget?.spent) }}</strong></div><div><span>{{ t('workspace.reserved') }}</span><strong>{{ formatMoney(budget?.reserved) }}</strong></div><div><span>{{ t('workspace.remaining') }}</span><strong>{{ formatMoney(budget?.remaining) }}</strong></div></div>
        <p v-if="budget?.over_budget" class="workspace-warning" role="status">{{ t('workspace.overBudget') }}</p>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { workspaceAPI, type WorkspaceBudget, type WorkspaceMember, type WorkspaceOverview } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const router = useRouter()
const store = useWorkspaceStore()
const app = useAppStore()
const overview = ref<WorkspaceOverview | null>(null)
const budget = ref<WorkspaceBudget | null>(null)
const budgetAvailable = ref(false)
const members = ref<WorkspaceMember[]>([])
const showCreateWorkspace = ref(false)
const workspaceSaving = ref(false)
const settingsSaving = ref(false)
let generation = 0
let controller: AbortController | null = null
const workspaceForm = reactive({ name: '', slug: '' })
const settingsForm = reactive({ name: '', slug: '', billing_owner_user_id: 0 })
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const projectsPath = computed(() => `/workspaces/${workspaceId.value}/projects`)
const finopsPath = computed(() => `/workspaces/${workspaceId.value}/finops`)
const overviewSummary = computed(() => overview.value?.summary || overview.value || {})
const kpis = computed(() => [
  { label: t('workspace.members'), value: String(overviewSummary.value.members ?? '—') },
  { label: t('workspace.requests'), value: String(overviewSummary.value.requests ?? '—') },
  { label: t('workspace.spend'), value: typeof overviewSummary.value.spend === 'number' ? formatMoney(overviewSummary.value.spend) : '—' },
])

function formatMoney(value?: number) { return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '—' }

async function load() {
  const id = store.selectedWorkspaceId
  const currentGeneration = ++generation
  controller?.abort()
  overview.value = null
  budget.value = null
  budgetAvailable.value = false
  members.value = []
  showCreateWorkspace.value = false
  Object.assign(settingsForm, { name: store.selectedWorkspace?.name || '', slug: store.selectedWorkspace?.slug || '', billing_owner_user_id: store.selectedWorkspace?.billing_owner_user_id || 0 })
  if (!id) return
  controller = new AbortController()
  const signal = controller.signal
  const [overviewResult, budgetResult, membersResult] = await Promise.allSettled([
    store.can('usage.read') ? workspaceAPI.getOverview(id, signal) : Promise.resolve(null),
    store.can('budget.read') ? workspaceAPI.getBudget(id, signal) : Promise.resolve(null),
    store.can('member.read') ? workspaceAPI.listMembers(id, { signal }) : Promise.resolve(null),
  ])
  if (currentGeneration !== generation) return
  overview.value = overviewResult.status === 'fulfilled' ? overviewResult.value : null
  budget.value = budgetResult.status === 'fulfilled' ? budgetResult.value : null
  budgetAvailable.value = budget.value !== null
  members.value = membersResult.status === 'fulfilled' ? membersResult.value?.items || [] : []
}

onMounted(load)
watch([() => store.selectedWorkspaceId, () => store.permissions], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })

async function createWorkspace() {
  workspaceSaving.value = true
  try {
    const created = await workspaceAPI.createWorkspace(workspaceForm)
    await store.loadWorkspaces()
    await store.selectWorkspace(created.id)
    await router.push(`/workspaces/${created.id}/overview`)
    showCreateWorkspace.value = false
    Object.assign(workspaceForm, { name: '', slug: '' })
  } catch (error) {
    app.showError((error as { message?: string })?.message || t('workspace.saveError'))
  } finally { workspaceSaving.value = false }
}

async function saveWorkspaceDetails() {
  if (!store.selectedWorkspaceId) return
  settingsSaving.value = true
  try {
    await workspaceAPI.updateWorkspace(store.selectedWorkspaceId, { name: settingsForm.name, slug: settingsForm.slug })
    await store.refreshWorkspace()
    app.showSuccess(t('common.saved'))
  } catch (error) {
    app.showError((error as { message?: string })?.message || t('workspace.saveError'))
  } finally { settingsSaving.value = false }
}

async function saveBillingOwner() {
  if (!store.selectedWorkspaceId || !settingsForm.billing_owner_user_id) return
  settingsSaving.value = true
  try {
    await workspaceAPI.updateWorkspace(store.selectedWorkspaceId, { billing_owner_user_id: settingsForm.billing_owner_user_id })
    await store.refreshWorkspace()
    app.showSuccess(t('common.saved'))
  } catch (error) {
    app.showError((error as { message?: string })?.message || t('workspace.saveError'))
  } finally { settingsSaving.value = false }
}
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-kpis { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; }
.workspace-kpi, .workspace-panel { border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); box-shadow: var(--shadow-xs); }
.workspace-kpi { display: grid; gap: 8px; padding: 16px; }
.workspace-kpi span, .workspace-budget__values span { color: var(--color-text-muted); font-size: 12px; }
.workspace-kpi strong { color: var(--color-text-primary); font-size: 24px; }
.workspace-panel { min-width: 0; padding: 18px; }
.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-panel__heading > * { min-width: 0; }
.workspace-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.workspace-panel p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.workspace-project-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(210px, 1fr)); gap: 10px; margin-top: 16px; }
.workspace-project { display: grid; gap: 8px; min-width: 0; border: 1px solid var(--color-border-subtle); border-radius: 10px; padding: 14px; color: inherit; text-decoration: none; }
.workspace-project:hover, .workspace-project:focus-visible { border-color: var(--color-primary-border); background: var(--color-primary-soft); }
.workspace-project__title { min-width: 0; color: var(--color-text-primary); font-weight: 700; overflow-wrap: anywhere; }
.workspace-project code { color: var(--color-text-muted); font-size: 12px; overflow-wrap: anywhere; }
.workspace-project__meta { display: flex; flex-wrap: wrap; gap: 6px; color: var(--color-text-muted); font-size: 12px; }
.workspace-project__meta span { border-radius: 999px; background: var(--color-surface-soft); padding: 3px 7px; white-space: nowrap; }
.workspace-empty { margin-top: 16px; border: 1px dashed var(--color-border); border-radius: 8px; padding: 24px; color: var(--color-text-muted); text-align: center; }
.workspace-budget__values { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 16px; }
.workspace-budget__values div { display: grid; gap: 5px; }
.workspace-budget__values strong { color: var(--color-text-primary); font-size: 18px; }
.workspace-warning { color: var(--color-warning) !important; }
@media (max-width: 640px) { .workspace-kpis, .workspace-budget__values { grid-template-columns: 1fr; } .workspace-panel__heading { align-items: stretch; flex-direction: column; } }
</style>
