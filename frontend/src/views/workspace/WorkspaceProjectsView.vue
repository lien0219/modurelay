<template>
  <WorkspaceFrame section="projects">
    <div class="workspace-page">
      <section class="workspace-panel">
        <div class="workspace-panel__heading"><div><h2>{{ t('workspace.projects') }}</h2><p>{{ t('workspace.projectsDescription') }}</p></div><button v-if="store.can('project.create')" type="button" class="btn btn-primary btn-sm" @click="openCreate">{{ t('workspace.createProject') }}</button></div>
        <div v-if="store.projectsLoading" class="workspace-empty" role="status">{{ t('common.loading') }}</div>
        <div v-else-if="store.projects.length" class="workspace-table-wrap">
          <table class="workspace-table">
            <thead><tr><th>{{ t('common.name') }}</th><th>{{ t('workspace.slug') }}</th><th>{{ t('common.status') }}</th><th>{{ t('common.actions') }}</th></tr></thead>
            <tbody><tr v-for="project in store.projects" :key="project.id"><td><strong>{{ project.name }}</strong><small>{{ project.description || '?' }}</small></td><td><code>{{ project.slug }}</code></td><td><span class="workspace-status" :data-status="project.status">{{ project.status }}</span></td><td><div class="workspace-actions"><RouterLink class="btn btn-secondary btn-sm" :to="`${basePath}/${project.id}`">{{ t('common.view') }}</RouterLink><RouterLink v-if="store.can('project_access.read')" class="btn btn-secondary btn-sm" :to="`${basePath}/${project.id}/access`">{{ t('workspace.projectAccess') }}</RouterLink><button v-if="store.can('project.update')" type="button" class="btn btn-ghost btn-sm" @click="editProject(project)">{{ t('common.edit') }}</button><button v-if="store.can('project.archive') && !project.is_default" type="button" class="btn btn-ghost btn-sm workspace-danger" @click="archiveProject(project.id)">{{ t('workspace.archive') }}</button></div></td></tr></tbody>
          </table>
        </div>
        <div v-else class="workspace-empty">{{ t('workspace.noProjects') }}</div>
      </section>
      <section v-if="showCreate || editing" class="workspace-panel workspace-form-panel">
        <h2>{{ editing ? t('workspace.editProject') : t('workspace.createProject') }}</h2>
        <form class="workspace-form" @submit.prevent="saveProject">
          <label><span>{{ t('common.name') }}</span><input v-model="form.name" class="input" required maxlength="100"></label>
          <label><span>{{ t('workspace.slug') }}</span><input v-model="form.slug" class="input" required pattern="[a-z0-9][a-z0-9-]{0,62}"></label>
          <label><span>{{ t('workspace.descriptionLabel') }}</span><textarea v-model="form.description" class="input" rows="3" maxlength="2000"></textarea></label>
          <label><span>{{ t('workspace.groupPolicy') }}</span><select v-model="form.group_policy" name="group-policy" class="input"><option v-for="mode in modes" :key="mode" :value="mode">{{ t(`workspace.accessModes.${mode}`) }}</option></select></label>
          <label v-if="form.group_policy === 'restricted'"><span>{{ t('workspace.allowedGroupIds') }}</span><textarea v-model="form.group_ids" name="allowed-group-ids" class="input" rows="2" required :placeholder="t('workspace.groupIdsHint')"></textarea></label>
          <label><span>{{ t('workspace.modelPolicy') }}</span><select v-model="form.model_policy" name="model-policy" class="input"><option v-for="mode in modes" :key="mode" :value="mode">{{ t(`workspace.accessModes.${mode}`) }}</option></select></label>
          <label v-if="form.model_policy === 'restricted'"><span>{{ t('workspace.allowedModels') }}</span><textarea v-model="form.models" name="allowed-models" class="input" rows="3" required :placeholder="t('workspace.modelsHint')"></textarea></label>
          <p class="workspace-policy-hint">{{ t('workspace.accessPolicyHint') }}</p>
          <div class="workspace-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button><button type="button" class="btn btn-secondary" @click="closeForm">{{ t('common.cancel') }}</button></div>
        </form>
      </section>
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import { workspaceAPI, type Project } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const showCreate = ref(false)
const editing = ref<Project | null>(null)
const saving = ref(false)
type AccessMode = 'inherit' | 'restricted' | 'deny'
const modes: AccessMode[] = ['inherit', 'restricted', 'deny']
const emptyForm = () => ({ name: '', slug: '', description: '', group_policy: 'inherit' as AccessMode, model_policy: 'inherit' as AccessMode, group_ids: '', models: '' })
const form = reactive(emptyForm())
const workspaceId = computed(() => store.selectedWorkspaceId || 0)
const basePath = computed(() => `/workspaces/${workspaceId.value}/projects`)

function policyMode(values?: unknown[] | null): AccessMode { return values == null ? 'inherit' : values.length ? 'restricted' : 'deny' }
function openCreate() { closeForm(); showCreate.value = true }
function editProject(project: Project) {
  editing.value = project
  Object.assign(form, { name: project.name, slug: project.slug, description: project.description || '', group_policy: policyMode(project.allowed_group_ids), model_policy: policyMode(project.allowed_models), group_ids: (project.allowed_group_ids || []).join('\n'), models: (project.allowed_models || []).join('\n') })
  showCreate.value = false
}
function closeForm() { showCreate.value = false; editing.value = null; Object.assign(form, emptyForm()) }

async function saveProject() {
  const id = workspaceId.value
  if (!id || saving.value || (editing.value && editing.value.workspace_id !== id)) return
  const groups = [...new Set(form.group_ids.split(/[\s,]+/).filter(Boolean).map(Number))]
  const models = [...new Set(form.models.split(/[\n,]/).map(value => value.trim()).filter(Boolean))]
  if ((form.group_policy === 'restricted' && (!groups.length || groups.some(value => !Number.isSafeInteger(value) || value <= 0))) || (form.model_policy === 'restricted' && !models.length)) {
    app.showError(t('workspace.invalidAccessPolicy'))
    return
  }
  const payload = {
    name: form.name, slug: form.slug, description: form.description,
    allowed_group_ids: form.group_policy === 'inherit' ? null : form.group_policy === 'deny' ? [] : groups,
    allowed_models: form.model_policy === 'inherit' ? null : form.model_policy === 'deny' ? [] : models,
  }
  saving.value = true
  try {
    if (editing.value) await workspaceAPI.updateProject(id, editing.value.id, payload)
    else await workspaceAPI.createProject(id, payload)
    if (id !== workspaceId.value) return
    await store.loadProjects(id, store.selectedProjectId || undefined)
    closeForm()
    app.showSuccess(t('common.saved'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.saveError')) }
  finally { saving.value = false }
}

async function archiveProject(projectId: number) {
  const id = workspaceId.value
  if (!id || !window.confirm(t('workspace.archiveConfirm'))) return
  try {
    await workspaceAPI.archiveProject(id, projectId)
    if (id === workspaceId.value) await store.loadProjects(id, store.selectedProjectId || undefined)
    app.showSuccess(t('common.deleted'))
  } catch (error) { app.showError((error as { message?: string })?.message || t('workspace.saveError')) }
}

onMounted(() => { if (workspaceId.value && !store.projects.length && !store.projectsLoading) void store.loadProjects() })
watch(workspaceId, closeForm)
</script>

<style scoped>
.workspace-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }.workspace-panel { min-width: 0; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }.workspace-panel__heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 16px; }.workspace-panel__heading > * { min-width: 0; }.workspace-table-wrap { min-width: 0; overflow-x: auto; margin-top: 16px; }.workspace-table { width: 100%; border-collapse: collapse; min-width: 680px; }.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; vertical-align: middle; }.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }.workspace-table td:first-child { color: var(--color-text-primary); }.workspace-table td small { display: block; margin-top: 3px; color: var(--color-text-muted); max-width: 300px; overflow-wrap: anywhere; }.workspace-status { border-radius: 999px; background: var(--color-surface-soft); padding: 3px 7px; font-size: 12px; white-space: nowrap; }.workspace-status[data-status="active"] { color: var(--color-success); }.workspace-actions { display: flex; flex-wrap: wrap; gap: 7px; }.workspace-danger { color: var(--color-danger); }.workspace-empty { margin-top: 16px; border: 1px dashed var(--color-border); border-radius: 8px; padding: 24px; color: var(--color-text-muted); text-align: center; }.workspace-form-panel { max-width: 680px; }.workspace-form { display: grid; gap: 14px; margin-top: 16px; }.workspace-form label { display: grid; gap: 6px; color: var(--color-text-secondary); font-size: 13px; }.workspace-form textarea { resize: vertical; }.workspace-form .workspace-actions { margin-top: 4px; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } }
</style>
