<template>
  <section class="access-recovery" role="status" aria-live="polite">
    <h3>{{ t('workspace.securityRecoveryTitle') }}</h3>
    <p>{{ t(workspaceSecurityMessageKey(error)) }}</p>
    <div class="access-recovery__actions">
      <button v-if="needsMFA && status?.feature_enabled && !status.enabled" type="button" class="btn btn-primary" data-testid="security-enroll-mfa" @click="setupOpen = true">{{ t('workspace.securityEnrollMFA') }}</button>
      <button v-else-if="needsMFA && status?.feature_enabled && status.enabled" type="button" class="btn btn-primary" data-testid="security-verify-mfa" @click="dialogOpen = true">{{ t('workspace.securityVerifyMFA') }}</button>
      <RouterLink v-if="!needsMFA || !status?.feature_enabled" class="btn btn-primary" :to="recoveryPath">{{ t('workspace.securitySignInAgain') }}</RouterLink>
      <RouterLink class="btn btn-secondary" to="/profile">{{ t('workspace.securityProfile') }}</RouterLink>
    </div>
    <p v-if="needsMFA" class="access-recovery__hint">{{ t('workspace.securityEnrollmentHint') }}</p>
    <p v-if="statusError" class="access-recovery__error" role="alert">{{ t('workspace.securityFactorUnavailable') }}</p>
    <TotpSetupModal v-if="setupOpen" @close="setupOpen = false" @success="afterEnrollment" />
    <BaseDialog :show="dialogOpen" :title="t('workspace.securityVerifyMFA')" width="narrow" :close-on-escape="!verifying" :show-close-button="!verifying" @close="closeDialog">
      <form data-testid="security-mfa-form" :aria-busy="verifying" @submit.prevent="verify">
        <label class="input-label" :for="codeID">{{ t('workspace.ssoTotpCode') }}</label>
        <input :id="codeID" ref="codeInput" v-model="code" name="workspace-mfa-code" class="input" type="text" inputmode="numeric" autocomplete="one-time-code" pattern="[0-9]{6}" maxlength="6" required :disabled="verifying" :aria-invalid="Boolean(verificationError)" :aria-describedby="verificationError ? `${codeID}-error` : undefined">
        <p v-if="verificationError" :id="`${codeID}-error`" class="access-recovery__error" role="alert" data-testid="security-mfa-error">{{ verificationError }}</p>
        <div class="access-recovery__actions"><button type="submit" class="btn btn-primary" :disabled="verifying || !/^[0-9]{6}$/.test(code)">{{ verifying ? t('common.verifying') : t('workspace.securityVerifyMFA') }}</button><button type="button" class="btn btn-secondary" :disabled="verifying" @click="closeDialog">{{ t('common.cancel') }}</button></div>
      </form>
    </BaseDialog>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TotpSetupModal from '@/components/user/profile/TotpSetupModal.vue'
import { totpAPI } from '@/api/totp'
import { useAuthStore } from '@/stores/auth'
import type { TotpStatus } from '@/types'
import { browserSessionIdentity, workspaceRecoveryPath, workspaceSecurityMessageKey, workspaceSecurityReason } from '@/utils/workspaceSecurity'

const props = withDefaults(defineProps<{ workspaceId: number; error: unknown; returnTo: string; requiresSSO?: boolean }>(), { requiresSSO: false })
const emit = defineEmits<{ verified: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const reason = computed(() => workspaceSecurityReason(props.error))
const needsMFA = computed(() => ['MFA_REQUIRED', 'MFA_ENROLLMENT_REQUIRED', 'STEP_UP_REQUIRED', 'STEP_UP_TOTP_NOT_ENABLED'].includes(reason.value))
const recoveryPath = computed(() => {
  const metadataSSO = (props.error as { metadata?: { requires_sso?: unknown } } | null)?.metadata?.requires_sso
  return workspaceRecoveryPath(props.workspaceId, reason.value, props.returnTo, props.requiresSSO || metadataSSO === true || metadataSSO === 'true')
})
const codeID = computed(() => `workspace-mfa-${props.workspaceId}`)
const status = ref<TotpStatus | null>(null)
const statusError = ref(false)
const setupOpen = ref(false)
const dialogOpen = ref(false)
const verifying = ref(false)
const code = ref('')
const codeInput = ref<HTMLInputElement | null>(null)
const verificationError = ref('')
let generation = 0
let alive = true
let currentSession = browserSessionIdentity()

async function loadStatus() {
  const expected = generation
  const session = browserSessionIdentity()
  statusError.value = false
  try { const result = await totpAPI.getStatus(); if (alive && expected === generation && session === browserSessionIdentity()) status.value = result }
  catch { if (alive && expected === generation) statusError.value = true }
}
async function afterEnrollment() { setupOpen.value = false; await loadStatus() }
function closeDialog() { if (verifying.value) return; dialogOpen.value = false; code.value = ''; verificationError.value = '' }
async function verify() {
  if (verifying.value || !dialogOpen.value || !/^[0-9]{6}$/.test(code.value) || currentSession !== browserSessionIdentity()) return
  const expected = generation
  const userID = auth.user?.id
  verifying.value = true
  verificationError.value = ''
  const proof = code.value
  code.value = ''
  try {
    const tokens = await totpAPI.upgradeSessionMFA(proof)
    if (!alive || expected !== generation || userID !== auth.user?.id) return
    auth.adoptStoredSessionTokens(tokens)
    currentSession = browserSessionIdentity()
    dialogOpen.value = false
    emit('verified')
  } catch (error) {
    if (alive && expected === generation) { verificationError.value = t(workspaceSecurityMessageKey(error)); await nextTick(); codeInput.value?.focus() }
  } finally { if (alive && expected === generation) verifying.value = false }
}
watch(dialogOpen, async open => { if (open) { verificationError.value = ''; code.value = ''; await nextTick(); codeInput.value?.focus() } })
watch([() => props.workspaceId, reason, () => props.returnTo], () => { ++generation; currentSession = browserSessionIdentity(); setupOpen.value = false; dialogOpen.value = false; verifying.value = false; code.value = ''; status.value = null; void loadStatus() })
onMounted(loadStatus)
onBeforeUnmount(() => { alive = false; ++generation; code.value = '' })
</script>

<style scoped>
.access-recovery { display: grid; gap: 12px; min-width: 0; padding: 18px; border: 1px solid var(--color-warning); border-radius: 12px; background: var(--color-surface); }
.access-recovery h3 { margin: 0; color: var(--color-text-primary); font-size: 16px; }
.access-recovery p { margin: 0; max-width: 620px; color: var(--color-text-secondary); font-size: 14px; line-height: 1.6; }
.access-recovery__actions { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 12px; }
.access-recovery .access-recovery__error { color: var(--color-danger); }
.access-recovery__hint { font-size: 13px; }
@media (max-width: 640px) { .access-recovery__actions .btn { min-height: 44px; } }
</style>
