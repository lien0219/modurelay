<template>
  <AppLayout>
    <section class="workspace-frame">
      <header class="workspace-frame__header">
        <div class="min-w-0">
          <p class="workspace-frame__eyebrow">{{ t('workspace.eyebrow') }}</p>
          <h1 class="workspace-frame__title">{{ selectedWorkspace?.name || t('workspace.title') }}</h1>
          <p class="workspace-frame__description">{{ t('workspace.description') }}</p>
        </div>
        <div class="workspace-frame__selectors" aria-label="Workspace context">
          <label class="workspace-frame__selector">
            <span>{{ t('workspace.workspace') }}</span>
            <select :value="selectedWorkspaceId || ''" :disabled="store.loading" @change="switchWorkspace">
              <option v-for="item in store.workspaces" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
          <label class="workspace-frame__selector">
            <span>{{ t('workspace.project') }}</span>
            <select :value="selectedProjectId || ''" :disabled="store.projectsLoading || !store.selectedWorkspace" @change="switchProject">
              <option v-for="item in store.projects" :key="item.id" :value="item.id">{{ item.name }}</option>
            </select>
          </label>
        </div>
      </header>

      <nav class="workspace-frame__tabs" :aria-label="t('workspace.sections')">
        <RouterLink v-for="item in tabs" :key="item.to" :to="item.to" class="workspace-frame__tab" :class="{ 'is-active': route.path === item.to || route.path.startsWith(`${item.to}/`) }">
          {{ item.label }}
        </RouterLink>
      </nav>

      <div v-if="store.error" class="workspace-frame__error" role="alert">
        <span>{{ t('workspace.loadError') }}</span>
        <button type="button" class="btn btn-secondary btn-sm" @click="reload">{{ t('common.retry') }}</button>
      </div>
      <div v-if="store.loading && !store.workspaces.length" class="workspace-frame__loading" role="status">{{ t('common.loading') }}</div>
      <div v-else-if="invalidRoute" class="workspace-frame__error" role="alert">{{ t('workspace.inaccessibleWorkspace') }}</div>
      <slot v-else />
    </section>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useWorkspaceStore } from '@/stores/workspace'

const props = withDefaults(defineProps<{ section?: string }>(), { section: 'overview' })
const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const store = useWorkspaceStore()
const selectedWorkspace = computed(() => store.selectedWorkspace)
const selectedWorkspaceId = computed(() => store.selectedWorkspaceId)
const selectedProjectId = computed(() => store.selectedProjectId)
const invalidRoute = computed(() => {
  const id = Number(route.params.workspaceId)
  return id > 0 && !store.loading && !store.workspaces.some(item => item.id === id)
})

const tabs = computed(() => {
  const id = selectedWorkspaceId.value || route.params.workspaceId || ''
  return [
    { to: `/workspaces/${id}/overview`, label: t('workspace.overview') },
    { to: `/workspaces/${id}/projects`, label: t('workspace.projects') },
    { to: `/workspaces/${id}/members`, label: t('workspace.members') },
    { to: `/workspaces/${id}/invitations`, label: t('workspace.invitations') },
    { to: `/workspaces/${id}/finops`, label: t('workspace.finops') },
    { to: `/workspaces/${id}/audit`, label: t('workspace.audit') },
    ...(store.can('webhook.read') ? [{ to: `/workspaces/${id}/webhooks`, label: t('workspace.webhooks') }] : []),
    ...(store.can('service_account.read') && route.params.projectId ? [{ to: `/workspaces/${id}/projects/${route.params.projectId}/service-accounts`, label: t('serviceAccounts.title') }] : []),
  ]
})

async function reload() {
  await store.loadWorkspaces()
}

async function switchWorkspace(event: Event) {
  const id = Number((event.target as HTMLSelectElement).value)
  if (!id) return
  await store.selectWorkspace(id)
  await router.push(`/workspaces/${id}/${props.section}`)
}

async function switchProject(event: Event) {
  const id = Number((event.target as HTMLSelectElement).value)
  if (!id) return
  await store.selectProject(id)
  if (route.params.projectId && store.selectedWorkspaceId) await router.push(`/workspaces/${store.selectedWorkspaceId}/projects/${id}`)
}

onMounted(async () => {
  if (!store.workspaces.length) await store.loadWorkspaces()
  if (route.name === 'WorkspaceRoot' && store.selectedWorkspaceId) {
    await router.replace(`/workspaces/${store.selectedWorkspaceId}/${props.section}`)
    return
  }
  const routeWorkspaceId = Number(route.params.workspaceId)
  if (routeWorkspaceId && store.selectedWorkspaceId !== routeWorkspaceId && store.workspaces.some(item => item.id === routeWorkspaceId)) await store.selectWorkspace(routeWorkspaceId)
})

watch(() => route.params.workspaceId, async (value) => {
  const id = Number(value)
  if (id && id !== store.selectedWorkspaceId && store.workspaces.some(item => item.id === id)) await store.selectWorkspace(id)
})
</script>

<style scoped>
.workspace-frame { width: 100%; max-width: 1440px; min-width: 0; box-sizing: border-box; margin: 0 auto; }
.workspace-frame__header { display: flex; min-width: 0; align-items: flex-end; justify-content: space-between; gap: 24px; padding: 8px 0 20px; }
.workspace-frame__header > * { min-width: 0; }
.workspace-frame__eyebrow { margin: 0 0 5px; color: var(--color-text-muted); font-size: 12px; font-weight: 700; letter-spacing: .08em; text-transform: uppercase; }
.workspace-frame__title { margin: 0; color: var(--color-text-primary); font-size: 24px; font-weight: 700; overflow-wrap: anywhere; }
.workspace-frame__description { max-width: 620px; margin: 6px 0 0; color: var(--color-text-secondary); font-size: 14px; }
.workspace-frame__selectors { display: flex; flex-wrap: wrap; gap: 10px; min-width: min(100%, 420px); justify-content: flex-end; }
.workspace-frame__selector { display: grid; gap: 5px; min-width: 180px; color: var(--color-text-muted); font-size: 11px; font-weight: 700; text-transform: uppercase; }
.workspace-frame__selector select { min-width: 0; height: 38px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface); color: var(--color-text-primary); padding: 0 30px 0 10px; }
.workspace-frame__tabs { display: flex; gap: 4px; overflow-x: auto; border-bottom: 1px solid var(--color-border); padding-bottom: 0; }
.workspace-frame__tab { flex: 0 0 auto; border-bottom: 2px solid transparent; padding: 10px 12px; color: var(--color-text-secondary); font-size: 13px; text-decoration: none; }
.workspace-frame__tab:hover, .workspace-frame__tab:focus-visible { color: var(--color-primary); }
.workspace-frame__tab.is-active { border-color: var(--color-primary); color: var(--color-primary); font-weight: 700; }
.workspace-frame__error { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-top: 18px; border: 1px solid color-mix(in srgb, var(--color-danger) 30%, var(--color-border)); border-radius: 10px; background: var(--color-surface); color: var(--color-danger); padding: 12px 14px; }
.workspace-frame__loading { padding: 48px 0; color: var(--color-text-muted); text-align: center; }
@media (max-width: 900px) { .workspace-frame__header { align-items: stretch; flex-direction: column; gap: 16px; } .workspace-frame__selectors { justify-content: stretch; min-width: 0; } .workspace-frame__selector { flex: 1 1 180px; } }
</style>
