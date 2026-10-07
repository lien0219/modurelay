<template>
  <WorkspaceFrame section="identity">
    <div class="identity-page">
      <section class="identity-panel">
        <div class="identity-heading">
          <div>
            <p class="workspace-eyebrow">{{ t('workspace.identityEyebrow') }}</p>
            <h2>{{ t('workspace.identityTitle') }}</h2>
            <p>{{ t('workspace.identityDescription') }}</p>
          </div>
        </div>
        <div v-if="!canRead" class="workspace-state">{{ isOrganization ? t('workspace.identityUnavailable') : t('workspace.identityPersonalUnsupported') }}</div>
        <div v-else-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="loadError" class="identity-error" role="alert">
          <span>{{ t('workspace.identityLoadError') }}</span>
          <button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('common.retry') }}</button>
        </div>
      </section>

      <template v-if="canRead && !loading && !loadError">
        <section class="identity-panel">
          <div class="identity-heading">
            <div><h2>{{ t('workspace.identityDomains') }}</h2><p>{{ t('workspace.identityDomainsDescription') }}</p></div>
            <form v-if="store.can('identity.manage')" class="identity-inline-form" data-testid="identity-domain-form" @submit.prevent="addDomain">
              <label class="sr-only" for="identity-domain">{{ t('workspace.identityDomain') }}</label>
              <input id="identity-domain" v-model="domainInput" class="input" type="text" autocomplete="url" required maxlength="255" :placeholder="t('workspace.identityDomainPlaceholder')" :disabled="domainSaving">
              <button type="submit" class="btn btn-primary btn-sm" :disabled="domainSaving || !domainInput.trim()">{{ domainSaving ? t('common.saving') : t('workspace.identityAddDomain') }}</button>
            </form>
          </div>

          <div v-if="domains.length" class="identity-list">
            <article v-for="domain in domains" :key="domain.id" class="identity-row">
              <div class="identity-row__main">
                <strong>{{ domain.normalized_domain || domain.domain }}</strong>
                <span class="identity-status" :class="`is-${domain.status}`">{{ domainStatusLabel(domain.status) }}</span>
                <small v-if="domain.last_error_code">{{ domain.last_error_code }}</small>
              </div>
              <div class="identity-row__actions">
                <button v-if="store.can('identity.manage') && domain.status !== 'revoked'" type="button" class="btn btn-secondary btn-sm" :data-testid="`identity-verify-domain-${domain.id}`" :disabled="domainActionId !== null" @click="verifyDomain(domain)">{{ domainActionId === domain.id ? t('common.processing') : t('workspace.identityVerifyDomain') }}</button>
                <button v-if="store.can('identity.manage') && !['verified', 'revoked'].includes(domain.status)" type="button" class="btn btn-ghost btn-sm" :data-testid="`identity-regenerate-domain-${domain.id}`" :disabled="domainActionId !== null" @click="regenerateDomainToken(domain)">{{ t('workspace.identityRegenerateToken') }}</button>
                <button v-if="store.can('identity.manage') && domain.status !== 'revoked'" type="button" class="btn btn-ghost btn-sm identity-danger" :data-testid="`identity-revoke-domain-${domain.id}`" :disabled="domainActionId !== null" @click="revokeDomain(domain)">{{ t('workspace.identityRevokeDomain') }}</button>
              </div>
            </article>
          </div>
          <div v-else class="workspace-state">{{ t('workspace.identityNoDomains') }}</div>
          <Pagination v-if="domainTotal > 50" :total="domainTotal" :page="domainPage" :page-size="50" :show-page-size-selector="false" @update:page="changePage('domains', $event)" />

          <div v-if="verificationResult" class="identity-verification" role="status" aria-live="polite">
            <div><strong>{{ t('workspace.identityDnsInstructions') }}</strong><p>{{ t('workspace.identityDnsInstructionsDescription') }}</p></div>
            <dl><div><dt>{{ t('workspace.identityDnsHost') }}</dt><dd><code>{{ verificationHost }}</code><button type="button" class="btn btn-ghost btn-sm" @click="copyVerification(verificationHost)">{{ t('workspace.identityCopyHost') }}</button></dd></div><div><dt>{{ t('workspace.identityDnsValue') }}</dt><dd><code>{{ verificationResult.verification_txt }}</code><button type="button" class="btn btn-ghost btn-sm" @click="copyVerification(verificationResult.verification_txt)">{{ t('workspace.identityCopyValue') }}</button></dd></div></dl>
          </div>
        </section>

        <section class="identity-panel">
          <div class="identity-heading">
            <div><h2>{{ t('workspace.identityProviders') }}</h2><p>{{ t('workspace.identityProvidersDescription') }}</p></div>
            <button v-if="store.can('identity.manage') && !providerFormOpen" type="button" class="btn btn-primary btn-sm" data-testid="identity-add-provider" @click="openCreateProvider">{{ t('workspace.identityAddProvider') }}</button>
          </div>

          <div v-if="providers.length" class="identity-list">
            <article v-for="provider in providers" :key="provider.id" class="identity-row identity-row--provider">
              <div class="identity-row__main">
                <div class="identity-provider-title"><strong>{{ provider.name }}</strong><span v-if="provider.is_default" class="identity-default">{{ t('workspace.identityDefault') }}</span><span class="identity-status" :class="`is-${provider.status}`">{{ providerStatusLabel(provider.status) }}</span></div>
                <small>{{ provider.provider_key }} · {{ provider.client_id }}</small>
                <code class="identity-issuer">{{ provider.issuer_url }}</code>
                <small :data-testid="`identity-validation-${provider.id}`" class="identity-provider-validation">
                  <span>{{ t('workspace.identityLastValidation') }}: {{ validationLabel(provider.last_validation_code) }}</span>
                  <time v-if="provider.last_validated_at" :datetime="provider.last_validated_at">{{ formatDateTime(provider.last_validated_at) }}</time>
                </small>
              </div>
              <div class="identity-row__actions">
                <button v-if="provider.status === 'active'" type="button" class="btn btn-secondary btn-sm" :disabled="providerActionId === provider.id" @click="testSSO(provider)">{{ providerActionId === provider.id ? t('common.processing') : t('workspace.identityTestSignIn') }}</button>
                <RouterLink v-if="provider.status === 'active'" class="btn btn-secondary btn-sm" :data-testid="`identity-link-provider-${provider.id}`" :to="linkProviderPath(provider)">{{ t('workspace.identityLinkAccount') }}</RouterLink>
                <button v-if="store.can('identity.manage')" type="button" class="btn btn-ghost btn-sm" :data-testid="`identity-edit-provider-${provider.id}`" :disabled="providerSaving" @click="editProvider(provider)">{{ t('common.edit') }}</button>
                <button type="button" class="btn btn-ghost btn-sm" @click="selectedMappingProvider = provider">{{ t('workspace.identityMappings') }}</button>
                <button v-if="store.can('identity.manage') && provider.status === 'active'" type="button" class="btn btn-ghost btn-sm identity-danger" :disabled="providerActionId === provider.id" @click="disableProvider(provider)">{{ t('workspace.identityDisableProvider') }}</button>
              </div>
            </article>
          </div>
          <div v-else class="workspace-state">{{ t('workspace.identityNoProviders') }}</div>
          <Pagination v-if="providerTotal > 50" :total="providerTotal" :page="providerPage" :page-size="50" :show-page-size-selector="false" @update:page="changePage('providers', $event)" />

          <form v-if="providerFormOpen" class="identity-provider-form" data-testid="identity-provider-form" @submit.prevent="saveProvider">
            <div class="identity-heading"><div><h3>{{ editingProvider ? t('workspace.identityEditProvider') : t('workspace.identityAddProvider') }}</h3><p>{{ t('workspace.identityProviderFormDescription') }}</p></div><button type="button" class="btn btn-ghost btn-sm" :disabled="providerSaving" @click="closeProviderForm">{{ t('common.cancel') }}</button></div>
            <div class="identity-fields">
              <label class="identity-field--wide"><span>{{ t('workspace.identityPreset') }}</span><select v-model="providerPreset" class="input" name="preset" @change="applyPreset"><option value="generic">{{ t('workspace.identityPresets.generic') }}</option><option value="entra">{{ t('workspace.identityPresets.entra') }}</option><option value="google">{{ t('workspace.identityPresets.google') }}</option><option value="okta">{{ t('workspace.identityPresets.okta') }}</option></select><small>{{ t('workspace.identityPresetHint') }}</small></label>
              <label><span>{{ t('workspace.identityProviderName') }}</span><input v-model="providerForm.name" class="input" name="name" required maxlength="120"></label>
              <label><span>{{ t('workspace.identityProviderKey') }}</span><input v-model="providerForm.provider_key" class="input" name="provider_key" required maxlength="80" pattern="[a-z0-9][a-z0-9_-]*"></label>
              <label class="identity-field--wide"><span>{{ t('workspace.identityIssuerUrl') }}</span><input v-model="providerForm.issuer_url" class="input" name="issuer_url" type="url" autocomplete="url" required maxlength="2048" placeholder="https://id.example.com"></label>
              <label><span>{{ t('workspace.identityClientId') }}</span><input v-model="providerForm.client_id" class="input" name="client_id" required maxlength="512" autocomplete="off"></label>
              <label><span>{{ t('workspace.identityTokenAuthMethod') }}</span><select v-model="providerForm.token_auth_method" class="input" name="token_auth_method" :disabled="providerForm.secret_action === 'remove'" :aria-invalid="Boolean(providerSecretError)" :aria-describedby="providerSecretError ? 'identity-secret-consistency' : undefined"><option value="client_secret_basic">{{ t('workspace.identityTokenAuthMethods.basic') }}</option><option value="client_secret_post">{{ t('workspace.identityTokenAuthMethods.post') }}</option><option value="none">{{ t('workspace.identityTokenAuthMethods.none') }}</option></select></label>
              <label v-if="editingProvider"><span>{{ t('workspace.identitySecretAction') }}</span><select v-model="providerForm.secret_action" class="input" name="secret_action" :aria-invalid="Boolean(providerSecretError)" :aria-describedby="providerSecretError ? 'identity-secret-consistency' : undefined"><option value="preserve">{{ t('workspace.identitySecretActions.preserve') }}</option><option value="replace">{{ t('workspace.identitySecretActions.replace') }}</option><option value="remove">{{ t('workspace.identitySecretActions.remove') }}</option></select></label>
              <label v-if="providerForm.token_auth_method !== 'none' && (!editingProvider || providerForm.secret_action === 'replace')"><span>{{ t('workspace.identityClientSecret') }}</span><input v-model="providerForm.client_secret" class="input" name="client_secret" type="password" autocomplete="new-password" required :placeholder="t('workspace.identitySecretRequired')"></label>
              <p v-if="providerSecretError" id="identity-secret-consistency" class="identity-field--wide identity-error" role="alert">{{ providerSecretError }}</p>
              <label class="identity-field--wide"><span>{{ t('workspace.identityScopes') }}</span><input v-model="providerForm.scopes" class="input" name="scopes" :placeholder="t('workspace.identityScopesPlaceholder')"></label>
              <label><span>{{ t('workspace.identityEmailClaim') }}</span><input v-model="providerForm.email_claim" class="input" name="email_claim" required maxlength="200"></label>
              <label><span>{{ t('workspace.identityNameClaim') }}</span><input v-model="providerForm.name_claim" class="input" name="name_claim" maxlength="200"></label>
              <label><span>{{ t('workspace.identityGroupsClaim') }}</span><input v-model="providerForm.groups_claim" class="input" name="groups_claim" maxlength="200"></label>
            </div>
            <fieldset class="identity-options">
              <legend>{{ t('workspace.identityProvisioning') }}</legend>
              <label class="identity-check"><input v-model="providerForm.jit_enabled" name="jit_enabled" type="checkbox"><span>{{ t('workspace.identityJitEnabled') }}</span></label>
              <p class="identity-provision-warning">{{ t('workspace.identityJitWarning') }}</p>
              <label><span>{{ t('workspace.identityDefaultRole') }}</span><select v-model="providerForm.default_role" name="default_role" class="input"><option v-for="role in managedRoles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select></label>
              <label class="identity-field--wide"><span>{{ t('workspace.identityAllowedDomains') }}</span><input v-model="providerForm.allowed_domains" name="allowed_domains" class="input" :placeholder="t('workspace.identityAllowedDomainsPlaceholder')"><small>{{ t('workspace.identityAllowedDomainsHint') }}</small></label>
            </fieldset>
            <div class="identity-options identity-options--checks">
              <label class="identity-check"><input v-model="providerForm.is_default" type="checkbox"><span>{{ t('workspace.identityMakeDefault') }}</span></label>
              <label class="identity-check"><input v-model="providerForm.discovery_enabled" name="discovery_enabled" type="checkbox"><span>{{ t('workspace.identityDiscoveryEnabled') }}</span></label>
            </div>
            <fieldset v-if="!providerForm.discovery_enabled" class="identity-options">
              <legend>{{ t('workspace.identityManualEndpoints') }}</legend>
              <label class="identity-field--wide"><span>{{ t('workspace.identityAuthorizationEndpoint') }}</span><input v-model="providerForm.authorization_endpoint" class="input" name="authorization_endpoint" type="url" required maxlength="2048"></label>
              <label class="identity-field--wide"><span>{{ t('workspace.identityTokenEndpoint') }}</span><input v-model="providerForm.token_endpoint" class="input" name="token_endpoint" type="url" required maxlength="2048"></label>
              <label class="identity-field--wide"><span>{{ t('workspace.identityJwksUri') }}</span><input v-model="providerForm.jwks_uri" class="input" name="jwks_uri" type="url" required maxlength="2048"></label>
              <label class="identity-field--wide"><span>{{ t('workspace.identityUserinfoEndpoint') }}</span><input v-model="providerForm.userinfo_endpoint" class="input" name="userinfo_endpoint" type="url" maxlength="2048"></label>
            </fieldset>
            <div class="identity-actions"><button type="submit" class="btn btn-primary" :disabled="providerSaving">{{ providerSaving ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-secondary" :disabled="providerSaving" @click="closeProviderForm">{{ t('common.cancel') }}</button></div>
          </form>
        </section>

        <WorkspaceIdentityMappings v-if="selectedMappingProvider" :workspace-id="workspaceId" :provider="selectedMappingProvider" @close="selectedMappingProvider = null" />

        <section class="identity-panel">
          <div class="identity-heading"><div><h2>{{ t('workspace.identitySecurityPolicy') }}</h2><p>{{ t('workspace.identitySecurityPolicyDescription') }}</p></div><span v-if="policy" class="identity-revision">{{ t('workspace.policyRevision', { revision: policy.revision }) }}</span></div>
          <div v-if="policyLoading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
          <div v-else-if="policy" class="identity-policy">
            <label class="identity-check identity-check--policy"><input v-model="policyForm.require_sso" name="require_sso" type="checkbox" :disabled="!store.can('workspace_sso.update') || (!policy.require_sso && !providerTotal)"><span><strong>{{ t('workspace.identityRequireSSO') }}</strong><small>{{ t('workspace.identityRequireSSOHint') }}</small></span></label>
            <p class="identity-provision-warning">{{ t('workspace.identityEnforcementPrerequisites') }}</p>
            <p v-if="!providerTotal" class="identity-provision-warning" role="status">{{ t('workspace.identityEnforcementNoProvider') }}</p>
            <label v-if="store.can('workspace_sso.update')" class="identity-field--grace"><span>{{ t('workspace.identityGraceUntil') }}</span><input v-model="policyForm.sso_grace_until" class="input" type="datetime-local"><small>{{ t('workspace.identityGraceHint') }}</small></label>
            <div v-if="store.can('workspace_sso.update')" class="identity-actions"><button type="button" class="btn btn-primary" data-testid="identity-save-policy" :disabled="policySaving || !policyDirty" @click="savePolicy">{{ policySaving ? t('common.saving') : t('common.save') }}</button></div>
            <p v-else class="identity-readonly">{{ t('workspace.identityReadOnly') }}</p>
          </div>
          <div v-else class="identity-error" role="alert">{{ t('workspace.identityPolicyLoadError') }}</div>
        </section>
      </template>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import WorkspaceIdentityMappings from '@/components/workspace/WorkspaceIdentityMappings.vue'
import Pagination from '@/components/common/Pagination.vue'
import { workspaceAPI, type EnterpriseDomain, type EnterpriseDomainCreateResult, type WorkspaceIdentityPolicy, type WorkspaceIdentityProvider, type WorkspaceIdentityProviderInput } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'
import { rememberEnterpriseSSONavigation } from '@/utils/enterpriseSSO'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const isOrganization = computed(() => store.selectedWorkspace?.type === 'organization')
const canRead = computed(() => isOrganization.value && store.can('identity.read'))
const domains = ref<EnterpriseDomain[]>([])
const providers = ref<WorkspaceIdentityProvider[]>([])
const domainPage = ref(1)
const providerPage = ref(1)
const domainTotal = ref(0)
const providerTotal = ref(0)
const policy = ref<WorkspaceIdentityPolicy | null>(null)
const loading = ref(false)
const loadError = ref(false)
const policyLoading = ref(false)
const domainSaving = ref(false)
const providerSaving = ref(false)
const policySaving = ref(false)
const domainActionId = ref<number | null>(null)
const providerActionId = ref<number | null>(null)
const domainInput = ref('')
const verificationResult = ref<EnterpriseDomainCreateResult | null>(null)
const providerFormOpen = ref(false)
const editingProvider = ref<WorkspaceIdentityProvider | null>(null)
const selectedMappingProvider = ref<WorkspaceIdentityProvider | null>(null)
const providerPreset = ref('generic')
const policyForm = reactive<{ require_sso: boolean; sso_grace_until: string }>({ require_sso: false, sso_grace_until: '' })
const providerForm = reactive({ name: '', provider_key: '', issuer_url: '', client_id: '', client_secret: '', secret_action: 'preserve' as 'preserve' | 'replace' | 'remove', token_auth_method: 'client_secret_basic' as 'client_secret_basic' | 'client_secret_post' | 'none', authorization_endpoint: '', token_endpoint: '', jwks_uri: '', userinfo_endpoint: '', scopes: 'openid profile email', email_claim: 'email', name_claim: 'name', groups_claim: 'groups', jit_enabled: false, default_role: 'viewer', allowed_domains: '', is_default: false, discovery_enabled: true })
const providerSecretError = computed(() => {
  if (!editingProvider.value || providerForm.secret_action === 'remove') return ''
  if (providerForm.token_auth_method === 'none' && (editingProvider.value.has_client_secret || providerForm.secret_action === 'replace')) return t('workspace.identityPublicClientSecretConflict')
  if (providerForm.token_auth_method !== 'none' && providerForm.secret_action === 'preserve' && !editingProvider.value.has_client_secret) return t('workspace.identityConfidentialClientSecretRequired')
  return ''
})
const verificationHost = computed(() => verificationResult.value?.domain.dns_host || (verificationResult.value ? `_modurelay-verification.${verificationResult.value.domain.normalized_domain}` : ''))
const managedRoles = ['viewer', 'developer', 'billing', 'admin']
const policyDirty = computed(() => Boolean(policy.value && (policy.value.require_sso !== policyForm.require_sso || (policy.value.sso_grace_until ? toLocalDateTime(policy.value.sso_grace_until) : '') !== policyForm.sso_grace_until)))
let generation = 0
let contextGeneration = 0
let controller: AbortController | null = null

function errorMessage(error: unknown, fallback: string): string { return (error as { message?: string })?.message || fallback }
function currentWorkspace(id: number, expectedContext = contextGeneration): boolean { return workspaceId.value === id && canRead.value && expectedContext === contextGeneration }
function toLocalDateTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}
function domainStatusLabel(status: string): string { return t(`workspace.identityDomainStatus.${status}`, status) }
function providerStatusLabel(status: string): string { return t(`workspace.identityProviderStatus.${status}`, status) }
function validationLabel(code: unknown): string {
  const knownCodes = ['SUCCESS', 'DISCOVERY_FAILED', 'ISSUER_MISMATCH', 'ENDPOINT_INVALID', 'CONFIGURATION_INVALID', 'VALIDATION_FAILED', 'PROVIDER_DISABLED']
  return typeof code === 'string' && knownCodes.includes(code) ? t(`workspace.identityValidationResults.${code}`) : t('workspace.identityValidationNotRecorded')
}
function resetProviderForm(): void {
  providerPreset.value = 'generic'
  Object.assign(providerForm, { name: '', provider_key: '', issuer_url: '', client_id: '', client_secret: '', secret_action: 'preserve', token_auth_method: 'client_secret_basic', authorization_endpoint: '', token_endpoint: '', jwks_uri: '', userinfo_endpoint: '', scopes: 'openid profile email', email_claim: 'email', name_claim: 'name', groups_claim: 'groups', jit_enabled: false, default_role: 'viewer', allowed_domains: '', is_default: false, discovery_enabled: true })
}
function openCreateProvider(): void { editingProvider.value = null; resetProviderForm(); providerFormOpen.value = true }
function editProvider(provider: WorkspaceIdentityProvider): void {
  editingProvider.value = provider
  Object.assign(providerForm, {
    name: provider.name,
    provider_key: provider.provider_key,
    issuer_url: provider.issuer_url,
    client_id: provider.client_id,
    client_secret: '',
    secret_action: 'preserve',
    token_auth_method: provider.token_auth_method || 'client_secret_basic',
    authorization_endpoint: provider.authorization_endpoint || '',
    token_endpoint: provider.token_endpoint || '',
    jwks_uri: provider.jwks_uri || '',
    userinfo_endpoint: provider.userinfo_endpoint || '',
    scopes: (provider.scopes || []).join(' '),
    email_claim: typeof provider.claim_mapping?.email === 'string' ? provider.claim_mapping.email : 'email',
    name_claim: typeof provider.claim_mapping?.name === 'string' ? provider.claim_mapping.name : 'name',
    groups_claim: typeof provider.claim_mapping?.groups === 'string' ? provider.claim_mapping.groups : 'groups',
    jit_enabled: provider.jit_config?.enabled ?? false,
    default_role: provider.jit_config?.default_role || 'viewer',
    allowed_domains: (provider.jit_config?.allowed_domains || []).join(', '),
    is_default: provider.is_default,
    discovery_enabled: provider.discovery_enabled,
  })
  providerFormOpen.value = true
}
function applyPreset(): void {
  const presets: Record<string, Partial<typeof providerForm>> = {
    generic: { scopes: 'openid profile email', email_claim: 'email', name_claim: 'name', groups_claim: 'groups' },
    entra: { name: 'Microsoft Entra ID', provider_key: 'entra', issuer_url: 'https://login.microsoftonline.com/<tenant-id>/v2.0', scopes: 'openid profile email', email_claim: 'email', name_claim: 'name', groups_claim: 'groups' },
    google: { name: 'Google Workspace', provider_key: 'google-workspace', issuer_url: 'https://accounts.google.com', scopes: 'openid profile email', email_claim: 'email', name_claim: 'name', groups_claim: '' },
    okta: { name: 'Okta', provider_key: 'okta', issuer_url: 'https://<your-org>.okta.com/oauth2/default', scopes: 'openid profile email groups', email_claim: 'email', name_claim: 'name', groups_claim: 'groups' },
  }
  Object.assign(providerForm, presets[providerPreset.value] || presets.generic)
}
function closeProviderForm(): void { providerFormOpen.value = false; editingProvider.value = null; resetProviderForm() }
function linkProviderPath(provider: WorkspaceIdentityProvider): string {
  const query = new URLSearchParams({ workspace_id: String(workspaceId.value), provider_id: String(provider.id), link: '1', return_to: `/workspaces/${workspaceId.value}/identity` })
  return `/auth/sso?${query.toString()}`
}
function changePage(resource: 'domains' | 'providers', page: number): void {
  if (loading.value || !Number.isSafeInteger(page) || page < 1) return
  if (resource === 'domains') domainPage.value = page
  else providerPage.value = page
  void load()
}

async function load(): Promise<void> {
  const id = workspaceId.value
  const current = ++generation
  controller?.abort()
  domains.value = []
  providers.value = []
  policy.value = null
  loadError.value = false
  if (!id || !canRead.value) { loading.value = false; policyLoading.value = false; return }
  controller = new AbortController()
  loading.value = true
  policyLoading.value = store.can('workspace_sso.update') || store.can('identity.read')
  try {
    const [domainResult, providerResult, securityPolicy] = await Promise.all([
      workspaceAPI.listDomains(id, { page: domainPage.value, page_size: 50, signal: controller.signal }),
      workspaceAPI.listIdentityProviders(id, { page: providerPage.value, page_size: 50, signal: controller.signal }),
      workspaceAPI.getIdentityPolicy(id, controller.signal),
    ])
    if (current !== generation) return
    domains.value = domainResult.items || []
    providers.value = providerResult.items || []
    domainTotal.value = domainResult.total || domains.value.length
    providerTotal.value = providerResult.total || providers.value.length
    setPolicy(securityPolicy)
  } catch (error) {
    if (current === generation) {
      loadError.value = true
      app.showError(errorMessage(error, t('workspace.identityLoadError')))
    }
  } finally {
    if (current === generation) { loading.value = false; policyLoading.value = false }
  }
}

function setPolicy(value: WorkspaceIdentityPolicy): void {
  policy.value = value
  policyForm.require_sso = value.require_sso
  policyForm.sso_grace_until = value.sso_grace_until ? toLocalDateTime(value.sso_grace_until) : ''
}

async function addDomain(): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  const domain = domainInput.value.trim()
  if (!id || !domain || domainSaving.value || !store.can('identity.manage')) return
  domainSaving.value = true
  try {
    const result = await workspaceAPI.createDomain(id, domain)
    if (!currentWorkspace(id, requestContext)) return
    verificationResult.value = result
    domainInput.value = ''
    await load()
    if (currentWorkspace(id, requestContext)) app.showSuccess(t('workspace.identityDomainAdded'))
  } catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityDomainError'))) }
  finally { if (currentWorkspace(id, requestContext)) domainSaving.value = false }
}

async function verifyDomain(domain: EnterpriseDomain): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || domainActionId.value !== null || !store.can('identity.manage')) return
  domainActionId.value = domain.id
  try { await workspaceAPI.verifyDomain(id, domain.id); if (!currentWorkspace(id, requestContext)) return; await load(); if (currentWorkspace(id, requestContext)) app.showSuccess(t('workspace.identityDomainChecked')) }
  catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityDomainError'))) }
  finally { if (currentWorkspace(id, requestContext)) domainActionId.value = null }
}

async function regenerateDomainToken(domain: EnterpriseDomain): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || domainActionId.value !== null || !store.can('identity.manage')) return
  domainActionId.value = domain.id
  try { const result = await workspaceAPI.regenerateDomainToken(id, domain.id); if (!currentWorkspace(id, requestContext)) return; verificationResult.value = result; await load() }
  catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityDomainError'))) }
  finally { if (currentWorkspace(id, requestContext)) domainActionId.value = null }
}

async function copyVerification(value: string): Promise<void> {
  if (!value) return
  try { await navigator.clipboard.writeText(value); app.showSuccess(t('workspace.identityCopied')) }
  catch { app.showError(t('workspace.identityCopyError')) }
}

async function revokeDomain(domain: EnterpriseDomain): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || domainActionId.value !== null || !store.can('identity.manage') || !window.confirm(t('workspace.identityRevokeDomainConfirm', { domain: domain.domain }))) return
  domainActionId.value = domain.id
  try {
    await workspaceAPI.revokeDomain(id, domain.id)
    if (!currentWorkspace(id, requestContext)) return
    if (verificationResult.value?.domain.id === domain.id) verificationResult.value = null
    await load()
    if (currentWorkspace(id, requestContext)) app.showSuccess(t('workspace.identityDomainRevoked'))
  } catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityDomainError'))) }
  finally { if (currentWorkspace(id, requestContext)) domainActionId.value = null }
}

async function saveProvider(): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || providerSaving.value || !store.can('identity.manage')) return
  if (providerSecretError.value) { app.showError(providerSecretError.value); return }
  if (providerForm.token_auth_method !== 'none' && (!editingProvider.value || providerForm.secret_action === 'replace') && !providerForm.client_secret.trim()) { app.showError(t('workspace.identitySecretRequired')); return }
  if (editingProvider.value && providerForm.secret_action === 'remove' && !window.confirm(t('workspace.identityRemoveSecretConfirm'))) return
  const payload: WorkspaceIdentityProviderInput = {
    provider_key: providerForm.provider_key.trim(),
    name: providerForm.name.trim(),
    issuer_url: providerForm.issuer_url.trim(),
    client_id: providerForm.client_id.trim(),
    ...(providerForm.token_auth_method !== 'none' && (!editingProvider.value || providerForm.secret_action === 'replace') ? { client_secret: providerForm.client_secret, secret_action: 'replace' as const } : { secret_action: providerForm.secret_action }),
    ...(editingProvider.value?.revision ? { revision: editingProvider.value.revision } : {}),
    token_auth_method: providerForm.secret_action === 'remove' ? 'none' : providerForm.token_auth_method,
    ...(!providerForm.discovery_enabled ? { authorization_endpoint: providerForm.authorization_endpoint.trim(), token_endpoint: providerForm.token_endpoint.trim(), jwks_uri: providerForm.jwks_uri.trim(), userinfo_endpoint: providerForm.userinfo_endpoint.trim() } : {}),
    scopes: providerForm.scopes.split(/[\s,]+/).map(scope => scope.trim()).filter(Boolean),
    is_default: providerForm.is_default,
    discovery_enabled: providerForm.discovery_enabled,
    claim_mapping: { ...(editingProvider.value?.claim_mapping || {}), email: providerForm.email_claim.trim(), ...(providerForm.name_claim.trim() ? { name: providerForm.name_claim.trim() } : {}), ...(providerForm.groups_claim.trim() ? { groups: providerForm.groups_claim.trim() } : {}) },
    jit_config: {
      enabled: providerForm.jit_enabled,
      default_role: providerForm.default_role,
      allowed_domains: providerForm.allowed_domains.split(/[\s,]+/).map(domain => domain.trim()).filter(Boolean),
      require_verified_email: true,
    },
  }
  if (!providerForm.name_claim.trim()) delete payload.claim_mapping.name
  if (!providerForm.groups_claim.trim()) delete payload.claim_mapping.groups
  providerSaving.value = true
  try {
    if (editingProvider.value) await workspaceAPI.updateIdentityProvider(id, editingProvider.value.id, payload)
    else await workspaceAPI.createIdentityProvider(id, payload)
    if (!currentWorkspace(id, requestContext)) return
    closeProviderForm()
    await load()
    if (currentWorkspace(id, requestContext)) app.showSuccess(t('common.saved'))
  } catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityProviderError'))) }
  finally { if (currentWorkspace(id, requestContext)) providerSaving.value = false }
}

async function disableProvider(provider: WorkspaceIdentityProvider): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || !store.can('identity.manage') || !window.confirm(t('workspace.identityDisableConfirm', { name: provider.name }))) return
  providerActionId.value = provider.id
  try { await workspaceAPI.disableIdentityProvider(id, provider.id); if (!currentWorkspace(id, requestContext)) return; await load(); if (currentWorkspace(id, requestContext)) app.showSuccess(t('workspace.identityProviderDisabled')) }
  catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityProviderError'))) }
  finally { if (currentWorkspace(id, requestContext)) providerActionId.value = null }
}

async function testSSO(provider: WorkspaceIdentityProvider): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || provider.status !== 'active') return
  providerActionId.value = provider.id
  try {
    const result = await workspaceAPI.startSSO(id, provider.id, `/workspaces/${id}/identity`)
    if (!currentWorkspace(id, requestContext)) return
    rememberEnterpriseSSONavigation(id, provider.id, `/workspaces/${id}/identity`)
    window.location.assign(result.authorization_url)
  } catch (error) {
    if (currentWorkspace(id, requestContext)) { app.showError(errorMessage(error, t('workspace.identitySSOStartError'))); providerActionId.value = null }
  }
}

async function savePolicy(): Promise<void> {
  const id = workspaceId.value
  const requestContext = contextGeneration
  if (!id || policySaving.value || !store.can('workspace_sso.update') || !policyDirty.value || (policyForm.require_sso && !providerTotal.value)) return
  const grace = policyForm.sso_grace_until ? new Date(policyForm.sso_grace_until) : null
  if (policyForm.sso_grace_until && Number.isNaN(grace?.getTime())) return
  policySaving.value = true
  try {
    const updated = await workspaceAPI.updateIdentityPolicy(id, { require_sso: policyForm.require_sso, sso_grace_until: grace?.toISOString() || null })
    if (!currentWorkspace(id, requestContext)) return
    setPolicy(updated)
    app.showSuccess(t('common.saved'))
  } catch (error) { if (currentWorkspace(id, requestContext)) app.showError(errorMessage(error, t('workspace.identityPolicySaveError'))) }
  finally { if (currentWorkspace(id, requestContext)) policySaving.value = false }
}

onMounted(load)
watch(() => providerForm.secret_action, action => {
  providerForm.client_secret = ''
  if (action === 'remove') providerForm.token_auth_method = 'none'
  else if (action === 'replace' && providerForm.token_auth_method === 'none') providerForm.token_auth_method = 'client_secret_basic'
})
watch([workspaceId, () => store.permissions], () => {
  ++contextGeneration
  verificationResult.value = null
  domainInput.value = ''
  domainPage.value = 1
  providerPage.value = 1
  domainTotal.value = 0
  providerTotal.value = 0
  domainSaving.value = false
  providerSaving.value = false
  policySaving.value = false
  domainActionId.value = null
  providerActionId.value = null
  selectedMappingProvider.value = null
  closeProviderForm()
  void load()
})
onBeforeUnmount(() => { ++generation; ++contextGeneration; controller?.abort() })
</script>

<style scoped>
.identity-page { display: grid; min-width: 0; gap: 16px; padding-top: 20px; }
.identity-panel { display: grid; min-width: 0; gap: 16px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.identity-heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.identity-heading > * { min-width: 0; }
.identity-heading h2, .identity-heading h3 { margin: 0; color: var(--color-text-primary); font-size: 17px; }
.identity-heading h3 { font-size: 15px; }
.identity-heading p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
.workspace-eyebrow { margin: 0 0 5px; color: var(--color-text-muted); font-size: 12px; font-weight: 700; }
.identity-inline-form { display: flex; flex-wrap: wrap; align-items: end; gap: 8px; }
.identity-inline-form .input { min-width: min(240px, 100%); min-height: 38px; }
.identity-list { display: grid; min-width: 0; }
.identity-row { display: flex; min-width: 0; align-items: center; justify-content: space-between; gap: 16px; border-top: 1px solid var(--color-border-subtle); padding: 14px 0; }
.identity-row:first-child { border-top: 0; }
.identity-row__main { display: grid; min-width: 0; gap: 5px; }
.identity-row__main > strong, .identity-provider-title > strong { color: var(--color-text-primary); overflow-wrap: anywhere; }
.identity-row__main small { color: var(--color-text-muted); font-size: 12px; overflow-wrap: anywhere; }
.identity-provider-title { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }
.identity-issuer { color: var(--color-text-secondary); font-size: 12px; overflow-wrap: anywhere; }
.identity-provider-validation { display: flex; min-width: 0; flex-wrap: wrap; align-items: baseline; gap: 4px 10px; }
.identity-status, .identity-default, .identity-revision { flex: 0 0 auto; border: 1px solid var(--color-border); border-radius: 999px; background: var(--color-surface-soft); color: var(--color-text-secondary); padding: 3px 8px; font-size: 12px; white-space: nowrap; }
.identity-status.is-verified, .identity-status.is-active { border-color: color-mix(in srgb, var(--color-success) 35%, var(--color-border)); color: var(--color-success); }
.identity-status.is-failed, .identity-status.is-disabled { border-color: color-mix(in srgb, var(--color-danger) 35%, var(--color-border)); color: var(--color-danger); }
.identity-default { color: var(--color-primary); }
.identity-row__actions, .identity-actions { display: flex; flex: 0 0 auto; flex-wrap: wrap; align-items: center; gap: 8px; }
.identity-danger { color: var(--color-danger); }
.identity-verification { display: grid; gap: 12px; border: 1px solid var(--color-primary-border); border-radius: 8px; background: var(--color-primary-soft); padding: 14px; }
.identity-verification strong { color: var(--color-text-primary); }
.identity-verification p { margin: 4px 0 0; color: var(--color-text-secondary); font-size: 13px; }
.identity-verification dl { display: grid; gap: 10px; margin: 0; }
.identity-verification dl > div { display: grid; gap: 4px; }
.identity-verification dt { color: var(--color-text-muted); font-size: 12px; font-weight: 600; }
.identity-verification dd { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 8px; margin: 0; }
.identity-verification code { color: var(--color-text-primary); overflow-wrap: anywhere; }
.identity-provider-form { display: grid; min-width: 0; gap: 16px; border-top: 1px solid var(--color-border); padding-top: 16px; }
.identity-fields { display: grid; min-width: 0; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; }
.identity-fields label, .identity-options > label:not(.identity-check), .identity-field--grace { display: grid; min-width: 0; gap: 6px; color: var(--color-text-secondary); font-size: 13px; font-weight: 600; }
.identity-fields .input, .identity-options .input { width: 100%; min-height: 40px; box-sizing: border-box; }
.identity-field--wide { grid-column: 1 / -1; }
.identity-fields small, .identity-options small, .identity-field--grace small { color: var(--color-text-muted); font-size: 12px; font-weight: 400; line-height: 1.45; }
.identity-json { min-height: 116px; resize: vertical; font-family: var(--font-mono, monospace); font-size: 12px; }
.identity-options { display: grid; min-width: 0; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px 16px; border: 1px solid var(--color-border-subtle); border-radius: 8px; margin: 0; padding: 14px; }
.identity-options legend { padding: 0 5px; color: var(--color-text-primary); font-size: 13px; font-weight: 700; }
.identity-options--checks { border: 0; padding: 0; }
.identity-check { display: flex; min-width: 0; align-items: flex-start; gap: 10px; color: var(--color-text-secondary); font-size: 13px; cursor: pointer; }
.identity-check input { width: 18px; height: 18px; flex: 0 0 auto; margin: 1px 0 0; accent-color: var(--color-primary); }
.identity-check span { display: grid; min-width: 0; gap: 4px; }
.identity-check strong { color: var(--color-text-primary); }
.identity-policy { display: grid; justify-items: start; gap: 16px; }
.identity-check--policy { align-items: flex-start; }
.identity-field--grace { width: min(100%, 440px); }
.identity-readonly { margin: 0; color: var(--color-text-muted); font-size: 13px; }
.identity-provision-warning { grid-column: 1 / -1; margin: 0; color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
.identity-error { display: flex; align-items: center; justify-content: space-between; gap: 12px; border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-border)); border-radius: 8px; color: var(--color-danger); padding: 12px; }
@media (max-width: 700px) { .identity-heading, .identity-row { align-items: stretch; flex-direction: column; } .identity-inline-form { align-items: stretch; flex-direction: column; } .identity-inline-form .input { min-width: 0; width: 100%; box-sizing: border-box; } .identity-row__actions { align-items: stretch; } .identity-fields, .identity-options { grid-template-columns: minmax(0, 1fr); } .identity-field--wide { grid-column: auto; } .identity-actions > .btn { flex: 1 1 140px; min-height: 40px; } }
</style>
