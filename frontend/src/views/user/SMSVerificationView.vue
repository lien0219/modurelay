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
          <label class="input-label">
            {{ t('sms.user.service') }}
            <select v-model="serviceCode" class="input mt-1">
              <option value="" disabled>{{ t('sms.user.selectService') }}</option>
              <option v-for="item in services" :key="item.code" :value="item.code">{{ item.name }}</option>
            </select>
          </label>
          <label class="input-label">
            {{ t('sms.user.country') }}
            <select v-model="countryCode" class="input mt-1">
              <option value="" disabled>{{ t('sms.user.selectCountry') }}</option>
              <option v-for="item in countries" :key="item.iso2" :value="item.iso2">{{ countryName(item) }} ({{ item.iso2 }})</option>
            </select>
          </label>
          <div v-if="productType === 'rental'" class="grid grid-cols-2 gap-2">
            <label class="input-label">{{ t('sms.user.duration') }}<input v-model.number="durationValue" class="input mt-1" type="number" min="1" /></label>
            <label class="input-label">
              {{ t('sms.user.unit') }}
              <select v-model="durationUnit" class="input mt-1">
                <option value="hour">{{ t('sms.user.hour') }}</option>
                <option value="day">{{ t('sms.user.day') }}</option>
                <option value="week">{{ t('sms.user.week') }}</option>
              </select>
            </label>
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
        <div v-if="ordersLoading" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.loading') }}</div>
        <div v-else-if="!orders.length" class="card p-8 text-center text-sm text-gray-500">{{ t('sms.user.noOrders') }}</div>
        <div v-for="order in orders" :key="order.id" class="card flex flex-col gap-3 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2"><span class="font-mono text-sm text-gray-900 dark:text-white">{{ order.id }}</span><span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span><span v-if="order.refund_status !== 'not_requested'" class="badge badge-warning">{{ refundLabel(order.refund_status) }}</span></div>
            <p class="mt-1 text-sm text-gray-500">{{ order.channel_name }}{{ t('sms.user.separator') }}{{ order.service_code }}{{ t('sms.user.separator') }}{{ order.country_code }}{{ t('sms.user.separator') }}{{ productTypeLabel(order.product_type) }}</p>
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
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { smsAPI, type SMSCountryItem, type SMSOrder, type SMSQuote, type SMSServiceItem } from '@/api/sms'
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
let timer: number | undefined

const tabs = computed(() => [
  { value: 'temporary' as const, label: t('sms.user.temporary') },
  { value: 'rental' as const, label: t('sms.user.rental') },
])
const quoteHint = computed(() => quotes.value.length ? t('sms.user.quoteHint') : t('sms.user.visibilityHint'))

function countryName(country: SMSCountryItem) {
  return locale.value.startsWith('zh') ? country.name_zh || country.name_en || country.iso2 : country.name_en || country.name_zh || country.iso2
}

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    active: t('sms.user.statuses.waitingSms'),
    reconciling: t('sms.user.statuses.reconciling'),
    provider_unknown: t('sms.user.statuses.providerUnknown'),
    completed: t('sms.user.statuses.completed'),
    cancelled: t('sms.user.statuses.cancelled'),
    failed: t('sms.user.statuses.failed'),
    refunded: t('sms.user.statuses.refunded'),
  }
  return labels[status] || status
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
    orders.value = await smsAPI.orders()
  } catch (error) {
    appStore.showError((error as { message?: string }).message || t('sms.user.errors.orders'))
  } finally {
    ordersLoading.value = false
  }
}

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
