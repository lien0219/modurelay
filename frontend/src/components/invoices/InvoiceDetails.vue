<template>
  <div class="space-y-5">
    <div class="flex flex-wrap items-center justify-between gap-3 rounded-xl bg-[var(--color-surface-soft)] p-4">
      <div class="min-w-0">
        <p class="break-all font-mono text-xs text-[var(--color-text-secondary)]">{{ invoice.application_no }}</p>
        <p class="mt-1 text-xl font-semibold tabular-nums">{{ formatInvoiceAmount(invoice.amount_cents) }}</p>
      </div>
      <InvoiceStatusBadge :status="invoice.status" />
    </div>
    <dl class="grid grid-cols-1 gap-x-6 gap-y-4 text-sm sm:grid-cols-2">
      <div v-for="field in details" :key="field.key" class="min-w-0" :class="field.wide ? 'sm:col-span-2' : ''">
        <dt class="text-xs text-[var(--color-text-muted)]">{{ t(`invoices.fields.${field.key}`) }}</dt>
        <dd class="mt-1 whitespace-pre-wrap break-words text-[var(--color-text-primary)] [overflow-wrap:anywhere]">{{ field.value || '—' }}</dd>
      </div>
    </dl>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { InvoiceApplication } from '@/types/invoice'
import { formatInvoiceAmount } from '@/utils/invoice'
import { formatDateTime } from '@/utils/format'
import InvoiceStatusBadge from './InvoiceStatusBadge.vue'

const props = defineProps<{ invoice: InvoiceApplication; showUser?: boolean }>()
const { t } = useI18n()
const details = computed(() => [
  ...(props.showUser ? [{ key: 'user', value: [props.invoice.user_name, props.invoice.user_email, `#${props.invoice.user_id}`].filter(Boolean).join(' · '), wide: true }] : []),
  { key: 'invoice_type', value: t(`invoices.types.${props.invoice.invoice_type}`) },
  { key: 'title_type', value: t(`invoices.titleTypes.${props.invoice.title_type}`) },
  ...(['title', 'tax_number', 'email', 'address', 'phone', 'bank_name', 'bank_account', 'remark', 'invoice_number', 'admin_remark'] as const).map(key => ({
    key, value: props.invoice[key], wide: key === 'remark' || key === 'admin_remark' || key === 'address',
  })),
  { key: 'created_at', value: formatDateTime(props.invoice.created_at) },
  { key: 'updated_at', value: formatDateTime(props.invoice.updated_at) },
  { key: 'processed_at', value: props.invoice.processed_at ? formatDateTime(props.invoice.processed_at) : '' },
])
</script>
