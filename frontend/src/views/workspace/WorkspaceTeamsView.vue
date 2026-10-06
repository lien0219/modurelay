<template>
  <WorkspaceFrame section="teams">
    <div class="workspace-page">
      <section class="workspace-panel">
        <div class="workspace-panel__heading">
          <div>
            <p class="workspace-eyebrow">{{ t('workspace.teams') }}</p>
            <h2>{{ t('workspace.teamsTitle') }}</h2>
            <p>{{ t('workspace.teamsDescription') }}</p>
          </div>
          <button v-if="store.can('team.create')" type="button" class="btn btn-primary btn-sm" @click="openCreate">
            {{ t('workspace.createTeam') }}
          </button>
        </div>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="teams.length" class="workspace-table-wrap">
          <table class="workspace-table">
            <thead>
              <tr><th>{{ t('workspace.teamName') }}</th><th>{{ t('workspace.teamSlug') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr>
            </thead>
            <tbody>
              <tr v-for="team in teams" :key="team.id" :class="{ 'is-selected': selectedTeam?.id === team.id }">
                <td><strong>{{ team.name }}</strong><small>#{{ team.id }}</small></td>
                <td><code>{{ team.slug }}</code></td>
                <td><span class="workspace-status">{{ team.status }}</span></td>
                <td>
                  <div class="workspace-actions">
                    <button type="button" class="btn btn-secondary btn-sm" @click="selectTeam(team)">{{ t('workspace.selectTeam') }}</button>
                    <button v-if="store.can('team.update')" type="button" class="btn btn-ghost btn-sm" @click="editTeam(team)">{{ t('common.edit') }}</button>
                    <button v-if="store.can('team.archive') && team.status === 'active'" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="archiveTeam(team)">{{ t('workspace.archiveTeam') }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-else class="workspace-state">{{ t('workspace.noTeams') }}</div>
      </section>

      <section v-if="showForm" class="workspace-panel workspace-form-panel">
        <div class="workspace-panel__heading"><div><h2>{{ editing ? t('workspace.editTeam') : t('workspace.createTeam') }}</h2><p>{{ t('workspace.teamFormDescription') }}</p></div></div>
        <form data-testid="team-create-form" class="workspace-form" @submit.prevent="saveTeam">
          <label><span>{{ t('workspace.teamName') }}</span><input v-model="teamForm.name" name="name" class="input" required maxlength="100"></label>
          <label><span>{{ t('workspace.teamSlug') }}</span><input v-model="teamForm.slug" name="slug" class="input" required pattern="[a-z0-9][a-z0-9-]{0,62}" maxlength="63"></label>
          <label><span>{{ t('workspace.teamDescriptionLabel') }}</span><textarea v-model="teamForm.description" name="description" class="input" rows="3" maxlength="2000"></textarea></label>
          <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-secondary" @click="closeForm">{{ t('common.cancel') }}</button></div>
        </form>
      </section>

      <section v-if="selectedTeam" class="workspace-panel">
        <div class="workspace-panel__heading">
          <div><p class="workspace-eyebrow">{{ t('workspace.teamMembers') }}</p><h2>{{ selectedTeam.name }}</h2><p>{{ t('workspace.teamMembersDescription') }}</p></div>
        </div>
        <form v-if="store.can('team.member.update') && availableMembers.length" data-testid="team-member-form" class="workspace-inline-form" @submit.prevent="addMember">
          <label><span>{{ t('workspace.addTeamMember') }}</span><select v-model="selectedMemberId" name="member-id" class="input" required><option value="">{{ t('workspace.selectMember') }}</option><option v-for="member in availableMembers" :key="member.id" :value="member.id">{{ member.username || member.email || `#${member.user_id}` }}</option></select></label>
          <button type="submit" class="btn btn-primary btn-sm" :disabled="memberSaving">{{ memberSaving ? t('common.saving') : t('common.add') }}</button>
        </form>
        <div v-if="membersLoading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="teamMembers.length" class="workspace-table-wrap">
          <table class="workspace-table"><thead><tr><th>{{ t('workspace.member') }}</th><th>{{ t('workspace.role') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr></thead><tbody>
            <tr v-for="member in teamMembers" :key="`${member.team_id}-${member.workspace_member_id}`"><td><strong>{{ memberName(member) }}</strong><small>#{{ member.workspace_member_id }}</small></td><td>{{ t(`workspace.roles.${member.role}`) || member.role }}</td><td><span class="workspace-status">{{ member.status }}</span></td><td><button v-if="store.can('team.member.update')" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="removeMember(member)">{{ t('workspace.remove') }}</button></td></tr>
          </tbody></table>
        </div>
        <div v-else class="workspace-state">{{ t('workspace.noTeamMembers') }}</div>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { workspaceAPI, type WorkspaceMember, type WorkspaceTeam, type WorkspaceTeamInput, type WorkspaceTeamMember } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const teams = ref<WorkspaceTeam[]>([])
const members = ref<WorkspaceMember[]>([])
const teamMembers = ref<WorkspaceTeamMember[]>([])
const selectedTeamId = ref<number | null>(null)
const selectedMemberId = ref('')
const loading = ref(false)
const membersLoading = ref(false)
const saving = ref(false)
const memberSaving = ref(false)
const showForm = ref(false)
const editing = ref<WorkspaceTeam | null>(null)
const teamForm = reactive<WorkspaceTeamInput>({ name: '', slug: '', description: '' })
let generation = 0
let teamMembersGeneration = 0
let controller: AbortController | null = null
let teamMembersController: AbortController | null = null

const selectedTeam = computed(() => teams.value.find(team => team.id === selectedTeamId.value) || null)
const availableMembers = computed(() => members.value.filter(member => member.status === 'active' && !teamMembers.value.some(item => item.workspace_member_id === member.id)))

function errorMessage(error: unknown, fallback: string): string { return (error as { message?: string })?.message || fallback }
function memberName(member: WorkspaceTeamMember): string { return member.email || `#${member.user_id}` }
function resetForm(): void { Object.assign(teamForm, { name: '', slug: '', description: '' }) }
function openCreate(): void { editing.value = null; resetForm(); showForm.value = true }
function editTeam(team: WorkspaceTeam): void { editing.value = team; Object.assign(teamForm, { name: team.name, slug: team.slug, description: team.description || '' }); showForm.value = true }
function closeForm(): void { showForm.value = false; editing.value = null; resetForm() }
function selectTeam(team: WorkspaceTeam): void { selectedTeamId.value = team.id; selectedMemberId.value = '' }

async function load(): Promise<void> {
  const id = workspaceId.value
  const currentGeneration = ++generation
  controller?.abort()
  teams.value = []
  members.value = []
  teamMembers.value = []
  selectedTeamId.value = null
  if (!id || !store.can('team.read')) { loading.value = false; return }
  controller = new AbortController()
  loading.value = true
  try {
    const [teamPage, memberPage] = await Promise.all([
      workspaceAPI.listTeams(id, { signal: controller.signal }),
      store.can('member.read') ? workspaceAPI.listMembers(id, { signal: controller.signal }) : Promise.resolve({ items: [] as WorkspaceMember[] }),
    ])
    if (currentGeneration !== generation) return
    teams.value = teamPage.items || []
    members.value = memberPage.items || []
    selectedTeamId.value = teams.value[0]?.id || null
  } catch (error) {
    if (currentGeneration === generation) app.showError(errorMessage(error, t('workspace.loadError')))
  } finally {
    if (currentGeneration === generation) loading.value = false
  }
}

async function loadTeamMembers(): Promise<void> {
  const id = workspaceId.value
  const teamId = selectedTeamId.value
  const currentGeneration = ++teamMembersGeneration
  teamMembersController?.abort()
  teamMembers.value = []
  selectedMemberId.value = ''
  if (!id || !teamId || !store.can('team.read')) { membersLoading.value = false; return }
  teamMembersController = new AbortController()
  membersLoading.value = true
  try {
    const result = await workspaceAPI.listTeamMembers(id, teamId, { signal: teamMembersController.signal })
    if (currentGeneration === teamMembersGeneration) teamMembers.value = result.items || []
  } catch (error) {
    if (currentGeneration === teamMembersGeneration) app.showError(errorMessage(error, t('workspace.teamMemberError')))
  } finally {
    if (currentGeneration === teamMembersGeneration) membersLoading.value = false
  }
}

async function saveTeam(): Promise<void> {
  const id = workspaceId.value
  if (!id || !teamForm.name.trim() || !teamForm.slug.trim()) return
  saving.value = true
  try {
    const payload = { name: teamForm.name.trim(), slug: teamForm.slug.trim(), description: teamForm.description?.trim() || '' }
    if (editing.value) await workspaceAPI.updateTeam(id, editing.value.id, payload)
    else await workspaceAPI.createTeam(id, payload)
    closeForm()
    await load()
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError(errorMessage(error, t('workspace.teamSaveError'))) }
  finally { saving.value = false }
}

async function archiveTeam(team: WorkspaceTeam): Promise<void> {
  const id = workspaceId.value
  if (!id || !window.confirm(t('workspace.archiveTeamConfirm'))) return
  try { await workspaceAPI.archiveTeam(id, team.id); await load(); app.showSuccess(t('common.deleted')) }
  catch (error) { app.showError(errorMessage(error, t('workspace.teamSaveError'))) }
}

async function addMember(): Promise<void> {
  const id = workspaceId.value
  const teamId = selectedTeamId.value
  const memberId = Number(selectedMemberId.value)
  if (!id || !teamId || !memberId) return
  memberSaving.value = true
  try { await workspaceAPI.addTeamMember(id, teamId, memberId); await loadTeamMembers(); app.showSuccess(t('common.saved')) }
  catch (error) { app.showError(errorMessage(error, t('workspace.teamMemberError'))) }
  finally { memberSaving.value = false }
}

async function removeMember(member: WorkspaceTeamMember): Promise<void> {
  const id = workspaceId.value
  const teamId = selectedTeamId.value
  if (!id || !teamId || !window.confirm(t('workspace.removeTeamMemberConfirm'))) return
  try { await workspaceAPI.removeTeamMember(id, teamId, member.workspace_member_id); await loadTeamMembers(); app.showSuccess(t('common.deleted')) }
  catch (error) { app.showError(errorMessage(error, t('workspace.teamMemberError'))) }
}

onMounted(load)
watch([workspaceId, () => store.permissions], () => void load())
watch(selectedTeamId, () => void loadTeamMembers())
onBeforeUnmount(() => { ++generation; ++teamMembersGeneration; controller?.abort(); teamMembersController?.abort() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-panel__heading > * { min-width: 0; }
.workspace-eyebrow { margin: 0 0 5px; color: var(--color-text-muted); font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.workspace-table-wrap { min-width: 0; overflow-x: auto; margin-top: 16px; }
.workspace-table { width: 100%; min-width: 720px; border-collapse: collapse; }
.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; }
.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }
.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }
.workspace-table td:first-child { color: var(--color-text-primary); }
.workspace-table tr.is-selected td { background: var(--color-primary-soft); }
.workspace-table small { display: block; margin-top: 3px; color: var(--color-text-muted); }
.workspace-status { border-radius: 999px; background: var(--color-surface-soft); padding: 3px 8px; font-size: 12px; white-space: nowrap; }
.workspace-actions { display: flex; flex-wrap: wrap; gap: 7px; }
.workspace-danger { color: var(--color-danger); }
.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
.workspace-form { display: grid; gap: 14px; margin-top: 16px; }
.workspace-form label, .workspace-inline-form label { display: grid; min-width: 0; gap: 6px; color: var(--color-text-secondary); font-size: 13px; font-weight: 600; }
.workspace-form .input, .workspace-inline-form .input { width: 100%; min-height: 38px; box-sizing: border-box; }
.workspace-inline-form { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; margin-top: 16px; }
.workspace-inline-form label { flex: 1 1 260px; }
.workspace-inline-form button { flex: 0 0 auto; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } .workspace-inline-form { align-items: stretch; flex-direction: column; } .workspace-inline-form button { width: 100%; } }
</style>
