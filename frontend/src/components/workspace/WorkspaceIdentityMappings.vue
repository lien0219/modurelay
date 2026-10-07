<template>
  <section class="mapping-panel">
    <div class="mapping-heading"><div><h2>{{ t('workspace.identityMappingsTitle', { name: provider.name }) }}</h2><p>{{ t('workspace.identityMappingsDescription') }}</p></div><button type="button" class="btn btn-ghost btn-sm" @click="emit('close')">{{ t('common.close') }}</button></div>
    <div v-if="loading" class="mapping-state" role="status">{{ t('common.loading') }}</div>
    <div v-else-if="loadError" class="mapping-error" role="alert"><span>{{ t('workspace.identityMappingLoadError') }}</span><button type="button" class="btn btn-secondary btn-sm" @click="load">{{ t('common.retry') }}</button></div>
    <form v-else data-testid="identity-mapping-form" class="mapping-form" @submit.prevent="save">
      <fieldset class="mapping-section">
        <legend>{{ t('workspace.identityRoleMappings') }}</legend>
        <div v-for="(mapping, index) in mappings.roles" :key="index" class="mapping-row">
          <label><span>{{ t('workspace.identityExternalGroup') }}</span><input v-model="mapping.claim_value" class="input" :name="`role-group-${index}`" required maxlength="512" :disabled="!canManage"></label>
          <label><span>{{ t('workspace.role') }}</span><select v-model="mapping.role" class="input" :name="`mapped-role-${index}`" :disabled="!canManage"><option v-for="role in managedRoles" :key="role" :value="role">{{ t(`workspace.roles.${role}`) }}</option></select></label>
          <label class="mapping-priority"><span>{{ t('workspace.identityPriority') }}</span><input v-model.number="mapping.priority" class="input" type="number" min="-100000" max="100000" required :disabled="!canManage"></label>
          <button v-if="canManage" type="button" class="btn btn-ghost btn-sm mapping-danger" :aria-label="t('workspace.identityRemoveRoleMapping', { group: mapping.claim_value || String(index + 1) })" @click="mappings.roles.splice(index, 1)">{{ t('common.remove') }}</button>
        </div>
        <p v-if="!mappings.roles.length" class="mapping-empty">{{ t('workspace.identityNoRoleMappings') }}</p>
        <button v-if="canManage && mappings.roles.length < 200" type="button" class="btn btn-secondary btn-sm" data-testid="identity-add-role-mapping" @click="mappings.roles.push({ claim_value: '', role: 'viewer', priority: 0 })">{{ t('workspace.identityAddRoleMapping') }}</button>
      </fieldset>
      <fieldset class="mapping-section">
        <legend>{{ t('workspace.identityTeamMappings') }}</legend>
        <div v-for="(mapping, index) in mappings.teams" :key="index" class="mapping-row mapping-row--team">
          <label><span>{{ t('workspace.identityExternalGroup') }}</span><input v-model="mapping.claim_value" class="input" :name="`team-group-${index}`" required maxlength="512" :disabled="!canManage"></label>
          <label><span>{{ t('workspace.teamSubject') }}</span><select v-model.number="mapping.team_id" class="input" :name="`mapped-team-${index}`" required :disabled="!canManage"><option :value="0" disabled>{{ t('workspace.identitySelectTeam') }}</option><option v-if="mapping.team_id > 0 && !teams.some(team => team.id === mapping.team_id)" :value="mapping.team_id">#{{ mapping.team_id }}</option><option v-for="team in teams" :key="team.id" :value="team.id">{{ team.name }}</option></select></label>
          <button v-if="canManage" type="button" class="btn btn-ghost btn-sm mapping-danger" :aria-label="t('workspace.identityRemoveTeamMapping', { group: mapping.claim_value || String(index + 1) })" @click="mappings.teams.splice(index, 1)">{{ t('common.remove') }}</button>
        </div>
        <p v-if="!mappings.teams.length" class="mapping-empty">{{ t('workspace.identityNoTeamMappings') }}</p>
        <button v-if="canManage && mappings.teams.length < 200" type="button" class="btn btn-secondary btn-sm" data-testid="identity-add-team-mapping" @click="mappings.teams.push({ claim_value: '', team_id: 0 })">{{ t('workspace.identityAddTeamMapping') }}</button>
        <button v-if="canManage && nextTeamPage <= totalTeamPages" type="button" class="btn btn-secondary btn-sm" data-testid="identity-load-more-teams" :disabled="loadingTeams" @click="loadMoreTeams">{{ loadingTeams ? t('common.loading') : t('workspace.identityLoadMoreTeams') }}</button>
      </fieldset>
      <p class="mapping-note">{{ t('workspace.identityMappingSafety') }}</p>
      <p v-if="validationError" class="mapping-error" role="alert">{{ t('workspace.identityMappingInvalid') }}</p>
      <div v-if="canManage" class="mapping-actions"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('common.save') }}</button></div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { workspaceAPI, type WorkspaceIdentityMappings, type WorkspaceIdentityProvider, type WorkspaceTeam } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const props = defineProps<{ workspaceId: number; provider: WorkspaceIdentityProvider }>()
const emit = defineEmits<{ close: []; saved: [] }>()
const { t } = useI18n()
const store = useWorkspaceStore()
const app = useAppStore()
const canManage = computed(() => store.can('identity.manage'))
const mappings = reactive<WorkspaceIdentityMappings>({ roles: [], teams: [] })
const teams = ref<WorkspaceTeam[]>([])
const nextTeamPage = ref(2)
const totalTeamPages = ref(1)
const loadingTeams = ref(false)
const loading = ref(false)
const loadError = ref(false)
const saving = ref(false)
const validationError = ref(false)
const managedRoles = ['viewer', 'developer', 'billing', 'admin']
let generation = 0
let controller: AbortController | null = null

async function load(): Promise<void> {
  const current = ++generation
  controller?.abort()
  controller = new AbortController()
  loading.value = true
  loadError.value = false
  validationError.value = false
  Object.assign(mappings, { roles: [], teams: [] })
  teams.value = []
  nextTeamPage.value = 2
  totalTeamPages.value = 1
  loadingTeams.value = false
  try {
    const [result, teamPage] = await Promise.all([
      workspaceAPI.getIdentityProviderMappings(props.workspaceId, props.provider.id, controller.signal),
      store.can('team.read') ? workspaceAPI.listTeams(props.workspaceId, { page: 1, page_size: 100, signal: controller.signal }) : Promise.resolve({ items: [] as WorkspaceTeam[], pages: 1 }),
    ])
    if (current !== generation) return
    mappings.roles = (result.roles || []).map(mapping => ({ ...mapping }))
    mappings.teams = (result.teams || []).map(mapping => ({ ...mapping }))
    teams.value = (teamPage.items || []).filter(team => team.status === 'active')
    totalTeamPages.value = teamPage.pages || 1
  } catch (error) {
    if (current === generation) { loadError.value = true; app.showError((error as { message?: string })?.message || t('workspace.identityMappingLoadError')) }
  } finally { if (current === generation) loading.value = false }
}

async function loadMoreTeams(): Promise<void> {
  if (loadingTeams.value || !canManage.value || !store.can('team.read') || nextTeamPage.value > totalTeamPages.value) return
  const current = generation
  const id = props.workspaceId
  const page = nextTeamPage.value
  loadingTeams.value = true
  try {
    const result = await workspaceAPI.listTeams(id, { page, page_size: 100, signal: controller?.signal })
    if (current !== generation) return
    const existingIds = new Set(teams.value.map(team => team.id))
    teams.value.push(...(result.items || []).filter(team => team.status === 'active' && !existingIds.has(team.id)))
    totalTeamPages.value = result.pages || totalTeamPages.value
    nextTeamPage.value = page + 1
  } catch (error) { if (current === generation) app.showError((error as { message?: string })?.message || t('workspace.identityMappingLoadError')) }
  finally { if (current === generation) loadingTeams.value = false }
}

async function save(): Promise<void> {
  if (saving.value || !canManage.value) return
  const current = generation
  const workspaceId = props.workspaceId
  const providerId = props.provider.id
  const payload: WorkspaceIdentityMappings = {
    roles: mappings.roles.map(mapping => ({ claim_value: mapping.claim_value.trim(), role: mapping.role, priority: mapping.priority })),
    teams: mappings.teams.map(mapping => ({ claim_value: mapping.claim_value.trim(), team_id: mapping.team_id })),
  }
  validationError.value = payload.roles.some(mapping => !mapping.claim_value || !Number.isInteger(mapping.priority) || mapping.priority < -100000 || mapping.priority > 100000 || !managedRoles.includes(mapping.role)) || payload.teams.some(mapping => !mapping.claim_value || !Number.isInteger(mapping.team_id) || mapping.team_id <= 0)
  if (validationError.value) return
  saving.value = true
  try {
    await workspaceAPI.updateIdentityProviderMappings(workspaceId, providerId, payload)
    if (current !== generation || props.workspaceId !== workspaceId || props.provider.id !== providerId) return
    app.showSuccess(t('common.saved'))
    emit('saved')
  } catch (error) {
    if (current === generation) app.showError((error as { message?: string })?.message || t('workspace.identityMappingSaveError'))
  } finally { if (current === generation) saving.value = false }
}

watch([() => props.workspaceId, () => props.provider.id, () => store.permissions], () => { saving.value = false; void load() }, { immediate: true })
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.mapping-panel { display: grid; min-width: 0; gap: 16px; border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }
.mapping-heading { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 12px; }
.mapping-heading > * { min-width: 0; }
.mapping-heading h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; overflow-wrap: anywhere; }
.mapping-heading p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
.mapping-form { display: grid; min-width: 0; gap: 20px; }
.mapping-section { display: grid; min-width: 0; justify-items: start; gap: 12px; border: 0; margin: 0; padding: 0; }
.mapping-section legend { padding: 0 0 12px; color: var(--color-text-primary); font-size: 14px; font-weight: 700; }
.mapping-row { display: grid; width: 100%; min-width: 0; grid-template-columns: minmax(0, 1.6fr) minmax(130px, 1fr) 110px auto; align-items: end; gap: 10px; }
.mapping-row--team { grid-template-columns: minmax(0, 1.6fr) minmax(130px, 1fr) auto; }
.mapping-row label { display: grid; min-width: 0; gap: 5px; color: var(--color-text-secondary); font-size: 12px; }
.mapping-row .input { width: 100%; min-width: 0; min-height: 38px; box-sizing: border-box; }
.mapping-empty, .mapping-note { margin: 0; color: var(--color-text-secondary); font-size: 13px; line-height: 1.5; }
.mapping-state { padding: 24px; color: var(--color-text-muted); text-align: center; }
.mapping-error { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin: 0; color: var(--color-danger); font-size: 13px; }
.mapping-danger { color: var(--color-danger); }
@media (max-width: 700px) { .mapping-heading { align-items: stretch; flex-direction: column; } .mapping-row, .mapping-row--team { grid-template-columns: minmax(0, 1fr); border-bottom: 1px solid var(--color-border-subtle); padding-bottom: 14px; } .mapping-row .btn, .mapping-actions .btn { min-height: 40px; } }
</style>
