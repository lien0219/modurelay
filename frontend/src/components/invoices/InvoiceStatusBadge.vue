<template>
  <span class="inline-flex whitespace-nowrap rounded-full px-2.5 py-1 text-xs font-medium" :class="colors[status]">
    {{ t(`invoices.status.${status}`) }}
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { InvoiceStatus } from '@/types/invoice'

defineProps<{ status: InvoiceStatus }>()
const { t } = useI18n()
const colors: Record<InvoiceStatus, string> = {
  pending: 'bg-[var(--color-surface-soft)] text-[var(--color-text-secondary)]',
  processing: 'bg-[var(--color-primary-soft)] text-[var(--color-info)]',
  issued: 'bg-[var(--color-surface-soft)] text-[var(--color-success)]',
  rejected: 'bg-[var(--color-surface-soft)] text-[var(--color-danger)]',
  cancelled: 'bg-[var(--color-surface-soft)] text-[var(--color-text-muted)]',
}
</script>
