<template>
  <WorkspaceFrame section="invitations">
    <div class="workspace-page">
      <section v-if="store.can('invitation.read')" class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><h2>{{ t('workspace.invitations') }}</h2><p>{{ t('workspace.invitationDescription') }}</p></div>
          <button v-if="store.can('member.invite')" type="button" class="btn btn-primary btn-sm" @click="showForm = !showForm">{{ t('workspace.createInvitation') }}</button>
        </div>
        <form v-if="showForm && store.can('member.invite')" class="workspace-form" @submit.prevent="createInvitation">
          <label><span>{{ t('workspace.email') }}</span><input v-model="form.email" class="input" type="email" required></label>
          <label><span>{{ t('workspace.role') }}</span><select v-model="form.role" class="input"><option v-for="role in roles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select></label>
          <div class="workspace-actions"><button class="btn btn-primary" type="submit" :disabled="saving">{{ saving ? t('common.saving') : t('common.create') }}</button><button type="button" class="btn btn-secondary" @click="showForm = false">{{ t('common.cancel') }}</button></div>
        </form>
        <div v-if="token" class="workspace-token" role="status"><span>{{ t('workspace.invitationToken') }}</span><code>{{ token }}</code><button type="button" class="btn btn-secondary btn-sm" @click="copy(token)">{{ t('workspace.copyToken') }}</button></div>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="invitations.length" class="workspace-table-wrap">
          <table class="workspace-table">
            <thead><tr><th>{{ t('workspace.email') }}</th><th>{{ t('workspace.role') }}</th><th>{{ t('workspace.expires') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
            <tbody><tr v-for="invitation in invitations" :key="invitation.id"><td>{{ invitation.email }}</td><td>{{ t(`workspace.roles.${invitation.role}`) }}</td><td>{{ formatDate(invitation.expires_at) }}</td><td>{{ invitation.revoked_at ? t('workspace.revoked') : invitation.accepted_at ? t('workspace.accepted') : t('workspace.pending') }}</td><td><button v-if="store.can('member.invite') && !invitation.revoked_at && !invitation.accepted_at" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="revoke(invitation.id)">{{ t('workspace.revoke') }}</button></td></tr></tbody>
          </table>
        </div>
        <div v-else class="workspace-state">{{ t('workspace.noInvitations') }}</div>
      </section>
      <section class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.acceptInvitation') }}</h2><p>{{ t('workspace.acceptInvitationDescription') }}</p></div></div>
        <form class="workspace-form" data-testid="accept-invitation-form" @submit.prevent="acceptInvitation">
          <label><span>{{ t('workspace.invitationToken') }}</span><input v-model="acceptToken" name="invitation-token" class="input" required autocomplete="off" maxlength="1024"></label>
          <div class="workspace-actions"><button class="btn btn-primary" type="submit" :disabled="accepting || !acceptToken.trim()">{{ accepting ? t('common.loading') : t('workspace.acceptInvitation') }}</button></div>
        </form>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { workspaceAPI, type WorkspaceInvitation } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const router = useRouter()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const invitations = ref<WorkspaceInvitation[]>([])
const showForm = ref(false)
const loading = ref(false)
const saving = ref(false)
const token = ref('')
const acceptToken = ref('')
const accepting = ref(false)
const roles = ['admin', 'developer', 'billing', 'viewer']
const form = reactive({ email: '', role: 'viewer' })
let generation = 0
let controller: AbortController | null = null

async function load() {
  const id = workspaceId.value
  const currentGeneration = ++generation
  controller?.abort()
  invitations.value = []
  if (!id || !store.can('invitation.read')) { loading.value = false; return }
  controller = new AbortController()
  loading.value = true
  try {
    const result = await workspaceAPI.listInvitations(id, { signal: controller.signal })
    if (currentGeneration === generation) invitations.value = result.items || []
  } catch (error) {
    if (currentGeneration === generation) app.showError((error as { message?: string })?.message || t('workspace.loadError'))
  } finally { if (currentGeneration === generation) loading.value = false }
}

async function createInvitation() {
  const id = workspaceId.value
  if (!id || saving.value) return
  saving.value = true
  try {
    const result = await workspaceAPI.createInvitation(id, { ...form })
    if (id !== workspaceId.value) return
    token.value = result.token
    showForm.value = false
    form.email = ''
    await load()
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.invitationError')) }
  finally { saving.value = false }
}

async function acceptInvitation() {
  const value = acceptToken.value.trim()
  if (!value || accepting.value) return
  accepting.value = true
  try {
    const accepted = await workspaceAPI.acceptInvitation(value)
    acceptToken.value = ''
    await store.loadWorkspaces()
    await store.selectWorkspace(accepted.id)
    await router.push(`/workspaces/${accepted.id}/overview`)
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.acceptInvitationError')) }
  finally { accepting.value = false }
}

async function revoke(invitationId: number) {
  const id = workspaceId.value
  if (!id || !window.confirm(t('workspace.revokeConfirm'))) return
  try {
    await workspaceAPI.revokeInvitation(id, invitationId)
    if (id === workspaceId.value) await load()
    app.showSuccess(t('common.deleted'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.invitationError')) }
}

async function copy(value: string) {
  try { await navigator.clipboard.writeText(value); app.showSuccess(t('common.copied')) }
  catch { app.showError(t('common.copyFailed')) }
}
function formatDate(value: string) { return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' }).format(new Date(value)) }

onMounted(load)
watch(workspaceId, () => { token.value = ''; showForm.value = false; Object.assign(form, { email: '', role: 'viewer' }) })
watch([workspaceId, () => store.permissions], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }.workspace-panel__heading > * { min-width: 0; }.workspace-form { display: grid; gap: 14px; max-width: 620px; margin-top: 18px; }.workspace-form label { display: grid; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }.workspace-actions { display: flex; flex-wrap: wrap; gap: 7px; }.workspace-token { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-top: 18px; border: 1px solid var(--color-primary-border); border-radius: 8px; background: var(--color-primary-soft); padding: 12px; }.workspace-token code { min-width: 0; overflow-wrap: anywhere; }.workspace-table-wrap { min-width: 0; overflow-x: auto; margin-top: 18px; }.workspace-table { width: 100%; min-width: 760px; border-collapse: collapse; }.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; }.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }.workspace-danger { color: var(--color-danger); }.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } }
</style>
