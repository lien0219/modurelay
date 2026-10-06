<template>
  <WorkspaceFrame section="projects">
    <div class="sa-page">
      <nav class="sa-actions" :aria-label="t('workspace.project')">
        <RouterLink class="btn btn-secondary btn-sm" :to="projectPath">{{ t('workspace.project') }}</RouterLink>
        <RouterLink v-if="accountId" class="btn btn-secondary btn-sm" :to="basePath">{{ t('serviceAccounts.title') }}</RouterLink>
      </nav>
      <section class="sa-panel">
        <header class="sa-heading">
          <div><h2>{{ account?.name || t('serviceAccounts.title') }}</h2><p>{{ account?.description || t('serviceAccounts.description') }}</p></div>
          <div class="sa-actions">
            <RouterLink v-if="account && store.can('policy.read')" class="btn btn-secondary btn-sm" :to="`${serviceAccountPath}/policy`">{{ t('workspace.policySettings') }}</RouterLink>
            <button v-if="!accountId && canMutate('service_account.create')" class="btn btn-primary btn-sm" data-testid="sa-create" @click="openAccountForm()">{{ t('serviceAccounts.create') }}</button>
            <button v-if="account && canMutate('service_account.update')" class="btn btn-secondary btn-sm" data-testid="sa-edit" @click="openAccountForm(account)">{{ t('common.edit') }}</button>
            <button v-if="account && canMutate('service_account.disable')" class="btn btn-secondary btn-sm" @click="ask(account.status === 'active' ? 'disable' : 'enable')">{{ t(account.status === 'active' ? 'serviceAccounts.disable' : 'serviceAccounts.enable') }}</button>
            <button class="btn btn-secondary btn-sm" :disabled="loading || saving" @click="load">{{ t('common.refresh') }}</button>
          </div>
        </header>
        <p v-if="!canRead" role="status">{{ t('serviceAccounts.unavailable') }}</p>
        <p v-else-if="loadError" role="alert">{{ t('serviceAccounts.loadError') }}</p>
        <p v-else-if="loading" role="status">{{ t('common.loading') }}</p>
        <template v-else-if="account">
          <dl class="sa-details">
            <div><dt>{{ t('workspace.slug') }}</dt><dd><code>{{ account.slug }}</code></dd></div>
            <div><dt>{{ t('common.status') }}</dt><dd>{{ statusLabel(account.status) }}</dd></div>
            <div><dt>{{ t('serviceAccounts.principal') }}</dt><dd>#{{ account.id }}</dd></div>
            <div><dt>{{ t('serviceAccounts.creator') }}</dt><dd>{{ account.created_by_user_id ? `#${account.created_by_user_id}` : '—' }}</dd></div>
            <div><dt>{{ t('serviceAccounts.created') }}</dt><dd>{{ date(account.created_at) }}</dd></div>
          </dl>
          <p v-if="account.status === 'disabled'" class="sa-warning" role="status">{{ t('serviceAccounts.parentDisabled') }}</p>
        </template>
        <div v-else-if="canRead && !loading" class="sa-table-wrap">
          <table v-if="accounts.length" class="sa-table">
            <thead><tr><th scope="col">{{ t('common.name') }}</th><th scope="col">{{ t('workspace.slug') }}</th><th scope="col">{{ t('common.status') }}</th><th scope="col">{{ t('serviceAccounts.created') }}</th></tr></thead>
            <tbody><tr v-for="item in accounts" :key="item.id"><td><RouterLink :to="`${basePath}/${item.id}`">{{ item.name }}</RouterLink><small>{{ item.description }}</small></td><td><code>{{ item.slug }}</code></td><td>{{ statusLabel(item.status) }}</td><td>{{ date(item.created_at) }}</td></tr></tbody>
          </table>
          <p v-else>{{ t('serviceAccounts.empty') }}</p>
          <div v-if="pages > 1" class="sa-actions sa-pagination"><button class="btn btn-secondary btn-sm" :disabled="page <= 1 || loading" @click="changePage(-1)">{{ t('common.back') }}</button><span>{{ t('serviceAccounts.page', { page, pages }) }}</span><button class="btn btn-secondary btn-sm" :disabled="page >= pages || loading" @click="changePage(1)">{{ t('common.next') }}</button></div>
        </div>
        <p v-if="canRead && project && !scopeActive" class="sa-warning" role="status">{{ t('serviceAccounts.scopeInactive') }}</p>
      </section>

      <section v-if="account && store.can('service_account.credential.read')" class="sa-panel">
        <header class="sa-heading"><h2>{{ t('serviceAccounts.credentials') }}</h2><button v-if="canMutate('service_account.credential.create') && account.status === 'active'" class="btn btn-primary btn-sm" data-testid="sa-create-credential" @click="openCredentialForm()">{{ t('serviceAccounts.createCredential') }}</button></header>
        <p v-if="credentialError" role="alert">{{ t('serviceAccounts.loadError') }}</p>
        <div v-else-if="credentials.length" class="sa-table-wrap">
          <table class="sa-table sa-credentials"><thead><tr><th scope="col">{{ t('common.name') }}</th><th scope="col">{{ t('serviceAccounts.masked') }}</th><th scope="col">{{ t('common.status') }}</th><th scope="col">{{ t('workspace.quotaUsage') }}</th><th scope="col">{{ t('workspace.expires') }}</th><th scope="col">{{ t('serviceAccounts.lastUsed') }}</th><th scope="col">{{ t('common.actions') }}</th></tr></thead>
            <tbody><template v-for="key in credentials" :key="key.id"><tr><td>{{ key.name }}<small>#{{ key.id }} · {{ t('workspace.group') }}: {{ key.group_id ?? '—' }}</small></td><td><code>••••{{ key.key_suffix }}</code></td><td>{{ statusLabel(credentialStatus(key)) }}</td><td>{{ money(key.quota_used) }} / {{ key.quota > 0 ? money(key.quota) : t('workspace.unlimited') }}</td><td>{{ key.expires_at ? date(key.expires_at) : t('serviceAccounts.noExpiry') }}</td><td>{{ key.last_used_at ? date(key.last_used_at) : t('serviceAccounts.never') }}</td><td><div class="sa-actions">
              <button v-if="key.status !== 'revoked' && canMutate('service_account.credential.update')" class="btn btn-secondary btn-sm" @click="openCredentialForm(key)">{{ t('common.edit') }}</button>
              <button v-if="key.status === 'active' && account.status === 'active' && canMutate('service_account.credential.rotate')" class="btn btn-secondary btn-sm" :data-testid="`sa-rotate-${key.id}`" @click="ask('rotate', key)">{{ t('serviceAccounts.rotate') }}</button>
              <button v-if="key.status !== 'revoked' && canMutate('service_account.credential.revoke')" class="btn btn-ghost btn-sm sa-danger" @click="ask('revoke', key)">{{ t('serviceAccounts.revoke') }}</button>
            </div></td></tr><tr><td colspan="7"><details><summary>{{ t('serviceAccounts.restrictions') }} · {{ t('serviceAccounts.limits') }}</summary><dl class="sa-details"><div><dt>{{ t('workspace.ipWhitelist') }}</dt><dd>{{ (key.ip_whitelist || []).join(', ') || '—' }}</dd></div><div><dt>{{ t('workspace.ipBlacklist') }}</dt><dd>{{ (key.ip_blacklist || []).join(', ') || '—' }}</dd></div><div><dt>{{ t('workspace.rateLimit5h') }}</dt><dd>{{ windowUsage(key.usage_5h, key.rate_limit_5h) }}</dd></div><div><dt>{{ t('workspace.rateLimit1d') }}</dt><dd>{{ windowUsage(key.usage_1d, key.rate_limit_1d) }}</dd></div><div><dt>{{ t('workspace.rateLimit7d') }}</dt><dd>{{ windowUsage(key.usage_7d, key.rate_limit_7d) }}</dd></div></dl></details></td></tr></template></tbody>
          </table>
        </div>
        <p v-else>{{ t('serviceAccounts.noCredentials') }}</p>
      </section>

      <section v-if="account && store.can('usage.read')" class="sa-panel">
        <header class="sa-heading"><div><h2>{{ t('serviceAccounts.usage') }}</h2><p>{{ t('serviceAccounts.usageHint') }}</p></div></header>
        <p v-if="usageError" role="alert">{{ t('workspace.loadError') }}</p>
        <template v-else-if="overview"><dl class="sa-details"><div><dt>{{ t('workspace.requests') }}</dt><dd>{{ overview.summary?.requests ?? 0 }}</dd></div><div><dt>{{ t('workspace.spend') }}</dt><dd>{{ money(overview.summary?.spend) }}</dd></div></dl><WorkspaceDailySpend v-if="overview.daily_spend" :rows="overview.daily_spend" :timezone="overview.summary?.timezone" /><WorkspaceUsageBreakdowns :overview="overview" /></template>
      </section>
    </div>

    <BaseDialog v-if="accountFormOpen" :show="true" :title="t(editingAccount ? 'serviceAccounts.edit' : 'serviceAccounts.create')" @close="accountFormOpen = false">
      <form class="sa-form" @submit.prevent="saveAccount"><label><span>{{ t('common.name') }}</span><input v-model="accountForm.name" class="input" name="sa-name" required maxlength="100"></label><label v-if="!editingAccount"><span>{{ t('workspace.slug') }}</span><input v-model="accountForm.slug" class="input" name="sa-slug" required maxlength="80" pattern="[a-z0-9]+(?:-[a-z0-9]+)*"><small>{{ t('serviceAccounts.slugHint') }}</small></label><label><span>{{ t('workspace.description') }}</span><textarea v-model="accountForm.description" class="input" maxlength="2000" rows="3"></textarea></label><div class="sa-actions"><button class="btn btn-primary" type="submit" :disabled="saving">{{ t('common.save') }}</button><button class="btn btn-secondary" type="button" @click="accountFormOpen = false">{{ t('common.cancel') }}</button></div></form>
    </BaseDialog>
    <BaseDialog v-if="credentialFormOpen" :show="true" :title="t(editingCredential ? 'serviceAccounts.editCredential' : 'serviceAccounts.createCredential')" width="wide" @close="credentialFormOpen = false">
      <form class="sa-form" @submit.prevent="saveCredential"><div class="sa-form-grid">
        <label><span>{{ t('workspace.keyName') }}</span><input v-model="credentialForm.name" class="input" name="credential-name" required maxlength="100"></label>
        <label><span>{{ t('workspace.group') }}</span><select v-model.number="credentialForm.group_id" class="input"><option :value="null">{{ t('workspace.noGroup') }}</option><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name }} (#{{ group.id }})</option></select></label>
        <label><span>{{ t('workspace.amount') }}</span><input v-model.number="credentialForm.quota" class="input" type="number" min="0" step="0.01"></label>
        <label v-if="editingCredential"><span>{{ t('workspace.expires') }}</span><input v-model="credentialForm.expires_at" class="input" type="date"></label>
        <label v-else><span>{{ t('workspace.expires') }}</span><input v-model.number="credentialForm.expires_in_days" class="input" type="number" min="1" :placeholder="t('serviceAccounts.noExpiry')"></label>
        <label><span>{{ t('workspace.rateLimit5h') }}</span><input v-model.number="credentialForm.rate_limit_5h" class="input" type="number" min="0" step="0.01"></label>
        <label><span>{{ t('workspace.rateLimit1d') }}</span><input v-model.number="credentialForm.rate_limit_1d" class="input" type="number" min="0" step="0.01"></label>
        <label><span>{{ t('workspace.rateLimit7d') }}</span><input v-model.number="credentialForm.rate_limit_7d" class="input" type="number" min="0" step="0.01"></label>
        <label><span>{{ t('workspace.ipWhitelist') }}</span><textarea v-model="credentialForm.ip_whitelist" class="input" rows="2" :placeholder="t('workspace.ipListHint')"></textarea></label>
        <label><span>{{ t('workspace.ipBlacklist') }}</span><textarea v-model="credentialForm.ip_blacklist" class="input" rows="2" :placeholder="t('workspace.ipListHint')"></textarea></label>
      </div><div class="sa-actions"><button class="btn btn-primary" type="submit" :disabled="saving || groupError">{{ t('common.save') }}</button><button class="btn btn-secondary" type="button" @click="credentialFormOpen = false">{{ t('common.cancel') }}</button></div><p v-if="groupError" role="alert">{{ t('workspace.loadError') }}</p></form>
    </BaseDialog>
    <BaseDialog v-if="confirmation" :show="true" :title="account?.name || t('serviceAccounts.title')" @close="confirmation = null"><p>{{ confirmationText }}</p><template #footer><button class="btn btn-secondary" :disabled="saving" @click="confirmation = null">{{ t('common.cancel') }}</button><button class="btn btn-primary" data-testid="sa-confirm" :disabled="saving" @click="confirmOperation">{{ t('common.confirm') }}</button></template></BaseDialog>
    <BaseDialog v-if="secret" :show="true" :title="t('serviceAccounts.secretTitle')" @close="clearSecret"><p>{{ t('serviceAccounts.secretHint') }}</p><p v-if="rotated" class="sa-warning">{{ t('serviceAccounts.rotationHint') }}</p><code class="sa-secret" data-testid="sa-secret">{{ secret }}</code><template #footer><button class="btn btn-secondary" @click="copySecret">{{ t('serviceAccounts.copy') }}</button><button class="btn btn-primary" data-testid="sa-secret-close" @click="clearSecret">{{ t('serviceAccounts.secretClose') }}</button></template></BaseDialog>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import WorkspaceDailySpend from '@/components/workspace/WorkspaceDailySpend.vue'
import WorkspaceUsageBreakdowns from '@/components/workspace/WorkspaceUsageBreakdowns.vue'
import { serviceAccountsAPI, type ServiceAccount, type ServiceAccountCredential, type CredentialInput } from '@/api/serviceAccounts'
import { workspaceAPI, type Project, type WorkspaceOverview, type WorkspaceAvailableGroup } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t, te, locale } = useI18n()
const route = useRoute(), router = useRouter(), store = useWorkspaceStore(), app = useAppStore()
const workspaceId = computed(() => Number(route.params.workspaceId)), projectId = computed(() => Number(route.params.projectId)), accountId = computed(() => Number(route.params.serviceAccountId) || 0)
const projectPath = computed(() => `/workspaces/${workspaceId.value}/projects/${projectId.value}`), basePath = computed(() => `${projectPath.value}/service-accounts`), serviceAccountPath = computed(() => `${basePath.value}/${accountId.value}`)
const canRead = computed(() => store.selectedWorkspaceId === workspaceId.value && store.can('service_account.read'))
const project = ref<Project | null>(null), account = ref<ServiceAccount | null>(null), accounts = ref<ServiceAccount[]>([]), credentials = ref<ServiceAccountCredential[]>([]), overview = ref<WorkspaceOverview | null>(null), groups = ref<WorkspaceAvailableGroup[]>([])
const loading = ref(false), saving = ref(false), loadError = ref(false), credentialError = ref(false), usageError = ref(false), groupError = ref(false), page = ref(1), pages = ref(1)
const scopeActive = computed(() => canRead.value && store.selectedWorkspace?.status === 'active' && project.value?.status === 'active')
function canMutate(permission: string) { return scopeActive.value && store.can(permission) && !saving.value }
const accountFormOpen = ref(false), editingAccount = ref(false), accountForm = reactive({ name: '', slug: '', description: '' })
const credentialFormOpen = ref(false), editingCredential = ref<ServiceAccountCredential | null>(null)
const emptyCredential = () => ({ name: '', group_id: null as number | null, quota: 0, expires_in_days: undefined as number | undefined, expires_at: '', rate_limit_5h: 0, rate_limit_1d: 0, rate_limit_7d: 0, ip_whitelist: '', ip_blacklist: '' })
const credentialForm = reactive(emptyCredential())
type Operation = { action: 'disable' | 'enable' | 'rotate' | 'revoke'; credential?: ServiceAccountCredential }
const confirmation = ref<Operation | null>(null), secret = ref(''), rotated = ref(false)
let generation = 0, mutationGeneration = 0, controller: AbortController | null = null
const confirmationText = computed(() => t(confirmation.value?.action === 'rotate' ? 'serviceAccounts.rotationHint' : confirmation.value?.action === 'revoke' ? 'serviceAccounts.confirmRevoke' : confirmation.value?.action === 'enable' ? 'serviceAccounts.confirmEnable' : 'serviceAccounts.confirmDisable'))
function clearSecret() { secret.value = ''; rotated.value = false }
function resetContext() { ++mutationGeneration; clearSecret(); accountFormOpen.value = false; credentialFormOpen.value = false; confirmation.value = null; saving.value = false }
function statusLabel(status: string) { return te(`serviceAccounts.${status}`) ? t(`serviceAccounts.${status}`) : status }
function credentialStatus(key: ServiceAccountCredential) { if (key.status === 'revoked') return 'revoked'; if (key.expires_at && new Date(key.expires_at).getTime() <= Date.now()) return 'expired'; if (key.quota > 0 && key.quota_used >= key.quota) return 'quota_exhausted'; return key.status }
function money(value?: number) { return typeof value === 'number' && Number.isFinite(value) ? `$${value.toFixed(2)}` : '—' }
function windowUsage(used?: number, limit?: number) { return `${money(used ?? 0)} / ${limit && limit > 0 ? money(limit) : t('workspace.unlimited')}` }
function date(value?: string | null) { return value && Number.isFinite(Date.parse(value)) ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—' }
function showError(error: unknown) { const status = (error as { status?: number; response?: { status?: number } })?.status || (error as { response?: { status?: number } })?.response?.status; app.showError(t(status === 409 ? 'serviceAccounts.idempotencyConflict' : 'serviceAccounts.saveError')) }
async function load() {
  const current = ++generation
  controller?.abort(); account.value = null; accounts.value = []; credentials.value = []; overview.value = null; project.value = null; groups.value = []
  loadError.value = false; credentialError.value = false; usageError.value = false; groupError.value = false
  if (!canRead.value) { loading.value = false; return }
  controller = new AbortController(); const signal = controller.signal
  const w = workspaceId.value, p = projectId.value, s = accountId.value
  loading.value = true
  try {
    const detail = await workspaceAPI.getProject(w, p, signal)
    if (current !== generation) return
    project.value = detail
    if (s) {
      const detail = await serviceAccountsAPI.get(w, p, s, signal)
      if (current !== generation) return
      account.value = detail
      const [keys, usage, available] = await Promise.allSettled([
        store.can('service_account.credential.read') ? serviceAccountsAPI.credentials(w, p, s, signal) : Promise.resolve([]),
        store.can('usage.read') ? workspaceAPI.getProjectOverview(w, p, signal, { service_account_id: s }) : Promise.resolve(null),
        store.can('service_account.credential.create') || store.can('service_account.credential.update') ? workspaceAPI.listAvailableGroups(w, p) : Promise.resolve([]),
      ])
      if (current !== generation) return
      credentials.value = keys.status === 'fulfilled' ? keys.value || [] : []; credentialError.value = keys.status === 'rejected'
      overview.value = usage.status === 'fulfilled' ? usage.value : null; usageError.value = usage.status === 'rejected'
      groups.value = available.status === 'fulfilled' ? available.value : []; groupError.value = available.status === 'rejected'
    } else {
      const result = await serviceAccountsAPI.list(w, p, { page: page.value, signal })
      if (current !== generation) return
      accounts.value = result.items || []; pages.value = result.pages || 1
    }
  } catch { if (current === generation) loadError.value = true }
  finally { if (current === generation) loading.value = false }
}
function changePage(delta: number) { page.value += delta; void load() }
function openAccountForm(item?: ServiceAccount) { if (!canMutate(item ? 'service_account.update' : 'service_account.create')) return; clearSecret(); editingAccount.value = Boolean(item); Object.assign(accountForm, { name: item?.name || '', slug: item?.slug || '', description: item?.description || '' }); accountFormOpen.value = true }
function openCredentialForm(key?: ServiceAccountCredential) { if (!canMutate(key ? 'service_account.credential.update' : 'service_account.credential.create')) return; clearSecret(); editingCredential.value = key || null; Object.assign(credentialForm, emptyCredential(), key ? { name: key.name, group_id: key.group_id ?? null, quota: key.quota, expires_at: key.expires_at?.slice(0, 10) || '', rate_limit_5h: key.rate_limit_5h || 0, rate_limit_1d: key.rate_limit_1d || 0, rate_limit_7d: key.rate_limit_7d || 0, ip_whitelist: (key.ip_whitelist || []).join('\n'), ip_blacklist: (key.ip_blacklist || []).join('\n') } : {}); credentialFormOpen.value = true }
async function saveAccount() {
  const editing = editingAccount.value
  if (!canMutate(editing ? 'service_account.update' : 'service_account.create') || !accountForm.name.trim()) return
  clearSecret(); const current = mutationGeneration; saving.value = true
  try {
    const input = { name: accountForm.name.trim(), description: accountForm.description.trim() }
    const result = editing ? await serviceAccountsAPI.update(workspaceId.value, projectId.value, accountId.value, input) : await serviceAccountsAPI.create(workspaceId.value, projectId.value, { ...input, slug: accountForm.slug.trim() })
    if (current !== mutationGeneration) return
    accountFormOpen.value = false; app.showSuccess(t('common.saved'))
    if (!editing) await router.push(`${basePath.value}/${result.id}`); else await load()
  } catch (error) { if (current === mutationGeneration) showError(error) }
  finally { if (current === mutationGeneration) saving.value = false }
}
function ipList(value: string) { return [...new Set(value.split(/[\n,]/).map(v => v.trim()).filter(Boolean))] }
async function saveCredential() {
  const key = editingCredential.value
  if (!canMutate(key ? 'service_account.credential.update' : 'service_account.credential.create') || groupError.value || !credentialForm.name.trim() || (!key && account.value?.status !== 'active')) return
  clearSecret(); const current = mutationGeneration; saving.value = true
  const f = credentialForm
  const input: CredentialInput = { name: f.name.trim(), group_id: f.group_id, quota: f.quota, rate_limit_5h: f.rate_limit_5h, rate_limit_1d: f.rate_limit_1d, rate_limit_7d: f.rate_limit_7d, ip_whitelist: ipList(f.ip_whitelist), ip_blacklist: ipList(f.ip_blacklist) }
  if (key) input.expires_at = f.expires_at ? `${f.expires_at}T23:59:59Z` : null
  else if (f.expires_in_days) input.expires_in_days = f.expires_in_days
  try {
    if (key) await serviceAccountsAPI.updateCredential(workspaceId.value, projectId.value, accountId.value, key.id, input)
    else {
      const result = await serviceAccountsAPI.createCredential(workspaceId.value, projectId.value, accountId.value, input, crypto.randomUUID())
      if (current !== mutationGeneration) { result.secret = ''; return }
      secret.value = result.secret; result.secret = ''
    }
    if (current !== mutationGeneration) return
    credentialFormOpen.value = false; await load()
  } catch (error) { if (current === mutationGeneration) showError(error) }
  finally { if (current === mutationGeneration) saving.value = false }
}
function ask(action: Operation['action'], credential?: ServiceAccountCredential) { const permission = action === 'rotate' || action === 'revoke' ? `service_account.credential.${action}` : 'service_account.disable'; if (!canMutate(permission)) return; clearSecret(); confirmation.value = { action, credential } }
async function confirmOperation() {
  const operation = confirmation.value
  if (!operation) return
  const permission = operation.action === 'rotate' || operation.action === 'revoke' ? `service_account.credential.${operation.action}` : 'service_account.disable'
  if (!canMutate(permission)) return
  clearSecret(); const current = mutationGeneration; saving.value = true
  const w = workspaceId.value, p = projectId.value, s = accountId.value
  try {
    if (operation.action === 'rotate' && operation.credential) {
      const result = await serviceAccountsAPI.rotate(w, p, s, operation.credential.id, crypto.randomUUID())
      if (current !== mutationGeneration) { result.secret = ''; return }
      secret.value = result.secret; result.secret = ''; rotated.value = true
    } else if (operation.action === 'revoke' && operation.credential) await serviceAccountsAPI.revoke(w, p, s, operation.credential.id)
    else await serviceAccountsAPI.setStatus(w, p, s, operation.action === 'enable' ? 'active' : 'disabled')
    if (current !== mutationGeneration) return
    confirmation.value = null; await load()
  } catch (error) { if (current === mutationGeneration) showError(error) }
  finally { if (current === mutationGeneration) saving.value = false }
}
async function copySecret() { try { await navigator.clipboard.writeText(secret.value); app.showSuccess(t('common.copied')) } catch { app.showError(t('common.copyFailed')) } }
onMounted(load)
watch([() => route.fullPath, () => store.selectedWorkspaceId, () => store.permissions, () => store.selectedWorkspace?.status], () => { resetContext(); page.value = 1; void load() })
onBeforeUnmount(() => { resetContext(); ++generation; controller?.abort() })
</script>

<style scoped>
.sa-page { display: grid; gap: 20px; padding-top: 20px; }.sa-panel { min-width: 0; padding: 18px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); box-shadow: var(--shadow-xs); }.sa-heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }.sa-heading > div { min-width: 0; }.sa-heading h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }.sa-panel p { margin: 8px 0; color: var(--color-text-secondary); font-size: 14px; }.sa-actions { display: flex; flex-wrap: wrap; align-items: center; gap: 8px; }.sa-table-wrap { overflow-x: auto; margin-top: 18px; }.sa-table { width: 100%; min-width: 560px; border-collapse: collapse; }.sa-credentials { min-width: 1040px; }.sa-table th,.sa-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; font-size: 14px; color: var(--color-text-secondary); }.sa-table th { color: var(--color-text-muted); font-size: 12px; }.sa-table small { display: block; margin-top: 4px; overflow-wrap: anywhere; }.sa-table a { color: var(--color-primary); }.sa-details { display: flex; flex-wrap: wrap; gap: 16px 28px; margin: 18px 0; }.sa-details div { min-width: 120px; max-width: 100%; }.sa-details dt { margin-bottom: 5px; color: var(--color-text-muted); font-size: 12px; }.sa-details dd { margin: 0; overflow-wrap: anywhere; color: var(--color-text-primary); font-size: 14px; font-variant-numeric: tabular-nums; }.sa-form { display: grid; gap: 16px; }.sa-form-grid { display: grid; grid-template-columns: repeat(2,minmax(0,1fr)); gap: 16px; }.sa-form label { display: grid; gap: 6px; color: var(--color-text-secondary); font-size: 14px; }.sa-form small { color: var(--color-text-muted); }.sa-panel .sa-warning,.sa-warning { color: var(--color-warning); }.sa-danger { color: var(--color-danger); }.sa-secret { display: block; overflow-wrap: anywhere; padding: 16px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface-soft); color: var(--color-text-primary); }.sa-pagination { margin-top: 16px; }details summary { cursor: pointer; }@media(max-width:640px) { .sa-heading { flex-direction: column; align-items: stretch; }.sa-form-grid { grid-template-columns: 1fr; } }
</style>
