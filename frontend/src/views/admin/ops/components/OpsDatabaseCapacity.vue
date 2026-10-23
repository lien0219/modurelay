<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { OpsDatabaseCapacity } from '@/api/admin/ops'
defineProps<{ capacity?: OpsDatabaseCapacity | null }>()
const { t } = useI18n()
</script>
<template>
  <section class="mt-4 border-t border-[var(--color-border)] pt-4" aria-labelledby="db-capacity-title">
    <h3 id="db-capacity-title" class="text-base font-semibold text-[var(--color-text-primary)]">{{ t('admin.ops.monitoring.capacity.title') }}</h3>
    <p class="mt-1 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.capacity.description') }}</p>
    <div class="mt-3 overflow-x-auto"><table class="w-full text-left text-sm"><thead class="bg-[var(--color-surface-soft)]"><tr><th class="p-3">{{ t('admin.ops.monitoring.capacity.instance') }}</th><th class="p-3">{{ t('admin.ops.monitoring.capacity.usage') }}</th><th class="p-3">{{ t('admin.ops.monitoring.capacity.poolBudget') }}</th><th class="p-3">{{ t('admin.ops.monitoring.capacity.source') }}</th></tr></thead><tbody>
      <tr v-for="key in ['postgresql', 'redis'] as const" :key="key" class="border-t border-[var(--color-border)]"><td class="p-3 font-medium">{{ key === 'postgresql' ? 'PostgreSQL' : 'Redis' }}</td><td class="whitespace-nowrap p-3"><span v-if="capacity?.[key].state === 'Available'" class="font-mono">{{ capacity[key].used ?? '—' }} / {{ capacity[key].limit ?? '—' }}</span><span v-else>{{ t('admin.ops.monitoring.states.Unknown') }}</span></td><td class="p-3 font-mono">{{ capacity?.[`${key}_pool_limit`] ?? '—' }}</td><td class="min-w-48 break-words p-3 text-xs">{{ capacity?.[key].source || t('admin.ops.monitoring.states.Unknown') }}</td></tr>
    </tbody></table></div>
    <div v-if="capacity?.redis.nodes?.length" class="mt-4 overflow-x-auto"><table class="w-full text-left text-sm">
      <caption class="mb-2 text-left font-medium">{{ t('admin.ops.monitoring.capacity.redisNodes') }}</caption>
      <thead><tr><th class="p-2">{{ t('admin.ops.monitoring.capacity.instance') }}</th><th class="p-2">{{ t('admin.ops.monitoring.capacity.usage') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.status') }}</th></tr></thead>
      <tbody><tr v-for="node in capacity.redis.nodes" :key="node.node" class="border-t border-[var(--color-border)]"><td class="p-2 font-mono">{{ node.node }}</td><td class="p-2 font-mono">{{ node.state === 'Available' ? `${node.used} / ${node.limit}` : '—' }}</td><td class="p-2">{{ t(`admin.ops.monitoring.states.${node.state}`) }}</td></tr></tbody>
    </table></div>
    <div v-if="capacity?.database_pools?.length" class="mt-4 overflow-x-auto"><table class="w-full text-left text-sm">
      <caption class="mb-2 text-left font-medium">{{ t('admin.ops.monitoring.capacity.pools') }}</caption>
      <thead><tr><th class="p-2">{{ t('admin.ops.monitoring.capacity.instance') }}</th><th class="p-2">{{ t('admin.ops.monitoring.capacity.poolRole') }}</th><th class="p-2">{{ t('admin.ops.monitoring.capacity.poolUsage') }}</th><th class="p-2">{{ t('admin.ops.monitoring.capacity.waits') }}</th></tr></thead>
      <tbody><tr v-for="pool in capacity.database_pools" :key="`${pool.instance}/${pool.role}`" class="border-t border-[var(--color-border)]"><td class="max-w-64 break-all p-2 font-mono text-xs">{{ pool.instance }}</td><td class="p-2">{{ t(`admin.ops.monitoring.capacity.roles.${pool.role}`) }}</td><td class="p-2 font-mono">{{ pool.active }} / {{ pool.idle }} / {{ pool.limit === 0 ? t('admin.ops.monitoring.capacity.unlimited') : pool.limit }}</td><td class="p-2 font-mono">{{ pool.wait_count }} / {{ pool.wait_seconds.toLocaleString(undefined, { maximumFractionDigits: 3 }) }} s</td></tr></tbody>
    </table></div>
  </section>
</template>
