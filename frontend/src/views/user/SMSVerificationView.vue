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
        <button v-for="tab in tabs" :key="tab.value" type="button" class="border-b-2 px-3 py-2 text-sm font-medium disabled:cursor-not-allowed disabled:opacity-40" :disabled="tab.disabled" :class="productType === tab.value && activeTab !== 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="productType === tab.value && activeTab !== 'orders'" @click="switchProductType(tab.value)">{{ tab.label }}</button>
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="activeTab === 'orders'" @click="activeTab = 'orders'; loadOrders()">{{ t('sms.user.orders') }}</button>
      </div>

      <section v-if="activeTab !== 'orders'" class="card space-y-5 p-5">
        <div>
          <div class="mb-3 flex items-center justify-between gap-3">
            <div>
              <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">1 · {{ t('sms.admin.channels') }}</p>
              <p class="mt-1 text-xs text-gray-500">先选择接码渠道，再按该供应商实时支持的平台和国家下单。</p>
            </div>
          </div>
          <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <button
              v-for="(provider, index) in providers"
              :key="provider.code"
              type="button"
              class="relative min-h-20 rounded-xl border p-4 text-left transition"
              :class="[
                providerCode === provider.code ? 'border-primary-500 bg-primary-50 ring-1 ring-primary-200 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-700',
                !provider.selectable ? 'cursor-not-allowed bg-gray-50 opacity-50 grayscale dark:bg-dark-800' : 'hover:border-primary-300'
              ]"
              :disabled="!provider.selectable"
              @click="switchProvider(provider.code)"
            >
              <div class="flex items-center justify-between gap-2">
                <span class="font-semibold text-gray-900 dark:text-white">渠道{{ index + 1 }}</span>
                <span v-if="provider.beta" class="rounded bg-gray-200 px-2 py-0.5 text-[10px] font-bold tracking-wider text-gray-600 dark:bg-dark-600 dark:text-gray-300">BETA</span>
              </div>
              <div class="mt-1 text-sm text-gray-500">{{ provider.name }}</div>
              <div v-if="!provider.selectable" class="mt-2 text-xs text-gray-400">暂未开放</div>
            </button>
          </div>
        </div>

        <div class="grid gap-4 lg:grid-cols-3">
          <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">2 · {{ t('sms.user.service') }}</p>
            <div class="mt-3"><input v-model.trim="serviceKeyword" class="input h-10 w-full" placeholder="搜索网站 / APP / 平台" /></div>
            <div class="mt-3 max-h-72 space-y-2 overflow-y-auto pr-1">
              <button v-for="option in filteredServiceOptions" :key="String(option.value)" type="button" class="flex min-h-10 w-full items-center justify-between gap-3 rounded-lg border px-3 text-left text-sm" :class="serviceCode === option.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-700'" @click="selectService(String(option.value))">
                <span class="min-w-0 truncate">{{ option.label }}</span><span class="shrink-0 font-mono text-[10px] text-gray-400">{{ option.value }}</span>
              </button>
              <div v-if="!filteredServiceOptions.length" class="py-8 text-center text-sm text-gray-400">当前渠道暂无匹配平台</div>
            </div>
          </div>
          <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
            <p class="text-xs font-semibold uppercase tracking-wide text-gray-500">3 · {{ t('sms.user.country') }}</p>
            <div class="mt-3"><input v-model.trim="countryKeyword" class="input h-10 w-full" placeholder="搜索国家 / 地区" :disabled="!serviceCode" /></div>
            <div class="mt-3 max-h-72 space-y-2 overflow-y-auto pr-1">
              <button v-for="option in filteredCountryOptions" :key="String(option.value)" type="button" class="flex min-h-11 w-full items-center justify-between gap-3 rounded-lg border px-3 text-left text-sm disabled:cursor-not-allowed disabled:opacity-45" :disabled="option.available === false" :class="countryCode === option.value ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-700'" @click="selectCountry(String(option.value))">
                <span class="min-w-0"><span class="block truncate">{{ option.label }}</span><span v-if="option.stock != null" class="block text-[10px] text-gray-400">库存 {{ option.stock }}</span></span>
                <span class="shrink-0 text-right"><span v-if="countryCode === option.value" class="block text-[10px] text-gray-500">{{ t('sms.user.selected') }}</span></span>
              </button>
              <div v-if="serviceCode && !filteredCountryOptions.length" class="py-8 text-center text-sm text-gray-400">当前平台暂无匹配国家</div>
            </div>
            <p v-if="!serviceCode" class="mt-3 text-xs text-gray-500">{{ t('sms.user.selectService') }}</p>
          </div>
          <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700"><p class="text-xs font-semibold uppercase tracking-wide text-gray-500">4 · {{ t('sms.user.purchase') }}</p><div class="mt-4 space-y-3"><p class="text-sm text-gray-600 dark:text-gray-300">{{ serviceLabel(serviceCode) }} · {{ countryLabel(countryCode) }}</p>
          <div v-if="productType === 'rental' && providerCode === 'smspva'" class="grid grid-cols-2 gap-2">
            <label class="block min-w-0"><span class="input-label">{{ t('sms.user.duration') }}</span><input v-model.number="durationValue" class="input h-[42px]" type="number" min="1" @change="reloadRentalCatalog" /></label>
            <Select v-model="durationUnit" :label="t('sms.user.unit')" :options="durationUnitOptions" @update:model-value="reloadRentalCatalog" />
          </div>
          <label v-if="currentProvider?.capabilities.supports_voice" class="block"><span class="input-label">验证码类型</span><select v-model.number="voiceMode" class="input h-[42px] w-full" @change="changeVoiceMode"><option v-for="item in voiceModeOptions" :key="item.value" :value="item.value">{{ item.label }}</option></select></label>
          <label v-if="currentProvider?.capabilities.supports_operator_selection" class="block"><span class="input-label">运营商</span><select v-model="operatorCode" class="input h-[42px] w-full" @change="quotes = []; loadQuotes()"><option value="any">Any / 自动选择</option><option v-for="item in operators.filter(op => op.code !== 'any')" :key="item.code" :value="item.code" :disabled="item.available === false">{{ item.name }}{{ item.stock != null ? ` · 库存 ${item.stock}` : '' }}</option></select></label>
          <label class="block"><span class="input-label">{{ t('sms.user.quantity') }}</span><input v-model.number="purchaseQuantity" class="input h-[42px] w-full" type="number" min="1" max="50" /></label><button type="button" class="btn btn-primary w-full" :disabled="!serviceCode || !countryCode || quoting" @click="loadQuotes">{{ quoting ? t('sms.user.quoting') : t('sms.user.getQuote') }}</button></div></div>
        </div>

        <p class="text-xs text-gray-500 dark:text-gray-400">{{ quoteHint }}</p>
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
              <span>{{ quote.capabilities.supports_cancel ? t('sms.user.capabilities.cancel') : t('sms.user.capabilities.noCancel') }}</span>
              <span>{{ quote.capabilities.supports_refund ? t('sms.user.capabilities.refund') : t('sms.user.capabilities.noRefund') }}</span>
            </div>
          </div>
          <div class="flex items-center justify-between gap-4 sm:justify-end">
            <div class="text-right"><div class="text-lg font-semibold tabular-nums text-gray-900 dark:text-white">{{ quote.sale_price.toFixed(4) }}</div><div class="text-xs text-gray-500">{{ t('sms.user.currency') }}</div></div>
            <button type="button" class="btn btn-primary" :disabled="purchasing" @click="purchase(quote)">{{ purchasing ? t('sms.user.processing') : purchaseQuantity > 1 ? t('sms.user.batchPurchase') : t('sms.user.purchase') }}</button>
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
        <div v-else class="card overflow-x-auto">
          <table class="min-w-[980px] w-full text-left text-sm">
            <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-4 py-3">{{ t('sms.user.order') }}</th><th class="px-4 py-3">{{ t('sms.user.service') }}</th><th class="px-4 py-3">{{ t('sms.user.country') }}</th><th class="px-4 py-3">{{ t('sms.user.phone') }}</th><th class="px-4 py-3">{{ t('sms.user.status') }}</th><th class="px-4 py-3">{{ t('sms.user.code') }}</th><th class="px-4 py-3">{{ t('sms.user.price') }}</th><th class="px-4 py-3">{{ t('sms.user.expiresIn') }}</th><th class="px-4 py-3">{{ t('sms.user.actions') }}</th></tr></thead>
            <tbody>
            <tr v-for="order in orders" :key="order.id" class="border-t border-gray-100 dark:border-dark-700">
              <td class="px-4 py-3 font-mono text-xs">{{ order.id }}</td><td class="px-4 py-3">{{ serviceLabel(order.service_code) }}</td><td class="px-4 py-3">{{ countryLabel(order.country_code) }}</td><td class="px-4 py-3 font-mono">{{ order.phone_number || '-' }}</td><td class="px-4 py-3"><span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span><span v-if="order.refund_status !== 'not_requested'" class="badge badge-warning ml-1">{{ refundLabel(order.refund_status) }}</span></td><td class="px-4 py-3"><div v-if="order.messages?.length" class="space-y-1"><div v-for="message in order.messages" :key="message.id"><code v-if="message.verification_code" class="font-mono font-semibold">{{ message.verification_code }}</code><span v-else class="text-gray-500">{{ message.message_text }}</span></div></div><span v-else>-</span></td><td class="px-4 py-3 tabular-nums">{{ order.price.toFixed(4) }}</td><td class="px-4 py-3 tabular-nums">{{ remainingLabel(order) }}</td><td class="px-4 py-3"><div class="flex min-w-max items-center gap-2"><button v-if="order.status === 'active' && (order.product_type === 'rental' ? order.capabilities?.supports_rental_cancel : order.capabilities?.supports_cancel !== false)" type="button" class="btn btn-secondary btn-sm" @click="cancel(order.id)">{{ t('sms.user.cancel') }}</button><button v-if="order.status === 'active' && order.product_type === 'rental' && order.capabilities?.supports_extend" type="button" class="btn btn-secondary btn-sm" @click="extend(order.id)">{{ t('sms.user.extend') }}</button><button v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_finish" type="button" class="btn btn-secondary btn-sm" @click="finish(order.id)">完成</button><button v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_ban" type="button" class="btn btn-secondary btn-sm" @click="ban(order.id)">封禁</button><button v-if="order.status === 'active' && order.product_type === 'temporary' && order.capabilities?.supports_refund !== false" type="button" class="btn btn-secondary btn-sm" @click="refund(order.id)">{{ t('sms.user.requestRefund') }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="refreshingId === order.id" :aria-label="t('common.refresh')" @click="refreshOrder(order.id)"><Icon name="refresh" size="sm" :class="refreshingId === order.id ? 'animate-spin' : ''" aria-hidden="true" /></button></div></td>
            </tr>
            </tbody>
          </table>
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
import { smsAPI, type SMSCountryItem, type SMSOperatorItem, type SMSOrder, type SMSOrderPage, type SMSProviderItem, type SMSQuote, type SMSServiceItem } from '@/api/sms'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores'

const { locale, t } = useI18n()
const appStore = useAppStore()
const productType = ref<'temporary' | 'rental'>('temporary')
const activeTab = ref<'temporary' | 'rental' | 'orders'>('temporary')
const providers = ref<SMSProviderItem[]>([])
const services = ref<SMSServiceItem[]>([])
const providerCode = ref('')
const countries = ref<SMSCountryItem[]>([])
const operators = ref<SMSOperatorItem[]>([])
const operatorCode = ref('any')
const voiceMode = ref(0)
const serviceKeyword = ref('')
const countryKeyword = ref('')
const serviceCode = ref('')
const countryCode = ref('US')
const durationValue = ref(1)
const durationUnit = ref('week')
const quotes = ref<SMSQuote[]>([])
const orders = ref<SMSOrder[]>([])
const loading = ref(false)
const quoting = ref(false)
const ordersLoading = ref(false)
const purchasing = ref(false)
const purchaseQuantity = ref(1)
const refreshingId = ref('')
const orderPagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 })
const orderDraft = reactive({ keyword: '', status: '' })
const orderFilters = reactive({ keyword: '', status: '' })
let pollTimer: number | undefined
let countdownTimer: number | undefined

const currentProvider = computed(() => providers.value.find(item => item.code === providerCode.value))
const tabs = computed(() => [
  { value: 'temporary' as const, label: t('sms.user.temporary'), disabled: !currentProvider.value?.capabilities.supports_temporary },
  { value: 'rental' as const, label: t('sms.user.rental'), disabled: !currentProvider.value?.capabilities.supports_rental },
])
const quoteHint = computed(() => quotes.value.length ? t('sms.user.quoteHint') : t('sms.user.visibilityHint'))
const serviceOptions = computed(() => services.value.map(item => ({ value: item.code, label: item.name, icon: item.icon || '' })))
const filteredServiceOptions = computed(() => {
  const q = serviceKeyword.value.toLowerCase()
  return serviceOptions.value.filter(item => !q || item.label.toLowerCase().includes(q) || item.value.toLowerCase().includes(q))
})
const countryOptions = computed(() => countries.value.map(item => ({ value: item.iso2, label: countryName(item), stock: item.stock, providerCost: item.provider_cost, available: item.available })))
const filteredCountryOptions = computed(() => {
  const q = countryKeyword.value.toLowerCase()
  return countryOptions.value.filter(item => !q || item.label.toLowerCase().includes(q) || item.value.toLowerCase().includes(q))
})
const durationUnitOptions = computed(() => {
  if (providerCode.value === 'smspva') {
    return [{ value: 'week', label: t('sms.user.week') }, { value: 'month', label: '月' }]
  }
  return [{ value: 'hour', label: t('sms.user.hour') }, { value: 'day', label: t('sms.user.day') }, { value: 'week', label: t('sms.user.week') }]
})
const voiceModeOptions = [
  { value: 0, label: '短信 SMS' },
  { value: 1, label: '来电显示 Caller ID' },
  { value: 2, label: '语音验证码 Voice' },
]
const orderStatuses = ['pending', 'active', 'reconciling', 'provider_unknown', 'completed', 'cancelled', 'failed', 'refunded', 'expired']
const orderStatusOptions = computed(() => orderStatuses.map(value => ({ value, label: statusLabel(value) })))
const serviceLabel = (code: string) => services.value.find(item => item.code === code)?.name || code
const countryLabel = (code: string) => { const item = countries.value.find(country => country.iso2 === code); return item ? countryName(item) : code }
function errorMessage(error: unknown, fallback: string) {
  const candidate = error as { message?: string; code?: string }
  if (candidate?.code === 'CANCEL_TOO_EARLY') return t('sms.user.errors.cancelTooEarly')
  if (candidate?.code === 'INSUFFICIENT_STOCK') return t('sms.user.errors.insufficientStock')
  return candidate?.message || fallback
}
const remainingLabel = (order: SMSOrder) => {
  const seconds = Math.max(0, order.remaining_seconds ?? (order.expires_at ? Math.floor((new Date(order.expires_at).getTime() - Date.now()) / 1000) : 0))
  if (seconds <= 0) return ['active', 'provider_unknown'].includes(order.status) ? t('sms.user.refundProcessing') : '-'
  const minutes = Math.floor(seconds / 60)
  return `${minutes}:${String(seconds % 60).padStart(2, '0')}`
}

function refreshCountdowns() {
  orders.value = orders.value.map(order => ({ ...order }))
}

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

function statusClass(status: string) {
  return status === 'completed' || status === 'refunded' ? 'badge-success' : status === 'failed' ? 'badge-danger' : status === 'cancelled' ? 'badge-gray' : 'badge-info'
}

async function loadAll() {
  loading.value = true
  try {
    providers.value = await smsAPI.providers()
    const preferred = providers.value.find(item => item.code === providerCode.value && item.selectable)
      || providers.value.find(item => item.code === '5sim' && item.selectable)
      || providers.value.find(item => item.selectable)
    providerCode.value = preferred?.code || ''
    await loadProviderCatalog()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.unavailable')))
  } finally {
    loading.value = false
  }
}

async function loadProviderCatalog() {
  services.value = []
  countries.value = []
  operators.value = []
  operatorCode.value = 'any'
  voiceMode.value = 0
  serviceKeyword.value = ''
  countryKeyword.value = ''
  serviceCode.value = ''
  countryCode.value = ''
  quotes.value = []
  if (!providerCode.value) return
  const nextServices = await smsAPI.providerServices(providerCode.value, {
    product_type: productType.value,
    duration_value: productType.value === 'rental' ? durationValue.value : undefined,
    duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
  })
  services.value = nextServices
  serviceCode.value = services.value[0]?.code || ''
  await loadServiceCountries()
}

async function switchProvider(code: string) {
  const provider = providers.value.find(item => item.code === code)
  if (!provider?.selectable || code === providerCode.value) return
  providerCode.value = code
  productType.value = provider.capabilities.supports_temporary ? 'temporary' : 'rental'
  durationUnit.value = provider.code === 'smspva' ? 'week' : 'hour'
  activeTab.value = productType.value
  loading.value = true
  try {
    await loadProviderCatalog()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.unavailable')))
  } finally {
    loading.value = false
  }
}

async function switchProductType(type: 'temporary' | 'rental') {
  productType.value = type
  activeTab.value = type
  if (type === 'rental' && providerCode.value === 'smspva' && !['week', 'month'].includes(durationUnit.value)) {
    durationUnit.value = 'week'
  }
  quotes.value = []
  await loadProviderCatalog()
  if (serviceCode.value && countryCode.value) await loadQuotes()
}

async function loadQuotes() {
  if (!serviceCode.value || !countryCode.value) return
  quoting.value = true
  try {
    quotes.value = await smsAPI.quotes({
      provider: providerCode.value,
      service: serviceCode.value,
      country: countryCode.value,
      product_type: productType.value,
      operator: operatorCode.value || 'any',
      voice_mode: voiceMode.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
  } catch (error) {
    quotes.value = []
    appStore.showError(errorMessage(error, t('sms.user.errors.quote')))
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
    await refreshActiveOrders()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.orders')))
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
    const quantity = Math.min(50, Math.max(1, Number(purchaseQuantity.value) || 1))
    const item = { channel_code: quote.channel_code, service_code: serviceCode.value, country_code: countryCode.value, product_type: productType.value, operator_code: operatorCode.value || 'any', voice_mode: voiceMode.value, duration_value: productType.value === 'rental' ? durationValue.value : undefined, duration_unit: productType.value === 'rental' ? durationUnit.value : undefined, quote_id: quote.quote_id, expected_price: quote.sale_price }
    if (quantity > 1) {
      const result = await smsAPI.purchaseBatch({ items: Array.from({ length: quantity }, () => item) }, key)
      if (result.partial_error) appStore.showError(result.partial_error)
    }
    else await smsAPI.purchase(item, key)
    activeTab.value = 'orders'
    await loadOrders()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.purchase')))
  } finally {
    purchasing.value = false
  }
}

async function cancel(id: string) {
  if (!window.confirm(t('common.confirm'))) return
  try {
    await smsAPI.cancel(id)
    await loadOrders()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.cancel')))
  }
}

async function finish(id: string) {
  if (!window.confirm('确认完成该接码订单？')) return
  try { await smsAPI.finish(id); await loadOrders() }
  catch (error) { appStore.showError(errorMessage(error, '完成订单失败')) }
}

async function ban(id: string) {
  if (!window.confirm('确认封禁该号码并进入退款确认流程？')) return
  try { await smsAPI.ban(id); await loadOrders() }
  catch (error) { appStore.showError(errorMessage(error, '封禁号码失败')) }
}

async function refund(id: string) {
  if (!window.confirm(t('common.confirm'))) return
  try {
    await smsAPI.refund(id)
    await loadOrders()
  } catch (error) {
    appStore.showError(errorMessage(error, t('sms.user.errors.refund')))
  }
}

async function selectService(code: string) {
  if (code === serviceCode.value) return
  serviceCode.value = code
  await loadServiceCountries()
}

async function selectCountry(code: string) {
  const country = countries.value.find(item => item.iso2 === code)
  if (!country || country.available === false) return
  countryCode.value = code
  quotes.value = []
  await loadOperators()
  await loadQuotes()
}

async function loadOperators() {
  operators.value = []
  operatorCode.value = 'any'
  if (!providerCode.value || !serviceCode.value || !countryCode.value) return
  if (!currentProvider.value?.capabilities.supports_operator_selection) return
  try {
    operators.value = await smsAPI.operators(providerCode.value, serviceCode.value, countryCode.value, {
      voice_mode: voiceMode.value,
      product_type: productType.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
    const any = operators.value.find(item => item.code === 'any')
    operatorCode.value = any?.code || operators.value.find(item => item.available !== false)?.code || 'any'
  } catch {
    operators.value = []
    operatorCode.value = 'any'
  }
}

async function reloadRentalCatalog() {
  if (productType.value !== 'rental') return
  quotes.value = []
  await loadProviderCatalog()
  if (serviceCode.value && countryCode.value) await loadQuotes()
}

async function changeVoiceMode() {
  quotes.value = []
  await loadOperators()
  if (countryCode.value) await loadQuotes()
}

async function loadServiceCountries() {
  countryKeyword.value = ''
  countryCode.value = ''
  quotes.value = []
  if (!serviceCode.value) return
  try {
    countries.value = await smsAPI.serviceCountries(providerCode.value, serviceCode.value, {
      product_type: productType.value,
      duration_value: productType.value === 'rental' ? durationValue.value : undefined,
      duration_unit: productType.value === 'rental' ? durationUnit.value : undefined,
    })
    countryCode.value = countries.value.find(item => item.available !== false)?.iso2 || ''
    await loadOperators()
  }
  catch (error) { appStore.showError(errorMessage(error, t('sms.user.errors.unavailable'))) }
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
    appStore.showError(errorMessage(error, t('sms.user.errors.extend')))
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

async function refreshActiveOrders() {
  const activeOrders = orders.value.filter(order => ['active', 'provider_unknown', 'reconciling'].includes(order.status)).slice(0, 20)
  if (!activeOrders.length) return
  const updates = await Promise.allSettled(activeOrders.map(order => smsAPI.order(order.id)))
  updates.forEach((result, index) => {
    if (result.status !== 'fulfilled' || !result.value) return
    const orderIndex = orders.value.findIndex(item => item.id === activeOrders[index].id)
    if (orderIndex >= 0) orders.value[orderIndex] = result.value
  })
}

onMounted(() => {
  loadAll()
  countdownTimer = window.setInterval(refreshCountdowns, 1000)
  pollTimer = window.setInterval(() => {
    if (activeTab.value === 'orders') void loadOrders()
  }, 10000)
})

onBeforeUnmount(() => {
  if (pollTimer) window.clearInterval(pollTimer)
  if (countdownTimer) window.clearInterval(countdownTimer)
})
</script>

