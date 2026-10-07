<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { workspaceAPI, type SCIMConnector, type SCIMDefaultRole, type SCIMGroupBinding, type SCIMToken, type WorkspaceTeam } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ workspaceId: number }>()
const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const canRead = computed(() => props.workspaceId > 0 && store.selectedWorkspace?.type === 'organization' && store.can('provisioning.read'))
const canManage = computed(() => canRead.value && store.can('provisioning.manage'))
const canRotate = computed(() => canRead.value && store.can('provisioning.token.rotate'))
const connectors = ref<SCIMConnector[]>([])
const selectedId = ref<number | null>(null)
const selected = computed(() => connectors.value.find(item => item.id === selectedId.value) || null)
const active = computed(() => selected.value?.status === 'active')
const activeTab = ref<'overview' | 'tokens' | 'groups'>('overview')
const loading = ref(false)
const detailLoading = ref(false)
const listError = ref(false)
const detailError = ref(false)
const actionError = ref('')
const pending = ref('')
const tokens = ref<SCIMToken[]>([])
const groups = ref<SCIMGroupBinding[]>([])
const teams = ref<WorkspaceTeam[]>([])
const teamSelection = reactive<Record<string, string>>({})
const nextTeamPage = ref(2)
const totalTeamPages = ref(1)
const loadingTeams = ref(false)
const formOpen = ref(false)
const editingId = ref<number | null>(null)
const form = reactive<{ name: string; default_role: SCIMDefaultRole }>({ name: '', default_role: 'viewer' })
const expiry = ref('')
const secret = ref('')
const copyMessage = ref('')
const copying = ref(false)
const roles: SCIMDefaultRole[] = ['viewer', 'developer', 'billing', 'admin']
let generation = 0
let controller: AbortController | null = null

function current(expected: number): boolean { return expected === generation && canRead.value }
function dismissSecret(): void { secret.value = ''; copyMessage.value = '' }
function invalidate(): number {
  ++generation
  controller?.abort()
  controller = new AbortController()
  dismissSecret()
  pending.value = ''
  copying.value = false
  loadingTeams.value = false
  loading.value = false
  detailLoading.value = false
  actionError.value = ''
  formOpen.value = false
  editingId.value = null
  expiry.value = ''
  return generation
}
function safeError(error: unknown): string {
  const item = error as { code?: unknown; reason?: unknown; status?: number }
  if (item?.code === 'RECENT_AUTH_REQUIRED' || item?.reason === 'RECENT_AUTH_REQUIRED') return t('workspace.scimRecentAuthRequired')
  if (item?.status === 409) return t('workspace.scimConflict')
  return t('workspace.scimActionError')
}
function timestamp(value?: string | null): string { return value ? formatDateTime(value) : t('workspace.scimNotRecorded') }
function tokenState(token: SCIMToken): string {
  if (token.status === 'revoked') return 'revoked'
  if (token.expires_at && new Date(token.expires_at).getTime() <= Date.now()) return 'expired'
  return 'active'
}
async function loadConnectors(): Promise<void> {
  const previousId = selectedId.value
  const expected = invalidate()
  listError.value = false
  detailError.value = false
  connectors.value = []
  selectedId.value = null
  tokens.value = []
  groups.value = []
  teams.value = []
  if (!canRead.value) return
  loading.value = true
  try {
    const result = await workspaceAPI.listSCIMConnectors(props.workspaceId, controller?.signal)
    if (!current(expected)) return
    connectors.value = result || []
    loading.value = false
    const target = connectors.value.find(item => item.id === previousId) || connectors.value[0]
    if (target) await selectConnector(target.id)
  } catch { if (current(expected)) listError.value = true }
  finally { if (current(expected)) loading.value = false }
}
async function loadDetails(expected: number): Promise<void> {
  const id = selectedId.value
  if (!current(expected) || !id) return
  detailLoading.value = true
  detailError.value = false
  tokens.value = []
  groups.value = []
  teams.value = []
  Object.keys(teamSelection).forEach(key => delete teamSelection[key])
  nextTeamPage.value = 2
  totalTeamPages.value = 1
  const workspaceId = props.workspaceId
  try {
    const [tokenRows, groupRows, teamPage] = await Promise.all([
      workspaceAPI.listSCIMTokens(workspaceId, id, controller?.signal),
      workspaceAPI.listSCIMGroups(workspaceId, id, controller?.signal),
      store.can('team.read') ? workspaceAPI.listTeams(workspaceId, { page: 1, page_size: 100, signal: controller?.signal }) : Promise.resolve({ items: [] as WorkspaceTeam[], pages: 1 }),
    ])
    if (!current(expected)) return
    tokens.value = tokenRows || []
    groups.value = groupRows || []
    for (const group of groups.value) teamSelection[group.id] = group.team_id === null ? '' : String(group.team_id)
    teams.value = (teamPage.items || []).filter(item => item.status === 'active')
    totalTeamPages.value = teamPage.pages || 1
  } catch { if (current(expected)) detailError.value = true }
  finally { if (current(expected)) detailLoading.value = false }
}
async function selectConnector(id: number): Promise<void> {
  const expected = invalidate()
  selectedId.value = connectors.value.some(item => item.id === id) ? id : null
  await loadDetails(expected)
}
async function changeTab(tab: 'overview' | 'tokens' | 'groups'): Promise<void> {
  if (activeTab.value === tab) return
  const expected = invalidate()
  activeTab.value = tab
  await loadDetails(expected)
}
async function mutate<T>(key: string, request: () => Promise<T>, success: (result: T, expected: number) => void | Promise<void>): Promise<void> {
  if (!canRead.value || loading.value || detailLoading.value || detailError.value || pending.value) return
  const expected = generation
  pending.value = key
  actionError.value = ''
  dismissSecret()
  try {
    const result = await request()
    if (!current(expected)) return
    await success(result, expected)
    if (current(expected)) app.showSuccess(t('workspace.scimSaved'))
  } catch (error) { if (current(expected)) actionError.value = safeError(error) }
  finally { if (current(expected)) pending.value = '' }
}
function openForm(edit = false): void {
  if (!canManage.value || pending.value || loading.value || (edit && !active.value)) return
  dismissSecret()
  editingId.value = edit ? selectedId.value : null
  form.name = edit ? selected.value?.name || '' : ''
  form.default_role = edit ? selected.value?.default_role || 'viewer' : 'viewer'
  formOpen.value = true
}
async function saveConnector(): Promise<void> {
  if (!canManage.value || !form.name.trim() || !roles.includes(form.default_role)) return
  const id = editingId.value
  const row = connectors.value.find(item => item.id === id)
  if (id && row?.status !== 'active') return
  const workspaceId = props.workspaceId
  const input = { name: form.name.trim(), default_role: form.default_role }
  await mutate('connector-save', () => id && row ? workspaceAPI.updateSCIMConnector(workspaceId, id, { ...input, revision: row.revision }) : workspaceAPI.createSCIMConnector(workspaceId, input), async updated => {
    connectors.value = id ? connectors.value.map(item => item.id === id ? updated : item) : [...connectors.value, updated]
    formOpen.value = false
    if (!id) await selectConnector(updated.id)
  })
}
async function disableConnector(): Promise<void> {
  const row = selected.value
  if (!row || !active.value || !canManage.value || pending.value || !window.confirm(t('workspace.scimDisableConfirm', { name: row.name }))) return
  const workspaceId = props.workspaceId
  await mutate('connector-disable', () => workspaceAPI.disableSCIMConnector(workspaceId, row.id, row.revision), () => {
    connectors.value = connectors.value.map(item => item.id === row.id ? { ...item, status: 'disabled', revision: item.revision + 1 } : item)
    formOpen.value = false
  })
}
async function createToken(): Promise<void> {
  const row = selected.value
  if (!row || !active.value || !canRotate.value) return
  const date = expiry.value ? new Date(expiry.value) : null
  if (date && (Number.isNaN(date.getTime()) || date.getTime() <= Date.now())) { actionError.value = t('workspace.scimExpiryInvalid'); return }
  const workspaceId = props.workspaceId
  await mutate('token-create', () => workspaceAPI.createSCIMToken(workspaceId, row.id, date ? { expires_at: date.toISOString() } : {}), result => {
    tokens.value = [...tokens.value.filter(item => item.id !== result.token.id), result.token]
    secret.value = result.secret
    expiry.value = ''
  })
}
async function revokeToken(token: SCIMToken): Promise<void> {
  const row = selected.value
  if (!row || !canRotate.value || pending.value || token.status !== 'active' || !window.confirm(t('workspace.scimRevokeConfirm', { prefix: token.token_prefix }))) return
  const workspaceId = props.workspaceId
  await mutate(`token-revoke-${token.id}`, () => workspaceAPI.revokeSCIMToken(workspaceId, row.id, token.id), () => {
    tokens.value = tokens.value.map(item => item.id === token.id ? { ...item, status: 'revoked', revoked_at: new Date().toISOString() } : item)
  })
}
async function bindGroup(group: SCIMGroupBinding, unbind = false): Promise<void> {
  const row = selected.value
  if (!row || !active.value || !canManage.value) return
  const teamId = unbind || !teamSelection[group.id] ? null : Number(teamSelection[group.id])
  if (teamId !== null && !teams.value.some(team => team.id === teamId)) { actionError.value = t('workspace.scimTeamInvalid'); return }
  const workspaceId = props.workspaceId
  await mutate(`group-${group.id}`, () => workspaceAPI.bindSCIMGroup(workspaceId, row.id, group.id, { revision: group.revision, team_id: teamId }), async (_, expected) => {
    const result = await workspaceAPI.listSCIMGroups(workspaceId, row.id, controller?.signal)
    if (!current(expected)) return
    groups.value = result || []
    for (const item of groups.value) teamSelection[item.id] = item.team_id === null ? '' : String(item.team_id)
  })
}
async function loadMoreTeams(): Promise<void> {
  if (!canManage.value || !store.can('team.read') || loadingTeams.value || pending.value || nextTeamPage.value > totalTeamPages.value) return
  const expected = generation
  const page = nextTeamPage.value
  loadingTeams.value = true
  try {
    const result = await workspaceAPI.listTeams(props.workspaceId, { page, page_size: 100, signal: controller?.signal })
    if (!current(expected)) return
    const ids = new Set(teams.value.map(item => item.id))
    teams.value.push(...(result.items || []).filter(item => item.status === 'active' && !ids.has(item.id)))
    totalTeamPages.value = result.pages || totalTeamPages.value
    nextTeamPage.value = page + 1
  } catch { if (current(expected)) actionError.value = t('workspace.scimLoadError') }
  finally { if (current(expected)) loadingTeams.value = false }
}
async function copy(value: string): Promise<void> {
  if (!canRead.value || !value || copying.value) return
  const expected = generation
  copying.value = true
  try { await navigator.clipboard.writeText(value); if (current(expected)) copyMessage.value = t('workspace.scimCopied') }
  catch { if (current(expected)) copyMessage.value = t('workspace.identityCopyError') }
  finally { if (current(expected)) copying.value = false }
}
watch([() => props.workspaceId, () => store.selectedWorkspace?.type, canRead, canManage, canRotate, () => store.can('team.read')], () => { selectedId.value = null; activeTab.value = 'overview'; void loadConnectors() }, { immediate: true, flush: 'sync' })
onBeforeUnmount(() => { invalidate(); secret.value = '' })
</script>

<template>
  <div v-if="canRead" class="scim-workbench" data-testid="scim-management">
    <div class="scim-actions"><button v-if="canManage" type="button" class="btn btn-primary btn-sm" data-testid="scim-add-connector" :disabled="loading || Boolean(pending)" @click="openForm()">{{ t('workspace.scimAddConnector') }}</button><button type="button" class="btn btn-secondary btn-sm" data-testid="scim-refresh" :disabled="loading || detailLoading || Boolean(pending)" @click="loadConnectors">{{ t('common.refresh') }}</button></div>
    <div v-if="loading" role="status">{{ t('common.loading') }}</div>
    <div v-else-if="listError" class="scim-error" role="alert">{{ t('workspace.scimLoadError') }}<button type="button" class="btn btn-secondary btn-sm" data-testid="scim-retry" @click="loadConnectors">{{ t('common.retry') }}</button></div>
    <template v-else>
      <p v-if="!connectors.length">{{ t('workspace.scimNoConnectors') }}</p>
      <label v-else class="scim-field"><span>{{ t('workspace.scimConnector') }}</span><select :value="selectedId" name="scim_connector" class="input" @change="selectConnector(Number(($event.target as HTMLSelectElement).value))"><option v-for="row in connectors" :key="row.id" :value="row.id">{{ row.name }} ? {{ t(`workspace.scimConnectorStates.${row.status}`) }}</option></select></label>
      <form v-if="formOpen" data-testid="scim-connector-form" class="scim-form" @submit.prevent="saveConnector">
        <label class="scim-field"><span>{{ t('workspace.scimName') }}</span><input v-model="form.name" class="input" name="scim_name" required maxlength="120" :disabled="Boolean(pending)"></label>
        <label class="scim-field"><span>{{ t('workspace.scimDefaultRole') }}</span><select v-model="form.default_role" name="scim_default_role" class="input" :disabled="Boolean(pending)"><option v-for="role in roles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select></label>
        <p>{{ t('workspace.scimDefaultRoleHint') }}</p><div class="scim-actions"><button type="submit" class="btn btn-primary" :disabled="Boolean(pending) || !form.name.trim()">{{ pending ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-secondary" :disabled="Boolean(pending)" @click="formOpen = false">{{ t('common.cancel') }}</button></div>
      </form>
      <template v-if="selected">
        <div class="scim-heading"><h3>{{ selected.name }}</h3><span class="scim-status">{{ t(`workspace.scimConnectorStates.${selected.status}`) }}</span><span>{{ t('workspace.scimDefaultRole') }}: {{ t(`workspace.roles.${selected.default_role}`) }}</span><span>{{ t('workspace.policyRevision', { revision: selected.revision }) }}</span></div>
        <div v-if="canManage && active" class="scim-actions"><button type="button" class="btn btn-secondary btn-sm" data-testid="scim-edit-connector" :disabled="detailLoading || Boolean(pending)" @click="openForm(true)">{{ t('common.edit') }}</button><button type="button" class="btn btn-secondary btn-sm" data-testid="scim-disable-connector" :disabled="detailLoading || Boolean(pending)" @click="disableConnector">{{ t('workspace.scimDisableConnector') }}</button></div>
        <p v-if="!active">{{ t('workspace.scimDisabledHint') }}</p>
        <nav class="scim-actions" :aria-label="t('workspace.scimSections')"><button v-for="name in (['overview', 'tokens', 'groups'] as const)" :key="name" type="button" class="btn btn-secondary btn-sm" :data-testid="`scim-tab-${name}`" :aria-pressed="activeTab === name" @click="changeTab(name)">{{ t(`workspace.scimTabs.${name}`) }}</button></nav>
        <div v-if="detailLoading" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="detailError" class="scim-error" role="alert">{{ t('workspace.scimLoadError') }}<button type="button" class="btn btn-secondary btn-sm" data-testid="scim-detail-retry" @click="selectConnector(selected.id)">{{ t('common.retry') }}</button></div>
        <template v-else>
          <div v-if="activeTab === 'overview'" class="scim-section">
            <dl><div><dt>{{ t('workspace.scimBaseURL') }}</dt><dd><template v-if="selected.base_url"><code>{{ selected.base_url }}</code><button type="button" class="btn btn-ghost btn-sm" data-testid="scim-copy-base" :disabled="copying" @click="copy(selected.base_url)">{{ t('workspace.scimCopyBaseURL') }}</button></template><span v-else>{{ t('workspace.scimBaseURLUnavailable') }}</span></dd></div><div><dt>{{ t('workspace.scimLastSync') }}</dt><dd>{{ timestamp(selected.last_sync_at) }}</dd></div><div><dt>{{ t('workspace.scimSyncOutcome') }}</dt><dd>{{ selected.last_error_code ? t('workspace.scimSyncFailed') : selected.last_sync_at ? t('workspace.scimSyncRecorded') : t('workspace.scimNotRecorded') }}</dd></div><div><dt>{{ t('workspace.scimFailureCount') }}</dt><dd data-testid="scim-failure-count">{{ selected.failure_count }}</dd></div></dl>
            <details><summary>{{ t('workspace.scimSetupTitle') }}</summary><p>{{ t('workspace.scimSetupEntra') }}</p><p>{{ t('workspace.scimSetupOkta') }}</p><p>{{ t('workspace.scimSetupMappings') }}</p><p>{{ t('workspace.scimSetupGroups') }}</p><p>{{ t('workspace.scimSetupLimits') }}</p></details>
          </div>
          <div v-else-if="activeTab === 'tokens'" class="scim-section">
            <p>{{ t('workspace.scimRotationHint') }}</p>
            <form v-if="canRotate && active" data-testid="scim-token-form" class="scim-form" @submit.prevent="createToken"><label class="scim-field"><span>{{ t('workspace.scimTokenExpiry') }}</span><input v-model="expiry" class="input" name="scim_expiry" type="datetime-local" :disabled="Boolean(pending)"><small>{{ t('workspace.scimExpiryHint') }}</small></label><button type="submit" class="btn btn-primary" data-testid="scim-create-token" :disabled="Boolean(pending)">{{ pending === 'token-create' ? t('common.processing') : t('workspace.scimCreateToken') }}</button></form>
            <p v-if="!tokens.length">{{ t('workspace.scimNoTokens') }}</p>
            <article v-for="token in tokens" :key="token.id" class="scim-row"><div class="scim-row-main"><code>{{ token.token_prefix }}</code><span class="scim-status">{{ t(`workspace.scimTokenStates.${tokenState(token)}`) }}</span><small>{{ t('workspace.scimTokenExpiry') }}: {{ token.expires_at ? timestamp(token.expires_at) : t('workspace.scimNoExpiry') }}</small><small>{{ t('workspace.scimTokenLastUsed') }}: {{ timestamp(token.last_used_at) }}</small></div><button v-if="canRotate && token.status === 'active'" type="button" class="btn btn-secondary btn-sm" :data-testid="`scim-revoke-${token.id}`" :disabled="Boolean(pending)" @click="revokeToken(token)">{{ t('workspace.scimRevokeToken') }}</button></article>
          </div>
          <div v-else class="scim-section"><p>{{ t('workspace.scimGroupsHint') }}</p><p v-if="!groups.length">{{ t('workspace.scimNoGroups') }}</p><article v-for="group in groups" :key="group.id" class="scim-row"><div class="scim-row-main"><strong>{{ group.display_name }}</strong><small>{{ group.external_id }}</small><label class="scim-field"><span>{{ t('workspace.scimTeam') }}</span><select v-model="teamSelection[group.id]" class="input" :name="`scim-team-${group.id}`" :disabled="!canManage || !active || Boolean(pending)"><option value="">{{ t('workspace.scimUnbound') }}</option><option v-if="group.team_id && !teams.some(team => team.id === group.team_id)" :value="String(group.team_id)" disabled>#{{ group.team_id }}</option><option v-for="team in teams" :key="team.id" :value="String(team.id)">{{ team.name }}</option></select></label></div><div v-if="canManage && active" class="scim-actions"><button type="button" class="btn btn-primary btn-sm" :data-testid="`scim-bind-${group.id}`" :disabled="Boolean(pending)" @click="bindGroup(group)">{{ t('common.save') }}</button><button v-if="group.team_id !== null" type="button" class="btn btn-secondary btn-sm" :data-testid="`scim-unbind-${group.id}`" :disabled="Boolean(pending)" @click="bindGroup(group, true)">{{ t('workspace.scimUnbind') }}</button></div></article><button v-if="canManage && store.can('team.read') && nextTeamPage <= totalTeamPages" type="button" class="btn btn-secondary btn-sm" data-testid="scim-load-more-teams" :disabled="loadingTeams || Boolean(pending)" @click="loadMoreTeams">{{ loadingTeams ? t('common.loading') : t('workspace.identityLoadMoreTeams') }}</button></div>
        </template>
      </template>
    </template>
    <p v-if="pending" role="status" aria-live="polite">{{ t('common.processing') }}</p>
    <div v-if="actionError" class="scim-error" role="alert">{{ actionError }}<button type="button" class="btn btn-secondary btn-sm" :disabled="Boolean(pending)" @click="loadConnectors">{{ t('common.refresh') }}</button></div>
    <p v-if="copyMessage && !secret" role="status" aria-live="polite">{{ copyMessage }}</p>
    <BaseDialog v-if="secret" :show="true" :title="t('workspace.scimSecretTitle')" @close="dismissSecret"><p>{{ t('workspace.scimSecretWarning') }}</p><code class="scim-secret" data-testid="scim-secret">{{ secret }}</code><p v-if="copyMessage" role="status" aria-live="polite">{{ copyMessage }}</p><template #footer><div class="scim-actions"><button type="button" class="btn btn-secondary" data-testid="scim-secret-copy" :disabled="copying" @click="copy(secret)">{{ t('workspace.scimCopyToken') }}</button><button type="button" class="btn btn-primary" data-testid="scim-secret-dismiss" @click="dismissSecret">{{ t('workspace.scimSecretDismiss') }}</button></div></template></BaseDialog>
  </div>
</template>

<style scoped>
.scim-workbench, .scim-section, .scim-form { display: grid; min-width: 0; gap: 14px; }
.scim-heading, .scim-actions { display: flex; min-width: 0; flex-wrap: wrap; align-items: center; gap: 8px; }
h3 { margin: 0; color: var(--color-text-primary); font-size: 16px; overflow-wrap: anywhere; }
p, small, dt, summary { color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
p { margin: 0; }
.scim-field { display: grid; min-width: 0; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }
.input { width: 100%; min-width: 0; min-height: 40px; box-sizing: border-box; }
.scim-form { border-top: 1px solid var(--color-border-subtle); padding-top: 14px; }
.scim-status { border: 1px solid var(--color-border); border-radius: 999px; padding: 3px 8px; background: var(--color-surface-soft); color: var(--color-text-secondary); font-size: 12px; white-space: nowrap; }
.scim-row { display: flex; min-width: 0; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; border-top: 1px solid var(--color-border-subtle); padding: 12px 0; }
.scim-row-main { display: grid; min-width: 0; flex: 1 1 220px; gap: 6px; }
strong, code, dd { color: var(--color-text-primary); overflow-wrap: anywhere; }
code { font-size: 12px; }
.scim-secret { display: block; white-space: pre-wrap; margin-top: 12px; }
dl, dl > div { display: grid; min-width: 0; gap: 6px; margin: 0; }
dl { gap: 12px; }
dd { margin: 0; }
.scim-error { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; color: var(--color-danger); font-size: 13px; }
summary { cursor: pointer; padding: 8px 0; }
details p { margin-top: 8px; }
summary:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 3px; }
@media (max-width: 700px) { .scim-row { align-items: stretch; flex-direction: column; } .scim-row-main { flex-basis: auto; } .btn { min-height: 40px; } }
</style>
