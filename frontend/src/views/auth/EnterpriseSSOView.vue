<template>
  <AuthLayout>
    <div class="sso-page">
      <div class="sso-heading"><h2>{{ t('workspace.ssoTitle') }}</h2><p>{{ required ? t('workspace.ssoRequiredDescription') : t('workspace.ssoDescription') }}</p></div>
      <p v-if="route.query.error === 'account_link_required'" class="sso-notice" role="status">{{ t('workspace.ssoLinkRequired') }}</p>
      <form class="sso-discovery-form" data-testid="sso-discovery-form" @submit.prevent="discover">
        <label for="sso-work-email">{{ t('workspace.ssoWorkEmail') }}</label>
        <input id="sso-work-email" v-model="email" class="input" type="email" autocomplete="email" required maxlength="254" :disabled="discovering || starting || recovering">
        <button type="submit" class="btn btn-primary" :disabled="discovering || starting || recovering || !email.trim()">{{ discovering ? t('common.loading') : t('workspace.ssoFindProvider') }}</button>
      </form>
      <p v-if="errorMessage" class="sso-error" role="alert">{{ errorMessage }}</p>
      <p v-if="discovered && !providers.length" class="sso-notice" role="status">{{ t('workspace.ssoNoProvider') }}</p>
      <div v-if="providers.length" class="sso-providers">
        <p v-if="auth.isAuthenticated" class="sso-notice">{{ t('workspace.ssoLinkDescription', { email: auth.user?.email || '' }) }}</p>
        <article v-for="provider in providers" :key="`${provider.workspace_id}-${provider.provider_id}`" class="sso-provider">
          <strong>{{ provider.name }}</strong>
          <button type="button" class="btn btn-primary" :disabled="starting" @click="start(provider, false)">{{ starting ? t('common.processing') : t('workspace.ssoContinue') }}</button>
          <button v-if="auth.isAuthenticated" type="button" class="btn btn-secondary" data-testid="sso-prepare-link" :disabled="starting || recovering" @click="prepareLink(provider)">{{ t('workspace.ssoLinkCurrentAccount') }}</button>
        </article>
      </div>

      <form v-if="linkProvider" class="sso-recovery-form" data-testid="sso-link-form" @submit.prevent="start(linkProvider, true)">
        <p class="sso-notice">{{ t('workspace.ssoLinkProofDescription', { name: linkProvider.name }) }}</p>
        <label for="sso-link-password">{{ t('workspace.ssoRecoveryPassword') }}</label><input id="sso-link-password" v-model="linkProof.password" class="input" type="password" autocomplete="current-password" required :disabled="starting">
        <template v-if="totpEnabled !== false"><label for="sso-link-totp">{{ t('workspace.ssoRecoveryTotp') }}</label><input id="sso-link-totp" v-model="linkProof.totp_code" class="input" type="text" inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern="[0-9]{6}" :required="totpEnabled === true" :disabled="starting"></template>
        <button type="submit" class="btn btn-primary" :disabled="starting">{{ starting ? t('common.processing') : t('workspace.ssoLinkCurrentAccount') }}</button>
        <button type="button" class="btn btn-secondary" :disabled="starting" @click="closeLink">{{ t('common.cancel') }}</button>
      </form>

      <section v-if="canRecover" class="sso-recovery">
        <button v-if="!recoveryOpen" type="button" class="btn btn-secondary" data-testid="sso-recovery-open" @click="recoveryOpen = true">{{ t('workspace.ssoRecoverAccess') }}</button>
        <form v-else class="sso-recovery-form" data-testid="sso-recovery-form" @submit.prevent="recover">
          <div><h3>{{ t('workspace.ssoRecoverAccess') }}</h3><p>{{ t('workspace.ssoRecoveryDescription') }}</p></div>
          <label for="sso-recovery-password">{{ t('workspace.ssoRecoveryPassword') }}</label><input id="sso-recovery-password" v-model="recovery.password" class="input" type="password" autocomplete="current-password" required :disabled="recovering">
          <template v-if="totpEnabled !== false"><label for="sso-recovery-totp">{{ t('workspace.ssoRecoveryTotp') }}</label><input id="sso-recovery-totp" v-model="recovery.totp_code" class="input" type="text" inputmode="numeric" autocomplete="one-time-code" maxlength="6" pattern="[0-9]{6}" :required="totpEnabled === true" :disabled="recovering"></template>
          <label for="sso-recovery-reason">{{ t('workspace.ssoRecoveryReason') }}</label><textarea id="sso-recovery-reason" v-model="recovery.reason" class="input" rows="3" minlength="10" maxlength="500" required :disabled="recovering"></textarea>
          <label class="sso-check"><input v-model="recoveryConfirmed" type="checkbox" required :disabled="recovering"><span>{{ t('workspace.ssoRecoveryConfirm') }}</span></label>
          <button type="submit" class="btn btn-primary" :disabled="recovering || !recoveryConfirmed">{{ recovering ? t('common.processing') : t('workspace.ssoRecoverAction') }}</button>
          <button type="button" class="btn btn-secondary" :disabled="recovering" @click="closeRecovery">{{ t('common.cancel') }}</button>
        </form>
      </section>
      <RouterLink class="sso-back" to="/login">{{ t('workspace.ssoBackToLogin') }}</RouterLink>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import { useAuthStore, useAppStore } from '@/stores'
import { useWorkspaceStore } from '@/stores/workspace'
import { enterpriseSSOAPI, type EnterpriseSSOProvider } from '@/api/enterpriseSSO'
import { workspaceAPI } from '@/api/workspace'
import { totpAPI } from '@/api/totp'
import { rememberEnterpriseSSONavigation, safeEnterpriseReturnTo } from '@/utils/enterpriseSSO'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const app = useAppStore()
const workspaces = useWorkspaceStore()
const workspaceId = computed(() => { const id = Number(route.query.workspace_id); return Number.isSafeInteger(id) && id > 0 ? id : 0 })
const required = computed(() => route.query.required === '1')
const failureContext = computed(() => required.value || route.query.error === 'provider_failure')
const canRecover = computed(() => failureContext.value && auth.isAuthenticated && workspaceId.value > 0 && Boolean(workspaces.workspaces.find(workspace => workspace.id === workspaceId.value && workspace.type === 'organization' && workspace.permissions?.includes('owner.manage'))))
const email = ref(auth.user?.email || '')
const providers = ref<EnterpriseSSOProvider[]>([])
const discovered = ref(false)
const discovering = ref(false)
const starting = ref(false)
const recovering = ref(false)
const errorMessage = ref('')
const recoveryOpen = ref(false)
const recoveryConfirmed = ref(false)
const recovery = reactive({ password: '', totp_code: '', reason: '' })
const linkProvider = ref<EnterpriseSSOProvider | null>(null)
const linkProof = reactive({ password: '', totp_code: '' })
const totpEnabled = ref<boolean | null>(null)
let generation = 0
let controller: AbortController | null = null
let alive = true

function closeRecovery(): void { recoveryOpen.value = false; recoveryConfirmed.value = false; Object.assign(recovery, { password: '', totp_code: '', reason: '' }) }
function closeLink(): void { linkProvider.value = null; Object.assign(linkProof, { password: '', totp_code: '' }) }
function prepareLink(provider: EnterpriseSSOProvider): void { closeLink(); linkProvider.value = provider }

async function discover(): Promise<void> {
  if (discovering.value || starting.value || recovering.value || !email.value.trim()) return
  const current = ++generation
  controller?.abort()
  controller = new AbortController()
  discovering.value = true
  discovered.value = false
  errorMessage.value = ''
  providers.value = []
  closeLink()
  try {
    const result = await enterpriseSSOAPI.discover(email.value.trim(), controller.signal)
    if (current !== generation) return
    providers.value = (result.providers || []).filter(provider => !workspaceId.value || provider.workspace_id === workspaceId.value)
    discovered.value = true
  } catch {
    if (current === generation) errorMessage.value = t('workspace.ssoDiscoveryError')
  } finally { if (current === generation) discovering.value = false }
}

async function start(provider: EnterpriseSSOProvider, link: boolean): Promise<void> {
  if (starting.value || recovering.value || !alive || (link && (!auth.isAuthenticated || !linkProof.password || (totpEnabled.value === true && !/^\d{6}$/.test(linkProof.totp_code))))) return
  const current = generation
  starting.value = true
  errorMessage.value = ''
  const returnTo = safeEnterpriseReturnTo(route.query.return_to, `/workspaces/${provider.workspace_id}/overview`)
  try {
    const result = link ? await enterpriseSSOAPI.startLink(provider.workspace_id, provider.provider_id, returnTo, { password: linkProof.password, totp_code: linkProof.totp_code }) : await workspaceAPI.startSSO(provider.workspace_id, provider.provider_id, returnTo)
    if (alive && current === generation) { rememberEnterpriseSSONavigation(provider.workspace_id, provider.provider_id, returnTo); window.location.assign(result.authorization_url) }
  } catch (error) {
    if (alive && current === generation) errorMessage.value = (error as { message?: string })?.message || t('workspace.identitySSOStartError')
  } finally {
    if (alive && current === generation) { starting.value = false; Object.assign(linkProof, { password: '', totp_code: '' }) }
  }
}

async function recover(): Promise<void> {
  if (recovering.value || starting.value || !canRecover.value || !recoveryConfirmed.value || !recovery.password || (totpEnabled.value === true && !/^\d{6}$/.test(recovery.totp_code)) || recovery.reason.trim().length < 10) return
  const current = generation
  const id = workspaceId.value
  recovering.value = true
  errorMessage.value = ''
  try {
    await enterpriseSSOAPI.recover(id, { password: recovery.password, totp_code: recovery.totp_code, reason: recovery.reason.trim(), confirmed: true })
    if (!alive || current !== generation) return
    closeRecovery()
    app.showSuccess(t('workspace.ssoRecoverySuccess'))
    await router.replace(`/workspaces/${id}/identity`)
  } catch (error) {
    if (alive && current === generation) errorMessage.value = (error as { message?: string })?.message || t('workspace.ssoRecoveryFailed')
  } finally { if (alive && current === generation) { recovering.value = false; recovery.password = ''; recovery.totp_code = '' } }
}

async function loadLinkedProvider(): Promise<void> {
  const id = workspaceId.value
  const providerId = Number(route.query.provider_id)
  const current = generation
  if (!auth.isAuthenticated || route.query.link !== '1' || !id || !Number.isSafeInteger(providerId) || providerId <= 0) return
  try {
    const provider = await workspaceAPI.getIdentityProvider(id, providerId)
    if (!alive || current !== generation) return
    if (provider.workspace_id !== id || provider.status !== 'active') return
    providers.value = [{ workspace_id: id, provider_id: provider.id, name: provider.name, is_default: provider.is_default }]
    prepareLink(providers.value[0])
  } catch { if (alive && current === generation) errorMessage.value = t('workspace.ssoDiscoveryError') }
}

async function loadTotpStatus(): Promise<void> {
  if (!auth.isAuthenticated) return
  const actorId = auth.user?.id
  try { const result = await totpAPI.getStatus(); if (alive && actorId === auth.user?.id) totpEnabled.value = result.enabled }
  catch { /* The server verifies required MFA even if status is unavailable. */ }
}

onMounted(async () => {
  if (auth.isAuthenticated) {
    await Promise.all([
      workspaces.workspaces.length ? Promise.resolve() : workspaces.loadWorkspaces(),
      loadTotpStatus(),
      loadLinkedProvider(),
    ])
  }
})
watch([() => route.query, () => auth.user?.id], ([, actorId], [, previousActorId]) => {
  ++generation
  controller?.abort()
  providers.value = []
  discovered.value = false
  discovering.value = false
  starting.value = false
  recovering.value = false
  errorMessage.value = ''
  closeLink()
  closeRecovery()
  if (actorId !== previousActorId) { totpEnabled.value = null; void loadTotpStatus() }
  void loadLinkedProvider()
})
watch(canRecover, allowed => { if (!allowed) closeRecovery() })
onBeforeUnmount(() => { alive = false; ++generation; controller?.abort(); closeRecovery(); closeLink() })
</script>

<style scoped>
.sso-page { display: grid; min-width: 0; gap: 20px; }
.sso-heading h2 { margin: 0; color: var(--color-text-primary); font-size: 24px; font-weight: 700; }
.sso-heading p, .sso-notice { margin: 6px 0 0; color: var(--color-text-secondary); font-size: 14px; line-height: 1.5; }
.sso-discovery-form, .sso-recovery-form { display: grid; min-width: 0; gap: 8px; }
.sso-discovery-form > label, .sso-recovery-form > label:not(.sso-check) { color: var(--color-text-secondary); font-size: 13px; font-weight: 600; }
.sso-discovery-form .input, .sso-recovery-form .input { width: 100%; min-width: 0; min-height: 42px; box-sizing: border-box; }
.sso-discovery-form > button { margin-top: 8px; }
.sso-error { margin: 0; color: var(--color-danger); font-size: 13px; line-height: 1.5; }
.sso-providers { display: grid; min-width: 0; gap: 14px; }
.sso-provider { display: grid; min-width: 0; gap: 10px; border: 1px solid var(--color-border); border-radius: 10px; background: var(--color-surface); padding: 14px; }
.sso-provider strong { color: var(--color-text-primary); overflow-wrap: anywhere; }
.sso-recovery { border-top: 1px solid var(--color-border); padding-top: 18px; }
.sso-recovery > button { width: 100%; }
.sso-recovery-form h3 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.sso-recovery-form p { margin: 6px 0 10px; color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
.sso-check { display: flex; align-items: flex-start; gap: 10px; margin: 8px 0; color: var(--color-text-secondary); font-size: 13px; }
.sso-check input { width: 18px; height: 18px; flex: 0 0 auto; margin-top: 2px; accent-color: var(--color-primary); }
.sso-back { color: var(--color-primary); font-size: 13px; text-align: center; text-decoration: underline; }
</style>
