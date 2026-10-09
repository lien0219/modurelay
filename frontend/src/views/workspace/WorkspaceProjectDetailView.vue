<template>
  <WorkspaceFrame section="projects">
    <div class="workspace-page">
      <section class="workspace-panel">
        <div class="workspace-panel__heading">
          <div>
            <p class="workspace-eyebrow">{{ t('workspace.project') }}</p>
            <h2>{{ project?.name || t('workspace.project') }}</h2>
            <p>{{ project?.description || '-' }}</p>
          </div>
          <div class="workspace-actions">
            <RouterLink v-if="store.can('policy.read')" class="btn btn-secondary btn-sm" :to="`/workspaces/${workspaceId}/projects/${projectId}/policy`">{{ t('workspace.policySettings') }}</RouterLink>
            <RouterLink v-if="store.can('project_access.read')" class="btn btn-secondary btn-sm" :to="`/workspaces/${workspaceId}/projects/${projectId}/access`">{{ t('workspace.projectAccess') }}</RouterLink>
            <button v-if="store.can('key.create')" type="button" class="btn btn-primary btn-sm" @click="openCreateForm">{{ t('workspace.createKey') }}</button>
          </div>
        </div>
        <dl class="workspace-details">
          <div><dt>{{ t('workspace.slug') }}</dt><dd><code>{{ project?.slug || '-' }}</code></dd></div>
          <div><dt>{{ t('common.status') }}</dt><dd>{{ project?.status || '-' }}</dd></div>
          <div><dt>{{ t('workspace.workspace') }}</dt><dd>{{ store.selectedWorkspace?.name || '-' }}</dd></div>
        </dl>
      </section>

      <section v-if="canArchiveProject || canRestoreProject || projectLifecycleError" class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.lifecycle.projectLifecycle') }}</h2><p>{{ t('workspace.lifecycle.projectArchiveHint') }}</p></div></div>
        <p v-if="projectLifecycleError" class="workspace-lifecycle-error" role="alert">{{ projectLifecycleError }}</p>
        <RouterLink v-if="projectLifecycleReauth" class="btn btn-secondary btn-sm" :to="{ path: '/login', query: { reauth: '1', redirect: route.fullPath } }">{{ t('workspace.lifecycle.signIn') }}</RouterLink>
        <p v-if="projectLifecycleNotice" role="status">{{ projectLifecycleNotice }}</p>
        <div class="workspace-actions">
          <button v-if="canArchiveProject" type="button" data-testid="archive-project" class="btn btn-secondary" :disabled="projectLifecycleBusy" @click="projectLifecycleAction = 'archive'">{{ t('workspace.archive') }}</button>
          <button v-if="canRestoreProject" type="button" data-testid="restore-project" class="btn btn-primary" :disabled="projectLifecycleBusy" @click="projectLifecycleAction = 'restore'">{{ t('workspace.lifecycle.restoreProject') }}</button>
        </div>
      </section>

      <section v-if="store.can('key.read')" class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><h2>{{ t('workspace.keys') }}</h2><p>{{ t('workspace.keyDescription') }}</p></div>
        </div>
        <div v-if="secret" class="workspace-token" role="status">
          <strong>{{ t('workspace.keySecret') }}</strong>
          <code>{{ secret }}</code>
          <button type="button" class="btn btn-secondary btn-sm" @click="copy(secret)">{{ t('workspace.copySecret') }}</button>
        </div>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="keys.length" class="workspace-table-wrap">
          <table class="workspace-table">
            <thead>
              <tr>
                <th>{{ t('workspace.keyName') }}</th>
                <th>{{ t('workspace.keySecret') }}</th>
                <th>{{ t('workspace.quotaUsage') }}</th>
                <th>{{ t('workspace.rateLimits') }}</th>
                <th>{{ t('common.status') }}</th>
                <th>{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <template v-for="key in keys" :key="key.id">
                <tr>
                  <td><strong>{{ key.name }}</strong><small>#{{ key.id }}</small></td>
                  <td><code>{{ key.key }}</code></td>
                  <td>{{ formatQuota(key) }}</td>
                  <td>{{ formatRateLimits(key) }}</td>
                  <td>{{ key.status }}</td>
                  <td>
                    <div class="workspace-actions">
                      <button v-if="store.can('key.update')" type="button" class="btn btn-secondary btn-sm" @click="openEditForm(key)">
                        {{ t('common.edit') }}
                      </button>
                      <button v-if="store.can('key.revoke') && key.status === 'active'" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="revokeKey(key.id)">
                        {{ t('workspace.revokeKey') }}
                      </button>
                    </div>
                  </td>
                </tr>
                <tr v-if="editingKey?.id === key.id" class="workspace-edit-row">
                  <td colspan="6">
                    <form class="workspace-form" @submit.prevent="updateKey">
                      <div class="workspace-form__heading">
                        <h3>{{ t('workspace.editKey') }}</h3>
                        <button type="button" class="btn btn-ghost btn-sm" @click="closeEditForm">{{ t('common.cancel') }}</button>
                      </div>
                      <div class="workspace-form-grid">
                        <label><span>{{ t('workspace.keyName') }}</span><input v-model="keyEditForm.name" class="input" required maxlength="100"></label>
                        <label><span>{{ t('workspace.group') }}</span><select v-model.number="keyEditForm.group_id" class="input"><option v-if="editingKey?.group_id == null" :value="null">{{ t('workspace.noGroup') }}</option><option v-for="group in availableGroups" :key="group.id" :value="group.id">{{ group.name }} (#{{ group.id }})</option></select></label>
                        <label><span>{{ t('workspace.amount') }}</span><input v-model.number="keyEditForm.quota" class="input" type="number" min="0" step="0.01"></label>
                        <label><span>{{ t('workspace.expires') }}</span><input v-model="keyEditForm.expires_at" class="input" type="date"></label>
                        <label><span>{{ t('workspace.rateLimit5h') }}</span><input v-model.number="keyEditForm.rate_limit_5h" class="input" type="number" min="0" step="0.01"></label>
                        <label><span>{{ t('workspace.rateLimit1d') }}</span><input v-model.number="keyEditForm.rate_limit_1d" class="input" type="number" min="0" step="0.01"></label>
                        <label><span>{{ t('workspace.rateLimit7d') }}</span><input v-model.number="keyEditForm.rate_limit_7d" class="input" type="number" min="0" step="0.01"></label>
                        <label class="workspace-form__wide"><span>{{ t('workspace.ipWhitelist') }}</span><textarea v-model="keyEditForm.ip_whitelist" class="input" rows="2" :placeholder="t('workspace.ipListHint')"></textarea></label>
                        <label class="workspace-form__wide"><span>{{ t('workspace.ipBlacklist') }}</span><textarea v-model="keyEditForm.ip_blacklist" class="input" rows="2" :placeholder="t('workspace.ipListHint')"></textarea></label>
                      </div>
                      <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
                    </form>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
        <div v-else class="workspace-state">{{ t('workspace.noKeys') }}</div>
      </section>

      <section v-if="store.can('budget.read')" class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.budget') }}</h2><p>{{ t('workspace.projectBudgetDescription') }}</p></div></div>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-if="projectBudget" class="workspace-budget__values">
          <div><span>{{ t('workspace.spent') }}</span><strong>{{ formatMoney(projectBudget.spent) }}</strong></div>
          <div><span>{{ t('workspace.reserved') }}</span><strong>{{ formatMoney(projectBudget.reserved) }}</strong></div>
          <div><span>{{ t('workspace.remaining') }}</span><strong>{{ formatMoney(projectBudget.remaining) }}</strong></div>
        </div>
        <p v-if="projectBudget?.over_budget" class="workspace-warning" role="status">{{ t('workspace.projectOverBudget') }}</p>
        <form v-if="projectBudget && store.can('budget.update')" class="workspace-form" @submit.prevent="saveProjectBudget">
          <h3>{{ t('workspace.policy') }}</h3>
          <div class="workspace-form-grid">
            <label><span>{{ t('workspace.amount') }}</span><input v-model.number="projectBudgetForm.amount" class="input" type="number" min="0" step="0.01" required></label>
            <label><span>{{ t('workspace.timezone') }}</span><input v-model="projectBudgetForm.timezone" class="input" maxlength="80" required></label>
            <label class="workspace-checkbox"><input v-model="projectBudgetForm.hard_limit" type="checkbox"><span>{{ t('workspace.hardLimit') }}</span></label>
            <label class="workspace-checkbox"><input v-model="projectBudgetForm.enabled" type="checkbox"><span>{{ t('workspace.enabled') }}</span></label>
          </div>
          <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="budgetSaving">{{ budgetSaving ? t('common.saving') : t('workspace.updateBudget') }}</button></div>
        </form>
        <div v-if="!loading && !projectBudget" class="workspace-state">{{ t('workspace.projectBudgetUnavailable') }}</div>
      </section>

      <section v-if="store.can('usage.read')" class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.usage') }}</h2><p>{{ t('workspace.usageDescription') }}</p></div></div>
        <label v-if="serviceAccountChoices.length" class="workspace-machine-filter"><span>{{ t('serviceAccounts.filter') }}</span><select v-model="selectedServiceAccountId" class="input" data-testid="service-account-usage-filter" :disabled="loading" @change="load"><option value="">{{ t('serviceAccounts.allPrincipals') }}</option><option v-for="row in serviceAccountChoices" :key="row.id" :value="row.id">{{ row.name || row.id }} (#{{ row.id }})</option></select></label>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <template v-else-if="projectUsage || projectOverview">
          <div class="workspace-project-usage"><span>{{ t('workspace.requests') }}: {{ projectUsageSummary.requests ?? '—' }}</span><span>{{ t('workspace.spend') }}: {{ formatMoney(projectUsageSummary.spend) }}</span></div>
          <p v-if="projectUsageSummary.start && projectUsageSummary.end" class="workspace-period">{{ projectUsageSummary.start }} – {{ projectUsageSummary.end }}</p>
          <WorkspaceDailySpend v-if="projectOverview?.daily_spend" :rows="projectOverview.daily_spend" :timezone="projectUsageSummary.timezone" />
          <WorkspaceUsageBreakdowns v-if="projectOverview" :overview="projectOverview" />
        </template>
        <div v-else class="workspace-state">{{ t('workspace.projectUsageUnavailable') }}</div>
      </section>

      <section v-if="showKeyForm" class="workspace-panel workspace-form-panel">
        <h2>{{ t('workspace.createKey') }}</h2>
        <form class="workspace-form" @submit.prevent="createKey">
          <div class="workspace-form-grid">
            <label><span>{{ t('workspace.keyName') }}</span><input v-model="keyForm.name" class="input" required maxlength="100"></label>
            <label><span>{{ t('workspace.group') }}</span><select v-model.number="keyForm.group_id" class="input"><option :value="null">{{ t('workspace.noGroup') }}</option><option v-for="group in availableGroups" :key="group.id" :value="group.id">{{ group.name }} (#{{ group.id }})</option></select></label>
            <label><span>{{ t('workspace.amount') }}</span><input v-model.number="keyForm.quota" class="input" type="number" min="0" step="0.01"></label>
            <label><span>{{ t('workspace.expires') }}</span><input v-model.number="keyForm.expires_in_days" class="input" type="number" min="1"></label>
            <label><span>{{ t('workspace.rateLimit5h') }}</span><input v-model.number="keyForm.rate_limit_5h" class="input" type="number" min="0" step="0.01"></label>
            <label><span>{{ t('workspace.rateLimit1d') }}</span><input v-model.number="keyForm.rate_limit_1d" class="input" type="number" min="0" step="0.01"></label>
            <label><span>{{ t('workspace.rateLimit7d') }}</span><input v-model.number="keyForm.rate_limit_7d" class="input" type="number" min="0" step="0.01"></label>
            <label class="workspace-form__wide"><span>{{ t('workspace.ipWhitelist') }}</span><textarea v-model="keyForm.ip_whitelist" class="input" rows="2" :placeholder="t('workspace.ipListHint')"></textarea></label>
            <label class="workspace-form__wide"><span>{{ t('workspace.ipBlacklist') }}</span><textarea v-model="keyForm.ip_blacklist" class="input" rows="2" :placeholder="t('workspace.ipListHint')"></textarea></label>
          </div>
          <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.create') }}</button><button type="button" class="btn btn-secondary" @click="closeCreateForm">{{ t('common.cancel') }}</button></div>
        </form>
      </section>
      <ConfirmDialog :show="projectLifecycleAction !== null" :title="projectLifecycleAction === 'archive' ? t('workspace.archive') : t('workspace.lifecycle.restoreProject')" :message="projectLifecycleAction === 'archive' ? t('workspace.archiveConfirm') : t('workspace.lifecycle.restoreHint')" :confirming="projectLifecycleBusy" @cancel="projectLifecycleAction = null" @confirm="confirmProjectLifecycle">
        <p v-if="projectLifecycleError" class="workspace-lifecycle-error" role="alert">{{ projectLifecycleError }}</p>
        <RouterLink v-if="projectLifecycleReauth" class="btn btn-secondary btn-sm" :to="{ path: '/login', query: { reauth: '1', redirect: route.fullPath } }">{{ t('workspace.lifecycle.signIn') }}</RouterLink>
      </ConfirmDialog>
      <TotpStepUpDialog :controller="projectStepUp" />
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import WorkspaceUsageBreakdowns from '@/components/workspace/WorkspaceUsageBreakdowns.vue'
import WorkspaceDailySpend from '@/components/workspace/WorkspaceDailySpend.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpCancelled } from '@/composables/useStepUp'
import { workspaceLifecycleAPI } from '@/api/workspaceLifecycle'
import { workspaceAPI, type Project, type ProjectKey, type WorkspaceAvailableGroup, type WorkspaceBudget, type WorkspaceOverview, type WorkspaceUsage, type ServiceAccountUsageBreakdown } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const route = useRoute()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => Number(route.params.workspaceId))
const projectId = computed(() => Number(route.params.projectId))
const project = ref<Project | null>(null)
const keys = ref<ProjectKey[]>([])
const projectBudget = ref<WorkspaceBudget | null>(null)
const projectUsage = ref<WorkspaceUsage | null>(null)
const projectOverview = ref<WorkspaceOverview | null>(null)
const selectedServiceAccountId = ref('')
const serviceAccountChoices = ref<ServiceAccountUsageBreakdown[]>([])
const loading = ref(false)
const budgetSaving = ref(false)
const availableGroups = ref<WorkspaceAvailableGroup[]>([])
const showKeyForm = ref(false)
const editingKey = ref<ProjectKey | null>(null)
const saving = ref(false)
const secret = ref('')
let loadGeneration = 0
let loadController: AbortController | null = null
const projectStepUp = useStepUp()
const projectLifecycleAction = ref<'archive' | 'restore' | null>(null)
const projectLifecycleBusy = ref(false)
const projectLifecycleError = ref('')
const projectLifecycleNotice = ref('')
const projectLifecycleReauth = ref(false)
let projectLifecycleGeneration = 0
let projectLifecycleController = new AbortController()
const activeParent = computed(() => store.selectedWorkspaceId === workspaceId.value && store.selectedWorkspace?.status === 'active')
const canArchiveProject = computed(() => activeParent.value && project.value?.status === 'active' && store.can('project.archive'))
const canRestoreProject = computed(() => activeParent.value && project.value?.status === 'archived' && store.can('project.restore'))

async function confirmProjectLifecycle() {
  const action = projectLifecycleAction.value
  if (projectLifecycleBusy.value || !action || (action === 'archive' ? !canArchiveProject.value : !canRestoreProject.value)) return
  const wid = workspaceId.value
  const pid = projectId.value
  const generation = projectLifecycleGeneration
  const signal = projectLifecycleController.signal
  const current = () => !signal.aborted && generation === projectLifecycleGeneration && wid === workspaceId.value && pid === projectId.value && wid === store.selectedWorkspaceId
  projectLifecycleBusy.value = true
  projectLifecycleError.value = ''; projectLifecycleReauth.value = false; projectLifecycleNotice.value = ''
  try {
    await projectStepUp.run(async () => {
      if (!current() || (action === 'archive' ? !canArchiveProject.value : !canRestoreProject.value)) throw { code: 'ERR_CANCELED' }
      if (action === 'archive') await workspaceAPI.archiveProject(wid, pid)
      else await workspaceLifecycleAPI.restoreProject(wid, pid, signal)
    })
    if (!current()) return
    projectLifecycleAction.value = null
    projectLifecycleNotice.value = t('workspace.lifecycle.operationSuccess')
    await store.loadProjects(wid, pid)
    if (current()) await load()
  } catch (error) {
    if (!current()) return
    const item = error as { code?: string; reason?: string; message?: string }
    if (item?.code === 'ERR_CANCELED') return
    projectLifecycleReauth.value = [item?.code, item?.reason].some(marker => Boolean(marker && ['RECENT_AUTH_REQUIRED', 'WORKSPACE_REAUTH_REQUIRED', 'MFA_REQUIRED', 'MFA_ENROLLMENT_REQUIRED', 'STEP_UP_TOTP_NOT_ENABLED'].includes(marker)))
    projectLifecycleError.value = isStepUpCancelled(error) ? t('workspace.lifecycle.verificationCancelled') : item?.message || t('workspace.lifecycle.requestError')
  } finally { if (current()) projectLifecycleBusy.value = false }
}

type KeyForm = {
  name: string
  group_id: number | null
  quota: number
  expires_in_days?: number
  expires_at: string
  rate_limit_5h: number
  rate_limit_1d: number
  rate_limit_7d: number
  ip_whitelist: string
  ip_blacklist: string
}

const emptyForm = (): KeyForm => ({ name: '', group_id: null, quota: 0, expires_in_days: undefined, expires_at: '', rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, ip_whitelist: '', ip_blacklist: '' })
const keyForm = reactive<KeyForm>(emptyForm())
const keyEditForm = reactive<KeyForm>(emptyForm())
const projectBudgetForm = reactive({ amount: 0, hard_limit: false, enabled: true, timezone: 'UTC' })
const projectUsageSummary = computed(() => projectOverview.value?.summary || projectUsage.value?.summary || projectUsage.value || {})

function parseIPList(value: string): string[] {
  return Array.from(new Set(value.split(/[\n,]/).map(item => item.trim()).filter(Boolean)))
}

function dateForInput(value?: string | null): string { return value ? value.slice(0, 10) : '' }
function dateForAPI(value: string): string | undefined { return value ? `${value}T23:59:59Z` : undefined }
function formatMoney(value?: number): string { return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '—' }
function formatQuota(key: ProjectKey): string { return key.quota ? `${formatMoney(key.quota_used ?? key.usage ?? 0)} / ${formatMoney(key.quota)}` : t('workspace.unlimited') }
function formatRateLimits(key: ProjectKey): string {
  const values = [key.rate_limit_5h, key.rate_limit_1d, key.rate_limit_7d].filter((value): value is number => typeof value === 'number' && value > 0)
  return values.length ? values.map(value => formatMoney(value)).join(' / ') : t('workspace.unlimited')
}

async function load() {
  const wid = workspaceId.value
  const pid = projectId.value
  const currentGeneration = ++loadGeneration
  loadController?.abort()
  if (!wid || !pid || store.selectedWorkspaceId !== wid) { loading.value = false; return }
  loadController = new AbortController()
  const signal = loadController.signal
  loading.value = true
  try {
    const detail = await workspaceAPI.getProject(wid, pid, signal)
    if (currentGeneration !== loadGeneration) return
    project.value = detail
    const [keyResult, budgetResult, groupsResult] = await Promise.allSettled([
      store.can('key.read') ? workspaceAPI.listKeys(wid, pid, { signal }) : Promise.resolve(null),
      store.can('budget.read') ? workspaceAPI.getProjectBudget(wid, pid, signal) : Promise.resolve(null),
      store.can('key.create') || store.can('key.update') ? workspaceAPI.listAvailableGroups(wid, pid) : Promise.resolve([]),
    ])
    if (currentGeneration !== loadGeneration) return
    keys.value = keyResult.status === 'fulfilled' ? keyResult.value?.items || [] : []
    projectBudget.value = budgetResult.status === 'fulfilled' ? budgetResult.value : null
    availableGroups.value = groupsResult.status === 'fulfilled' ? groupsResult.value : []
    const policy = projectBudget.value?.policy || projectBudget.value?.project || projectBudget.value?.workspace
    Object.assign(projectBudgetForm, { amount: policy?.amount ?? 0, hard_limit: policy?.hard_limit ?? false, enabled: policy?.enabled ?? true, timezone: policy?.timezone || 'UTC' })
    const params: Record<string, unknown> = projectBudget.value?.period_start && projectBudget.value.period_end
      ? { start: projectBudget.value.period_start, end: projectBudget.value.period_end, timezone: policy?.timezone || 'UTC' }
      : {}
    if (selectedServiceAccountId.value) params.service_account_id = Number(selectedServiceAccountId.value)
    const [usageResult, overviewResult] = await Promise.allSettled([
      store.can('usage.read') ? workspaceAPI.getProjectUsage(wid, pid, params, signal) : Promise.resolve(null),
      store.can('usage.read') ? workspaceAPI.getProjectOverview(wid, pid, signal, params) : Promise.resolve(null),
    ])
    if (currentGeneration !== loadGeneration) return
    projectUsage.value = usageResult.status === 'fulfilled' ? usageResult.value : null
    projectOverview.value = overviewResult.status === 'fulfilled' ? overviewResult.value : null
    if (!selectedServiceAccountId.value) serviceAccountChoices.value = (projectOverview.value?.service_accounts || []).filter(row => /^[1-9]\d*$/.test(row.id))
  } catch (error) {
    if (currentGeneration === loadGeneration) app.showError((error as { message?: string })?.message || t('workspace.loadError'))
  } finally {
    if (currentGeneration === loadGeneration) loading.value = false
  }
}

async function saveProjectBudget() {
  budgetSaving.value = true
  try {
    const wid = workspaceId.value
    const pid = projectId.value
    const result = await workspaceAPI.updateProjectBudget(wid, pid, { ...projectBudgetForm })
    if (wid !== workspaceId.value || pid !== projectId.value) return
    projectBudget.value = result
    app.showSuccess(t('common.saved'))
  } catch (error) {
    app.showError((error as { message?: string })?.message || t('workspace.saveError'))
  } finally { budgetSaving.value = false }
}

function openCreateForm() { Object.assign(keyForm, emptyForm()); editingKey.value = null; showKeyForm.value = true }
function closeCreateForm() { showKeyForm.value = false }
function openEditForm(key: ProjectKey) {
  showKeyForm.value = false
  editingKey.value = key
  Object.assign(keyEditForm, { name: key.name, group_id: key.group_id ?? null, quota: key.quota || 0, expires_at: dateForInput(key.expires_at), rate_limit_5h: key.rate_limit_5h || 0, rate_limit_1d: key.rate_limit_1d || 0, rate_limit_7d: key.rate_limit_7d || 0, ip_whitelist: (key.ip_whitelist || []).join('\n'), ip_blacklist: (key.ip_blacklist || []).join('\n') })
}
function closeEditForm() { editingKey.value = null }

async function createKey() {
  saving.value = true
  try {
    const wid = workspaceId.value
    const pid = projectId.value
    const result = await workspaceAPI.createKey(wid, pid, { name: keyForm.name, group_id: keyForm.group_id, quota: keyForm.quota, expires_in_days: keyForm.expires_in_days, rate_limit_5h: keyForm.rate_limit_5h, rate_limit_1d: keyForm.rate_limit_1d, rate_limit_7d: keyForm.rate_limit_7d, ip_whitelist: parseIPList(keyForm.ip_whitelist), ip_blacklist: parseIPList(keyForm.ip_blacklist) })
    if (wid !== workspaceId.value || pid !== projectId.value) return
    secret.value = result.key
    closeCreateForm()
    await load()
    app.showSuccess(t('common.saved'))
  } catch (error) {
    app.showError((error as { message?: string })?.message || t('workspace.keyError'))
  } finally { saving.value = false }
}

async function updateKey() {
  if (!editingKey.value) return
  saving.value = true
  try {
    await workspaceAPI.updateKey(workspaceId.value, projectId.value, editingKey.value.id, { name: keyEditForm.name, group_id: keyEditForm.group_id, quota: keyEditForm.quota, expires_at: dateForAPI(keyEditForm.expires_at) || '', rate_limit_5h: keyEditForm.rate_limit_5h, rate_limit_1d: keyEditForm.rate_limit_1d, rate_limit_7d: keyEditForm.rate_limit_7d, ip_whitelist: parseIPList(keyEditForm.ip_whitelist), ip_blacklist: parseIPList(keyEditForm.ip_blacklist) })
    closeEditForm()
    await load()
    app.showSuccess(t('common.saved'))
  } catch (error) {
    app.showError((error as { message?: string })?.message || t('workspace.keyError'))
  } finally { saving.value = false }
}

async function revokeKey(keyId: number) {
  if (!window.confirm(t('workspace.revokeKeyConfirm'))) return
  try { await workspaceAPI.revokeKey(workspaceId.value, projectId.value, keyId); await load(); app.showSuccess(t('common.deleted')) } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.keyError')) }
}
async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); app.showSuccess(t('common.copied')) } catch { app.showError(t('common.copyFailed')) }
}

onMounted(load)
watch([workspaceId, projectId, () => store.selectedWorkspaceId, () => store.selectedWorkspace?.status, () => store.permissions.join('|')], () => {
  ++projectLifecycleGeneration; projectLifecycleController.abort(); projectLifecycleController = new AbortController(); projectStepUp.onCancel()
  projectLifecycleAction.value = null; projectLifecycleBusy.value = false; projectLifecycleError.value = ''; projectLifecycleNotice.value = ''; projectLifecycleReauth.value = false
}, { flush: 'sync' })
watch([workspaceId, projectId], () => {
  ++loadGeneration
  loadController?.abort()
  project.value = null
  keys.value = []
  projectBudget.value = null
  projectUsage.value = null
  projectOverview.value = null
  selectedServiceAccountId.value = ''
  serviceAccountChoices.value = []
  availableGroups.value = []
  secret.value = ''
  closeCreateForm()
  closeEditForm()
}, { flush: 'sync' })
watch([workspaceId, projectId, () => store.selectedWorkspaceId, () => store.permissions], () => void load())
watch([() => store.projects, projectId], () => {
  if (store.selectedWorkspaceId === workspaceId.value && store.projects.some(item => item.id === projectId.value) && store.selectedProjectId !== projectId.value) void store.selectProject(projectId.value)
}, { immediate: true })
onBeforeUnmount(() => { ++loadGeneration; loadController?.abort(); ++projectLifecycleGeneration; projectLifecycleController.abort(); projectStepUp.onCancel() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.workspace-panel__heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.workspace-panel h3 { margin: 0; color: var(--color-text-primary); font-size: 15px; }
.workspace-panel p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.workspace-lifecycle-error { color: var(--color-danger) !important; overflow-wrap: anywhere; }
.workspace-machine-filter { display: grid; gap: 6px; margin-top: 16px; max-width: 360px; color: var(--color-text-secondary); font-size: 14px; }
.workspace-eyebrow { color: var(--color-text-muted) !important; font-size: 12px !important; font-weight: 700; text-transform: uppercase; }
.workspace-details { display: grid; min-width: 0; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 20px 0 0; }
.workspace-details div { display: grid; gap: 5px; }
.workspace-details dt { color: var(--color-text-muted); font-size: 12px; }
.workspace-details dd { margin: 0; color: var(--color-text-primary); overflow-wrap: anywhere; }
.workspace-table-wrap { overflow-x: auto; margin-top: 18px; }
.workspace-table { width: 100%; min-width: 960px; border-collapse: collapse; }
.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; vertical-align: middle; }
.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }
.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }
.workspace-table small { display: block; margin-top: 3px; color: var(--color-text-muted); }
.workspace-edit-row td { background: var(--color-surface-soft); }
.workspace-form-panel { max-width: 760px; }
.workspace-form { display: grid; gap: 14px; margin-top: 16px; }
.workspace-form__heading { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.workspace-form-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.workspace-form label { display: grid; gap: 6px; min-width: 0; color: var(--color-text-secondary); font-size: 13px; }
.workspace-form__wide { grid-column: 1 / -1; }
.workspace-form textarea { resize: vertical; }
.workspace-actions { display: flex; flex-wrap: wrap; gap: 7px; }
.workspace-token { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-top: 18px; border: 1px solid var(--color-primary-border); border-radius: 8px; background: var(--color-primary-soft); padding: 12px; }
.workspace-token code { min-width: 0; overflow-wrap: anywhere; }
.workspace-budget__values { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin-top: 18px; }
.workspace-budget__values div { display: grid; gap: 5px; min-width: 0; }
.workspace-budget__values span { color: var(--color-text-muted); font-size: 12px; }
.workspace-budget__values strong { color: var(--color-text-primary); font-size: 20px; font-variant-numeric: tabular-nums; }
.workspace-form .workspace-checkbox { display: flex; align-items: center; gap: 8px; min-height: 36px; }
.workspace-checkbox input { width: 16px; height: 16px; accent-color: var(--color-primary); }
.workspace-project-usage { display: flex; flex-wrap: wrap; gap: 18px; margin-top: 18px; color: var(--color-text-secondary); font-size: 14px; font-variant-numeric: tabular-nums; }
.workspace-period { overflow-wrap: anywhere; font-size: 12px !important; }
.workspace-warning { color: var(--color-warning) !important; }
.workspace-danger { color: var(--color-danger); }
.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } .workspace-details, .workspace-form-grid, .workspace-budget__values { grid-template-columns: 1fr; } .workspace-form__wide { grid-column: auto; } }
</style>
