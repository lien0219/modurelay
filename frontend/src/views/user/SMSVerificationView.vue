<template>
  <AppLayout>
    <div class="mx-auto w-full max-w-[1600px] space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">ModuRelay</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.smsService') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('sms.user.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadAll">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" />
        </button>
      </header>

      <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('nav.smsService')">
        <button v-for="tab in tabs" :key="tab.value" type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="productType === tab.value && activeTab !== 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="productType === tab.value && activeTab !== 'orders'" @click="productType = tab.value; activeTab = tab.value; loadQuotes()">{{ tab.label }}</button>
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="activeTab === 'orders'" @click="activeTab = 'orders'; loadOrders()">{{ t('sms.user.orders') }}</button>
      </div>

      <section v-if="activeTab !== 'orders'" class="card space-y-4 p-5">
        <div class="grid gap-4 sm:grid-cols-3">
          <Select v-model="serviceCode" :label="t('sms.user.service')" :options="serviceOptions" :placeholder="t('sms.user.selectService')" searchable>
            <template #selected="{ option }"><VerificationIdentity v-if="option" kind="platform" :code="String(option.value)" :label="String(option.label)" :icon="String(option.icon || '')" :show-code="false" /><span v-else>{{ t('sms.user.selectService') }}</span></template>
            <template #option="{ option }"><VerificationIdentity kind="platform" :code="String(option.value)" :label="String(option.label)" :icon="String(option.icon || '')" /></template>
          </Select>
          <Select v-model="countryCode" :label="t('sms.user.country')" :options="countryOptions" :placeholder="t('sms.user.selectCountry')" searchable>
            <template #selected="{ option }"><VerificationIdentity v-if="option" kind="country" :code="String(option.value)" :label="String(option.label)" :show-code="false" /><span v-else>{{ t('sms.user.selectCountry') }}</span></template>
            <template #option="{ option }"><VerificationIdentity kind="country" :code="String(option.value)" :label="String(option.label)" /></template>
          </Select>
          <div v-if="productType === 'rental'" class="grid grid-cols-2 gap-2">
            <label class="block min-w-0"><span class="input-label">{{ t('sms.user.duration') }}</span><input v-model.number="durationValue" class="input h-[42px]" type="number" min="1" /></label>
            <Select v-model="durationUnit" :label="t('sms.user.unit')" :options="durationUnitOptions" />
          </div>
        </div>
        <div class="flex flex-wrap items-center justify-between gap-3">
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ quoteHint }}</span>
          <button type="button" class="btn btn-primary" :disabled="!serviceCode || !countryCode || quoting" @click="loadQuotes">{{ quoting ? t('sms.user.quoting') : t('sms.user.getQuote') }}</button>
        </div>
      </section>

      <section v-if="activeTab !== 'orders'" class="space-y-3">
        <div v-if="quoting" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.checkingStock') }}</div>
        <div v-else-if="!quotes.length" class="card p-8 text-center text-sm text-gray-500">{{ serviceCode && countryCode ? t('sms.user.noChannel') : t('sms.user.chooseForQuote') }}</div>
        <div v-for="quote in quotes" :key="quote.quote_id" class="card flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="font-semibold text-gray-900 dark:text-white">{{ quote.public_name }}</h2>
              <span class="badge" :class="quote.channel_role === 'primary' ? 'badge-info' : 'badge-gray'">{{ quote.channel_role === 'primary' ? t('sms.admin.primary') : t('sms.admin.backup') }}</span>
            </div>
            <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500 dark:text-gray-400">
              <span>{{ t('sms.user.stock') }}: {{ quote.stock }}</span>
              <span>{{ t('sms.user.eta') }}: {{ quote.estimated_delivery_seconds }}{{ t('sms.user.seconds') }}</span>
              <span v-if="quote.success_rate != null">{{ t('sms.user.successRate') }}: {{ Math.round(quote.success_rate * 100) }}%{{ t('sms.user.separator') }}{{ quote.success_rate_grade || '-' }}</span>
              <span v-else>{{ t('sms.user.insufficientSuccessData') }}</span>
            </div>
          </div>
          <div class="flex items-center justify-between gap-4 sm:justify-end">
            <div class="text-right"><div class="text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ quote.sale_price.toFixed(4) }}</div><div class="text-xs text-gray-500">{{ t('sms.user.currency') }}</div></div>
            <button type="button" class="btn btn-primary" :disabled="purchasing" @click="purchase(quote)">{{ purchasing ? t('sms.user.processing') : t('sms.user.purchase') }}</button>
          </div>
        </div>
      </section>

      <section v-else class="space-y-3">
        <form class="glass-panel grid items-end gap-3 rounded-xl p-4 md:grid-cols-[minmax(240px,1fr)_220px_auto]" @submit.prevent="applyOrderFilters">
          <label class="block min-w-0"><span class="input-label">{{ t('verificationRecords.filters.keyword') }}</span><input v-model.trim="orderDraft.keyword" class="input h-[42px]" :placeholder="t('verificationRecords.filters.keywordPlaceholder')" /></label>
          <Select v-model="orderDraft.status" :label="t('verificationRecords.filters.outcome')" :options="orderStatusOptions" :placeholder="t('verificationRecords.filters.allOutcomes')" clearable searchable />
          <div class="flex h-[42px] items-center gap-2 self-end"><button type="submit" class="btn btn-primary h-[42px]" :disabled="ordersLoading"><Icon name="search" size="sm" aria-hidden="true" />{{ t('common.search') }}</button><button type="button" class="btn btn-secondary h-[42px]" :disabled="ordersLoading" @click="resetOrderFilters"><Icon name="eraser" size="sm" aria-hidden="true" />{{ t('common.reset') }}</button></div>
        </form>
        <div v-if="ordersLoading" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.loading') }}</div>
        <div v-else-if="!orders.length" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.noOrders') }}</div>
        <div v-for="order in orders" :key="order.id" class="card flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2"><span class="font-mono text-sm text-gray-900 dark:text-white">{{ order.id }}</span><span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span><span v-if="order.refund_status !== 'not_requested'" class="badge badge-warning">{{ refundLabel(order.refund_status) }}</span></div>
            <div class="mt-2 flex flex-wrap items-center gap-4 text-sm text-gray-500"><VerificationIdentity kind="platform" :code="order.service_code" :label="serviceLabel(order.service_code)" :show-code="false" /><VerificationIdentity kind="country" :code="order.country_code" :label="countryLabel(order.country_code)" :show-code="false" /><span>{{ order.channel_name }}{{ t('sms.user.separator') }}{{ productTypeLabel(order.product_type) }}</span></div>
            <p v-if="order.phone_number" class="mt-1 font-mono text-sm text-gray-800 dark:text-gray-200">{{ order.phone_number }}</p>
            <p v-if="order.refund_reason" class="mt-1 text-xs text-amber-700 dark:text-amber-300">{{ order.refund_reason }}</p>
          </div>
          <div class="flex items-center gap-2">
            <button v-if="order.status === 'active'" type="button" class="btn btn-secondary btn-sm" @click="cancel(order.id)">{{ t('sms.user.cancel') }}</button>
            <button v-if="order.status === 'active' && order.product_type === 'rental'" type="button" class="btn btn-secondary btn-sm" @click="extend(order.id)">{{ t('sms.user.extend') }}</button>
            <button v-if="order.status === 'active' && order.product_type === 'temporary'" type="button" class="btn btn-secondary btn-sm" @click="refund(order.id)">{{ t('sms.user.requestRefund') }}</button>
            <button type="button" class="btn btn-secondary btn-sm" :disabled="refreshingId === order.id" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="refreshOrder(order.id)"><Icon name="refresh" size="sm" :class="refreshingId === order.id ? 'animate-spin' : ''" aria-hidden="true" /></button>
          </div>
        </div>
        <div v-if="orderPagination.total > 0" class="card overflow-hidden"><Pagination :page="orderPagination.page" :total="orderPagination.total" :page-size="orderPagination.pageSize" @update:page="changeOrderPage" @update:page-size="changeOrderPageSize" /></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import VerificationIdentity from '@/components/verification/VerificationIdentity.vue'
import { smsAPI, type SMSCountryItem, type SMSOrder, type SMSOrderPage, type SMSQuote, type SMSServiceItem } from '@/api/sms'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores'

const { locale, t } = useI18n()
const appStore = useAppStore()
const productType = ref<'temporary' | 'rental'>('temporary')
const activeTab = ref<'temporary' | 'rental' | 'orders'>('temporary')
const services = ref<SMSServiceItem[]>([])
const countries = ref<SMSCountryItem[]>([])
const serviceCode = ref('')
const countryCode = ref('US')
const durationValue = ref(1)
const durationUnit = ref('hour')
const quotes = ref<SMSQuote[]>([])
const orders = ref<SMSOrder[]>([])
const loading = ref(false)
const quoting = ref(false)
const ordersLoading = ref(false)
const purchasing = ref(false)
const refreshingId = ref('')
const orderPagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 })
const orderDraft = reactive({ keyword: '', status: '' })
const orderFilters = reactive({ keyword: '', status: '' })
let timer: number | undefined

const tabs = computed(() => [
  { value: 'temporary' as const, label: t('sms.user.temporary') },
  { value: 'rental' as const, label: t('sms.user.rental') },
])
const quoteHint = computed(() => quotes.value.length ? t('sms.user.quoteHint') : t('sms.user.visibilityHint'))
const serviceOptions = computed(() => services.value.map(item => ({ value: item.code, label: item.name, icon: item.icon || '' })))
const countryOptions = computed(() => countries.value.map(item => ({ value: item.iso2, label: countryName(item) })))
const durationUnitOptions = computed(() => [{ value: 'hour', label: t('sms.user.hour') }, { value: 'day', label: t('sms.user.day') }, { value: 'week', label: t('sms.user.week') }])
const orderStatuses = ['pending', 'active', 'reconciling', 'provider_unknown', 'completed', 'cancelled', 'failed', 'refunded', 'expired']
const orderStatusOptions = computed(() => orderStatuses.map(value => ({ value, label: statusLabel(value) })))
const serviceLabel = (code: string) => services.value.find(item => item.code === code)?.name || code
const countryLabel = (code: string) => { const item = countries.value.find(country => country.iso2 === code); return item ? countryName(item) : code }

function countryName(country: SMSCountryItem) {
  return locale.value.startsWith('zh') ? country.name_zh || country.name_en || country.iso2 : country.name_en || country.name_zh || country.iso2
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    pending: t('sms.user.statuses.pending'),
    active: t('sms.user.statuses.waitingSms'),
    reconciling: t('sms.user.statuses.reconciling'),
    provider_unknown: t('sms.user.statuses.providerUnknown'),
    completed: t('sms.user.statuses.completed'),
    cancelled: t('sms.user.statuses.cancelled'),
    failed: t('sms.user.statuses.failed'),
    refunded: t('sms.user.statuses.refunded'),
    expired: t('sms.user.statuses.expired'),
  }
  return labels[status] || t('sms.user.statuses.unknown')
}

function productTypeLabel(type: SMSOrder['product_type']) {
  return type === 'rental' ? t('sms.user.productTypes.rental') : t('sms.user.productTypes.temporary')
}

function statusClass(status: string) {
  return status === 'completed' || status === 'refunded' ? 'badge-success' : status === 'failed' ? 'badge-danger' : status === 'cancelled' ? 'badge-gray' : 'badge-info'
}

function refundLabel(status: string) {
  const labels: Record<string, string> = { approved: t('sms.user.refunds.approved'), rejected: t('sms.user.refunds.rejected'), pending: t('sms.user.refunds.pending') }
  return labels[status] || status
}

async function extend(id: string) {
  const raw = window.prompt(t('sms.user.hoursToExtend'), '1')
  const hours = Number(raw)
  if (!Number.isFinite(hours) || hours <= 0) return
  try {
    await smsAPI.extendRental(id, { duration_value: hours, duration_unit: 'hour' }, `sms-renew-${id}-${Date.now()}`)
    await loadOrders()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.extend'))
  }
}

async function loadAll() {
  loading.value = true
  try {
    const [nextServices, nextCountries] = await Promise.all([smsAPI.services(), smsAPI.countries()])
    services.value = nextServices
    countries.value = nextCountries
    if (!serviceCode.value) serviceCode.value = services.value[0]?.code || ''
    await loadQuotes()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.unavailable'))
  } finally {
    loading.value = false
  }
}

async function loadQuotes() {
  if (!serviceCode.value || !countryCode.value) return
  quoting.value = true
  try {
    quotes.value = await smsAPI.quotes({ service: serviceCode.value, country: countryCode.value, product_type: productType.value })
  } catch (error) {
    quotes.value = []
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.quote'))
  } finally {
    quoting.value = false
  }
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const result = await smsAPI.orders({ page: orderPagination.page, page_size: orderPagination.pageSize, ...orderFilters }) as SMSOrderPage | SMSOrder[]
    if (Array.isArray(result)) {
      orders.value = result
      orderPagination.total = result.length
    } else {
      orders.value = result.items
      orderPagination.total = result.total
      orderPagination.page = result.page
      orderPagination.pageSize = result.page_size
    }
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.orders'))
  } finally {
    ordersLoading.value = false
  }
}

function applyOrderFilters() { Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function resetOrderFilters() { Object.assign(orderDraft, { keyword: '', status: '' }); Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function changeOrderPage(page: number) { orderPagination.page = page; void loadOrders() }
function changeOrderPageSize(pageSize: number) { orderPagination.pageSize = pageSize; orderPagination.page = 1; void loadOrders() }

async function purchase(quote: SMSQuote) {
  purchasing.value = true
  try {
    const key = `sms-${Date.now()}-${Math.random().toString(36).slice(2)}`
    await smsAPI.purchase({ channel_code: quote.channel_code, service_code: serviceCode.value, country_code: countryCode.value, product_type: productType.value, duration_value: productType.value === 'rental' ? durationValue.value : undefined, duration_unit: productType.value === 'rental' ? durationUnit.value : undefined, expected_price: quote.sale_price }, key)
    activeTab.value = 'orders'
    await loadOrders()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.purchase'))
  } finally {
    purchasing.value = false
  }
}

async function cancel(id: string) {
  try {
    await smsAPI.cancel(id)
    await loadOrders()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.cancel'))
  }
}

async function refund(id: string) {
  try {
    await smsAPI.refund(id)
    await loadOrders()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.refund'))
  }
}

async function refreshOrder(id: string) {
  refreshingId.value = id
  try {
    const updated = await smsAPI.order(id)
    const index = orders.value.findIndex((item) => item.id === id)
    if (index >= 0) orders.value[index] = updated
  } finally {
    refreshingId.value = ''
  }
}

onMounted(() => {
  loadAll()
  timer = window.setInterval(() => {
    if (activeTab.value === 'orders') loadOrders()
  }, 10000)
})

onBeforeUnmount(() => {
  if (timer) window.clearInterval(timer)
})
</script>
