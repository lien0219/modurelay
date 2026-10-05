<template>
  <div class="workspace-breakdowns">
    <section v-for="dimension in dimensions" :key="dimension.key" class="workspace-breakdown">
      <h3>{{ dimension.label }}</h3>
      <div v-if="dimension.rows.length" class="workspace-breakdown__table-wrap">
        <table>
          <caption class="sr-only">{{ dimension.label }}</caption>
          <thead><tr><th scope="col">{{ t('common.name') }}</th><th scope="col">{{ t('workspace.requests') }}</th><th scope="col">{{ t('workspace.spend') }}</th></tr></thead>
          <tbody><tr v-for="row in dimension.rows" :key="row.id"><td>{{ row.name || row.id || '—' }}</td><td>{{ row.requests }}</td><td>{{ formatMoney(row.spend) }}</td></tr></tbody>
        </table>
      </div>
      <p v-else>{{ t('workspace.noUsage') }}</p>
    </section>
    <section v-if="overview.service_accounts?.length" class="workspace-breakdown workspace-breakdown--machines" data-testid="service-account-breakdown">
      <h3>{{ t('serviceAccounts.breakdown') }}</h3>
      <div class="workspace-breakdown__table-wrap">
        <table>
          <caption class="sr-only">{{ t('serviceAccounts.breakdown') }}</caption>
          <thead><tr><th scope="col">{{ t('common.name') }}</th><th scope="col">{{ t('workspace.requests') }}</th><th scope="col">{{ t('workspace.spend') }}</th><th scope="col">{{ t('serviceAccounts.tokens') }}</th><th scope="col">{{ t('workspace.models') }}</th></tr></thead>
          <tbody><tr v-for="row in overview.service_accounts" :key="row.id">
            <td><RouterLink v-if="accountLink(row.id)" :to="accountLink(row.id)!">{{ row.name || row.id }}</RouterLink><template v-else>{{ row.id === 'legacy' ? t('serviceAccounts.legacy') : row.name || row.id }}</template></td>
            <td>{{ row.requests }}</td><td>{{ formatMoney(row.spend) }}</td><td>{{ row.id === 'legacy' ? '—' : row.tokens }}</td><td class="workspace-breakdown__models">{{ row.models?.join(', ') || '—' }}</td>
          </tr></tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { WorkspaceOverview } from '@/api/workspace'

const props = defineProps<{ overview: WorkspaceOverview; includeProjects?: boolean }>()
const { t } = useI18n()
const dimensions = computed(() => [
  ...(props.includeProjects ? [{ key: 'projects', label: t('workspace.projects'), rows: props.overview.projects || [] }] : []),
  { key: 'platforms', label: t('workspace.platforms'), rows: props.overview.platforms || [] },
  { key: 'models', label: t('workspace.models'), rows: props.overview.models || [] },
  { key: 'api_keys', label: t('workspace.apiKeys'), rows: props.overview.api_keys || [] },
])

function formatMoney(value: number) { return Number.isFinite(value) ? `$${value.toFixed(2)}` : '—' }
function accountLink(id: string): string | undefined {
  const summary = props.overview.summary
  return summary?.workspace_id && summary.project_id && /^[1-9]\d*$/.test(id)
    ? `/workspaces/${summary.workspace_id}/projects/${summary.project_id}/service-accounts/${id}`
    : undefined
}
</script>

<style scoped>
.workspace-breakdowns { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 20px; margin-top: 20px; }
.workspace-breakdown { min-width: 0; }
.workspace-breakdown--machines { grid-column: 1 / -1; }
.workspace-breakdown--machines table { min-width: 540px; }
.workspace-breakdown a { color: var(--color-primary); }
.workspace-breakdown td.workspace-breakdown__models { white-space: normal; overflow-wrap: anywhere; max-width: 280px; }
.workspace-breakdown h3 { margin: 0 0 10px; color: var(--color-text-primary); font-size: 16px; }
.workspace-breakdown p { margin: 0; color: var(--color-text-muted); font-size: 13px; }
.workspace-breakdown__table-wrap { overflow-x: auto; }
.workspace-breakdown table { width: 100%; min-width: 300px; border-collapse: collapse; }
.workspace-breakdown th, .workspace-breakdown td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 8px; font-size: 13px; text-align: right; }
.workspace-breakdown th { color: var(--color-text-muted); font-size: 12px; }
.workspace-breakdown td { color: var(--color-text-secondary); font-variant-numeric: tabular-nums; }
.workspace-breakdown th:first-child, .workspace-breakdown td:first-child { text-align: left; }
.workspace-breakdown td:first-child { overflow-wrap: anywhere; }
.workspace-breakdown td:not(:first-child) { white-space: nowrap; }
@media (max-width: 760px) { .workspace-breakdowns { grid-template-columns: 1fr; } }
</style>
