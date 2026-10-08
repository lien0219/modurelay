<template>
  <WorkspaceFrame section="security">
    <div class="security-page">
      <section class="security-panel">
        <div class="security-heading"><div><p class="security-eyebrow">{{ t('workspace.securityEyebrow') }}</p><h2>{{ t('workspace.securityTitle') }}</h2><p>{{ t('workspace.securityDescription') }}</p></div><span v-if="policy" class="security-revision">{{ t('workspace.policyRevision', { revision: policy.revision }) }}</span></div>
        <p v-if="!isOrganization" class="security-state">{{ t('workspace.securityPersonalUnsupported') }}</p>
        <p v-else-if="!canRead" class="security-state">{{ t('workspace.securityUnavailable') }}</p>
        <p v-else-if="!routeReady || loading" class="security-state" role="status">{{ t('common.loading') }}</p>
        <div v-else-if="loadError" role="alert" class="security-error"><p>{{ t('workspace.securityLoadError') }}</p><button type="button" class="btn btn-secondary" @click="load">{{ t('common.retry') }}</button></div>
        <p v-else-if="!canUpdate" class="security-notice">{{ t('workspace.securityReadOnly') }}</p>
      </section>

      <WorkspaceAccessRecovery v-if="recoveryError && canRead && routeReady && !loading" :workspace-id="workspaceId" :error="recoveryError" :requires-s-s-o="recoveryRequiresSSO" :return-to="route.fullPath" @verified="afterVerification" />
      <template v-if="policy && canRead && routeReady && !loading && !loadError">
        <form class="security-panel security-form" data-testid="security-form" @submit.prevent="previewPolicy">
          <fieldset :disabled="!canUpdate || saving" class="security-section">
            <legend>{{ t('workspace.securityAuthentication') }}</legend>
            <label class="security-check"><input v-model="form.require_sso" name="require_sso" type="checkbox"><span><strong>{{ t('workspace.identityRequireSSO') }}</strong><small>{{ t('workspace.securitySSOHint') }}</small></span></label>
            <label class="security-check"><input v-model="form.require_mfa" name="require_mfa" type="checkbox"><span><strong>{{ t('workspace.securityRequireMFA') }}</strong><small>{{ t('workspace.securityMFAHint') }}</small></span></label>
            <div class="security-fields">
              <label><span>{{ t('workspace.identityGraceUntil') }}</span><input v-model="form.sso_grace_until" name="sso_grace_until" class="input" type="datetime-local"><small>{{ t('workspace.securityGraceHint') }}</small></label>
              <label><span>{{ t('workspace.securitySessionAge') }}</span><input v-model="form.session_max_age_seconds" name="session_max_age_seconds" class="input" type="number" step="1" :min="ageMinimum" :max="ageMaximum" :aria-invalid="Boolean(validationError)" :aria-describedby="validationError ? 'security-validation' : 'security-age-hint'"><small id="security-age-hint">{{ t('workspace.securitySessionAgeHint', { min: ageMinimum, max: ageMaximum }) }}</small></label>
            </div>
            <p class="security-notice">{{ t('workspace.securityLegacySessionHint') }}</p>
          </fieldset>
          <fieldset :disabled="!canUpdate || saving" class="security-section">
            <legend>{{ t('workspace.securityMembership') }}</legend>
            <label><span>{{ t('workspace.securityInvitationPolicy') }}</span><select v-model="form.invitation_policy" name="invitation_policy" class="input"><option value="any">{{ t('workspace.securityInvitationsAny') }}</option><option value="verified_domains_only">{{ t('workspace.securityInvitationsVerified') }}</option><option value="disabled">{{ t('workspace.securityInvitationsDisabled') }}</option></select></label>
            <label class="security-check"><input v-model="form.allow_external_members" name="allow_external_members" type="checkbox"><span><strong>{{ t('workspace.securityAllowExternal') }}</strong><small>{{ t('workspace.securityExternalHint') }}</small></span></label>
            <label class="security-check"><input v-model="form.workspace_jit_enabled" name="workspace_jit_enabled" type="checkbox"><span><strong>{{ t('workspace.securityJIT') }}</strong><small>{{ t('workspace.securityJITHint') }}</small></span></label>
            <p class="security-notice">{{ t('workspace.securityRetainedMembers', { count: policy.external_member_count ?? 0 }) }}</p>
            <div class="security-domains"><strong>{{ t('workspace.identityDomains') }}</strong><ul v-if="policy.verified_domains?.length"><li v-for="domain in policy.verified_domains" :key="domain"><code>{{ domain }}</code></li></ul><p v-else>{{ t('workspace.securityNoVerifiedDomains') }}</p></div>
            <p v-if="domainRestriction && !policy.verified_domains?.length" class="security-warning" role="status">{{ t('workspace.securityNoDomainWarning') }}</p>
          </fieldset>
          <fieldset :disabled="!canUpdate || saving" class="security-section">
            <legend>{{ t('workspace.securityProviders') }}</legend>
            <label><span>{{ t('workspace.securityProviderMode') }}</span><select v-model="form.approved_identity_provider_mode" name="approved_identity_provider_mode" class="input"><option value="any_active">{{ t('workspace.securityProvidersAny') }}</option><option value="selected">{{ t('workspace.securityProvidersSelected') }}</option></select></label>
            <p class="security-notice">{{ t('workspace.securityProvidersHint') }}</p>
            <template v-if="form.approved_identity_provider_mode === 'selected'">
              <p v-if="providersError" class="security-warning" role="alert">{{ t('workspace.securityProviderLoadError') }}</p>
              <div class="security-provider-list"><label v-for="provider in providerOptions" :key="provider.id" class="security-check" :data-testid="`security-provider-${provider.id}`"><input v-model="form.approved_identity_provider_ids" type="checkbox" :value="provider.id" :disabled="!canUpdate || saving || (provider.status !== 'active' && !form.approved_identity_provider_ids.includes(provider.id))"><span>{{ provider.name }} <small>{{ provider.status === 'active' ? t('workspace.identityProviderStatus.active') : provider.status === 'unknown' ? t('workspace.securityProviderDetailsUnavailable') : t('workspace.securityProviderUnavailable') }}</small></span></label></div>
              <p v-if="!form.approved_identity_provider_ids.length" class="security-warning" role="status">{{ t('workspace.securityNoApprovedProviders') }}</p>
            </template>
          </fieldset>
          <div v-if="validationError" id="security-validation" ref="validationSummary" class="security-error" role="alert" tabindex="-1" data-testid="security-validation">{{ validationError }}</div>
          <div v-if="actionError" class="security-error" role="alert">{{ actionError }}</div>
          <div v-if="conflict" class="security-warning" role="alert" data-testid="security-conflict"><p>{{ t('workspace.securityConflict') }}</p><button type="button" class="btn btn-secondary" @click="load">{{ t('workspace.securityRefreshPolicy') }}</button></div>
          <div v-if="canUpdate" class="security-actions"><button type="button" class="btn btn-secondary" data-testid="security-preview" :disabled="previewing || saving || !dirty || conflict" @click="previewPolicy">{{ previewing ? t('common.loading') : t('workspace.securityPreview') }}</button><button type="button" class="btn btn-primary" data-testid="identity-save-policy" :disabled="!canSave" @click="openConfirmation">{{ saving ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-ghost" :disabled="saving" @click="resetDraft">{{ t('workspace.securityReset') }}</button></div>
        </form>
        <section v-if="preview" class="security-panel" role="status" aria-live="polite" data-testid="security-preview-result"><h3>{{ t('workspace.securityPreviewTitle') }}</h3><p>{{ t(preview.decision?.allowed ? 'workspace.securityDecisionAllowed' : workspaceSecurityMessageKey({ reason: preview.decision?.reason })) }}</p><p>{{ t('workspace.securityPreviewHint') }}</p><p v-if="preview.prerequisite_reason" class="security-warning">{{ t(workspaceSecurityMessageKey({ reason: preview.prerequisite_reason })) }}</p></section>
      </template>
      <ConfirmDialog :show="Boolean(pending)" :title="t('workspace.securityConfirmTitle')" :message="t('workspace.securityConfirmDescription')" :confirming="saving" :confirm-text="t('common.save')" @confirm="savePolicy" @cancel="pending = null"><ul class="security-confirm-list"><li>{{ t('workspace.securityConfirmAuth') }}</li><li>{{ t('workspace.securityConfirmMembers') }}</li></ul></ConfirmDialog>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import WorkspaceAccessRecovery from '@/components/workspace/WorkspaceAccessRecovery.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { workspaceAPI, type WorkspaceSecurityPolicy, type WorkspaceSecurityPolicyPatch, type WorkspaceIdentityProvider } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'
import { browserSessionIdentity, isWorkspaceAssuranceRequired, workspaceSecurityMessageKey, workspaceSecurityReason } from '@/utils/workspaceSecurity'

const { t } = useI18n()
const route = useRoute()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const isOrganization = computed(() => store.selectedWorkspace?.type === 'organization')
const routeReady = computed(() => Number(route.params.workspaceId) === workspaceId.value)
const canRead = computed(() => isOrganization.value && store.can('workspace_security.read'))
const canUpdate = computed(() => canRead.value && store.can('workspace_security.update'))
const policy = ref<WorkspaceSecurityPolicy | null>(null)
const providers = ref<WorkspaceIdentityProvider[]>([])
const loading = ref(false)
const loadError = ref(false)
const providersError = ref(false)
const previewing = ref(false)
const saving = ref(false)
const conflict = ref(false)
const validationError = ref('')
const validationSummary = ref<HTMLElement | null>(null)
const actionError = ref('')
const preview = ref<WorkspaceSecurityPolicy | null>(null)
const recoveryError = ref<unknown>(null)
const recoveryRequiresSSO = computed(() => Boolean(preview.value?.require_sso ?? policy.value?.require_sso))
const defaults = () => ({ require_sso: false, require_mfa: false, sso_grace_until: '', session_max_age_seconds: '' as string | number, invitation_policy: 'any' as WorkspaceSecurityPolicy['invitation_policy'], allow_external_members: true, workspace_jit_enabled: true, approved_identity_provider_mode: 'any_active' as WorkspaceSecurityPolicy['approved_identity_provider_mode'], approved_identity_provider_ids: [] as number[] })
const form = reactive(defaults())
const ageMinimum = computed(() => policy.value?.session_max_age_min_seconds ?? 900)
const ageMaximum = computed(() => policy.value?.session_max_age_max_seconds ?? 2592000)
const domainRestriction = computed(() => !form.allow_external_members || form.invitation_policy === 'verified_domains_only')
const providerOptions = computed(() => {
  const options = [...providers.value]
  for (const id of form.approved_identity_provider_ids) if (!options.some(provider => provider.id === id)) options.push({ id, name: t('workspace.securityProviderID', { id }), status: canUpdate.value && store.can('identity.read') && !providersError.value ? 'unavailable' : 'unknown' } as WorkspaceIdentityProvider)
  return options
})
let generation = 0
let controller: AbortController | null = null
let previewController: AbortController | null = null
let previewGeneration = 0
let previewSnapshot = ''
let loadedSession = ''
let originalGraceInput = ''
const pending = ref<{ workspaceId: number; generation: number; session: string; snapshot: string; payload: WorkspaceSecurityPolicyPatch } | null>(null)
const signature = computed(() => JSON.stringify(form))
const original = ref('')
const dirty = computed(() => Boolean(policy.value && signature.value !== original.value))
const canSave = computed(() => canUpdate.value && routeReady.value && dirty.value && !saving.value && !previewing.value && !conflict.value && Boolean(preview.value?.decision?.allowed) && !preview.value?.prerequisite_reason && previewSnapshot === signature.value)

function current(id: number, expected: number, session: string): boolean { return id === workspaceId.value && routeReady.value && canRead.value && expected === generation && session === browserSessionIdentity() }
function localDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16)
}
function setPolicy(value: WorkspaceSecurityPolicy) {
  policy.value = value
  originalGraceInput = value.sso_grace_until ? localDateTime(value.sso_grace_until) : ''
  Object.assign(form, { require_sso: value.require_sso, require_mfa: value.require_mfa, sso_grace_until: originalGraceInput, session_max_age_seconds: value.session_max_age_seconds ?? '', invitation_policy: value.invitation_policy, allow_external_members: value.allow_external_members, workspace_jit_enabled: value.workspace_jit_enabled, approved_identity_provider_mode: value.approved_identity_provider_mode, approved_identity_provider_ids: [...(value.approved_identity_provider_ids || [])] })
  original.value = signature.value
}
function invalidatePreview() { ++previewGeneration; previewController?.abort(); preview.value = null; previewSnapshot = ''; previewing.value = false; pending.value = null }
function resetDraft() { if (policy.value) setPolicy(policy.value); invalidatePreview(); validationError.value = ''; actionError.value = '' }

async function load() {
  const expected = ++generation
  const id = workspaceId.value
  loadedSession = browserSessionIdentity()
  const session = loadedSession
  controller?.abort()
  invalidatePreview()
  policy.value = null
  providers.value = []
  loadError.value = false
  providersError.value = false
  conflict.value = false
  actionError.value = ''
  recoveryError.value = null
  validationError.value = ''
  if (!canRead.value || !routeReady.value || !id) { loading.value = false; return }
  controller = new AbortController()
  const signal = controller.signal
  loading.value = true
  try {
    const value = await workspaceAPI.getSecurityPolicy(id, signal)
    if (!current(id, expected, session)) return
    setPolicy(value)
    if (value.decision && !value.decision.allowed) recoveryError.value = { reason: value.decision.reason }
    if (store.can('identity.read') && canUpdate.value) {
      try {
        const all: WorkspaceIdentityProvider[] = []
        for (let page = 1; ; page++) {
          const result = await workspaceAPI.listIdentityProviders(id, { page, page_size: 50, signal })
          if (!current(id, expected, session)) return
          all.push(...(result.items || []))
          const pages = result.pages || Math.ceil((result.total || 0) / 50)
          if (pages ? page >= pages : (result.items || []).length < 50) break
        }
        providers.value = all
      } catch { if (current(id, expected, session)) providersError.value = true }
    }
  } catch (error) {
    if (current(id, expected, session)) {
      if (isWorkspaceAssuranceRequired(error)) recoveryError.value = error
      else loadError.value = true
    }
  } finally { if (current(id, expected, session)) loading.value = false }
}

function candidate(): WorkspaceSecurityPolicyPatch | null {
  const age = form.session_max_age_seconds === '' ? null : Number(form.session_max_age_seconds)
  if (age !== null && (!Number.isSafeInteger(age) || age < ageMinimum.value || age > ageMaximum.value)) { validationError.value = t('workspace.securitySessionAgeInvalid', { min: ageMinimum.value, max: ageMaximum.value }); return null }
  const grace = form.sso_grace_until ? new Date(form.sso_grace_until) : null
  if (grace && Number.isNaN(grace.getTime())) { validationError.value = t('workspace.securityGraceInvalid'); return null }
  // Preserve server precision when the minute-resolution input is unchanged.
  const graceUntil = form.sso_grace_until === originalGraceInput ? policy.value!.sso_grace_until ?? null : grace?.toISOString() ?? null
  return { expected_revision: policy.value!.revision, require_sso: form.require_sso, require_mfa: form.require_mfa, sso_grace_until: graceUntil, session_max_age_seconds: age, invitation_policy: form.invitation_policy, allow_external_members: form.allow_external_members, workspace_jit_enabled: form.workspace_jit_enabled, approved_identity_provider_mode: form.approved_identity_provider_mode, approved_identity_provider_ids: [...form.approved_identity_provider_ids] }
}
function handleError(error: unknown) {
  if (workspaceSecurityReason(error) === 'WORKSPACE_SECURITY_POLICY_CONFLICT') { conflict.value = true; invalidatePreview() }
  else if (isWorkspaceAssuranceRequired(error)) recoveryError.value = error
  else actionError.value = t(workspaceSecurityMessageKey(error))
}
async function previewPolicy() {
  if (!canUpdate.value || !routeReady.value || previewing.value || saving.value || !dirty.value || conflict.value || loadedSession !== browserSessionIdentity()) return
  validationError.value = ''
  actionError.value = ''
  const payload = candidate()
  if (!payload) { await nextTick(); validationSummary.value?.focus(); return }
  const id = workspaceId.value
  const expected = generation
  const session = browserSessionIdentity()
  const snapshot = signature.value
  const request = ++previewGeneration
  previewController?.abort()
  previewController = new AbortController()
  previewing.value = true
  try {
    const result = await workspaceAPI.previewSecurityPolicy(id, payload, previewController.signal)
    if (!current(id, expected, session) || request !== previewGeneration || snapshot !== signature.value) return
    preview.value = result
    previewSnapshot = snapshot
    recoveryError.value = result.decision && !result.decision.allowed ? { reason: result.decision.reason } : null
  } catch (error) { if (current(id, expected, session) && request === previewGeneration) handleError(error) }
  finally { if (current(id, expected, session) && request === previewGeneration) previewing.value = false }
}
function openConfirmation() {
  if (!canSave.value || loadedSession !== browserSessionIdentity()) return
  const payload = candidate()
  if (payload) pending.value = { workspaceId: workspaceId.value, generation, session: browserSessionIdentity(), snapshot: signature.value, payload }
}
async function savePolicy() {
  const operation = pending.value
  if (!operation || saving.value || !canSave.value || operation.snapshot !== signature.value || !current(operation.workspaceId, operation.generation, operation.session)) { pending.value = null; return }
  saving.value = true
  try {
    const updated = await workspaceAPI.updateSecurityPolicy(operation.workspaceId, operation.payload)
    if (!current(operation.workspaceId, operation.generation, operation.session)) return
    setPolicy(updated)
    invalidatePreview()
    app.showSuccess(t('common.saved'))
  } catch (error) { if (current(operation.workspaceId, operation.generation, operation.session)) { pending.value = null; handleError(error) } }
  finally { if (current(operation.workspaceId, operation.generation, operation.session)) saving.value = false }
}
function afterVerification() {
  if (!policy.value) { void load(); return }
  loadedSession = browserSessionIdentity()
  recoveryError.value = null
  invalidatePreview()
  void previewPolicy()
}
watch(signature, () => { invalidatePreview(); validationError.value = ''; actionError.value = '' }, { flush: 'sync' })
watch([workspaceId, () => route.params.workspaceId, () => store.permissions], () => { ++generation; controller?.abort(); saving.value = false; pending.value = null; void load() }, { flush: 'sync' })
function sessionChanged() { if (loadedSession !== browserSessionIdentity()) { ++generation; controller?.abort(); invalidatePreview(); void load() } }
onMounted(() => { window.addEventListener('storage', sessionChanged); void load() })
onBeforeUnmount(() => { ++generation; invalidatePreview(); controller?.abort(); window.removeEventListener('storage', sessionChanged) })
</script>

<style scoped>
.security-page { display: grid; min-width: 0; gap: 16px; padding-top: 20px; }
.security-panel { display: grid; min-width: 0; gap: 16px; padding: 18px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); box-shadow: var(--shadow-xs); }
.security-heading { display: flex; min-width: 0; justify-content: space-between; align-items: flex-start; gap: 16px; }
.security-heading > div { min-width: 0; }
.security-heading h2, .security-panel h3 { margin: 0; font-size: 18px; color: var(--color-text-primary); }
.security-heading p { max-width: 620px; margin: 6px 0 0; font-size: 14px; color: var(--color-text-secondary); }
.security-heading .security-eyebrow { margin: 0 0 5px; font-size: 12px; color: var(--color-text-muted); }
.security-revision { white-space: nowrap; font-size: 12px; color: var(--color-text-muted); }
.security-form { gap: 24px; }
.security-section { display: grid; gap: 16px; min-width: 0; margin: 0; padding: 0 0 24px; border: 0; border-bottom: 1px solid var(--color-border-subtle); }
.security-section legend { margin-bottom: 16px; font-size: 16px; font-weight: 600; color: var(--color-text-primary); }
.security-section label:not(.security-check) { display: grid; gap: 6px; max-width: 620px; color: var(--color-text-secondary); font-size: 14px; }
.security-fields { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; max-width: 820px; }
.security-check { display: flex; align-items: flex-start; gap: 10px; min-height: 36px; color: var(--color-text-primary); font-size: 14px; }
.security-check input { flex-shrink: 0; margin-top: 4px; accent-color: var(--color-primary); }
.security-check span { min-width: 0; overflow-wrap: anywhere; }
.security-check small, .security-section label small { display: block; max-width: 620px; margin-top: 4px; font-size: 13px; line-height: 1.5; color: var(--color-text-secondary); }
.security-notice, .security-domains, .security-panel > p, .security-confirm-list { margin: 0; max-width: 820px; font-size: 14px; line-height: 1.6; color: var(--color-text-secondary); }
.security-domains ul { display: flex; flex-wrap: wrap; gap: 8px 16px; list-style: none; padding: 0; }
.security-domains code { overflow-wrap: anywhere; }
.security-warning { margin: 0; color: var(--color-warning); font-size: 14px; line-height: 1.6; }
.security-error { color: var(--color-danger); font-size: 14px; line-height: 1.6; }
.security-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.security-provider-list { display: grid; gap: 10px; }
.security-state { padding: 24px 0; color: var(--color-text-muted); }
@media (max-width: 640px) { .security-heading { flex-direction: column; }.security-fields { grid-template-columns: 1fr; }.security-section .input, .security-actions .btn { min-height: 44px; }.security-check { min-height: 44px; } }
</style>
