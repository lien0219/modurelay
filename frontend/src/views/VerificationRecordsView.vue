<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-[1800px] space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">{{ t(isAdmin ? 'verificationRecords.adminEyebrow' : 'verificationRecords.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t(isAdmin ? 'verificationRecords.adminTitle' : 'verificationRecords.title') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t(isAdmin ? 'verificationRecords.adminDescription' : 'verificationRecords.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadRecords">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" />
        </button>
      </header>

      <section class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5 2xl:grid-cols-8" :aria-label="t('verificationRecords.summary.title')">
        <div v-for="item in summaryItems" :key="item.key" class="card min-w-0 p-4">
          <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</div>
          <div class="mt-1 break-words font-semibold tabular-nums text-gray-900 dark:text-white" :class="item.compact ? 'text-base leading-6' : 'text-2xl'">{{ item.value }}</div>
        </div>
      </section>

      <form class="glass-panel rounded-xl p-4" @submit.prevent="applyFilters">
        <div class="grid items-end gap-3 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-[150px_170px_minmax(190px,1fr)_minmax(190px,1fr)_minmax(230px,1.2fr)_160px_160px_auto]">
          <Select v-model="draftFilters.type" :label="t('verificationRecords.filters.type')" :options="typeOptions" clearable />
          <Select v-model="draftFilters.outcome" :label="t('verificationRecords.filters.outcome')" :options="outcomeOptions" clearable />
          <Select
            v-model="draftFilters.platform"
            :label="t('verificationRecords.filters.platform')"
            :options="platformOptions"
            :placeholder="t('verificationRecords.filters.allPlatforms')"
            :search-placeholder="t('verificationRecords.filters.searchPlatform')"
            :loading="platformCatalog.loading"
            :has-more="platformCatalog.page < platformCatalog.pages"
            searchable remote clearable
            @search="query => searchCatalog('platform', query)"
            @load-more="loadMoreCatalog('platform')"
          >
            <template #selected="{ option }"><VerificationIdentity v-if="option" kind="platform" :code="String(option.value)" :label="String(option.label)" :icon="String(option.icon || '')" :show-code="false" /><span v-else>{{ t('verificationRecords.filters.allPlatforms') }}</span></template>
            <template #option="{ option }"><VerificationIdentity kind="platform" :code="String(option.value)" :label="String(option.label)" :icon="String(option.icon || '')" /></template>
          </Select>
          <Select
            v-model="draftFilters.country"
            :label="t('verificationRecords.filters.country')"
            :options="countryOptions"
            :placeholder="t('verificationRecords.filters.allCountries')"
            :search-placeholder="t('verificationRecords.filters.searchCountry')"
            :loading="countryCatalog.loading"
            :has-more="countryCatalog.page < countryCatalog.pages"
            :disabled="draftFilters.type === 'email'"
            searchable remote clearable
            @search="query => searchCatalog('country', query)"
            @load-more="loadMoreCatalog('country')"
          >
            <template #selected="{ option }"><VerificationIdentity v-if="option" kind="country" :code="String(option.value)" :label="String(option.label)" :show-code="false" /><span v-else>{{ t('verificationRecords.filters.allCountries') }}</span></template>
            <template #option="{ option }"><VerificationIdentity kind="country" :code="String(option.value)" :label="String(option.label)" /></template>
          </Select>
          <label class="block min-w-0"><span class="input-label">{{ t('verificationRecords.filters.keyword') }}</span><input v-model.trim="draftFilters.keyword" class="input h-[42px]" :placeholder="t(isAdmin ? 'verificationRecords.filters.adminKeywordPlaceholder' : 'verificationRecords.filters.keywordPlaceholder')" /></label>
          <label class="block min-w-0"><span class="input-label">{{ t('verificationRecords.filters.from') }}</span><input v-model="draftFilters.created_from" type="date" class="input h-[42px]" /></label>
          <label class="block min-w-0"><span class="input-label">{{ t('verificationRecords.filters.to') }}</span><input v-model="draftFilters.created_to" type="date" class="input h-[42px]" /></label>
          <div class="flex h-[42px] items-center gap-2 self-end">
            <button type="submit" class="btn btn-primary h-[42px] shrink-0 whitespace-nowrap" :disabled="loading"><Icon name="search" size="sm" aria-hidden="true" /><span>{{ t('common.search') }}</span></button>
            <button type="button" class="btn btn-secondary h-[42px] shrink-0 whitespace-nowrap" :disabled="loading" @click="resetFilters"><Icon name="eraser" size="sm" aria-hidden="true" /><span>{{ t('common.reset') }}</span></button>
          </div>
        </div>
      </form>

      <VerificationAnalytics :analytics="analytics" :admin="isAdmin" :verification-type="activeFilters.type" :platform-labels="platformLabels" :country-labels="countryLabels" />

      <section class="card min-w-0 overflow-hidden" :aria-busy="loading">
        <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-5 py-4 dark:border-dark-700">
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('verificationRecords.tableTitle') }}</h2>
          <span class="text-sm tabular-nums text-gray-500 dark:text-gray-400">{{ t('verificationRecords.resultCount', { count: pagination.total }) }}</span>
        </div>

        <div v-if="isAdmin" class="mx-4 mt-4">
          <BulkActionBar :show="selectedRecords.length > 0" :selected-count="selectedRecords.length">
            <button type="button" class="btn btn-secondary btn-sm" @click="exportSelected"><Icon name="download" size="sm" aria-hidden="true" />{{ t('verificationRecords.bulk.export') }}</button>
            <button type="button" class="btn btn-ghost btn-sm" @click="clearSelection">{{ t('verificationRecords.bulk.clear') }}</button>
          </BulkActionBar>
        </div>

        <div v-if="isAdmin" class="max-w-full overflow-x-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500" tabindex="0" :aria-label="t('verificationRecords.adminTableLabel')">
          <table class="min-w-[3300px] text-left text-sm">
            <thead class="whitespace-nowrap bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="w-12 px-4 py-3"><input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :checked="allPageSelected" :aria-label="t('verificationRecords.bulk.selectPage')" @change="toggleCurrentPage" /></th>
                <th class="px-4 py-3">{{ t('verificationRecords.columns.createdAt') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.type') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.user') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.orderNo') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.platform') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.country') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.channel') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.target') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.provider') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.result') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.refund') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.saleAmount') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.providerCost') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.userDebit') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.reserved') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.captured') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.released') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.refundedAmount') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.requests') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.error') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading && records.length === 0"><td colspan="21" class="px-5 py-12 text-center text-gray-500">{{ t('common.loading') }}</td></tr>
              <tr v-else-if="records.length === 0"><td colspan="21" class="px-5 py-12 text-center text-gray-500">{{ t('verificationRecords.empty') }}</td></tr>
              <tr v-for="record in records" :key="record.id" class="border-t border-gray-100 align-top dark:border-dark-700">
                <td class="px-4 py-3"><input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500" :checked="selectedIds.has(record.id)" :aria-label="`${t('verificationRecords.columns.select')} ${record.order_no}`" @change="toggleRecord(record)" /></td>
                <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(record.created_at) }}</td><td class="whitespace-nowrap px-4 py-3"><span class="badge badge-info">{{ typeLabel(record.verification_type) }}</span></td><td class="px-4 py-3"><div class="min-w-52 font-medium text-gray-900 dark:text-white">{{ record.user_email || '-' }}</div><div class="text-xs text-gray-500">ID {{ record.user_id }}</div></td><td class="whitespace-nowrap px-4 py-3 font-mono text-xs">{{ record.order_no }}</td>
                <td class="px-4 py-3"><VerificationIdentity kind="platform" :code="record.service_code" :label="platformLabel(record.service_code)" /></td><td class="px-4 py-3"><VerificationIdentity v-if="record.verification_type === 'sms' && record.region" kind="country" :code="record.region" :label="countryLabel(record.region)" /><span v-else class="text-gray-400">-</span></td>
                <td class="px-4 py-3"><div class="min-w-44">{{ record.channel_name }}</div><div class="font-mono text-xs text-gray-500">{{ record.channel_code }}</div></td><td class="px-4 py-3 font-mono text-xs"><div class="min-w-52 break-all">{{ record.target || '-' }}</div></td><td class="whitespace-nowrap px-4 py-3 font-mono text-xs">{{ record.provider_code || '-' }}</td><td class="px-4 py-3"><span class="badge whitespace-nowrap" :class="outcomeClass(record.outcome)">{{ outcomeLabel(record.outcome) }}</span><div class="mt-1 whitespace-nowrap font-mono text-xs text-gray-500">{{ record.status }}</div></td><td class="px-4 py-3"><div class="whitespace-nowrap">{{ refundStatusLabel(record.refund_status) }}</div><div v-if="record.refund_reason" class="mt-1 max-w-52 text-xs text-gray-500">{{ refundReasonLabel(record.refund_reason) }}</div></td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.sale_amount, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.provider_cost, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.user_debit_amount, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.reserved_amount, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.captured_amount, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.released_amount, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.refunded_amount, record.currency) }}</td><td class="px-4 py-3 text-center tabular-nums">{{ record.provider_request_count }}</td><td class="px-4 py-3"><div class="min-w-72 font-mono text-xs text-red-600 dark:text-red-300">{{ record.error_code || '-' }}</div><div v-if="record.error_message" class="mt-1 max-w-96 whitespace-normal break-words text-xs text-gray-500 dark:text-gray-400">{{ record.error_message }}</div></td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-else class="max-w-full overflow-x-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500" tabindex="0" :aria-label="t('verificationRecords.userTableLabel')">
          <table class="w-full min-w-[1750px] text-left text-sm">
            <thead class="whitespace-nowrap bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400"><tr><th class="px-4 py-3">{{ t('verificationRecords.columns.createdAt') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.type') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.orderNo') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.platform') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.country') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.channel') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.target') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.result') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.amount') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.refundedAmount') }}</th><th class="px-4 py-3">{{ t('verificationRecords.columns.note') }}</th></tr></thead>
            <tbody>
              <tr v-if="loading && records.length === 0"><td colspan="11" class="px-5 py-12 text-center text-gray-500">{{ t('common.loading') }}</td></tr><tr v-else-if="records.length === 0"><td colspan="11" class="px-5 py-12 text-center text-gray-500">{{ t('verificationRecords.empty') }}</td></tr>
              <tr v-for="record in records" :key="record.id" class="border-t border-gray-100 dark:border-dark-700"><td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500 dark:text-gray-400">{{ formatDate(record.created_at) }}</td><td class="whitespace-nowrap px-4 py-3"><span class="badge badge-info">{{ typeLabel(record.verification_type) }}</span></td><td class="whitespace-nowrap px-4 py-3 font-mono text-xs">{{ record.order_no }}</td><td class="px-4 py-3"><VerificationIdentity kind="platform" :code="record.service_code" :label="platformLabel(record.service_code)" /></td><td class="px-4 py-3"><VerificationIdentity v-if="record.verification_type === 'sms' && record.region" kind="country" :code="record.region" :label="countryLabel(record.region)" /><span v-else class="text-gray-400">-</span></td><td class="px-4 py-3"><div class="min-w-44">{{ record.channel_name }}</div><div class="font-mono text-xs text-gray-500">{{ record.channel_code }}</div></td><td class="px-4 py-3 font-mono text-xs"><div class="min-w-52 break-all">{{ record.target || '-' }}</div></td><td class="px-4 py-3"><span class="badge whitespace-nowrap" :class="outcomeClass(record.outcome)">{{ outcomeLabel(record.outcome) }}</span></td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.sale_amount, record.currency) }}</td><td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ money(record.refunded_amount, record.currency) }}</td><td class="px-4 py-3"><div class="max-w-64 whitespace-normal break-words text-xs text-gray-500 dark:text-gray-400">{{ record.public_error_message || '-' }}</div></td></tr>
            </tbody>
          </table>
        </div>

        <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.pageSize" @update:page="changePage" @update:page-size="changePageSize" />
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { saveAs } from 'file-saver'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import BulkActionBar from '@/components/common/BulkActionBar.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import VerificationAnalytics from '@/components/verification/VerificationAnalytics.vue'
import VerificationIdentity from '@/components/verification/VerificationIdentity.vue'
import verificationRecordsAPI, { type VerificationOutcome, type VerificationRecord, type VerificationRecordAnalytics, type VerificationRecordOption, type VerificationRecordQuery, type VerificationRecordSummary, type VerificationType } from '@/api/verificationRecords'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCurrency, formatDate } from '@/utils/format'

const { t, locale } = useI18n()
const route = useRoute()
const appStore = useAppStore()
const isAdmin = computed(() => route.meta.requiresAdmin === true)
const outcomes: VerificationOutcome[] = ['processing', 'success', 'failed', 'refunded', 'cancelled', 'expired']
const typeOptions = computed(() => [{ value: 'sms', label: t('verificationRecords.types.sms') }, { value: 'email', label: t('verificationRecords.types.email') }])
const outcomeOptions = computed(() => outcomes.map(value => ({ value, label: t(`verificationRecords.outcomes.${value}`) })))
const emptySummary: VerificationRecordSummary = { total: 0, processing: 0, success: 0, failed: 0, refunded: 0, cancelled: 0, expired: 0, sale_amount: 0, user_debit_amount: 0, reserved_amount: 0, captured_amount: 0, released_amount: 0, refunded_amount: 0 }
const emptyAnalytics = (): VerificationRecordAnalytics => ({ by_platform: [], by_country: [], by_type: [], financial: [] })
const records = ref<VerificationRecord[]>([])
const loading = ref(false)
const summary = ref<VerificationRecordSummary>({ ...emptySummary })
const analytics = ref<VerificationRecordAnalytics>(emptyAnalytics())
const pagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 })
const blankFilters = (): VerificationRecordQuery => ({ type: '', outcome: '', platform: '', country: '', keyword: '', created_from: '', created_to: '' })
const draftFilters = reactive<VerificationRecordQuery>(blankFilters())
const activeFilters = reactive<VerificationRecordQuery>(blankFilters())
const selectedIds = ref(new Set<string>())

interface CatalogState { items: VerificationRecordOption[]; page: number; pages: number; query: string; loading: boolean }
const platformCatalog = reactive<CatalogState>({ items: [], page: 1, pages: 1, query: '', loading: false })
const countryCatalog = reactive<CatalogState>({ items: [], page: 1, pages: 1, query: '', loading: false })
const catalogState = (kind: 'platform' | 'country') => kind === 'platform' ? platformCatalog : countryCatalog
const localizedOptionLabel = (option: VerificationRecordOption) => locale.value.startsWith('zh') ? option.label : option.label_en || option.label
const platformOptions = computed(() => platformCatalog.items.map(item => ({ ...item, label: localizedOptionLabel(item) })))
const countryOptions = computed(() => countryCatalog.items.map(item => ({ ...item, label: localizedOptionLabel(item) })))
const platformLabels = computed(() => Object.fromEntries(platformOptions.value.map(item => [item.value, item.label])))
const countryLabels = computed(() => Object.fromEntries(countryOptions.value.map(item => [item.value, item.label])))
const platformLabel = (code: string) => platformLabels.value[code] || code
const countryLabel = (code: string) => countryLabels.value[code] || code

const financialValue = (field: 'sale_amount' | 'provider_cost' | 'estimated_profit') => analytics.value.financial.length ? analytics.value.financial.map(item => formatCurrency(item[field], item.currency)).join(' / ') : '-'
const summaryItems = computed(() => {
  const items: Array<{ key: string; label: string; value: string | number; compact?: boolean }> = [
    { key: 'total', label: t('verificationRecords.summary.total'), value: summary.value.total }, { key: 'processing', label: t('verificationRecords.summary.processing'), value: summary.value.processing }, { key: 'success', label: t('verificationRecords.summary.success'), value: summary.value.success }, { key: 'failed', label: t('verificationRecords.summary.failed'), value: summary.value.failed }, { key: 'refunded', label: t('verificationRecords.summary.refunded'), value: summary.value.refunded },
  ]
  if (isAdmin.value) items.push({ key: 'amount', label: t('verificationRecords.summary.totalAmount'), value: financialValue('sale_amount'), compact: true }, { key: 'cost', label: t('verificationRecords.summary.totalCost'), value: financialValue('provider_cost'), compact: true }, { key: 'profit', label: t('verificationRecords.summary.profit'), value: financialValue('estimated_profit'), compact: true })
  return items
})
const selectedRecords = computed(() => records.value.filter(record => selectedIds.value.has(record.id)))
const allPageSelected = computed(() => records.value.length > 0 && records.value.every(record => selectedIds.value.has(record.id)))

function outcomeClass(outcome: VerificationOutcome): string { if (outcome === 'success') return 'badge-success'; if (outcome === 'failed') return 'badge-danger'; if (outcome === 'processing') return 'badge-info'; if (outcome === 'cancelled') return 'badge-gray'; return 'badge-warning' }
const outcomeLabel = (outcome: VerificationOutcome) => t(`verificationRecords.outcomes.${outcome}`)
const typeLabel = (type: VerificationType) => t(`verificationRecords.types.${type}`)
const refundStatuses = new Set(['not_requested', 'pending', 'approved', 'rejected', 'released', 'not_applicable', 'manual_review', 'succeeded'])
const refundReasons: Record<string, string> = {
  'provider did not confirm cancellation': 'providerDidNotConfirmCancellation',
  'provider refund is being confirmed': 'providerRefundConfirming',
  'provider cancellation/refund is being confirmed': 'providerCancellationRefundConfirming',
  'provider confirmed cancellation/timeout': 'providerConfirmedCancellationTimeout',
  'provider refused refund': 'providerRefusedRefund',
  'provider refund confirmed': 'providerRefundConfirmed',
  'provider refund confirmed during reconciliation': 'providerRefundConfirmedDuringReconciliation',
  'provider status confirms cancellation/timeout refund': 'providerStatusConfirmedCancellationTimeoutRefund',
  'verification SMS was already received; cancellation/refund is no longer available': 'verificationAlreadyReceived',
  'cancelled before target email': 'cancelledBeforeTargetEmail',
  'user requested refund before target email': 'userRefundBeforeTargetEmail',
  'manual review required': 'manualReviewRequired',
}
const refundStatusLabel = (status: string) => refundStatuses.has(status) ? t(`verificationRecords.refundStatuses.${status}`) : status
const refundReasonLabel = (reason: string) => refundReasons[reason] ? t(`verificationRecords.refundReasons.${refundReasons[reason]}`) : reason
const money = (amount: number | undefined, currency: string) => formatCurrency(amount ?? 0, currency || 'CNY')

async function loadCatalog(kind: 'platform' | 'country', query = '', page = 1, append = false): Promise<void> {
  const state = catalogState(kind)
  state.loading = true
  try {
    const result = await verificationRecordsAPI.options({ kind, type: kind === 'platform' ? draftFilters.type : 'sms', query, page, page_size: 20 }, isAdmin.value)
    state.items = append ? [...state.items, ...result.items.filter(item => !state.items.some(current => current.value === item.value))] : result.items
    state.page = result.page; state.pages = result.pages; state.query = query
  } catch (error) { appStore.showError((error as { message?: string })?.message || t('verificationRecords.loadError')) }
  finally { state.loading = false }
}
const searchCatalog = (kind: 'platform' | 'country', query: string) => { void loadCatalog(kind, query, 1, false) }
const loadMoreCatalog = (kind: 'platform' | 'country') => { const state = catalogState(kind); if (!state.loading && state.page < state.pages) void loadCatalog(kind, state.query, state.page + 1, true) }

async function loadRecords(): Promise<void> {
  loading.value = true
  try {
    const result = await verificationRecordsAPI.list({ ...activeFilters, page: pagination.page, page_size: pagination.pageSize }, isAdmin.value)
    records.value = result.items; summary.value = result.summary; analytics.value = result.analytics || emptyAnalytics(); pagination.total = result.total; pagination.page = result.page; pagination.pageSize = result.page_size; clearSelection()
  } catch (error) { appStore.showError((error as { message?: string })?.message || t('verificationRecords.loadError')) }
  finally { loading.value = false }
}

function applyFilters(): void { Object.assign(activeFilters, draftFilters); pagination.page = 1; void loadRecords() }
function resetFilters(): void { Object.assign(draftFilters, blankFilters()); Object.assign(activeFilters, blankFilters()); pagination.page = 1; void Promise.all([loadCatalog('platform'), loadCatalog('country'), loadRecords()]) }
function changePage(page: number): void { pagination.page = page; void loadRecords() }
function changePageSize(pageSize: number): void { pagination.pageSize = pageSize; pagination.page = 1; void loadRecords() }
function toggleRecord(record: VerificationRecord): void { const next = new Set(selectedIds.value); next.has(record.id) ? next.delete(record.id) : next.add(record.id); selectedIds.value = next }
function toggleCurrentPage(): void { selectedIds.value = allPageSelected.value ? new Set() : new Set(records.value.map(record => record.id)) }
function clearSelection(): void { selectedIds.value = new Set() }
function csvCell(value: unknown): string { const text = String(value ?? ''); return /[",\r\n]/.test(text) ? `"${text.replace(/"/g, '""')}"` : text }
function exportSelected(): void {
  const headers = ['order_no', 'verification_type', 'platform', 'country', 'target', 'outcome', 'sale_amount', 'provider_cost', 'currency', 'created_at']
  const lines = selectedRecords.value.map(record => [record.order_no, record.verification_type, record.service_code, record.region || '', record.target, record.outcome, record.sale_amount, record.provider_cost ?? '', record.currency, record.created_at].map(csvCell).join(','))
  saveAs(new Blob([`\uFEFF${headers.join(',')}\r\n${lines.join('\r\n')}`], { type: 'text/csv;charset=utf-8' }), `verification-records-${new Date().toISOString().slice(0, 10)}.csv`)
  appStore.showSuccess(t('verificationRecords.bulk.exported', { count: selectedRecords.value.length }))
}

watch(() => draftFilters.type, type => { if (type === 'email') draftFilters.country = ''; void loadCatalog('platform'); if (type !== 'email' && countryCatalog.items.length === 0) void loadCatalog('country') })
onMounted(() => { void Promise.all([loadCatalog('platform'), loadCatalog('country'), loadRecords()]) })
</script>
