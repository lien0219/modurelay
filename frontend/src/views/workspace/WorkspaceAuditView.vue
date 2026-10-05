<template>
  <WorkspaceFrame section="audit">
    <section class="workspace-panel workspace-page"><div class="workspace-panel__heading"><div><h2>{{ t('workspace.audit') }}</h2><p>{{ t('workspace.auditDescription') }}</p></div><button type="button" class="btn btn-secondary btn-sm" :disabled="loading" @click="load">{{ t('common.refresh') }}</button></div><div v-if="loading" class="workspace-state" role="status">{{ t('common.loading') }}</div><div v-else-if="audit.length" class="workspace-table-wrap"><table class="workspace-table"><thead><tr><th>{{ t('workspace.action') }}</th><th>{{ t('workspace.actor') }}</th><th>{{ t('workspace.target') }}</th><th>{{ t('workspace.created') }}</th></tr></thead><tbody><tr v-for="event in audit" :key="event.id"><td><code>{{ event.action }}</code></td><td>#{{ event.actor_user_id }}</td><td>{{ event.target_type }} #{{ event.target_id }}</td><td>{{ formatDate(event.created_at) }}</td></tr></tbody></table></div><div v-else class="workspace-state">{{ t('workspace.noAudit') }}</div></section>
  </WorkspaceFrame>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'; import { useI18n } from 'vue-i18n'; import WorkspaceFrame from '@/components/workspace/WorkspaceFrame.vue'; import { workspaceAPI, type WorkspaceAudit } from '@/api/workspace'; import { useWorkspaceStore } from '@/stores/workspace'; import { useAppStore } from '@/stores/app'
const { t } = useI18n(); const store = useWorkspaceStore(); const app = useAppStore(); const audit = ref<WorkspaceAudit[]>([]); const loading = ref(false)
let generation = 0
let controller: AbortController | null = null
async function load() {
  const id = store.selectedWorkspaceId
  const currentGeneration = ++generation
  controller?.abort()
  audit.value = []
  if (!id || !store.can('audit.read')) { loading.value = false; return }
  controller = new AbortController()
  loading.value = true
  try {
    const result = await workspaceAPI.listAudit(id, { signal: controller.signal })
    if (currentGeneration === generation) audit.value = result.items || []
  } catch (error) {
    if (currentGeneration === generation) app.showError((error as { message?: string })?.message || t('workspace.loadError'))
  } finally { if (currentGeneration === generation) loading.value = false }
}
function formatDate(value?: string) { return value ? new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—' }
onMounted(load)
watch([() => store.selectedWorkspaceId, () => store.permissions], () => void load())
onBeforeUnmount(() => { ++generation; controller?.abort() })
</script>

<style scoped>
.workspace-page { padding-top: 20px; }.workspace-panel { border: 1px solid var(--color-border); border-radius: 12px; background: var(--color-surface); padding: 18px; box-shadow: var(--shadow-xs); }.workspace-panel__heading { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; }.workspace-panel h2 { margin: 0; color: var(--color-text-primary); font-size: 17px; }.workspace-panel p { margin: 5px 0 0; color: var(--color-text-secondary); font-size: 13px; }.workspace-table-wrap { overflow-x: auto; margin-top: 18px; }.workspace-table { width: 100%; min-width: 640px; border-collapse: collapse; }.workspace-table th, .workspace-table td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 10px; text-align: left; }.workspace-table th { color: var(--color-text-muted); font-size: 12px; text-transform: uppercase; }.workspace-table td { color: var(--color-text-secondary); font-size: 13px; }.workspace-state { padding: 32px 0; color: var(--color-text-muted); text-align: center; }
@media (max-width: 640px) { .workspace-panel__heading { align-items: stretch; flex-direction: column; } }
</style>
