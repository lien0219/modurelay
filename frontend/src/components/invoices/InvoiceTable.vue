<template>
  <DataTable :columns="columns" :data="items" :loading="loading">
    <template #cell-application_no="{ value }">
      <span class="break-all font-mono text-xs sm:whitespace-nowrap">{{ value }}</span>
    </template>
    <template #cell-user_email="{ row }">
      <div class="min-w-40 max-w-64 break-words">
        <p>{{ row.user_email || `#${row.user_id}` }}</p>
        <p v-if="row.user_name" class="text-xs text-[var(--color-text-muted)]">{{ row.user_name }}</p>
      </div>
    </template>
    <template #cell-title="{ value }"><span class="inline-block min-w-32 max-w-64 break-words">{{ value }}</span></template>
    <template #cell-amount_cents="{ value }"><span class="whitespace-nowrap font-medium tabular-nums">{{ formatInvoiceAmount(value) }}</span></template>
    <template #cell-status="{ value }"><InvoiceStatusBadge :status="value" /></template>
    <template #cell-created_at="{ value }"><span class="whitespace-nowrap text-xs">{{ formatDateTime(value) }}</span></template>
    <template #cell-actions="{ row }"><slot name="actions" :row="row" /></template>
  </DataTable>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import type { InvoiceApplication } from '@/types/invoice'
import { formatDateTime } from '@/utils/format'
import { formatInvoiceAmount } from '@/utils/invoice'
import InvoiceStatusBadge from './InvoiceStatusBadge.vue'

const props = defineProps<{ items: InvoiceApplication[]; loading: boolean; showUser?: boolean }>()
const { t } = useI18n()
const columns = computed<Column[]>(() => [
  { key: 'application_no', label: t('invoices.fields.application_no') },
  ...(props.showUser ? [{ key: 'user_email', label: t('invoices.fields.user') }] : []),
  { key: 'title', label: t('invoices.fields.title') },
  { key: 'amount_cents', label: t('invoices.fields.amount') },
  { key: 'status', label: t('invoices.fields.status') },
  { key: 'created_at', label: t('invoices.fields.created_at') },
  { key: 'actions', label: t('common.actions') },
])
</script>
