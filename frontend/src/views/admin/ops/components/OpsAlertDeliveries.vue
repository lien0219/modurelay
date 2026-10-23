<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type OpsAlertDelivery } from '@/api/admin/ops'
const props = defineProps<{ eventId: number }>()
const { t } = useI18n()
const deliveries = ref<OpsAlertDelivery[]>([])
const loading = ref(false)
const error = ref(false)
const retrying = ref<number | null>(null)
let sequence = 0
async function load() {
  const request = ++sequence
  loading.value = true
  error.value = false
  deliveries.value = []
  try { const data = await opsAPI.listAlertDeliveries(props.eventId); if (request === sequence) deliveries.value = data }
  catch { if (request === sequence) error.value = true }
  finally { if (request === sequence) loading.value = false }
}
async function retry(id: number) {
  retrying.value = id
  error.value = false
  try { await opsAPI.retryAlertDelivery(id); await load() }
  catch { error.value = true }
  finally { retrying.value = null }
}
watch(() => props.eventId, load, { immediate: true })
onBeforeUnmount(() => { sequence++ })
</script>
<template>
  <section class="space-y-3 border-t border-[var(--color-border)] pt-4">
    <div class="flex flex-wrap items-center justify-between gap-2"><h4 class="text-sm font-semibold">{{ t('admin.ops.monitoring.deliveries.title') }}</h4><button class="btn btn-sm btn-secondary" :disabled="loading" @click="load">{{ t('common.refresh') }}</button></div>
    <p v-if="error" role="alert" class="text-sm text-[var(--color-danger)]">{{ t('admin.ops.monitoring.deliveries.failed') }}</p>
    <p v-if="loading" role="status" class="text-sm">{{ t('common.loading') }}</p>
    <p v-else-if="!deliveries.length && !error" class="text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.deliveries.empty') }}</p>
    <div v-else class="overflow-x-auto"><table class="w-full text-left text-sm"><thead><tr><th class="p-2">{{ t('admin.ops.monitoring.deliveries.target') }}</th><th class="p-2">{{ t('admin.ops.monitoring.deliveries.status') }}</th><th class="p-2">{{ t('admin.ops.monitoring.deliveries.attempts') }}</th><th class="p-2">{{ t('admin.ops.monitoring.deliveries.nextAttempt') }}</th><th class="p-2">{{ t('admin.ops.monitoring.deliveries.actions') }}</th></tr></thead>
      <tbody><tr v-for="delivery in deliveries" :key="delivery.id" class="border-t border-[var(--color-border)]"><td class="max-w-xs break-all p-2"><span class="font-mono">{{ delivery.target }}</span><p class="mt-1 text-xs text-[var(--color-text-secondary)]">{{ delivery.channel }} · {{ t(`admin.ops.monitoring.deliveries.transitions.${delivery.transition}`) }}</p></td><td class="p-2">{{ t(`admin.ops.monitoring.deliveries.states.${delivery.status}`) }}<p v-if="delivery.last_error" class="mt-1 max-w-xs break-words text-xs text-[var(--color-danger)]">{{ delivery.last_error }}</p><details v-if="delivery.attempt_history?.length" class="mt-2 text-xs"><summary class="cursor-pointer py-1">{{ t('admin.ops.monitoring.deliveries.history') }}</summary><ol class="mt-2 space-y-2"><li v-for="(attempt, index) in delivery.attempt_history" :key="index" class="max-w-sm break-words">{{ new Date(attempt.completed_at).toLocaleString() }} · #{{ attempt.attempt }} · {{ attempt.status }}<p v-if="attempt.error">{{ attempt.error }}</p></li></ol></details></td><td class="p-2 font-mono">{{ delivery.attempts }}</td><td class="whitespace-nowrap p-2">{{ delivery.next_attempt_at ? new Date(delivery.next_attempt_at).toLocaleString() : '—' }}</td><td class="p-2"><button v-if="delivery.status === 'failed'" class="btn btn-sm btn-secondary" :disabled="retrying !== null" @click="retry(delivery.id)">{{ t('admin.ops.monitoring.deliveries.retry') }}</button></td></tr></tbody>
    </table></div>
  </section>
</template>
