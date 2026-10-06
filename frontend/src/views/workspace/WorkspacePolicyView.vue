<template>
  <WorkspaceFrame :section="projectId ? 'projects' : 'policy'">
    <div class="workspace-policy-page">
      <nav class="workspace-policy-nav" :aria-label="t('workspace.policyNavigation')">
        <RouterLink v-if="projectId" class="btn btn-secondary btn-sm" :to="projectPath">{{ t('workspace.project') }}</RouterLink>
        <RouterLink v-if="serviceAccountId" class="btn btn-secondary btn-sm" :to="serviceAccountPath">{{ t('serviceAccounts.title') }}</RouterLink>
      </nav>

      <div v-if="!canRead" class="workspace-policy-state" :role="store.loading ? 'status' : 'alert'">
        {{ store.loading ? t('common.loading') : t('workspace.policyUnavailable') }}
      </div>
      <PolicySettingsPanel
        v-else
        :policy="policy"
        :scope-label="scopeLabel"
        :editable="canEdit"
        :loading="loading"
        :saving="saving"
        :error="loadError"
        :conflict="revisionConflict"
        @save="save"
        @reload="load"
      />

      <PolicySettingsPanel
        v-if="canRead && projectId"
        :policy="policy"
        :effective="effective"
        :scope-label="t('workspace.effectivePolicy')"
        :editable="false"
        :loading="effectiveLoading"
        :error="effectiveError"
      />
    </div>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'
import PolicySettingsPanel from '@/components/workspace/PolicySettingsPanel.vue'
import { policyAPI, type EffectivePolicy, type Policy, type PolicyScope, type PolicyUpdate } from '@/api/policies'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const route = useRoute()
const store = useWorkspaceStore()
const app = useAppStore()
const workspaceId = computed(() => Number(route.params.workspaceId) || 0)
const projectId = computed(() => Number(route.params.projectId) || 0)
const serviceAccountId = computed(() => Number(route.params.serviceAccountId) || 0)
const scope = computed<PolicyScope>(() => serviceAccountId.value ? 'service_account' : projectId.value ? 'project' : 'workspace')
const scopeLabel = computed(() => t(scope.value === 'service_account' ? 'workspace.serviceAccountPolicy' : scope.value === 'project' ? 'workspace.projectPolicy' : 'workspace.workspacePolicy'))
const projectPath = computed(() => `/workspaces/${workspaceId.value}/projects/${projectId.value}`)
const serviceAccountPath = computed(() => `${projectPath.value}/service-accounts/${serviceAccountId.value}`)
const canRead = computed(() => store.selectedWorkspaceId === workspaceId.value && store.can('policy.read'))
const canEdit = computed(() => canRead.value && store.can(`${scope.value}_policy.update`))
const policy = ref<Policy | null>(null)
const effective = ref<EffectivePolicy | null>(null)
const loading = ref(false), effectiveLoading = ref(false), saving = ref(false), loadError = ref(false), effectiveError = ref(false)
const revisionConflict = ref(false)
let generation = 0
let controller: AbortController | null = null

async function load() {
  const current = ++generation
  controller?.abort()
  loading.value = false; effectiveLoading.value = false
  saving.value = false
  policy.value = null; effective.value = null; loadError.value = false; effectiveError.value = false
  revisionConflict.value = false
  if (!workspaceId.value || !canRead.value) return
  controller = new AbortController()
  const w = workspaceId.value, p = projectId.value, s = serviceAccountId.value
  loading.value = true
  try {
    policy.value = scope.value === 'workspace' ? await policyAPI.getWorkspace(w, controller.signal) : scope.value === 'project' ? await policyAPI.getProject(w, p, controller.signal) : await policyAPI.getServiceAccount(w, p, s, controller.signal)
  } catch (error) {
    if (current === generation) { loadError.value = true; app.showError((error as { message?: string })?.message || t('workspace.policyLoadError')) }
  } finally {
    if (current === generation) loading.value = false
  }
  if (current !== generation || !p || loadError.value) return
  effectiveLoading.value = true
  try {
    effective.value = await policyAPI.getEffective(w, p, s || undefined, controller.signal)
  } catch (error) {
    if (current === generation) { effectiveError.value = true; app.showError((error as { message?: string })?.message || t('workspace.policyLoadError')) }
  } finally {
    if (current === generation) effectiveLoading.value = false
  }
}

async function save(input: PolicyUpdate) {
  if (!policy.value || !canEdit.value || saving.value) return
  const current = generation
  saving.value = true
  try {
    const w = workspaceId.value, p = projectId.value, s = serviceAccountId.value
    const updated = scope.value === 'workspace' ? await policyAPI.updateWorkspace(w, input) : scope.value === 'project' ? await policyAPI.updateProject(w, p, input) : await policyAPI.updateServiceAccount(w, p, s, input)
    if (current !== generation) return
    policy.value = updated
    revisionConflict.value = false
    app.showSuccess(t('common.saved'))
    if (p && current === generation) {
      const updatedEffective = await policyAPI.getEffective(w, p, s || undefined)
      if (current === generation) effective.value = updatedEffective
    }
  } catch (error) {
    if (current !== generation) return
    if ((error as { status?: number })?.status === 409) revisionConflict.value = true
    app.showError((error as { status?: number; message?: string })?.status === 409 ? t('workspace.policyRevisionConflict') : (error as { message?: string })?.message || t('workspace.policySaveError'))
  } finally { if (current === generation) saving.value = false }
}

onMounted(load)
watch([() => route.fullPath, () => store.selectedWorkspaceId, () => store.permissions], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.workspace-policy-page { display: grid; min-width: 0; gap: 18px; padding-top: 20px; }
.workspace-policy-nav { display: flex; flex-wrap: wrap; gap: 8px; }
.workspace-policy-state { padding: 28px 0; color: var(--color-text-muted); text-align: center; }
</style>
