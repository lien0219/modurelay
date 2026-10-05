<template>
  <WorkspaceFrame section="members">
    <section class="workspace-panel workspace-page">
      <div class="workspace-panel__heading"><div><h2>{{ t('workspace.members') }}</h2><p>{{ t('workspace.membersDescription') }}</p></div><RouterLink v-if="store.can('member.invite')" class="btn btn-primary btn-sm" :to="`/workspaces/${workspaceId}/invitations`">{{ t('workspace.invite') }}</RouterLink></div>
      <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="members.length" class="workspace-table-wrap"><table class="workspace-table"><thead><tr><th>{{ t('workspace.member') }}</th><th>{{ t('workspace.role') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr></thead><tbody><tr v-for="member in members" :key="member.id"><td><strong>{{ member.username || member.email || `#${member.user_id}` }}</strong><small>#{{ member.user_id }}</small></td><td><label v-if="canManageMember(member, 'member.update')" class="sr-only" :for="`workspace-member-role-${member.id}`">{{ t('workspace.roleFor') }} {{ member.username || member.email || `#${member.user_id}` }}</label><select v-if="canManageMember(member, 'member.update')" :id="`workspace-member-role-${member.id}`" :value="member.role" class="workspace-inline-select" @change="updateMember(member, { role: ($event.target as HTMLSelectElement).value })"><option v-for="role in roles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select><span v-else>{{ t(`workspace.roles.${member.role}`) }}</span></td><td><span class="workspace-status">{{ member.status }}</span></td><td><div class="workspace-actions"><button v-if="canManageMember(member, 'member.update')" type="button" class="btn btn-secondary btn-sm" @click="updateMember(member, { status: member.status === 'active' ? 'suspended' : 'active' })">{{ member.status === 'active' ? t('workspace.suspend') : t('workspace.activate') }}</button><button v-if="canManageMember(member, 'member.remove')" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="removeMember(member)">{{ t('workspace.remove') }}</button></div></td></tr></tbody></table></div>
      <div v-else class="workspace-state">{{ t('workspace.noMembers') }}</div>
    </section>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { workspaceAPI, type WorkspaceMember } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n(); const store = useWorkspaceStore(); const app = useAppStore()
const members = ref<WorkspaceMember[]>([])
const loading = ref(false)
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const roles = computed(() => store.can('owner.manage') ? ['owner', 'admin', 'developer', 'billing', 'viewer'] : ['admin', 'developer', 'billing', 'viewer'])
let generation = 0
let controller: AbortController | null = null

function canManageMember(member: WorkspaceMember, permission: string) {
  return store.can(permission) && (member.role !== 'owner' || store.can('owner.manage'))
}

async function load() {
  const id = workspaceId.value
  const currentGeneration = ++generation
  controller?.abort()
  members.value = []
  if (!id || !store.can('member.read')) { loading.value = false; return }
  controller = new AbortController()
  loading.value = true
  try {
    const result = await workspaceAPI.listMembers(id, { signal: controller.signal })
    if (currentGeneration === generation) members.value = result.items || []
  } catch (error) {
    if (currentGeneration === generation) app.showError((error as { message?: string })?.message || t('workspace.loadError'))
  } finally { if (currentGeneration === generation) loading.value = false }
}

async function updateMember(member: WorkspaceMember, payload: { role?: string; status?: string }) {
  const id = workspaceId.value
  if (member.workspace_id !== id) return
  try {
    await workspaceAPI.updateMember(id, member.user_id, payload)
    if (id === workspaceId.value) await load()
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.updateMemberError')) }
}

async function removeMember(member: WorkspaceMember) {
  const id = workspaceId.value
  if (member.workspace_id !== id || !window.confirm(t('workspace.removeConfirm'))) return
  try {
    await workspaceAPI.removeMember(id, member.user_id)
    if (id === workspaceId.value) await load()
    app.showSuccess(t('common.deleted'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.updateMemberError')) }
}
onMounted(load)
watch([workspaceId, () => store.permissions], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }.workspace-panel__heading > * { min-width: 0; }.workspace-table-wrap { min-width: 0; overflow-x: auto; margin-top: 16px; }.workspace-table { width: 100%; min-width: 720px; border-collapse: collapse; }.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; }.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }.workspace-table td:first-child { color: var(--color-text-primary); }.workspace-table small { display: block; margin-top: 3px; color: var(--color-text-muted); }.workspace-inline-select { max-width: 140px; border: 1px solid var(--color-border); border-radius: 7px; background: var(--color-surface); color: var(--color-text-primary); padding: 5px 8px; }.workspace-status { border-radius: 999px; background: var(--color-surface-soft); padding: 3px 8px; font-size: 12px; white-space: nowrap; }.workspace-actions { display: flex; flex-wrap: wrap; gap: 7px; }.workspace-danger { color: var(--color-danger); }.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } }
</style>
