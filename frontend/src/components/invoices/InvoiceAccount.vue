<template>
  <section class="space-y-4 border-t border-[var(--color-border)] pt-5" :aria-busy="loading">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h4 class="text-base font-semibold">{{ t('invoices.accountTitle') }}</h4>
      <button class="btn btn-secondary" :disabled="loading" :aria-expanded="expanded" aria-controls="invoice-account-records" data-testid="invoice-account-toggle" @click="toggleAccount">{{ expanded ? t('invoices.hideAccount') : t('invoices.viewAccount') }}</button>
    </div>
    <div v-if="expanded" id="invoice-account-records" class="space-y-3">
      <div v-if="account" class="flex flex-wrap items-center justify-between gap-3 rounded-lg bg-[var(--color-surface-soft)] p-3">
        <p class="min-w-0 break-words text-sm [overflow-wrap:anywhere]">{{ account.user.email }} <span v-if="account.user.username" class="text-[var(--color-text-secondary)]">· {{ account.user.username }}</span></p>
        <p class="text-sm">{{ t('invoices.walletBalance') }} <strong class="tabular-nums">{{ formatPaymentAmount(account.user.balance, 'USD') }} USD</strong></p>
      </div>
      <p class="text-xs text-[var(--color-text-muted)]">{{ t('invoices.accountHelp') }}</p>
      <p v-if="error" class="text-sm text-[var(--color-danger)]" role="alert">{{ error }} <button class="btn btn-secondary btn-sm" :disabled="loading" @click="loadAccount">{{ t('invoices.retry') }}</button></p>
      <OrderTable :orders="account?.orders.items || []" :loading="loading" />
      <Pagination v-if="account && account.orders.total > 0" :page="page" :total="account.orders.total" :page-size="pageSize" @update:page="changePage" @update:page-size="changePageSize" />
    </div>
  </section>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { InvoiceAccount } from '@/types/invoice'
import OrderTable from '@/components/payment/OrderTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import { formatPaymentAmount } from '@/components/payment/currency'
import { extractI18nErrorMessage } from '@/utils/apiError'

const props = defineProps<{ load: (params: { page: number; page_size: number }) => Promise<{ data: InvoiceAccount }> }>()
const { t } = useI18n()
const expanded = ref(false)
const account = ref<InvoiceAccount | null>(null)
const loading = ref(false)
const error = ref('')
const page = ref(1)
const pageSize = ref(10)
let request = 0
async function loadAccount() {
  const current = ++request
  loading.value = true
  error.value = ''
  try {
    const { data } = await props.load({ page: page.value, page_size: pageSize.value })
    if (request === current) account.value = data
  } catch (cause) { if (request === current) error.value = extractI18nErrorMessage(cause, t, 'invoices.errors', t('invoices.loadFailed')) }
  finally { if (request === current) loading.value = false }
}
function toggleAccount() { expanded.value = !expanded.value; if (expanded.value) void loadAccount() }
function changePage(value: number) { page.value = value; void loadAccount() }
function changePageSize(value: number) { pageSize.value = value; changePage(1) }
onBeforeUnmount(() => { request++ })
</script>
