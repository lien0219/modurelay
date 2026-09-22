<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-[1800px] space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">MODURELAY</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.emailService') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('email.user.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadAll">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" />
        </button>
      </header>

      <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('nav.emailService')">
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'public' ? activeTabClass : inactiveTabClass" role="tab" :aria-selected="activeTab === 'public'" @click="switchMode('public')">{{ t('email.user.publicInbox') }}</button>
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'private' ? activeTabClass : inactiveTabClass" role="tab" :aria-selected="activeTab === 'private'" @click="switchMode('private')">{{ t('email.user.privateInbox') }}</button>
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'orders' ? activeTabClass : inactiveTabClass" role="tab" :aria-selected="activeTab === 'orders'" @click="openOrders">{{ t('email.user.orders') }}</button>
      </div>

      <template v-if="activeTab !== 'orders'">
        <section class="card p-5">
          <div class="grid gap-4 xl:grid-cols-[1fr_1.2fr_1.2fr]">
            <div>
              <div class="mb-2 text-xs font-semibold text-gray-500">1 · {{ t('email.user.channel') }}</div>
              <button type="button" class="w-full rounded-xl border p-4 text-left transition" :class="activeTab === 'public' ? selectedCardClass : normalCardClass" @click="switchMode('public')">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="font-semibold text-gray-900 dark:text-white">{{ t('email.user.channel1') }}</div>
                    <div class="mt-1 text-xs text-gray-500">{{ t('email.user.freePublicChannel') }}</div>
                  </div>
                  <span class="badge badge-success">{{ t('email.user.free') }}</span>
                </div>
              </button>
              <button type="button" class="mt-2 w-full rounded-xl border p-4 text-left transition" :class="activeTab === 'private' ? selectedCardClass : normalCardClass" @click="switchMode('private')">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <div class="font-semibold text-gray-900 dark:text-white">{{ t('email.user.channel2') }}</div>
                    <div class="mt-1 text-xs text-gray-500">{{ t('email.user.paidPrivateChannel') }}</div>
                  </div>
                  <span class="badge badge-info">{{ t('email.user.private') }}</span>
                </div>
              </button>
            </div>

            <div>
              <div class="mb-2 text-xs font-semibold text-gray-500">2 · {{ t('email.user.addressType') }}</div>
              <Select v-model="addressType" :options="addressTypeOptions" />
            </div>

            <div>
              <div class="mb-2 text-xs font-semibold text-gray-500">3 · {{ t('email.user.confirmSelection') }}</div>
              <div class="rounded-xl border border-gray-200 p-4 dark:border-dark-700">
                <div class="space-y-2 text-sm">
                  <div class="flex justify-between gap-3"><span class="text-gray-500">{{ t('email.user.addressType') }}</span><strong class="text-gray-900 dark:text-white">{{ addressTypeLabel(addressType) }}</strong></div>
                  <div class="flex justify-between gap-3"><span class="text-gray-500">{{ t('email.user.privacy') }}</span><strong class="text-gray-900 dark:text-white">{{ activeTab === 'public' ? t('email.user.public') : t('email.user.private') }}</strong></div>
                  <div v-if="selectedQuote" class="flex justify-between gap-3 border-t border-gray-100 pt-2 dark:border-dark-700">
                    <span class="text-gray-500">{{ t('email.user.price') }}</span>
                    <strong class="text-base text-gray-900 dark:text-white">{{ priceLabel(selectedQuote.sale_price) }}</strong>
                  </div>
                </div>
                <button type="button" class="btn btn-primary mt-4 w-full" :disabled="quoting" @click="loadQuotes">
                  {{ quoting ? t('email.user.checking') : t('email.user.getQuote') }}
                </button>
                <button v-if="selectedQuote" type="button" class="btn btn-primary mt-2 w-full" :disabled="purchasing" @click="purchase(selectedQuote)">
                  {{ purchasing ? t('email.user.processing') : (selectedQuote.sale_price === 0 ? t('email.user.generateFree') : t('email.user.purchasePrivate')) }}
                </button>
              </div>
            </div>
          </div>
        </section>

        <section v-if="quotes.length > 1" class="grid gap-3 lg:grid-cols-2">
          <button v-for="quote in quotes" :key="quote.quote_id" type="button" class="card p-4 text-left transition" :class="selectedQuote?.quote_id === quote.quote_id ? 'ring-2 ring-primary-500' : ''" @click="selectedQuoteId = quote.quote_id">
            <div class="flex items-center justify-between gap-3">
              <div>
                <div class="font-semibold text-gray-900 dark:text-white">{{ quote.public_name }}</div>
                <div class="mt-1 text-xs text-gray-500">{{ privacyLabel(quote.privacy_level) }} · {{ t('email.user.eta') }} {{ quote.estimated_delivery_seconds }}{{ t('email.user.seconds') }}</div>
              </div>
              <strong>{{ priceLabel(quote.sale_price) }}</strong>
            </div>
          </button>
        </section>

        <section v-if="!quoting && quoteLoaded && !quotes.length" class="card p-8 text-center text-sm text-gray-500">
          {{ t('email.user.noChannel') }}
        </section>

        <section v-if="currentOrder" class="card p-5">
          <div class="flex flex-wrap items-start justify-between gap-3">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold text-gray-900 dark:text-white">{{ statusLabel(currentOrder.status) }}</span>
                <span class="text-sm text-gray-500">{{ currentOrder.channel_name }}</span>
              </div>
              <div class="mt-4 grid gap-4 sm:grid-cols-[minmax(260px,1fr)_120px_minmax(220px,1fr)]">
                <div>
                  <div class="text-xs text-gray-500">{{ t('email.user.emailAddress') }}</div>
                  <div class="mt-1 flex min-w-0 items-center gap-2">
                    <strong class="min-w-0 break-all font-mono text-base text-gray-900 dark:text-white">{{ currentOrder.email_address || '...' }}</strong>
                    <CopyButton v-if="currentOrder.email_address" :text="currentOrder.email_address" class="shrink-0" />
                  </div>
                </div>
                <div>
                  <div class="text-xs text-gray-500">{{ t('email.user.validFor') }}</div>
                  <strong class="mt-1 block font-mono text-base text-gray-900 dark:text-white">{{ isTerminal(currentOrder) ? t('email.user.windowEnded') : remainingTime }}</strong>
                </div>
                <div>
                  <div class="text-xs text-gray-500">{{ t('email.user.latestCode') }}</div>
                  <div v-if="latestVerificationCode" class="mt-1 flex items-center gap-2">
                    <code class="font-mono text-xl font-semibold text-gray-900 dark:text-white">{{ latestVerificationCode }}</code>
                    <CopyButton :text="latestVerificationCode" />
                  </div>
                  <div v-else class="mt-1 text-sm text-gray-500">{{ t('email.user.waiting') }}</div>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <button v-if="canCancel(currentOrder)" type="button" class="btn btn-secondary btn-sm" :disabled="actionId === currentOrder.id" @click="cancelOrder(currentOrder)">{{ t('email.user.cancel') }}</button>
              <button type="button" class="btn btn-secondary btn-sm" :disabled="refreshingId === currentOrder.id" @click="refreshCurrentOrder()"><Icon name="refresh" size="sm" :class="refreshingId === currentOrder.id ? 'animate-spin' : ''" /></button>
            </div>
          </div>

          <div class="mt-5 border-t border-gray-100 pt-4 dark:border-dark-700">
            <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
              <div class="flex items-center gap-2">
                <h3 class="font-semibold text-gray-900 dark:text-white">{{ t('email.user.inbox') }}</h3>
                <span class="badge badge-gray">{{ t('email.user.messageCount', { count: currentOrder.messages?.length || 0 }) }}</span>
              </div>
              <span v-if="!isTerminal(currentOrder)" class="text-xs text-gray-500">{{ t('email.user.inboxReceiving') }}</span>
            </div>
            <div v-if="!currentOrder.messages?.length" class="rounded-xl bg-gray-50 p-6 text-center text-sm text-gray-500 dark:bg-dark-800">
              {{ t('email.user.inboxEmpty') }}
            </div>
            <div v-else class="space-y-3">
              <div v-for="message in currentOrder.messages" :key="message.id" class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
                <div class="flex flex-wrap justify-between gap-2 text-sm">
                  <span class="font-medium text-gray-900 dark:text-white">{{ message.from_name || message.from_address }}</span>
                  <time class="text-xs text-gray-500">{{ new Date(message.received_at).toLocaleString() }}</time>
                </div>
                <p class="mt-1 text-sm text-gray-700 dark:text-gray-200">{{ message.subject }}</p>
                <div v-if="message.verification_code" class="mt-3 flex flex-wrap items-center gap-2">
                  <span class="text-xs text-gray-500">{{ t('email.user.code') }}</span>
                  <code class="rounded bg-white px-2 py-1 font-mono text-lg font-semibold text-gray-900 dark:bg-dark-700 dark:text-white">{{ message.verification_code }}</code>
                  <CopyButton :text="message.verification_code" />
                </div>
                <div v-if="message.verification_url" class="mt-2 flex flex-wrap items-center gap-2">
                  <span class="min-w-0 flex-1 truncate text-xs text-gray-500">{{ message.verification_url }}</span>
                  <CopyButton :text="message.verification_url" />
                  <button type="button" class="btn btn-secondary btn-sm" @click="openVerificationURL(message.verification_url)">{{ t('email.user.safeOpen') }}</button>
                </div>
                <details class="mt-3">
                  <summary class="cursor-pointer text-sm font-medium text-primary-600 dark:text-primary-300">{{ t('email.user.viewMessage') }}</summary>
                  <pre class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-gray-200">{{ message.text_body }}</pre>
                </details>
              </div>
            </div>
          </div>
        </section>
      </template>

      <section v-else class="space-y-3">
        <form class="glass-panel grid items-end gap-3 rounded-xl p-4 md:grid-cols-[minmax(240px,1fr)_220px_auto]" @submit.prevent="applyOrderFilters">
          <label class="block min-w-0">
            <span class="input-label">{{ t('verificationRecords.filters.keyword') }}</span>
            <input v-model.trim="orderDraft.keyword" class="input h-[42px]" :placeholder="t('verificationRecords.filters.keywordPlaceholder')" />
          </label>
          <Select v-model="orderDraft.status" :label="t('verificationRecords.filters.outcome')" :options="orderStatusOptions" :placeholder="t('verificationRecords.filters.allOutcomes')" clearable searchable />
          <div class="flex h-[42px] items-center gap-2 self-end">
            <button type="submit" class="btn btn-primary h-[42px]" :disabled="ordersLoading"><Icon name="search" size="sm" />{{ t('common.search') }}</button>
            <button type="button" class="btn btn-secondary h-[42px]" :disabled="ordersLoading" @click="resetOrderFilters"><Icon name="eraser" size="sm" />{{ t('common.reset') }}</button>
          </div>
        </form>

        <div v-if="ordersLoading" class="card p-8 text-center text-sm text-gray-500">{{ t('email.user.loading') }}</div>
        <div v-else-if="!orders.length" class="card p-8 text-center text-sm text-gray-500">{{ t('email.user.noOrders') }}</div>

        <div v-else class="card overflow-x-auto">
          <table class="min-w-full text-sm">
            <thead class="border-b border-gray-100 bg-gray-50/70 text-left text-xs text-gray-500 dark:border-dark-700 dark:bg-dark-800/50">
              <tr>
                <th class="px-4 py-3">{{ t('email.user.channel') }}</th>
                <th class="px-4 py-3">{{ t('email.user.emailAddress') }}</th>
                <th class="px-4 py-3">{{ t('email.user.addressType') }}</th>
                <th class="px-4 py-3">{{ t('email.user.status') }}</th>
                <th class="px-4 py-3">{{ t('email.user.code') }}</th>
                <th class="px-4 py-3">{{ t('email.user.price') }}</th>
                <th class="px-4 py-3">{{ t('email.user.action') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="order in orders" :key="order.id" class="border-b border-gray-100 last:border-0 dark:border-dark-700">
                <td class="px-4 py-3">{{ order.channel_name }}</td>
                <td class="px-4 py-3">
                  <div class="flex max-w-[360px] items-center gap-2">
                    <span class="truncate font-mono">{{ order.email_address || '-' }}</span>
                    <CopyButton v-if="order.email_address" :text="order.email_address" class="shrink-0" />
                  </div>
                </td>
                <td class="px-4 py-3">{{ addressTypeLabel(order.address_type) }}</td>
                <td class="px-4 py-3"><span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span></td>
                <td class="px-4 py-3"><code v-if="firstCode(order)" class="font-mono font-semibold">{{ firstCode(order) }}</code><span v-else>-</span></td>
                <td class="px-4 py-3 tabular-nums">{{ priceLabel(order.price) }}</td>
                <td class="px-4 py-3"><button type="button" class="btn btn-secondary btn-sm" @click="openOrder(order)">{{ t('common.view') }}</button></td>
              </tr>
            </tbody>
          </table>
        </div>

        <div v-if="orderPagination.total > 0" class="card overflow-hidden">
          <Pagination :page="orderPagination.page" :total="orderPagination.total" :page-size="orderPagination.pageSize" @update:page="changeOrderPage" @update:page-size="changeOrderPageSize" />
        </div>
      </section>

      <ConfirmDialog
        :show="confirmState.show"
        :title="confirmState.title"
        :message="confirmState.message"
        :confirm-text="confirmState.confirmText"
        :cancel-text="confirmState.cancelText"
        :danger="confirmState.danger"
        :confirming="confirmingAction"
        @confirm="runConfirmedAction"
        @cancel="closeConfirm"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import CopyButton from '@/components/common/CopyButton.vue'
import { emailAPI, type EmailOrder, type EmailOrderPage, type EmailQuote } from '@/api/email'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()

const activeTab = ref<'public' | 'private' | 'orders'>('public')
const quotes = ref<EmailQuote[]>([])
const orders = ref<EmailOrder[]>([])
const addressType = ref('gmail')
const selectedQuoteId = ref('')
const currentOrder = ref<EmailOrder | null>(null)
const loading = ref(false)
const quoting = ref(false)
const quoteLoaded = ref(false)
const ordersLoading = ref(false)
const purchasing = ref(false)
const refreshingId = ref('')
const actionId = ref('')
const confirmingAction = ref(false)
const confirmState = reactive({
  show: false,
  title: '',
  message: '',
  confirmText: '',
  cancelText: '',
  danger: false,
  action: null as null | (() => Promise<void>),
})
const now = ref(Date.now())
let activeOrderTimer: number | undefined
let ordersTimer: number | undefined
let clockTimer: number | undefined

const orderPagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 })
const orderDraft = reactive({ keyword: '', status: '' })
const orderFilters = reactive({ keyword: '', status: '' })

const activeTabClass = 'border-primary-600 text-primary-600'
const inactiveTabClass = 'border-transparent text-gray-500'
const selectedCardClass = 'border-primary-500 bg-primary-50/60 ring-1 ring-primary-400 dark:bg-primary-950/20'
const normalCardClass = 'border-gray-200 hover:border-gray-300 dark:border-dark-700'

const addressTypeOptions = computed(() => activeTab.value === 'private'
  ? [
      { value: 'gmail_real', label: t('email.user.gmailReal') },
      { value: 'gmail_alias', label: t('email.user.gmailAlias') },
      { value: 'outlook_real', label: t('email.user.outlookReal') },
      { value: 'outlook_alias', label: t('email.user.outlookAlias') },
    ]
  : [
      { value: 'gmail', label: t('email.user.gmail') },
      { value: 'outlook', label: t('email.user.outlook') },
      { value: 'hotmail', label: t('email.user.hotmail') },
    ])

const selectedQuote = computed(() => quotes.value.find(item => item.quote_id === selectedQuoteId.value) || quotes.value[0])
const latestVerificationCode = computed(() => currentOrder.value?.messages?.find(item => item.verification_code)?.verification_code || '')
const remainingSeconds = computed(() => {
  if (!currentOrder.value?.expires_at || isTerminal(currentOrder.value)) return 0
  return Math.max(0, Math.floor((new Date(currentOrder.value.expires_at).getTime() - now.value) / 1000))
})
const remainingTime = computed(() => {
  const seconds = remainingSeconds.value
  const minutes = Math.floor(seconds / 60)
  return `${minutes}:${String(seconds % 60).padStart(2, '0')}`
})
const emailStatuses = ['reserved', 'generating_inbox', 'waiting_email', 'email_received', 'verification_extracted', 'completed', 'reconciling', 'expired', 'refunded', 'failed', 'cancelled']
const orderStatusOptions = computed(() => emailStatuses.map(value => ({ value, label: statusLabel(value) })))

function statusLabel(status: string) { return t(`email.user.statuses.${status}`, status) }
function statusClass(status: string) { return ['completed', 'refunded'].includes(status) ? 'badge-success' : status === 'failed' ? 'badge-danger' : ['expired', 'cancelled'].includes(status) ? 'badge-warning' : 'badge-info' }
function addressTypeLabel(value: string) {
  const key: Record<string, string> = {
    gmail: 'gmail', outlook: 'outlook', hotmail: 'hotmail',
    gmail_real: 'gmailReal', gmail_alias: 'gmailAlias',
    outlook_real: 'outlookReal', outlook_alias: 'outlookAlias',
  }
  return t(`email.user.${key[value] || 'gmail'}`)
}
function privacyLabel(value: string) { return value === 'private_api' ? t('email.user.private') : t('email.user.public') }
function priceLabel(value: number) { return Number(value) === 0 ? t('email.user.free') : `$${Number(value).toFixed(4)}` }
function firstCode(order: EmailOrder) { return order.messages?.find(item => item.verification_code)?.verification_code || '' }
function canCancel(order: EmailOrder) { return ['reserved', 'generating_inbox', 'reconciling', 'waiting_email', 'email_received', 'verification_extracted'].includes(order.status) }

function resetSelection() {
  quotes.value = []
  selectedQuoteId.value = ''
  quoteLoaded.value = false
}

function switchMode(mode: 'public' | 'private') {
  activeTab.value = mode
  addressType.value = mode === 'public' ? 'gmail' : 'gmail_real'
  resetSelection()
}

watch(addressType, () => resetSelection())

async function loadAll() {
  loading.value = true
  try {
    if (activeTab.value === 'orders') await loadOrders()
  } catch (error) {
    appStore.showError(errorMessage(error, t('email.user.errors.unavailable')))
  } finally {
    loading.value = false
  }
}

async function loadQuotes() {
  if (activeTab.value === 'orders') return
  quoting.value = true
  quoteLoaded.value = false
  try {
    const result = await emailAPI.quotes({ address_type: addressType.value })
    const wantedChannel = activeTab.value === 'public' ? 'email_channel_1' : 'email_channel_2'
    quotes.value = result.filter(item => item.channel_code === wantedChannel)
    selectedQuoteId.value = quotes.value[0]?.quote_id || ''
    quoteLoaded.value = true
  } catch (error) {
    quotes.value = []
    quoteLoaded.value = true
    appStore.showError(errorMessage(error, t('email.user.errors.quote')))
  } finally {
    quoting.value = false
  }
}

async function purchase(quote: EmailQuote) {
  purchasing.value = true
  try {
    const order = await emailAPI.purchase({
      channel_code: quote.channel_code,
      address_type: addressType.value,
      expected_price: quote.sale_price,
      quote_id: quote.quote_id,
    }, `email-${quote.quote_id}-${Date.now()}`)
    currentOrder.value = order
    appStore.showSuccess(quote.sale_price === 0 ? t('email.user.freeGenerated') : t('email.user.purchaseSuccess'))
    startActiveOrderPolling()
    resetSelection()
  } catch (error) {
    appStore.showError(errorMessage(error, t('email.user.errors.purchase')))
  } finally {
    purchasing.value = false
  }
}

function isTerminal(order: EmailOrder) {
  return ['completed', 'expired', 'refunded', 'failed', 'cancelled'].includes(order.status)
}

function stopActiveOrderPolling() {
  if (activeOrderTimer) window.clearTimeout(activeOrderTimer)
  activeOrderTimer = undefined
}

function startActiveOrderPolling() {
  stopActiveOrderPolling()
  if (!currentOrder.value || isTerminal(currentOrder.value)) return
  activeOrderTimer = window.setTimeout(async () => {
    await refreshCurrentOrder(false)
    startActiveOrderPolling()
  }, 3000)
}

async function refreshCurrentOrder(showError = true) {
  if (!currentOrder.value) return
  const id = currentOrder.value.id
  refreshingId.value = id
  try {
    currentOrder.value = await emailAPI.order(id)
    if (currentOrder.value && isTerminal(currentOrder.value)) stopActiveOrderPolling()
  } catch (error) {
    if (showError) appStore.showError(errorMessage(error, t('email.user.errors.orders')))
  } finally {
    refreshingId.value = ''
  }
}

function askConfirm(options: { title: string; message: string; confirmText?: string; danger?: boolean; action: () => Promise<void> }) {
  confirmState.title = options.title
  confirmState.message = options.message
  confirmState.confirmText = options.confirmText || t('common.confirm')
  confirmState.cancelText = t('common.cancel')
  confirmState.danger = options.danger ?? false
  confirmState.action = options.action
  confirmState.show = true
}

function closeConfirm() {
  if (confirmingAction.value) return
  confirmState.show = false
  confirmState.action = null
}

async function runConfirmedAction() {
  if (!confirmState.action || confirmingAction.value) return
  confirmingAction.value = true
  try {
    await confirmState.action()
    confirmState.show = false
    confirmState.action = null
  } finally {
    confirmingAction.value = false
  }
}

function cancelOrder(order: EmailOrder) {
  askConfirm({
    title: t('email.user.cancelConfirmTitle'),
    message: t('email.user.cancelConfirm'),
    confirmText: t('email.user.cancel'),
    danger: true,
    action: async () => {
      actionId.value = order.id
      try {
        await emailAPI.cancel(order.id)
        if (currentOrder.value?.id === order.id) await refreshCurrentOrder(false)
        await loadOrders().catch(() => undefined)
      } catch (error) {
        appStore.showError(errorMessage(error, t('email.user.errors.cancel')))
      } finally {
        actionId.value = ''
      }
    },
  })
}

function openOrders() {
  activeTab.value = 'orders'
  void loadOrders()
}

function openOrder(order: EmailOrder) {
  currentOrder.value = order
  activeTab.value = order.channel_code === 'email_channel_1' ? 'public' : 'private'
  startActiveOrderPolling()
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const result = await emailAPI.orders({ page: orderPagination.page, page_size: orderPagination.pageSize, ...orderFilters }) as EmailOrderPage | EmailOrder[]
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
    appStore.showError(errorMessage(error, t('email.user.errors.orders')))
  } finally {
    ordersLoading.value = false
  }
}

function applyOrderFilters() { Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function resetOrderFilters() { Object.assign(orderDraft, { keyword: '', status: '' }); Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function changeOrderPage(page: number) { orderPagination.page = page; void loadOrders() }
function changeOrderPageSize(pageSize: number) { orderPagination.pageSize = pageSize; orderPagination.page = 1; void loadOrders() }

function openVerificationURL(value?: string) {
  if (!value) return
  try {
    const parsed = new URL(value)
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') throw new Error('unsupported')
    window.open(parsed.toString(), '_blank', 'noopener,noreferrer')
  } catch {
    appStore.showError(t('email.user.invalidVerificationURL'))
  }
}

function errorMessage(error: unknown, fallback: string) {
  return (error as { message?: string })?.message || fallback
}

onMounted(() => {
  void loadAll()
  clockTimer = window.setInterval(() => { now.value = Date.now() }, 1000)
  ordersTimer = window.setInterval(() => {
    if (activeTab.value === 'orders') void loadOrders()
  }, 10000)
})

onBeforeUnmount(() => {
  stopActiveOrderPolling()
  if (ordersTimer) window.clearInterval(ordersTimer)
  if (clockTimer) window.clearInterval(clockTimer)
})
</script>
