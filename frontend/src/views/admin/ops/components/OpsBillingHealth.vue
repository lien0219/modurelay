<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI, type OpsBillingHealth } from '@/api/admin/ops'
const { t } = useI18n()
const data = ref<OpsBillingHealth | null>(null)
const loading = ref(false)
const error = ref(false)
const pendingRecheck = ref<number | null>(null)
const notes = ref<Record<number, string>>({})
const gapNotes = ref<Record<string, string>>({})
const pendingAcknowledgment = ref<string | null>(null)
async function acknowledgeGap(instanceId: string) {
  const note = gapNotes.value[instanceId]?.trim()
  if (!note || pendingAcknowledgment.value != null) return
  pendingAcknowledgment.value = instanceId
  error.value = false
  try { await opsAPI.acknowledgeBillingRecordingGap(instanceId, note); await load() }
  catch { error.value = true }
  finally { pendingAcknowledgment.value = null }
}
async function recheck(id: number) {
  const note = notes.value[id]?.trim()
  if (!note || pendingRecheck.value != null) return
  pendingRecheck.value = id
  error.value = false
  try { await opsAPI.recheckBillingOperation(id, note); await load() }
  catch { error.value = true }
  finally { pendingRecheck.value = null }
}
let controller: AbortController | null = null
async function load() {
  controller?.abort()
  const active = new AbortController()
  controller = active
  loading.value = true
  error.value = false
  try { const response = await opsAPI.getBillingHealth({ signal: active.signal }); if (!active.signal.aborted) data.value = response }
  catch { if (!active.signal.aborted) { data.value = null; error.value = true } }
  finally { if (!active.signal.aborted) loading.value = false }
}
onMounted(load)
onBeforeUnmount(() => controller?.abort())
</script>
<template>
  <section class="mt-6 border-t border-[var(--color-border)] pt-5" aria-labelledby="billing-monitor-title">
    <div class="flex flex-wrap items-center justify-between gap-2"><h3 id="billing-monitor-title" class="text-base font-semibold">{{ t('admin.ops.monitoring.billing.title') }}</h3><button class="btn btn-sm btn-secondary" :disabled="loading" @click="load">{{ t('common.refresh') }}</button></div>
    <p class="mt-2 text-sm text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.billing.description') }}</p>
    <p class="mt-3 text-sm" role="status">{{ loading ? t('common.loading') : t(`admin.ops.monitoring.states.${data?.state ?? 'Unknown'}`) }}<span v-if="data?.checked_at"> · {{ new Date(data.checked_at).toLocaleString() }}</span></p>
    <p v-if="error" role="alert" class="mt-2 text-sm text-[var(--color-danger)]">{{ t('admin.ops.monitoring.queryFailed') }}</p>
    <dl class="mt-3 flex flex-wrap gap-6 text-sm"><div><dt>{{ t('admin.ops.monitoring.billing.missingUsage') }}</dt><dd class="mt-1 font-mono text-lg">{{ data?.missing_usage_logs ?? t('admin.ops.monitoring.states.Unknown') }}</dd></div><div><dt>{{ t('admin.ops.monitoring.billing.mismatches') }}</dt><dd class="mt-1 font-mono text-lg">{{ data?.amount_mismatches ?? t('admin.ops.monitoring.states.Unknown') }}</dd></div></dl>
    <template v-if="data?.operational_state">
      <p class="mt-4 text-sm" role="status">{{ t('admin.ops.monitoring.billing.operations') }} · {{ t(`admin.ops.monitoring.states.${data.operational_state}`) }}</p>
      <dl class="mt-3 flex flex-wrap gap-6 text-sm"><div v-for="key in ['failed_settlements', 'compensation_backlog', 'ledger_mismatches'] as const" :key="key"><dt>{{ t(`admin.ops.monitoring.metrics.billing_${key}`) }}</dt><dd class="mt-1 font-mono text-lg">{{ data[key] ?? t('admin.ops.monitoring.states.Unknown') }}</dd></div></dl>
    </template>
    <p v-if="data?.observation_unpersisted" role="alert" class="mt-3 text-sm text-[var(--color-warning)]">{{ t('admin.ops.monitoring.billing.unpersisted', { count: data.observation_unpersisted }) }}</p>
    <div v-if="data?.recording_gaps?.length" class="mt-4 overflow-x-auto"><table class="w-full text-left text-sm">
      <caption class="pb-2 text-left">{{ t('admin.ops.monitoring.billing.gapHint') }}</caption>
      <thead><tr><th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.instance') }}</th><th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.missed') }}</th><th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.acknowledged') }}</th><th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.acknowledge') }}</th></tr></thead>
      <tbody><tr v-for="gap in data.recording_gaps" :key="gap.instance_id" class="border-t border-[var(--color-border)]"><td class="max-w-64 break-all p-2 font-mono text-xs">{{ gap.instance_id }}<time class="mt-1 block" :datetime="gap.last_missed_at">{{ new Date(gap.last_missed_at).toLocaleString() }}</time></td><td class="p-2 font-mono">{{ gap.missed_count }}</td><td class="p-2 font-mono">{{ gap.acknowledged_count }}</td><td class="min-w-64 p-2"><form v-if="gap.missed_count > gap.acknowledged_count" class="flex flex-col gap-2" data-testid="recording-gap-form" @submit.prevent="acknowledgeGap(gap.instance_id)"><label class="text-xs">{{ t('admin.ops.monitoring.billing.note') }}<input v-model="gapNotes[gap.instance_id]" class="input mt-1" maxlength="1000" required /></label><button class="btn btn-sm btn-secondary" type="submit" :disabled="pendingAcknowledgment != null || !gapNotes[gap.instance_id]?.trim()">{{ t('admin.ops.monitoring.billing.acknowledge') }}</button></form><span v-else>{{ t('admin.ops.monitoring.billing.acknowledged') }}</span></td></tr></tbody>
    </table></div>
    <div v-if="data?.operations?.length" class="mt-4 overflow-x-auto"><table class="w-full text-left text-sm">
      <caption class="pb-2 text-left">{{ t('admin.ops.monitoring.billing.recheckHint') }}</caption>
      <thead><tr><th class="p-2">Request ID</th><th class="p-2">{{ t('admin.ops.monitoring.billing.operation') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.status') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.errorCode') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.recheck') }}</th></tr></thead>
      <tbody><tr v-for="operation in data.operations" :key="operation.id" class="border-t border-[var(--color-border)]"><td class="max-w-64 break-all p-2 font-mono text-xs">{{ operation.request_id }}</td><td class="p-2 font-mono">{{ operation.operation }}</td><td class="p-2">{{ t(`admin.ops.monitoring.billing.operationStates.${operation.state}`) }}</td><td class="p-2 font-mono">{{ operation.error_code || '—' }}</td><td class="min-w-64 p-2"><form v-if="operation.state === 'manual_required'" class="flex flex-col gap-2" @submit.prevent="recheck(operation.id)"><label class="text-xs">{{ t('admin.ops.monitoring.billing.note') }}<input v-model="notes[operation.id]" class="input mt-1" maxlength="1000" required /></label><button class="btn btn-sm btn-secondary" type="submit" :disabled="pendingRecheck != null || !notes[operation.id]?.trim()">{{ t('admin.ops.monitoring.billing.recheck') }}</button></form><span v-else>—</span></td></tr></tbody>
    </table></div>
    <div v-if="data?.ledger_findings?.length" class="mt-4 overflow-x-auto"><table class="w-full text-left text-sm">
      <caption class="pb-2 text-left">{{ t('admin.ops.monitoring.billing.ledger') }}</caption>
      <thead><tr><th class="p-2">{{ t('admin.ops.monitoring.billing.account') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.opening') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.expectedBalance') }}</th><th class="p-2">{{ t('admin.ops.monitoring.billing.actualBalance') }}</th></tr></thead>
      <tbody><tr v-for="finding in data.ledger_findings" :key="`${finding.account_type}/${finding.account_id}/${finding.field}`" class="border-t border-[var(--color-border)]"><td class="p-2 font-mono">{{ finding.account_type }} / {{ finding.account_id }} / {{ finding.field }}</td><td class="p-2 font-mono">{{ finding.opening_amount }}</td><td class="p-2 font-mono">{{ finding.expected_amount }}</td><td class="p-2 font-mono">{{ finding.actual_amount ?? t('admin.ops.monitoring.states.Unknown') }}</td></tr></tbody>
    </table></div>
    <p v-if="data?.truncated" class="mt-3 text-sm text-[var(--color-warning)]">{{ t('admin.ops.monitoring.billing.truncated') }}</p>
    <div v-if="data?.findings.length" class="mt-4 overflow-auto">
      <table class="w-full text-left text-sm">
        <caption class="pb-2 text-left text-[var(--color-text-secondary)]">{{ t('admin.ops.monitoring.billing.history') }}</caption>
        <thead><tr>
          <th scope="col" class="p-2">Request ID</th>
          <th scope="col" class="p-2">Settlement ID</th>
          <th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.status') }}</th>
          <th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.kind') }}</th>
          <th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.expected') }}</th>
          <th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.actual') }}</th>
          <th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.lastChecked') }}</th>
          <th scope="col" class="p-2">{{ t('admin.ops.monitoring.billing.resolvedAt') }}</th>
        </tr></thead>
        <tbody><tr v-for="finding in data.findings" :key="finding.id" class="border-t border-[var(--color-border)]">
          <td class="break-all p-2 font-mono">{{ finding.request_id }}</td>
          <td class="break-all p-2 font-mono">{{ finding.settlement_id }}</td>
          <td class="whitespace-nowrap p-2" :class="finding.resolved_at ? 'text-[var(--color-text-secondary)]' : 'text-[var(--color-warning)]'">
            {{ t(`admin.ops.monitoring.billing.${finding.resolved_at ? 'resolved' : 'active'}`) }}
          </td>
          <td class="p-2">{{ finding.kind }}</td>
          <td class="p-2 font-mono">{{ finding.expected_cost }}</td>
          <td class="p-2 font-mono">{{ finding.actual_cost ?? t('admin.ops.monitoring.states.Unknown') }}</td>
          <td class="whitespace-nowrap p-2"><time :datetime="finding.last_seen_at">{{ new Date(finding.last_seen_at).toLocaleString() }}</time></td>
          <td class="whitespace-nowrap p-2"><time v-if="finding.resolved_at" :datetime="finding.resolved_at">{{ new Date(finding.resolved_at).toLocaleString() }}</time><span v-else>—</span></td>
        </tr></tbody>
      </table>
    </div>
  </section>
</template>
