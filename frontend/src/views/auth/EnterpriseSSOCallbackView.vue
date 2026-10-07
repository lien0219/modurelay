<template>
  <AuthLayout>
    <div class="sso-callback">
      <h2>{{ t('workspace.ssoTitle') }}</h2>
      <template v-if="requiresTotp">
        <p id="sso-totp-description">{{ t('workspace.ssoTotpDescription') }}</p>
        <form class="sso-totp-form" :aria-busy="processing" @submit.prevent="submitTotp">
          <label class="input-label" for="sso-totp-code">{{ t('workspace.ssoTotpCode') }}</label>
          <input
            id="sso-totp-code"
            ref="totpInput"
            v-model="totpCode"
            class="input"
            :class="{ 'input-error': errorMessage }"
            type="text"
            inputmode="numeric"
            autocomplete="one-time-code"
            pattern="[0-9]{6}"
            maxlength="6"
            required
            :disabled="processing"
            :aria-invalid="Boolean(errorMessage)"
            :aria-describedby="errorMessage ? 'sso-totp-description sso-totp-error' : 'sso-totp-description'"
          />
          <p v-if="errorMessage" id="sso-totp-error" class="sso-error" role="alert">{{ errorMessage }}</p>
          <button class="btn btn-primary" type="submit" :disabled="processing">
            <span :role="processing ? 'status' : undefined">{{ processing ? t('common.verifying') : t('workspace.ssoTotpVerify') }}</span>
          </button>
        </form>
        <RouterLink class="btn btn-secondary" :to="retryPath">{{ t('workspace.ssoTryAgain') }}</RouterLink>
      </template>
      <p v-else-if="processing" role="status">{{ t('workspace.ssoCompleting') }}</p>
      <template v-else>
        <p class="sso-error" role="alert">{{ errorMessage }}</p>
        <RouterLink class="btn btn-primary" :to="retryPath">{{ t('workspace.ssoTryAgain') }}</RouterLink>
        <RouterLink class="btn btn-secondary" :to="existingAccountPath">{{ t('workspace.ssoUseExistingAccount') }}</RouterLink>
      </template>
    </div>
  </AuthLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { AuthLayout } from '@/components/layout'
import { useAuthStore, useAppStore } from '@/stores'
import { enterpriseSSOAPI } from '@/api/enterpriseSSO'
import { persistOAuthTokenContext } from '@/api/auth'
import { clearEnterpriseSSONavigation, getEnterpriseSSONavigation, safeEnterpriseReturnTo } from '@/utils/enterpriseSSO'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const app = useAppStore()
const processing = ref(true)
const requiresTotp = ref(false)
const totpCode = ref('')
const totpInput = ref<HTMLInputElement | null>(null)
const errorMessage = ref('')
const needsLink = ref(false)
const navigationHint = getEnterpriseSSONavigation()
let alive = true
const retryPath = computed(() => {
  const query = new URLSearchParams({ error: needsLink.value ? 'account_link_required' : 'provider_failure' })
  const workspaceId = Number(route.query.workspace_id) || navigationHint?.workspace_id || 0
  if (Number.isSafeInteger(workspaceId) && workspaceId > 0) query.set('workspace_id', String(workspaceId))
  if (navigationHint?.workspace_id === workspaceId) query.set('return_to', navigationHint.return_to)
  return `/auth/sso?${query.toString()}`
})
const existingAccountPath = computed(() => `/login?redirect=${encodeURIComponent(retryPath.value)}`)

function errorCode(error: unknown): string | undefined {
  const candidate = error as { code?: string; reason?: string } | null
  return candidate?.reason || candidate?.code
}

function completionError(error: unknown): string {
  const code = errorCode(error)
  needsLink.value = code === 'OIDC_ACCOUNT_LINK_REQUIRED' || code === 'SSO_LINK_REQUIRED'
  return needsLink.value ? t('workspace.ssoLinkRequired') : t('workspace.ssoCompletionFailed')
}

async function focusTotpInput() {
  await nextTick()
  if (alive) totpInput.value?.focus()
}

async function completeSignIn(code?: string) {
  processing.value = true
  errorMessage.value = ''
  try {
    const result = code === undefined ? await enterpriseSSOAPI.exchange() : await enterpriseSSOAPI.exchange(code)
    if (!alive) return
    if (result.requires_2fa === true) {
      requiresTotp.value = true
      processing.value = false
      await focusTotpInput()
      return
    }
    if (!result.access_token || !Number.isSafeInteger(result.workspace_id) || result.workspace_id <= 0) throw new Error('invalid enterprise completion')
    requiresTotp.value = false
    persistOAuthTokenContext(result)
    await auth.setToken(result.access_token)
    if (!alive) return
    clearEnterpriseSSONavigation()
    app.showSuccess(t('workspace.ssoSignInSuccess'))
    await router.replace(safeEnterpriseReturnTo(result.return_to, `/workspaces/${result.workspace_id}/overview`))
  } catch (error) {
    if (!alive) return
    processing.value = false
    const code = errorCode(error)
    if (requiresTotp.value && (code === 'TOTP_INVALID_CODE' || code === 'TOTP_TOO_MANY_ATTEMPTS')) {
      errorMessage.value = t(code === 'TOTP_INVALID_CODE' ? 'workspace.ssoTotpInvalidCode' : 'workspace.ssoTotpTooManyAttempts')
      await focusTotpInput()
    } else {
      requiresTotp.value = false
      errorMessage.value = completionError(error)
    }
  }
}

async function submitTotp() {
  if (processing.value || !requiresTotp.value) return
  if (!/^[0-9]{6}$/.test(totpCode.value)) {
    errorMessage.value = t('workspace.ssoTotpInvalidCode')
    await focusTotpInput()
    return
  }
  const code = totpCode.value
  totpCode.value = ''
  await completeSignIn(code)
}

onMounted(async () => {
  if (route.query.error) {
    errorMessage.value = completionError({ code: String(route.query.error) })
    processing.value = false
    return
  }
  await completeSignIn()
})
onBeforeUnmount(() => { alive = false; totpCode.value = '' })
</script>

<style scoped>
.sso-callback { display: grid; min-width: 0; gap: 16px; text-align: center; }
.sso-callback h2 { margin: 0; color: var(--color-text-primary); font-size: 24px; font-weight: 700; }
.sso-callback p { margin: 0; color: var(--color-text-secondary); font-size: 14px; line-height: 1.5; }
.sso-callback .sso-error { color: var(--color-danger); }
.sso-totp-form { display: grid; min-width: 0; gap: 12px; text-align: left; }
.sso-totp-form .input-label { margin: 0; }
.sso-totp-form .input, .sso-totp-form .btn { min-height: 44px; }
</style>
