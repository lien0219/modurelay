<template>
  <section v-if="store.can('lifecycle.read')" class="lifecycle-panel" data-testid="lifecycle-panel" :aria-busy="loading || busy || undefined">
    <div class="lifecycle-heading">
      <div><h2>{{ t('workspace.lifecycle.title') }}</h2><p>{{ t('workspace.lifecycle.description') }}</p></div>
      <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || busy" @click="refresh">{{ t('workspace.lifecycle.refresh') }}</button>
    </div>
    <p v-if="loading && !summary" role="status">{{ t('common.loading') }}</p>
    <div v-if="errorMessage" class="lifecycle-error" role="alert">
      <p>{{ errorMessage }}</p>
      <RouterLink v-if="authRecovery" data-testid="lifecycle-sign-in" class="btn btn-secondary btn-sm" :to="signInPath">{{ t('workspace.lifecycle.signIn') }}</RouterLink>
    </div>
    <p v-if="notice" class="lifecycle-success" role="status">{{ notice }}</p>

    <template v-if="summary">
      <div class="lifecycle-section">
        <h3>{{ t('workspace.lifecycle.retention') }}</h3>
        <p>{{ t('workspace.lifecycle.retentionHint') }}</p>
        <div class="lifecycle-table-scroll">
          <table class="lifecycle-table">
            <caption class="sr-only">{{ t('workspace.lifecycle.retention') }}</caption>
            <thead><tr><th scope="col">{{ t('workspace.lifecycle.category') }}</th><th scope="col">{{ t('workspace.lifecycle.effectiveRetention') }}</th><th scope="col">{{ t('workspace.lifecycle.minimum') }}</th><th scope="col">{{ t('workspace.lifecycle.actions') }}</th></tr></thead>
            <tbody>
              <tr v-for="policy in summary.retention" :key="policy.category">
                <th scope="row">{{ label('categories', policy.category) }}</th>
                <td>{{ policy.retention_days === 0 ? t('workspace.lifecycle.indefinite') : t('workspace.lifecycle.days', { days: policy.retention_days }) }}<span v-if="policy.protected" class="lifecycle-meta">{{ t('workspace.lifecycle.protected') }}</span></td>
                <td>{{ policy.minimum_days > 0 ? t('workspace.lifecycle.days', { days: policy.minimum_days }) : t('workspace.lifecycle.indefinite') }}</td>
                <td>
                  <form v-if="canEditPolicy(policy)" class="lifecycle-retention-form" @submit.prevent="saveRetention(policy)">
                    <label :for="`retention-${workspaceId}-${policy.category}`" class="sr-only">{{ t('workspace.lifecycle.retentionFor', { category: label('categories', policy.category) }) }}</label>
                    <input :id="`retention-${workspaceId}-${policy.category}`" v-model="retentionDraft[policy.category]" :data-testid="`retention-days-${policy.category}`" class="input" type="number" min="0" max="36500" step="1" :disabled="busy" :aria-invalid="!validRetention(policy) || undefined" :aria-describedby="`retention-hint-${workspaceId}`">
                    <button type="submit" class="btn btn-secondary btn-sm" :data-testid="`retention-save-${policy.category}`" :disabled="busy || !validRetention(policy)">{{ t('common.save') }}</button>
                  </form>
                  <span v-else>{{ t('workspace.lifecycle.readOnly') }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <p :id="`retention-hint-${workspaceId}`">{{ t('workspace.lifecycle.retentionFloorHint') }}</p>
      </div>

      <div v-if="canArchive || canRestore" class="lifecycle-section">
        <h3>{{ t('workspace.lifecycle.archiveRestore') }}</h3>
        <p>{{ t('workspace.lifecycle.archiveHint') }}</p>
        <div class="lifecycle-actions">
          <button v-if="canArchive" type="button" data-testid="archive-workspace" class="btn btn-secondary" :disabled="busy" @click="confirmation = 'archive'">{{ t('workspace.lifecycle.archiveWorkspace') }}</button>
          <button v-if="canRestore" type="button" data-testid="restore-workspace" class="btn btn-primary" :disabled="busy" @click="confirmation = 'restore'">{{ t('workspace.lifecycle.restoreWorkspace') }}</button>
        </div>
      </div>

      <div v-if="store.can('export.read')" class="lifecycle-section">
        <div class="lifecycle-heading">
          <div><h3>{{ t('workspace.lifecycle.exports') }}</h3><p>{{ t('workspace.lifecycle.exportHint') }}</p></div>
          <button v-if="canCreateExport" type="button" data-testid="create-export" class="btn btn-primary btn-sm" :disabled="busy" @click="createExport">{{ t('workspace.lifecycle.createExport') }}</button>
        </div>
        <p v-if="!summary.capabilities.export_enabled">{{ t('workspace.lifecycle.exportDisabled') }}</p>
        <p v-if="!exports.length">{{ t('workspace.lifecycle.noExports') }}</p>
        <div v-else class="lifecycle-table-scroll">
          <table class="lifecycle-table">
            <caption class="sr-only">{{ t('workspace.lifecycle.exports') }}</caption>
            <thead><tr><th scope="col">{{ t('workspace.lifecycle.exportJob') }}</th><th scope="col">{{ t('common.status') }}</th><th scope="col">{{ t('workspace.lifecycle.progress') }}</th><th scope="col">{{ t('workspace.lifecycle.snapshot') }}</th><th scope="col">{{ t('workspace.lifecycle.artifact') }}</th><th scope="col">{{ t('workspace.lifecycle.actions') }}</th></tr></thead>
            <tbody><tr v-for="job in exports" :key="job.id">
              <th scope="row"><code>{{ job.id }}</code><span class="lifecycle-meta">{{ t('workspace.lifecycle.created') }}: {{ formatDate(job.created_at) }}</span><span class="lifecycle-meta">{{ t('workspace.lifecycle.updated') }}: {{ formatDate(job.updated_at) }}</span></th>
              <td>{{ label('states', job.state) }}<span v-if="job.failure_code" class="lifecycle-error lifecycle-meta">{{ job.failure_code }}</span></td>
              <td>{{ t('workspace.lifecycle.processed', { count: job.progress }) }}<span class="lifecycle-meta">{{ t('workspace.lifecycle.attempts', { count: job.attempts }) }}</span></td>
              <td>{{ formatDate(job.data_cutoff) }}<span class="lifecycle-meta">{{ t('workspace.lifecycle.expires') }}: {{ job.state === 'completed' && job.expires_at === null ? t('workspace.lifecycle.indefinite') : formatDate(job.expires_at) }}</span></td>
              <td>{{ t('workspace.lifecycle.bytes', { count: job.size_bytes }) }}<code v-if="job.artifact_sha256" class="lifecycle-meta">SHA-256: {{ job.artifact_sha256 }}</code><span v-if="job.completed_at" class="lifecycle-meta">{{ t('workspace.lifecycle.completed') }}: {{ formatDate(job.completed_at) }}</span></td>
              <td><div class="lifecycle-actions">
                <button v-if="canDownload(job)" type="button" :data-testid="`download-export-${job.id}`" class="btn btn-secondary btn-sm" :disabled="busy" @click="downloadExport(job)">{{ t('workspace.lifecycle.download') }}</button>
                <button v-if="store.can('export.cancel') && ['pending', 'running'].includes(job.state)" type="button" :data-testid="`cancel-export-${job.id}`" class="btn btn-secondary btn-sm" :disabled="busy" @click="cancelExport(job)">{{ t('workspace.lifecycle.cancelExport') }}</button>
              </div></td>
            </tr></tbody>
          </table>
        </div>
        <div v-if="exportPages > 1" class="lifecycle-actions" :aria-label="t('workspace.lifecycle.exportPagination')">
          <button type="button" class="btn btn-secondary btn-sm" :disabled="exportPage <= 1 || loading || busy" @click="changeExportPage(-1)">{{ t('workspace.lifecycle.previous') }}</button>
          <span>{{ t('workspace.lifecycle.page', { page: exportPage, pages: exportPages }) }}</span>
          <button type="button" class="btn btn-secondary btn-sm" :disabled="exportPage >= exportPages || loading || busy" @click="changeExportPage(1)">{{ t('workspace.lifecycle.next') }}</button>
        </div>
      </div>

      <div class="lifecycle-section">
        <h3>{{ t('workspace.lifecycle.deletion') }}</h3>
        <p>{{ t('workspace.lifecycle.deletionHint') }}</p>
        <p>{{ t('workspace.lifecycle.retainedConfiguration') }}</p>
        <p v-if="!summary.capabilities.purge_enabled">{{ t('workspace.lifecycle.purgeDisabled') }}</p>
        <div v-if="summary.deletion" class="lifecycle-job" aria-live="polite">
          <dl class="lifecycle-details">
            <div><dt>{{ t('common.status') }}</dt><dd>{{ label('states', summary.deletion.state) }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.businessClosure') }}</dt><dd>{{ summary.deletion.business_closed ? t('workspace.lifecycle.businessClosed') : t('workspace.lifecycle.businessNotClosed') }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.protectedEvidence') }}</dt><dd>{{ summary.deletion.protected_evidence_retained ? t('workspace.lifecycle.evidenceRetained') : t('workspace.lifecycle.evidenceStatusUnknown') }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.phase') }}</dt><dd>{{ label('phases', summary.deletion.phase) }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.progress') }}</dt><dd>{{ t('workspace.lifecycle.processed', { count: summary.deletion.progress }) }} · {{ t('workspace.lifecycle.attempts', { count: summary.deletion.attempts }) }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.earliestPurge') }}</dt><dd>{{ formatDate(summary.deletion.earliest_purge_at) }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.created') }}</dt><dd>{{ formatDate(summary.deletion.created_at) }}</dd></div>
            <div><dt>{{ t('workspace.lifecycle.updated') }}</dt><dd>{{ formatDate(summary.deletion.updated_at) }}</dd></div>
            <div v-if="summary.deletion.completed_at"><dt>{{ t('workspace.lifecycle.completed') }}</dt><dd>{{ formatDate(summary.deletion.completed_at) }}</dd></div>
          </dl>
          <p v-if="summary.deletion.failure_code" class="lifecycle-error">{{ summary.deletion.failure_code }}</p>
          <ul v-if="summary.deletion.blocking_reasons?.length" class="lifecycle-blockers"><li v-for="blocker in summary.deletion.blocking_reasons" :key="blocker.code">{{ blocker.reason }} ({{ blocker.count }})</li></ul>
          <div class="lifecycle-actions">
            <button v-if="canCancelDeletion" type="button" data-testid="cancel-deletion" class="btn btn-secondary" :disabled="busy" @click="cancelDeletion">{{ t('workspace.lifecycle.cancelDeletion') }}</button>
            <button v-if="canRetryDeletion" type="button" data-testid="retry-deletion" class="btn btn-secondary" :disabled="busy" @click="retryDeletion">{{ t('workspace.lifecycle.retryDeletion') }}</button>
          </div>
        </div>
        <div v-if="canRequestDeletion" class="lifecycle-actions">
          <button type="button" data-testid="deletion-preflight" class="btn btn-secondary" :disabled="busy" @click="checkPreflight">{{ t('workspace.lifecycle.checkPreflight') }}</button>
          <button type="button" data-testid="request-deletion" class="btn btn-danger" :disabled="busy || !preflight?.eligible" @click="beginDeletion">{{ t('workspace.lifecycle.requestDeletion') }}</button>
        </div>
        <div v-if="preflight" class="lifecycle-preflight" aria-live="polite">
          <p>{{ preflight.eligible ? t('workspace.lifecycle.eligible') : t('workspace.lifecycle.blocked') }}</p>
          <ul v-if="preflight.blocking_reasons?.length" class="lifecycle-blockers"><li v-for="blocker in preflight.blocking_reasons" :key="blocker.code">{{ blocker.reason }} ({{ blocker.count }})</li></ul>
          <p>{{ t('workspace.lifecycle.earliestPurge') }}: {{ formatDate(preflight.earliest_purge_at) }}</p>
          <p v-if="preflight.counts_capped">{{ t('workspace.lifecycle.cappedCounts') }}</p>
          <dl class="lifecycle-details">
            <div v-for="(count, resource) in preflight.resource_counts" :key="resource"><dt>{{ label('resources', resource) }}</dt><dd>{{ count }}</dd></div>
            <div v-for="(count, resource) in preflight.protected_records" :key="`protected-${resource}`"><dt>{{ t('workspace.lifecycle.protectedResource', { resource: label('resources', resource) }) }}</dt><dd>{{ count }}</dd></div>
          </dl>
          <h4>{{ t('workspace.lifecycle.purgeScope') }}</h4><ul><li v-for="scope in preflight.estimated_purge_scope" :key="scope">{{ label('scopes', scope) }}</li></ul>
          <h4>{{ t('workspace.lifecycle.retainedScope') }}</h4><ul><li v-for="scope in preflight.retained_scope" :key="scope">{{ label('scopes', scope) }}</li></ul>
          <p v-if="preflight.protected_evidence_retained">{{ t('workspace.lifecycle.evidenceRetained') }}</p>
        </div>
      </div>
    </template>

    <BaseDialog :show="deletionModal" :title="t('workspace.lifecycle.confirmDeletion')" :close-on-escape="!busy" :show-close-button="!busy" @close="closeDeletion">
      <div ref="deletionDialogBody" class="lifecycle-dialog">
        <p>{{ t('workspace.lifecycle.deletionHint') }}</p>
        <p>{{ t('workspace.lifecycle.retainedConfiguration') }}</p>
        <p v-if="preflight">{{ t('workspace.lifecycle.earliestPurge') }}: {{ formatDate(preflight.earliest_purge_at) }}</p>
        <ul v-if="preflight?.blocking_reasons?.length" class="lifecycle-blockers"><li v-for="blocker in preflight.blocking_reasons" :key="blocker.code">{{ blocker.reason }} ({{ blocker.count }})</li></ul>
        <p v-if="busy" role="status">{{ t('common.processing') }}</p>
        <div v-if="errorMessage" class="lifecycle-error" role="alert"><p>{{ errorMessage }}</p><RouterLink v-if="authRecovery" class="btn btn-secondary btn-sm" :to="signInPath">{{ t('workspace.lifecycle.signIn') }}</RouterLink></div>
        <p v-if="challengeExpired" role="status">{{ t('workspace.lifecycle.challengeExpired') }}</p>
        <p v-else-if="challenge">{{ t('workspace.lifecycle.confirmationExpires') }}: {{ formatDate(challenge.expires_at) }}</p>
        <label :for="`deletion-name-${workspaceId}`">{{ t('workspace.lifecycle.exactName', { name: workspaceName }) }}</label>
        <input :id="`deletion-name-${workspaceId}`" v-model="confirmationName" data-testid="deletion-name" class="input" autocomplete="off" :disabled="busy" :aria-describedby="`deletion-name-hint-${workspaceId}`">
        <p :id="`deletion-name-hint-${workspaceId}`">{{ t('workspace.lifecycle.exactNameHint') }}</p>
      </div>
      <template #footer><div class="lifecycle-actions">
        <button type="button" class="btn btn-secondary" :disabled="busy" @click="closeDeletion">{{ t('common.cancel') }}</button>
        <button v-if="challengeExpired || !challenge" type="button" class="btn btn-secondary" :disabled="busy" @click="generateChallenge">{{ t('workspace.lifecycle.refreshChallenge') }}</button>
        <button type="button" data-testid="confirm-deletion" class="btn btn-danger" :disabled="!canConfirmDeletion" @click="confirmDeletion">{{ t('workspace.lifecycle.requestDeletion') }}</button>
      </div></template>
    </BaseDialog>
    <ConfirmDialog :show="confirmation !== null" :title="confirmation === 'archive' ? t('workspace.lifecycle.archiveWorkspace') : t('workspace.lifecycle.restoreWorkspace')" :message="confirmation === 'archive' ? t('workspace.lifecycle.archiveConfirm') : t('workspace.lifecycle.restoreHint')" :confirming="busy" @cancel="confirmation = null" @confirm="confirmArchiveRestore">
      <div v-if="errorMessage" class="lifecycle-error" role="alert"><p>{{ errorMessage }}</p><RouterLink v-if="authRecovery" class="btn btn-secondary btn-sm" :to="signInPath">{{ t('workspace.lifecycle.signIn') }}</RouterLink></div>
    </ConfirmDialog>
    <TotpStepUpDialog :controller="stepUp" />
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { useStepUp, isStepUpCancelled } from '@/composables/useStepUp'
import { workspaceAPI } from '@/api/workspace'
import { workspaceLifecycleAPI as api, type LifecycleChallenge, type LifecycleExportJob, type LifecyclePreflight, type LifecycleRetentionPolicy, type LifecycleSummary } from '@/api/workspaceLifecycle'
import { useWorkspaceStore } from '@/stores/workspace'

const { t, te } = useI18n()
const store = useWorkspaceStore()
const route = useRoute()
const stepUp = useStepUp()
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const workspaceName = computed(() => store.selectedWorkspace?.name || '')
const workspaceStatus = computed(() => store.selectedWorkspace?.status || '')
const signInPath = computed(() => ({ path: '/login', query: { reauth: '1', redirect: route.fullPath } }))
const summary = ref<LifecycleSummary | null>(null)
const exports = ref<LifecycleExportJob[]>([])
const exportPage = ref(1)
const exportPages = ref(0)
const retentionDraft = reactive<Record<string, string | number>>({})
const loading = ref(false)
const busy = ref(false)
const errorMessage = ref('')
const authRecovery = ref(false)
const notice = ref('')
const preflight = ref<LifecyclePreflight | null>(null)
const deletionModal = ref(false)
const deletionDialogBody = ref<HTMLElement | null>(null)
const confirmationName = ref('')
const challenge = ref<LifecycleChallenge | null>(null)
const challengeExpired = ref(false)
const confirmation = ref<'archive' | 'restore' | null>(null)
let generation = 0
let loadSequence = 0
let mounted = true
let controller = new AbortController()
let pollTimer: ReturnType<typeof setTimeout> | undefined
let challengeTimer: ReturnType<typeof setTimeout> | undefined
type Context = { id: number; generation: number; signal: AbortSignal }
const canArchive = computed(() => workspaceStatus.value === 'active' && store.can('workspace.archive'))
const canRestore = computed(() => workspaceStatus.value === 'archived' && store.can('workspace.restore'))
const canCreateExport = computed(() => Boolean(summary.value?.capabilities.export_enabled && ['active', 'archived'].includes(workspaceStatus.value) && store.can('export.create')))
const canRequestDeletion = computed(() => Boolean(summary.value?.capabilities.purge_enabled && ['active', 'archived'].includes(workspaceStatus.value) && store.can('deletion.request')))
const canCancelDeletion = computed(() => {
  const job = summary.value?.deletion
  return Boolean(store.can('deletion.cancel') && workspaceStatus.value === 'pending_deletion' && job && ['pending', 'blocked', 'failed'].includes(job.state) && job.phase === 'credentials' && job.cursor === 0 && job.progress === 0)
})
const canRetryDeletion = computed(() => Boolean(summary.value?.capabilities.purge_enabled && store.can('deletion.retry') && ['pending_deletion', 'purging'].includes(workspaceStatus.value) && summary.value?.deletion && ['blocked', 'failed'].includes(summary.value.deletion.state)))
const canConfirmDeletion = computed(() => Boolean(deletionModal.value && canRequestDeletion.value && !busy.value && challenge.value?.token && !challengeExpired.value && challenge.value.preflight.eligible && preflight.value?.eligible && confirmationName.value === workspaceName.value))

function context(): Context { return { id: workspaceId.value, generation, signal: controller.signal } }
function current(ctx: Context): boolean { return mounted && !ctx.signal.aborted && ctx.id === workspaceId.value && ctx.generation === generation }
function label(group: string, value: string): string { const key = `workspace.lifecycle.${group}.${value}`; return te(key) ? t(key) : value.replace(/_/g, ' ') }
function formatDate(value?: string | null): string { if (!value) return '—'; const date = new Date(value); return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString() }
function canEditPolicy(policy: LifecycleRetentionPolicy): boolean { return workspaceStatus.value === 'active' && store.can('lifecycle.manage') && (!policy.protected || policy.minimum_days > 0) }
function validRetention(policy: LifecycleRetentionPolicy): boolean { const value = retentionDraft[policy.category]; const days = Number(value); return value !== undefined && String(value).trim() !== '' && Number.isSafeInteger(days) && days <= 36500 && (days === 0 || days >= policy.minimum_days) }
function canDownload(job: LifecycleExportJob): boolean { return store.can('export.download') && job.state === 'completed' && (job.expires_at === null || new Date(job.expires_at).getTime() > Date.now()) }
function showError(error: unknown) {
  if (isStepUpCancelled(error)) { errorMessage.value = t('workspace.lifecycle.verificationCancelled'); return }
  const item = error as { code?: string; reason?: string; message?: string }
  if (item?.code === 'ERR_CANCELED') return
  const markers = [item?.code, item?.reason]
  authRecovery.value = markers.some(marker => Boolean(marker && ['RECENT_AUTH_REQUIRED', 'WORKSPACE_REAUTH_REQUIRED', 'MFA_REQUIRED', 'MFA_ENROLLMENT_REQUIRED', 'STEP_UP_TOTP_NOT_ENABLED', 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'].includes(marker)))
  errorMessage.value = item?.message || t('workspace.lifecycle.requestError')
}
function clearChallenge() { clearTimeout(challengeTimer); challengeTimer = undefined; challenge.value = null; challengeExpired.value = false; confirmationName.value = '' }
function closeDeletion() { if (busy.value) return; deletionModal.value = false; clearChallenge() }
function setRetentionDraft(previous?: LifecycleRetentionPolicy[]) {
  const policies = summary.value?.retention || []
  for (const key of Object.keys(retentionDraft)) if (!policies.some(policy => policy.category === key)) delete retentionDraft[key]
  for (const policy of policies) {
    const draft = retentionDraft[policy.category]
    const previousDays = previous?.find(item => item.category === policy.category)?.retention_days
    if (!previous || draft === undefined || String(draft) === String(previousDays)) retentionDraft[policy.category] = String(policy.retention_days)
  }
}
function trapDeletionFocus(event: KeyboardEvent) {
  if (event.key !== 'Tab' || !deletionModal.value || stepUp.visible.value) return
  const dialog = deletionDialogBody.value?.closest<HTMLElement>('[role="dialog"]')
  if (!dialog) return
  const controls = Array.from(dialog.querySelectorAll<HTMLElement>('button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])')).filter(control => !control.hidden && control.tabIndex !== -1)
  if (!controls.length) { event.preventDefault(); dialog.tabIndex = -1; dialog.focus(); return }
  const first = controls[0], last = controls[controls.length - 1]
  if (!dialog.contains(document.activeElement)) { event.preventDefault(); first.focus() }
  else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
  else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
}

async function load(ctx: Context) {
  if (!ctx.id || !current(ctx) || !store.can('lifecycle.read')) return
  const sequence = ++loadSequence
  loading.value = true
  clearTimeout(pollTimer)
  const results = await Promise.allSettled([
    api.getLifecycle(ctx.id, ctx.signal),
    store.can('export.read') ? api.listExports(ctx.id, exportPage.value, 10, ctx.signal) : Promise.resolve(null),
    workspaceAPI.getWorkspace(ctx.id, ctx.signal),
  ])
  if (!current(ctx) || sequence !== loadSequence) return
  const [lifecycleResult, exportResult, workspaceResult] = results
  if (lifecycleResult.status === 'fulfilled') { const previous = summary.value?.retention; summary.value = lifecycleResult.value; setRetentionDraft(previous) } else showError(lifecycleResult.reason)
  if (exportResult.status === 'fulfilled') { exports.value = exportResult.value?.items || []; exportPages.value = exportResult.value?.pages || 0 } else showError(exportResult.reason)
  if (workspaceResult.status === 'fulfilled' && workspaceResult.value) {
    const latest = workspaceResult.value
    store.workspaces = store.workspaces.map(item => item.id === ctx.id ? latest : item)
    const permissions = latest.permissions || []
    if (permissions.length !== store.permissions.length || permissions.some(permission => !store.permissions.includes(permission))) store.permissions = permissions
  } else if (workspaceResult.status === 'rejected') showError(workspaceResult.reason)
  if (!current(ctx) || sequence !== loadSequence) return
  loading.value = false
  schedulePoll(ctx)
}
function schedulePoll(ctx: Context) {
  clearTimeout(pollTimer)
  if (!current(ctx)) return
  const inProgress = ['pending', 'running'].includes(summary.value?.deletion?.state || '') || exports.value.some(job => ['pending', 'running'].includes(job.state))
  if (inProgress) pollTimer = setTimeout(() => { if (current(ctx)) { if (busy.value) schedulePoll(ctx); else void load(ctx) } }, 15000)
}
async function refresh() { if (busy.value || loading.value) return; await load(context()) }
async function changeExportPage(delta: number) { if (busy.value || loading.value) return; exportPage.value += delta; await load(context()) }

async function operation(permission: string, action: (ctx: Context) => Promise<void>, refreshAfter = true, success = true) {
  if (busy.value || !workspaceId.value || !store.can(permission)) return
  const ctx = context()
  busy.value = true
  errorMessage.value = ''; authRecovery.value = false; notice.value = ''
  try {
    await stepUp.run(async () => {
      if (!current(ctx) || !store.can(permission)) throw { code: 'ERR_CANCELED' }
      await action(ctx)
    })
    if (!current(ctx)) return
    if (success) notice.value = t('workspace.lifecycle.operationSuccess')
    if (refreshAfter) await load(ctx)
  } catch (error) { if (current(ctx)) showError(error) }
  finally { if (current(ctx)) { busy.value = false; schedulePoll(ctx) } }
}
async function saveRetention(policy: LifecycleRetentionPolicy) {
  if (!canEditPolicy(policy) || !validRetention(policy)) return
  const days = Number(retentionDraft[policy.category])
  await operation('lifecycle.manage', async ctx => { const policies = await api.updateRetention(ctx.id, policy.category, days, ctx.signal); if (current(ctx) && summary.value) { const previous = summary.value.retention; summary.value.retention = policies; setRetentionDraft(previous); retentionDraft[policy.category] = String(policies.find(item => item.category === policy.category)?.retention_days ?? days) } }, false)
}
async function confirmArchiveRestore() {
  const action = confirmation.value
  if ((action === 'archive' && !canArchive.value) || (action === 'restore' && !canRestore.value) || !action) return
  await operation(action === 'archive' ? 'workspace.archive' : 'workspace.restore', async ctx => {
    if (action === 'archive') await workspaceAPI.archiveWorkspace(ctx.id)
    else await api.restoreWorkspace(ctx.id, ctx.signal)
    if (current(ctx)) confirmation.value = null
  })
}
async function createExport() { if (canCreateExport.value) await operation('export.create', async ctx => { await api.createExport(ctx.id, ctx.signal); if (current(ctx)) exportPage.value = 1 }) }
async function cancelExport(job: LifecycleExportJob) { if (['pending', 'running'].includes(job.state)) await operation('export.cancel', async ctx => { await api.cancelExport(ctx.id, job.id, ctx.signal) }) }
async function downloadExport(job: LifecycleExportJob) {
  if (!canDownload(job)) return
  await operation('export.download', async ctx => {
    let token = ''
    try {
      const grant = await api.authorizeDownload(ctx.id, job.id, ctx.signal)
      if (!current(ctx)) return
      const expiry = new Date(grant.expires_at).getTime()
      if (!Number.isFinite(expiry) || expiry <= Date.now()) throw new Error(t('workspace.lifecycle.downloadExpired'))
      token = grant.token
      const blob = await api.redeemDownload(ctx.id, job.id, token, ctx.signal)
      token = ''
      if (!current(ctx)) return
      const objectURL = URL.createObjectURL(blob)
      try {
        const link = document.createElement('a')
        link.href = objectURL; link.download = `workspace-${ctx.id}-${job.id}.zip`
        document.body.appendChild(link)
        try { link.click() } finally { link.remove() }
      } finally { URL.revokeObjectURL(objectURL) }
    } finally { token = '' }
  }, false)
}
async function checkPreflight() {
  if (!canRequestDeletion.value) return
  await operation('deletion.request', async ctx => { const result = await api.getDeletionPreflight(ctx.id, ctx.signal); if (current(ctx)) preflight.value = result }, false, false)
}
async function beginDeletion() { if (!canRequestDeletion.value || !preflight.value?.eligible || busy.value) return; deletionModal.value = true; await generateChallenge() }
async function generateChallenge() {
  if (!canRequestDeletion.value) return
  clearChallenge()
  await operation('deletion.request', async ctx => {
    const result = await api.createDeletionChallenge(ctx.id, ctx.signal)
    if (!current(ctx) || !deletionModal.value) return
    preflight.value = result.preflight
    const remaining = new Date(result.expires_at).getTime() - Date.now()
    if (!Number.isFinite(remaining) || remaining <= 0) { challengeExpired.value = true; return }
    challenge.value = result
    challengeTimer = setTimeout(() => { if (current(ctx)) { challenge.value = null; challengeExpired.value = true } }, remaining)
  }, false, false)
}
async function confirmDeletion() {
  if (!canConfirmDeletion.value || !challenge.value || new Date(challenge.value.expires_at).getTime() <= Date.now()) return
  const payload = { name: confirmationName.value, token: challenge.value.token }
  clearTimeout(challengeTimer)
  challenge.value = null
  await operation('deletion.request', async ctx => {
    const job = await api.requestDeletion(ctx.id, { ...payload }, ctx.signal)
    if (!current(ctx)) return
    if (summary.value) summary.value.deletion = job
    deletionModal.value = false; clearChallenge()
  })
  payload.token = ''
}
async function cancelDeletion() { const job = summary.value?.deletion; if (job && canCancelDeletion.value) await operation('deletion.cancel', async ctx => { await api.cancelDeletion(ctx.id, job.id, ctx.signal) }) }
async function retryDeletion() { const job = summary.value?.deletion; if (job && canRetryDeletion.value) await operation('deletion.retry', async ctx => { await api.retryDeletion(ctx.id, job.id, ctx.signal) }) }

watch([workspaceId, workspaceName, workspaceStatus, () => store.permissions.join('|')], () => {
  ++generation; ++loadSequence; controller.abort(); controller = new AbortController()
  clearTimeout(pollTimer); clearChallenge(); stepUp.onCancel()
  summary.value = null; exports.value = []; exportPage.value = 1; exportPages.value = 0
  preflight.value = null; deletionModal.value = false; confirmation.value = null
  loading.value = false; busy.value = false; errorMessage.value = ''; authRecovery.value = false; notice.value = ''
  setRetentionDraft()
  void load(context())
}, { immediate: true, flush: 'sync' })
onMounted(() => document.addEventListener('keydown', trapDeletionFocus, true))
onBeforeUnmount(() => { mounted = false; ++generation; controller.abort(); clearTimeout(pollTimer); clearChallenge(); stepUp.onCancel(); document.removeEventListener('keydown', trapDeletionFocus, true) })
</script>

<style scoped>
.lifecycle-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.lifecycle-heading { display: flex; align-items: flex-start; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
.lifecycle-heading > div { min-width: 0; flex: 1; }
.lifecycle-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.lifecycle-panel h3, .lifecycle-panel h4 { margin: 0; color: var(--color-text-primary); font-size: 15px; }
.lifecycle-panel p, .lifecycle-dialog p { margin: 8px 0; color: var(--color-text-secondary); font-size: 14px; }
.lifecycle-section { min-width: 0; margin-top: 20px; border-top: 1px solid var(--color-border-subtle); padding-top: 18px; }
.lifecycle-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 10px; }
.lifecycle-table-scroll { max-width: 100%; margin-top: 12px; overflow-x: auto; }
.lifecycle-table { width: 100%; border-collapse: collapse; color: var(--color-text-secondary); font-size: 14px; text-align: left; }
.lifecycle-table th, .lifecycle-table td { min-width: 130px; border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; vertical-align: top; }
.lifecycle-table thead { background: var(--color-surface-soft); }
.lifecycle-table th { color: var(--color-text-primary); font-weight: 600; }
.lifecycle-table code { display: block; max-width: 240px; overflow-wrap: anywhere; }
.lifecycle-meta { display: block; margin-top: 6px; color: var(--color-text-muted); font-size: 12px; }
.lifecycle-retention-form { display: flex; align-items: center; gap: 8px; }
.lifecycle-retention-form input { width: 100px; }
.lifecycle-details { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 12px; margin: 14px 0; }
.lifecycle-details > div { min-width: 0; }
.lifecycle-details dt { color: var(--color-text-muted); font-size: 12px; }
.lifecycle-details dd { margin: 4px 0 0; color: var(--color-text-primary); font-size: 14px; overflow-wrap: anywhere; }
.lifecycle-preflight ul { padding-left: 20px; color: var(--color-text-secondary); font-size: 14px; }
.lifecycle-blockers { color: var(--color-warning); padding-left: 20px; }
.lifecycle-error, .lifecycle-error p { color: var(--color-danger) !important; overflow-wrap: anywhere; }
.lifecycle-success { color: var(--color-success) !important; }
.lifecycle-dialog { display: grid; gap: 8px; }
.lifecycle-dialog label { color: var(--color-text-primary); font-size: 14px; overflow-wrap: anywhere; }
.lifecycle-panel button:focus-visible, .lifecycle-panel a:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 3px; }
@media (max-width: 640px) { .lifecycle-panel { padding: 16px; } .lifecycle-heading { align-items: stretch; flex-direction: column; } .lifecycle-actions .btn { min-height: 44px; } }
</style>
