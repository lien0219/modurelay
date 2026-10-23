<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type OpsMonitoringQuery, type OpsKubernetesEventsResult } from '@/api/admin/ops'
const props = defineProps<{ query: OpsMonitoringQuery; enabled: boolean }>()
const { t } = useI18n()
const data = ref<OpsKubernetesEventsResult | null>(null)
const loading = ref(false)
const failed = ref(false)
const unsupported = computed(() => !!props.query.service && !props.query.deployment && !props.query.pod && !props.query.uid)
let controller: AbortController | null = null
async function load() {
  controller?.abort()
  const active = new AbortController()
  controller = active
  data.value = null
  failed.value = false
  if (!props.enabled || unsupported.value) { loading.value = false; return }
  loading.value = true
  const { cluster, namespace, pod, uid, start, end, deployment } = props.query
  const object = deployment && !pod && !uid ? { object_kind: 'Deployment', object_name: deployment } : {}
  try {
    const response = await opsAPI.getKubernetesEvents({ cluster, namespace, pod, uid, start, end, ...object, limit: 100 }, { signal: active.signal })
    if (!active.signal.aborted) data.value = response
  } catch { if (!active.signal.aborted) failed.value = true }
  finally { if (!active.signal.aborted) loading.value = false }
}
watch(() => [props.query, props.enabled], load, { immediate: true, deep: true })
onBeforeUnmount(() => controller?.abort())
</script>
<template>
  <section class="mt-6 border-t border-[var(--color-border)] pt-5" aria-labelledby="pod-events-title">
    <div class="flex flex-wrap items-center justify-between gap-2"><h3 id="pod-events-title" class="text-base font-semibold">{{ t('admin.ops.monitoring.events.title') }}</h3><button class="btn btn-sm btn-secondary" :disabled="loading || !enabled || unsupported" @click="load">{{ t('common.refresh') }}</button></div>
    <p class="mt-2 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.events.description') }}</p>
    <p v-if="!enabled" class="mt-3 text-sm">{{ t('admin.ops.monitoring.events.disabled') }}</p>
    <p v-else-if="unsupported" class="mt-3 text-sm">{{ t('admin.ops.monitoring.events.drillDown') }}</p>
    <template v-else>
      <p class="mt-3 text-sm" role="status">{{ loading ? t('common.loading') : t(`admin.ops.monitoring.states.${data?.state ?? 'Unknown'}`) }}<span v-if="data?.last_collected_at"> · {{ new Date(data.last_collected_at).toLocaleString() }}</span></p>
      <p v-if="failed" role="alert" class="mt-2 text-sm text-[var(--color-danger)]">{{ t('admin.ops.monitoring.queryFailed') }}</p>
      <p v-if="data?.state === 'Unknown'" class="mt-2 text-sm text-[var(--color-warning)]">{{ t('admin.ops.monitoring.events.stale') }}</p>
      <p v-if="data?.state === 'Available' && !data.events.length" class="mt-3 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.events.empty') }}</p>
      <div v-if="data?.events.length" class="mt-3 overflow-auto"><table class="w-full text-left text-sm"><thead><tr><th class="p-2">{{ t('admin.ops.monitoring.events.object') }}</th><th class="p-2">{{ t('admin.ops.monitoring.events.reason') }}</th><th class="p-2">{{ t('admin.ops.monitoring.events.message') }}</th><th class="p-2">{{ t('admin.ops.monitoring.events.count') }}</th><th class="p-2">{{ t('admin.ops.monitoring.events.lastSeen') }}</th></tr></thead>
        <tbody><tr v-for="event in data.events" :key="`${event.cluster}/${event.event_uid}`" class="border-t border-[var(--color-border)]"><td class="min-w-48 max-w-sm break-all p-2 font-mono text-xs">{{ event.cluster }} / {{ event.namespace }} / {{ event.object_kind || 'Pod' }} / {{ event.object_name || event.pod }}<div class="mt-1">{{ event.object_uid || event.pod_uid }}</div></td><td class="whitespace-nowrap p-2">{{ event.reason }}<div class="mt-1 text-xs">{{ event.type }}</div></td><td class="min-w-64 max-w-lg break-words p-2">{{ event.message }}<div class="mt-1 text-xs text-[var(--color-text-secondary)]">{{ event.source }}</div></td><td class="p-2 font-mono">{{ event.count }}</td><td class="whitespace-nowrap p-2">{{ new Date(event.last_seen_at).toLocaleString() }}</td></tr></tbody>
      </table></div>
    </template>
  </section>
</template>
