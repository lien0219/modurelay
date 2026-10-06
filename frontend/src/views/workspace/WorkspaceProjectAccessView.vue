<template>
  <WorkspaceFrame section="projects">
    <div class="workspace-page">
      <section class="workspace-panel">
        <div class="workspace-panel__heading">
          <div>
            <p class="workspace-eyebrow">{{ t('workspace.projectAccess') }}</p>
            <h2>{{ project?.name || t('workspace.project') }}</h2>
            <p>{{ t('workspace.projectAccessDescription') }}</p>
          </div>
          <RouterLink class="btn btn-secondary btn-sm" :to="`/workspaces/${workspaceId}/projects/${projectId}`">{{ t('common.back') }}</RouterLink>
        </div>
        <div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <dl v-else-if="project" class="workspace-details"><div><dt>{{ t('workspace.slug') }}</dt><dd><code>{{ project.slug }}</code></dd></div><div><dt>{{ t('common.status') }}</dt><dd>{{ project.status }}</dd></div><div><dt>{{ t('workspace.workspace') }}</dt><dd>{{ store.selectedWorkspace?.name || '-' }}</dd></div></dl>
        <div v-else class="workspace-state">{{ t('workspace.projectAccessUnavailable') }}</div>
      </section>

      <section v-if="store.can('project_access.read')" class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.projectAccessMode') }}</h2><p>{{ modeDescription }}</p></div></div>
        <label class="workspace-mode-field"><span>{{ t('workspace.projectAccessMode') }}</span><select data-testid="project-access-mode" class="input" :value="projectAccessMode" :disabled="!store.can('workspace.project_access.update') || modeSaving" @change="changeMode"><option value="all_projects">{{ t('workspace.allProjects') }}</option><option value="assigned_projects">{{ t('workspace.assignedProjects') }}</option></select></label>
        <p v-if="!store.can('workspace.project_access.update')" class="workspace-help">{{ t('workspace.projectAccessReadOnly') }}</p>
      </section>

      <section v-if="store.can('project_access.read')" class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.projectAccessGrants') }}</h2><p>{{ t('workspace.projectAccessGrantDescription') }}</p></div></div>
        <form v-if="store.can('project_access.update')" data-testid="project-access-form" class="workspace-inline-form" @submit.prevent="createGrant">
          <label><span>{{ t('workspace.subjectType') }}</span><select v-model="grantForm.subject_type" name="subject-type" class="input"><option value="team">{{ t('workspace.teamSubject') }}</option><option value="member">{{ t('workspace.memberSubject') }}</option></select></label>
          <label><span>{{ t('workspace.subject') }}</span><select v-model="grantForm.subject_id" name="subject-id" class="input" required><option value="">{{ t('workspace.selectSubject') }}</option><option v-for="subject in subjectOptions" :key="subject.id" :value="subject.id">{{ subjectLabel(subject) }}</option></select></label>
          <label><span>{{ t('workspace.grantRole') }}</span><select v-model="grantForm.role" name="role" class="input"><option v-for="role in projectRoles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select></label>
          <button type="submit" class="btn btn-primary btn-sm" :disabled="grantSaving || !subjectOptions.length">{{ grantSaving ? t('common.saving') : t('workspace.createGrant') }}</button>
        </form>
        <div v-if="grantsLoading" class="workspace-state" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="grants.length" class="workspace-table-wrap"><table class="workspace-table"><thead><tr><th>{{ t('workspace.subject') }}</th><th>{{ t('workspace.subjectType') }}</th><th>{{ t('workspace.grantRole') }}</th><th>{{ t('common.actions') }}</th></tr></thead><tbody>
          <tr v-for="grant in grants" :key="grant.id"><td><strong>{{ grantSubjectLabel(grant) }}</strong><small>#{{ grant.subject_id }}</small></td><td>{{ grant.subject_type === 'team' ? t('workspace.teamSubject') : t('workspace.memberSubject') }}</td><td><select v-if="store.can('project_access.update')" class="workspace-inline-select" :value="grant.role" @change="updateGrant(grant, ($event.target as HTMLSelectElement).value)"><option v-for="role in projectRoles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select><span v-else>{{ t(`workspace.roles.${grant.role}`) }}</span></td><td><button v-if="store.can('project_access.update')" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="deleteGrant(grant)">{{ t('workspace.deleteGrant') }}</button></td></tr>
        </tbody></table></div>
        <div v-else class="workspace-state">{{ t('workspace.noAccessGrants') }}</div>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { workspaceAPI, type Project, type ProjectAccessGrant, type ProjectAccessGrantInput, type ProjectAccessRole, type WorkspaceMember, type WorkspaceTeam } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const route = useRoute()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => Number(route.params.workspaceId))
const projectId = computed(() => Number(route.params.projectId))
const project = ref<Project | null>(null)
const grants = ref<ProjectAccessGrant[]>([])
const teams = ref<WorkspaceTeam[]>([])
const members = ref<WorkspaceMember[]>([])
const projectAccessMode = ref<'all_projects' | 'assigned_projects'>('all_projects')
const loading = ref(false)
const grantsLoading = ref(false)
const modeSaving = ref(false)
const grantSaving = ref(false)
const grantForm = reactive<ProjectAccessGrantInput>({ subject_type: 'team', subject_id: 0, role: 'developer' })
const projectRoles: ProjectAccessRole[] = ['viewer', 'developer', 'admin']
let generation = 0
let controller: AbortController | null = null

const modeDescription = computed(() => projectAccessMode.value === 'assigned_projects' ? t('workspace.assignedProjectsDescription') : t('workspace.allProjectsDescription'))
const subjectOptions = computed(() => grantForm.subject_type === 'team' ? teams.value.filter(team => team.status === 'active') : members.value.filter(member => member.status === 'active'))

function errorMessage(error: unknown, fallback: string): string { return (error as { message?: string })?.message || fallback }
function subjectLabel(subject: WorkspaceTeam | WorkspaceMember): string { return 'name' in subject ? subject.name : subject.username || subject.email || `#${subject.user_id}` }
function grantSubjectLabel(grant: ProjectAccessGrant): string {
  const subject = grant.subject_type === 'team' ? teams.value.find(item => item.id === grant.subject_id) : members.value.find(item => item.id === grant.subject_id)
  return subject ? subjectLabel(subject) : `#${grant.subject_id}`
}
function resetGrantForm(): void { grantForm.subject_id = 0; grantForm.role = 'developer' }

async function load(): Promise<void> {
  const wid = workspaceId.value
  const pid = projectId.value
  const currentGeneration = ++generation
  controller?.abort()
  project.value = null
  grants.value = []
  teams.value = []
  members.value = []
  if (!wid || !pid) { loading.value = false; return }
  controller = new AbortController()
  loading.value = true
  grantsLoading.value = true
  try {
    const [projectResult, grantResult, teamResult, memberResult] = await Promise.all([
      workspaceAPI.getProject(wid, pid, controller.signal),
      store.can('project_access.read') ? workspaceAPI.listProjectAccessGrants(wid, pid, { signal: controller.signal }) : Promise.resolve({ items: [] as ProjectAccessGrant[] }),
      store.can('team.read') ? workspaceAPI.listTeams(wid, { signal: controller.signal }) : Promise.resolve({ items: [] as WorkspaceTeam[] }),
      store.can('member.read') ? workspaceAPI.listMembers(wid, { signal: controller.signal }) : Promise.resolve({ items: [] as WorkspaceMember[] }),
    ])
    if (currentGeneration !== generation) return
    project.value = projectResult
    grants.value = grantResult.items || []
    teams.value = teamResult.items || []
    members.value = memberResult.items || []
    projectAccessMode.value = store.selectedWorkspace?.project_access_mode === 'assigned_projects' ? 'assigned_projects' : 'all_projects'
    grantsLoading.value = false
  } catch (error) {
    if (currentGeneration === generation) app.showError(errorMessage(error, t('workspace.projectAccessError')))
  } finally {
    if (currentGeneration === generation) { loading.value = false; grantsLoading.value = false }
  }
}

async function changeMode(event: Event): Promise<void> {
  const mode = (event.target as HTMLSelectElement).value as 'all_projects' | 'assigned_projects'
  const wid = workspaceId.value
  if (!wid || mode === projectAccessMode.value) return
  modeSaving.value = true
  try {
    const updated = await workspaceAPI.updateProjectAccessMode(wid, mode)
    projectAccessMode.value = updated.project_access_mode === 'assigned_projects' ? 'assigned_projects' : mode
    const selected = store.workspaces.find(item => item.id === wid)
    if (selected) selected.project_access_mode = projectAccessMode.value
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError(errorMessage(error, t('workspace.projectAccessModeError'))) }
  finally { modeSaving.value = false }
}

async function createGrant(): Promise<void> {
  const wid = workspaceId.value
  const pid = projectId.value
  const subjectId = Number(grantForm.subject_id)
  if (!wid || !pid || !subjectId) return
  grantSaving.value = true
  try { await workspaceAPI.createProjectAccessGrant(wid, pid, { subject_type: grantForm.subject_type, subject_id: subjectId, role: grantForm.role }); resetGrantForm(); await load(); app.showSuccess(t('common.saved')) }
  catch (error) { app.showError(errorMessage(error, t('workspace.projectAccessError'))) }
  finally { grantSaving.value = false }
}

async function updateGrant(grant: ProjectAccessGrant, role: string): Promise<void> {
  const wid = workspaceId.value
  const pid = projectId.value
  if (!wid || !pid || !role || role === grant.role) return
  try { await workspaceAPI.updateProjectAccessGrant(wid, pid, grant.id, { subject_type: grant.subject_type, subject_id: grant.subject_id, role: role as ProjectAccessRole }); await load(); app.showSuccess(t('common.saved')) }
  catch (error) { app.showError(errorMessage(error, t('workspace.projectAccessError'))) }
}

async function deleteGrant(grant: ProjectAccessGrant): Promise<void> {
  const wid = workspaceId.value
  const pid = projectId.value
  if (!wid || !pid || !window.confirm(t('workspace.deleteGrantConfirm'))) return
  try { await workspaceAPI.deleteProjectAccessGrant(wid, pid, grant.id); await load(); app.showSuccess(t('common.deleted')) }
  catch (error) { app.showError(errorMessage(error, t('workspace.projectAccessError'))) }
}

onMounted(load)
watch([workspaceId, projectId, () => store.permissions, () => store.selectedWorkspace?.project_access_mode], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }
.workspace-panel__heading > * { min-width: 0; }
.workspace-eyebrow { margin: 0 0 5px; color: var(--color-text-muted); font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.workspace-details { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 12px; margin: 18px 0 0; }
.workspace-details div { min-width: 0; border-top: 1px solid var(--color-border-subtle); padding-top: 10px; }
.workspace-details dt { color: var(--color-text-muted); font-size: 12px; }
.workspace-details dd { margin: 5px 0 0; color: var(--color-text-primary); overflow-wrap: anywhere; }
.workspace-mode-field { display: grid; max-width: 420px; gap: 6px; margin-top: 16px; color: var(--color-text-secondary); font-size: 13px; font-weight: 600; }
.workspace-inline-form { display: flex; flex-wrap: wrap; align-items: end; gap: 12px; margin-top: 16px; }
.workspace-inline-form label { display: grid; min-width: 180px; flex: 1 1 200px; gap: 6px; color: var(--color-text-secondary); font-size: 13px; font-weight: 600; }
.workspace-inline-form .input { width: 100%; min-height: 38px; box-sizing: border-box; }
.workspace-inline-form button { flex: 0 0 auto; }
.workspace-help { margin: 12px 0 0; color: var(--color-text-muted); font-size: 13px; }
.workspace-table-wrap { min-width: 0; overflow-x: auto; margin-top: 16px; }
.workspace-table { width: 100%; min-width: 640px; border-collapse: collapse; }
.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; }
.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }
.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }
.workspace-table td:first-child { color: var(--color-text-primary); }
.workspace-table small { display: block; margin-top: 3px; color: var(--color-text-muted); }
.workspace-inline-select { min-height: 32px; border: 1px solid var(--color-border); border-radius: 7px; background: var(--color-surface); color: var(--color-text-primary); padding: 4px 8px; }
.workspace-danger { color: var(--color-danger); }
.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } .workspace-details { grid-template-columns: 1fr; } .workspace-inline-form { align-items: stretch; flex-direction: column; } .workspace-inline-form button { width: 100%; } }
</style>
