<template>
  <AppLayout>
    <main class="email-page">
      <div class="email-shell">
        <section class="email-hero">
          <div class="email-hero__copy">
            <p class="email-eyebrow">MODURELAY</p>
            <h1>{{ t('nav.emailService') }}</h1>
            <p class="email-hero__description">{{ t('email.user.heroDescription') }}</p>

            <div class="email-hero__highlights">
              <div>
                <span class="email-highlight__icon"><Icon name="bolt" size="md" /></span>
                <span><strong>{{ t('email.user.heroSupportTitle') }}</strong><small>{{ t('email.user.heroSupportDesc') }}</small></span>
              </div>
              <div>
                <span class="email-highlight__icon"><Icon name="users" size="md" /></span>
                <span><strong>{{ t('email.user.heroModeTitle') }}</strong><small>{{ t('email.user.heroModeDesc') }}</small></span>
              </div>
              <div>
                <span class="email-highlight__icon"><Icon name="mail" size="md" /></span>
                <span><strong>{{ t('email.user.heroContentTitle') }}</strong><small>{{ t('email.user.heroContentDesc') }}</small></span>
              </div>
            </div>
          </div>

          <div class="email-hero__visual" aria-hidden="true">
            <span class="email-visual-orbit email-visual-orbit--one"></span>
            <span class="email-visual-orbit email-visual-orbit--two"></span>
            <span class="email-visual-bubble email-visual-bubble--left"><Icon name="chat" size="md" /></span>
            <span class="email-visual-bubble email-visual-bubble--right"><Icon name="arrowRight" size="md" /></span>
            <div class="email-envelope">
              <span class="email-envelope__paper">
                <small>CODE</small>
                <strong>123456</strong>
              </span>
              <span class="email-envelope__body"><Icon name="mail" size="xl" /></span>
            </div>
            <span class="email-hero__slogan">{{ t('email.user.heroSlogan') }}</span>
          </div>
        </section>

        <nav class="email-tabs" role="tablist" :aria-label="t('nav.emailService')">
          <button
            type="button"
            :class="{ 'is-active': activeTab === 'public' }"
            role="tab"
            :aria-selected="activeTab === 'public'"
            @click="switchMode('public')"
          >
            {{ t('email.user.publicInbox') }}
          </button>
          <button
            type="button"
            :class="{ 'is-active': activeTab === 'private' }"
            role="tab"
            :aria-selected="activeTab === 'private'"
            @click="switchMode('private')"
          >
            {{ t('email.user.privateInbox') }}
          </button>
          <button
            type="button"
            :class="{ 'is-active': activeTab === 'orders' }"
            role="tab"
            :aria-selected="activeTab === 'orders'"
            @click="openOrders"
          >
            {{ t('email.user.orders') }}
          </button>

          <button
            type="button"
            class="email-tabs__refresh"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="loadAll"
          >
            <Icon name="refresh" size="sm" :class="{ 'is-spinning': loading }" />
          </button>
        </nav>

        <template v-if="activeTab !== 'orders'">
          <section class="email-config">
            <header class="email-config__header">
              <div>
                <h2>{{ t('email.user.setupTitle') }}</h2>
                <p>{{ t('email.user.setupDescription') }}</p>
              </div>
              <span class="email-guide-chip"><Icon name="book" size="sm" />{{ t('email.user.guide') }}</span>
            </header>

            <div class="email-config__grid">
              <section class="email-step">
                <header class="email-step__header">
                  <span class="email-step__number">1</span>
                  <div>
                    <h3>{{ t('email.user.stepChannelTitle') }}</h3>
                    <p>{{ t('email.user.stepChannelDesc') }}</p>
                  </div>
                </header>

                <button
                  type="button"
                  class="email-channel-card"
                  :class="{ 'is-selected': activeTab === 'public' }"
                  @click="switchMode('public')"
                >
                  <span class="email-channel-card__radio"></span>
                  <span class="email-channel-card__icon"><Icon name="database" size="md" /></span>
                  <span class="email-channel-card__copy">
                    <strong>{{ t('email.user.channel1') }}</strong>
                    <small>{{ t('email.user.freePublicChannel') }}</small>
                  </span>
                  <span class="email-channel-card__badge email-channel-card__badge--free">{{ t('email.user.free') }}</span>
                </button>

                <button
                  type="button"
                  class="email-channel-card"
                  :class="{ 'is-selected': activeTab === 'private' }"
                  @click="switchMode('private')"
                >
                  <span class="email-channel-card__radio"></span>
                  <span class="email-channel-card__icon"><Icon name="database" size="md" /></span>
                  <span class="email-channel-card__copy">
                    <strong>{{ t('email.user.channel2') }}</strong>
                    <small>{{ t('email.user.paidPrivateChannel') }}</small>
                  </span>
                  <span class="email-channel-card__badge">{{ t('email.user.private') }}</span>
                </button>
              </section>

              <section class="email-step">
                <header class="email-step__header">
                  <span class="email-step__number">2</span>
                  <div>
                    <h3>{{ t('email.user.stepTypeTitle') }}</h3>
                    <p>{{ t('email.user.stepTypeDesc') }}</p>
                  </div>
                </header>

                <div class="email-type-select">
                  <span class="email-type-select__icon"><Icon name="mail" size="md" /></span>
                  <Select v-model="addressType" :options="addressTypeOptions" />
                </div>

                <div class="email-type-note">
                  <span><Icon name="infoCircle" size="sm" /></span>
                  <div>
                    <strong>{{ t('email.user.typeHintTitle') }}</strong>
                    <p>{{ t('email.user.typeHintDesc') }}</p>
                  </div>
                </div>
              </section>

              <section class="email-step">
                <header class="email-step__header">
                  <span class="email-step__number">3</span>
                  <div>
                    <h3>{{ t('email.user.stepConfirmTitle') }}</h3>
                    <p>{{ t('email.user.stepConfirmDesc') }}</p>
                  </div>
                </header>

                <div class="email-confirm-card">
                  <dl>
                    <div><dt>{{ t('email.user.channel') }}</dt><dd>{{ activeTab === 'public' ? t('email.user.channel1') : t('email.user.channel2') }}</dd></div>
                    <div><dt>{{ t('email.user.addressType') }}</dt><dd>{{ addressTypeLabel(addressType) }}</dd></div>
                    <div><dt>{{ t('email.user.privacy') }}</dt><dd>{{ activeTab === 'public' ? t('email.user.public') : t('email.user.private') }}</dd></div>
                    <div v-if="selectedQuote"><dt>{{ t('email.user.price') }}</dt><dd>{{ priceLabel(selectedQuote.sale_price) }}</dd></div>
                  </dl>

                  <button type="button" class="email-primary-action" :disabled="quoting" @click="loadQuotes">
                    <span v-if="quoting" class="email-spinner"></span>
                    <Icon v-else name="bolt" size="sm" />
                    {{ quoting ? t('email.user.checking') : t('email.user.getQuote') }}
                  </button>

                  <button
                    v-if="selectedQuote"
                    type="button"
                    class="email-primary-action email-primary-action--secondary"
                    :disabled="purchasing"
                    @click="purchase(selectedQuote)"
                  >
                    <span v-if="purchasing" class="email-spinner"></span>
                    <Icon v-else name="mail" size="sm" />
                    {{ purchasing ? t('email.user.processing') : (selectedQuote.sale_price === 0 ? t('email.user.generateFree') : t('email.user.purchasePrivate')) }}
                  </button>
                </div>
              </section>
            </div>
          </section>

          <section v-if="quotes.length > 1" class="email-quotes">
            <button
              v-for="quote in quotes"
              :key="quote.quote_id"
              type="button"
              class="email-quote-card"
              :class="{ 'is-selected': selectedQuote?.quote_id === quote.quote_id }"
              @click="selectedQuoteId = quote.quote_id"
            >
              <span class="email-quote-card__icon"><Icon name="mail" size="md" /></span>
              <span>
                <strong>{{ emailChannelLabel(quote.channel_code) }}</strong>
                <small>{{ privacyLabel(quote.privacy_level) }} · {{ t('email.user.eta') }} {{ quote.estimated_delivery_seconds }}{{ t('email.user.seconds') }}</small>
              </span>
              <strong>{{ priceLabel(quote.sale_price) }}</strong>
            </button>
          </section>

          <section v-if="!quoting && quoteLoaded && !quotes.length" class="email-empty-inline">
            <Icon name="inbox" size="lg" />
            <span>{{ t('email.user.noChannel') }}</span>
          </section>

          <section v-if="currentOrder" class="email-live-order">
            <header class="email-live-order__header">
              <div>
                <p class="email-section-kicker">{{ t('email.user.activeInboxEyebrow') }}</p>
                <h2>{{ t('email.user.activeInbox') }}</h2>
              </div>
              <div class="email-live-order__actions">
                <span class="email-status" :class="statusClass(currentOrder.status)">{{ statusLabel(currentOrder.status) }}</span>
                <button
                  v-if="canCancel(currentOrder)"
                  type="button"
                  class="email-secondary-button"
                  :disabled="actionId === currentOrder.id"
                  @click="cancelOrder(currentOrder)"
                >
                  {{ t('email.user.cancel') }}
                </button>
                <button
                  type="button"
                  class="email-icon-button"
                  :disabled="refreshingId === currentOrder.id"
                  :title="t('common.refresh')"
                  @click="refreshCurrentOrder()"
                >
                  <Icon name="refresh" size="sm" :class="{ 'is-spinning': refreshingId === currentOrder.id }" />
                </button>
              </div>
            </header>

            <div class="email-live-order__summary">
              <div>
                <span>{{ t('email.user.emailAddress') }}</span>
                <strong>{{ currentOrder.email_address || '...' }}</strong>
                <CopyButton v-if="currentOrder.email_address" :text="currentOrder.email_address" />
              </div>
              <div>
                <span>{{ t('email.user.validFor') }}</span>
                <strong>{{ isTerminal(currentOrder) ? t('email.user.windowEnded') : remainingTime }}</strong>
              </div>
              <div>
                <span>{{ t('email.user.latestCode') }}</span>
                <template v-if="latestVerificationCode">
                  <strong class="email-live-order__code">{{ latestVerificationCode }}</strong>
                  <CopyButton :text="latestVerificationCode" />
                </template>
                <small v-else>{{ t('email.user.waiting') }}</small>
              </div>
            </div>

            <div class="email-inbox">
              <header>
                <div>
                  <h3>{{ t('email.user.inbox') }}</h3>
                  <span>{{ t('email.user.messageCount', { count: currentOrder.messages?.length || 0 }) }}</span>
                </div>
                <small v-if="!isTerminal(currentOrder)">{{ t('email.user.inboxReceiving') }}</small>
              </header>

              <div v-if="!currentOrder.messages?.length" class="email-inbox__empty">
                <Icon name="inbox" size="lg" />
                <span>{{ t('email.user.inboxEmpty') }}</span>
              </div>

              <div v-else class="email-message-list">
                <article v-for="message in currentOrder.messages" :key="message.id" class="email-message">
                  <header>
                    <div>
                      <strong>{{ message.from_name || message.from_address }}</strong>
                      <small>{{ message.from_address }}</small>
                    </div>
                    <time>{{ new Date(message.received_at).toLocaleString() }}</time>
                  </header>
                  <p>{{ message.subject }}</p>
                  <div v-if="message.verification_code" class="email-message__code">
                    <span>{{ t('email.user.code') }}</span>
                    <code>{{ message.verification_code }}</code>
                    <CopyButton :text="message.verification_code" />
                  </div>
                  <details>
                    <summary>{{ t('email.user.viewMessage') }}</summary>
                    <pre>{{ message.text_body }}</pre>
                  </details>
                </article>
              </div>
            </div>
          </section>

          <section class="email-recent">
            <header class="email-recent__header">
              <div>
                <p class="email-section-kicker">{{ t('email.user.recentEyebrow') }}</p>
                <h2>{{ t('email.user.recentTitle') }}</h2>
                <p>{{ t('email.user.recentDescription') }}</p>
              </div>
              <div>
                <button type="button" class="email-icon-button" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadAll">
                  <Icon name="refresh" size="sm" />
                </button>
                <button type="button" class="email-text-button" @click="openOrders">
                  {{ t('email.user.viewAll') }} <Icon name="arrowRight" size="sm" />
                </button>
              </div>
            </header>

            <div v-if="recentOrders.length" class="email-records">
              <div class="email-records__head" aria-hidden="true">
                <span>{{ t('email.user.orderNo') }}</span>
                <span>{{ t('email.user.channel') }}</span>
                <span>{{ t('email.user.emailAddress') }}</span>
                <span>{{ t('email.user.addressType') }}</span>
                <span>{{ t('email.user.status') }}</span>
                <span>{{ t('email.user.code') }}</span>
                <span>{{ t('email.user.price') }}</span>
                <span>{{ t('email.user.action') }}</span>
              </div>
              <div v-for="order in recentOrders" :key="order.id" class="email-record-row">
                <CopyableIdentifier :value="order.id" max-width="100%" />
                <span>{{ emailChannelLabel(order.channel_code) }}</span>
                <span class="email-record-row__address">
                  <code>{{ order.email_address || '-' }}</code>
                  <CopyButton v-if="order.email_address" :text="order.email_address" />
                </span>
                <span>{{ addressTypeLabel(order.address_type) }}</span>
                <span><span class="email-status" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span></span>
                <span><code v-if="firstCode(order)">{{ firstCode(order) }}</code><span v-else>-</span></span>
                <span>{{ priceLabel(order.price) }}</span>
                <span><button type="button" class="email-row-action" @click="openOrder(order)">{{ t('common.view') }}</button></span>
              </div>
            </div>

            <div v-else class="email-recent__empty">
              <Icon name="inbox" size="lg" />
              <span>{{ t('email.user.noOrders') }}</span>
            </div>
          </section>
        </template>

        <section v-else class="email-orders">
          <form class="email-orders__filters" @submit.prevent="applyOrderFilters">
            <label>
              <span>{{ t('verificationRecords.filters.keyword') }}</span>
              <div class="email-filter-input">
                <Icon name="search" size="sm" />
                <input v-model.trim="orderDraft.keyword" :placeholder="t('verificationRecords.filters.keywordPlaceholder')" />
              </div>
            </label>

            <div class="email-filter-status">
              <Select
                v-model="orderDraft.status"
                :label="t('verificationRecords.filters.outcome')"
                :options="orderStatusOptions"
                :placeholder="t('verificationRecords.filters.allOutcomes')"
                clearable
                searchable
              />
            </div>

            <button type="submit" class="email-primary-action email-orders__search" :disabled="ordersLoading">
              <Icon name="search" size="sm" />{{ t('common.search') }}
            </button>
            <button type="button" class="email-secondary-button email-orders__reset" :disabled="ordersLoading" @click="resetOrderFilters">
              <Icon name="eraser" size="sm" />{{ t('common.reset') }}
            </button>
          </form>

          <div v-if="ordersLoading" class="email-orders__state">{{ t('email.user.loading') }}</div>
          <div v-else-if="!orders.length" class="email-orders__state">
            <Icon name="inbox" size="lg" />
            <span>{{ t('email.user.noOrders') }}</span>
          </div>

          <section v-else class="email-orders__table">
            <div class="email-records__head" aria-hidden="true">
              <span>{{ t('email.user.orderNo') }}</span>
              <span>{{ t('email.user.channel') }}</span>
              <span>{{ t('email.user.emailAddress') }}</span>
              <span>{{ t('email.user.addressType') }}</span>
              <span>{{ t('email.user.status') }}</span>
              <span>{{ t('email.user.code') }}</span>
              <span>{{ t('email.user.price') }}</span>
              <span>{{ t('email.user.action') }}</span>
            </div>

            <div v-for="order in orders" :key="order.id" class="email-record-row">
              <CopyableIdentifier :value="order.id" max-width="100%" />
              <span>{{ emailChannelLabel(order.channel_code) }}</span>
              <span class="email-record-row__address">
                <code>{{ order.email_address || '-' }}</code>
                <CopyButton v-if="order.email_address" :text="order.email_address" />
              </span>
              <span>{{ addressTypeLabel(order.address_type) }}</span>
              <span><span class="email-status" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span></span>
              <span><code v-if="firstCode(order)">{{ firstCode(order) }}</code><span v-else>-</span></span>
              <span>{{ priceLabel(order.price) }}</span>
              <span><button type="button" class="email-row-action" @click="openOrder(order)">{{ t('common.view') }}</button></span>
            </div>
          </section>

          <div v-if="orderPagination.total > 0" class="email-orders__pagination">
            <Pagination
              :page="orderPagination.page"
              :total="orderPagination.total"
              :page-size="orderPagination.pageSize"
              @update:page="changeOrderPage"
              @update:page-size="changeOrderPageSize"
            />
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
    </main>
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
import CopyableIdentifier from '@/components/common/CopyableIdentifier.vue'
import { emailAPI, type EmailMessage, type EmailOrder, type EmailOrderPage, type EmailQuote } from '@/api/email'
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
const ordersRefreshing = ref(false)
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
const ORDER_LIST_POLL_INTERVAL_MS = 15_000
const ACTIVE_ORDER_POLL_INTERVAL_MS = 3_000

let activeOrderTimer: number | undefined
let ordersTimer: number | undefined
let clockTimer: number | undefined

const orderPagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 })
const orderDraft = reactive({ keyword: '', status: '' })
const orderFilters = reactive({ keyword: '', status: '' })

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
const recentOrders = computed(() => {
  const merged = currentOrder.value
    ? [currentOrder.value, ...orders.value.filter(order => order.id !== currentOrder.value?.id)]
    : orders.value
  return merged.slice(0, 3)
})
const latestVerificationCode = computed(() => currentOrder.value ? firstCode(currentOrder.value) : '')
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
function emailChannelLabel(code: string) {
  const normalized = String(code || '').trim().toLowerCase()
  if (normalized === 'email_channel_1') return t('email.user.channel1')
  if (normalized === 'email_channel_2') return t('email.user.channel2')
  return t('email.user.channel')
}
function privacyLabel(value: string) { return value === 'private_api' ? t('email.user.private') : t('email.user.public') }
function priceLabel(value: number) { return Number(value) === 0 ? t('email.user.free') : `${Number(value).toFixed(4)}` }
function latestCodeFromMessages(messages?: EmailMessage[]) {
  const coded = (messages || []).filter(item => item.verification_code?.trim())
  if (!coded.length) return ''

  let latest = coded[0]
  let latestAt = Date.parse(latest.received_at)
  for (const message of coded.slice(1)) {
    const receivedAt = Date.parse(message.received_at)
    if (Number.isFinite(receivedAt) && (!Number.isFinite(latestAt) || receivedAt > latestAt)) {
      latest = message
      latestAt = receivedAt
    }
  }
  return latest.verification_code?.trim() || ''
}
function firstCode(order: EmailOrder) {
  return order.latest_verification_code?.trim() || latestCodeFromMessages(order.messages)
}
function canCancel(order: EmailOrder) { return ['reserved', 'generating_inbox', 'reconciling', 'waiting_email', 'email_received', 'verification_extracted'].includes(order.status) }

function resetSelection() {
  quotes.value = []
  selectedQuoteId.value = ''
  quoteLoaded.value = false
}

function switchMode(mode: 'public' | 'private') {
  activeTab.value = mode
  addressType.value = mode === 'public' ? 'gmail' : 'gmail_real'
  now.value = Date.now()
  if (currentOrder.value && !isTerminal(currentOrder.value)) startActiveOrderPolling()
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

function pageIsVisible() {
  return typeof document === 'undefined' || document.visibilityState === 'visible'
}

function hasActiveOrders() {
  return orders.value.some(order => !isTerminal(order))
}

function stopOrdersPolling() {
  if (ordersTimer) window.clearTimeout(ordersTimer)
  ordersTimer = undefined
}

function scheduleOrdersPolling() {
  stopOrdersPolling()
  if (activeTab.value !== 'orders' || !pageIsVisible() || !hasActiveOrders()) return

  ordersTimer = window.setTimeout(() => {
    ordersTimer = undefined
    void loadOrders({ silent: true })
  }, ORDER_LIST_POLL_INTERVAL_MS)
}

function stopActiveOrderPolling() {
  if (activeOrderTimer) window.clearTimeout(activeOrderTimer)
  activeOrderTimer = undefined
}

function startActiveOrderPolling() {
  stopActiveOrderPolling()
  if (
    !currentOrder.value
    || isTerminal(currentOrder.value)
    || activeTab.value === 'orders'
    || !pageIsVisible()
  ) return

  activeOrderTimer = window.setTimeout(async () => {
    activeOrderTimer = undefined
    await refreshCurrentOrder(false)
    startActiveOrderPolling()
  }, ACTIVE_ORDER_POLL_INTERVAL_MS)
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
  stopActiveOrderPolling()
  stopOrdersPolling()
  void loadOrders()
}

async function openOrder(order: EmailOrder) {
  stopActiveOrderPolling()
  currentOrder.value = order
  activeTab.value = order.channel_code === 'email_channel_1' ? 'public' : 'private'
  const id = order.id
  refreshingId.value = id
  try {
    const detail = await emailAPI.order(id)
    if (currentOrder.value?.id === id) currentOrder.value = detail
  } catch (error) {
    appStore.showError(errorMessage(error, t('email.user.errors.orders')))
  } finally {
    if (refreshingId.value === id) refreshingId.value = ''
  }
  if (currentOrder.value?.id === id && !isTerminal(currentOrder.value)) startActiveOrderPolling()
}

async function loadOrders(options: { silent?: boolean } = {}) {
  const silent = options.silent === true && orders.value.length > 0
  if (ordersLoading.value || ordersRefreshing.value) return

  if (silent) ordersRefreshing.value = true
  else ordersLoading.value = true

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
    // Background polling must never replace the current table with an error or
    // cause layout movement. Keep the last successful snapshot until the next
    // poll; explicit user actions still surface failures.
    if (!silent) appStore.showError(errorMessage(error, t('email.user.errors.orders')))
  } finally {
    if (silent) ordersRefreshing.value = false
    else ordersLoading.value = false
    scheduleOrdersPolling()
  }
}

function applyOrderFilters() { Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function resetOrderFilters() { Object.assign(orderDraft, { keyword: '', status: '' }); Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() }
function changeOrderPage(page: number) { orderPagination.page = page; void loadOrders() }
function changeOrderPageSize(pageSize: number) { orderPagination.pageSize = pageSize; orderPagination.page = 1; void loadOrders() }

function handleVisibilityChange() {
  if (!pageIsVisible()) {
    stopOrdersPolling()
    stopActiveOrderPolling()
    return
  }

  now.value = Date.now()
  if (activeTab.value === 'orders') {
    stopOrdersPolling()
    void loadOrders({ silent: orders.value.length > 0 })
    return
  }

  if (currentOrder.value && !isTerminal(currentOrder.value)) {
    void refreshCurrentOrder(false).finally(() => startActiveOrderPolling())
  }
}

function errorMessage(error: unknown, fallback: string) {
  return (error as { message?: string })?.message || fallback
}

onMounted(() => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  void loadAll()
  clockTimer = window.setInterval(() => {
    if (activeTab.value !== 'orders' && currentOrder.value && !isTerminal(currentOrder.value)) {
      now.value = Date.now()
    }
  }, 1000)
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  stopOrdersPolling()
  stopActiveOrderPolling()
  if (clockTimer) window.clearInterval(clockTimer)
})
</script>

<style scoped>
.email-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.email-shell {
  width: min(1480px, calc(100% - 40px));
  margin-inline: auto;
  padding: 24px 0 52px;
}

.email-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.16fr) minmax(400px, .84fr);
  min-height: 282px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background:
    radial-gradient(circle at 14% 20%, color-mix(in srgb, var(--color-primary) 7%, transparent), transparent 36%),
    var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.email-hero__copy {
  min-width: 0;
  padding: 34px 38px;
}

.email-eyebrow,
.email-section-kicker {
  margin: 0;
  color: var(--color-accent);
  font-size: .7rem;
  font-weight: 760;
  letter-spacing: .11em;
  text-transform: uppercase;
}

.email-hero h1 {
  margin: .62rem 0 0;
  font-size: clamp(2.05rem, 3vw, 2.85rem);
  font-weight: 740;
  line-height: 1.14;
  letter-spacing: -.045em;
}

.email-hero__description {
  max-width: 700px;
  margin: .72rem 0 0;
  color: var(--color-text-secondary);
  font-size: .9rem;
  line-height: 1.68;
}

.email-hero__highlights {
  display: grid;
  max-width: 780px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin-top: 1.55rem;
}

.email-hero__highlights > div {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
}

.email-highlight__icon {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 11px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.email-hero__highlights > div > span:last-child {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.email-hero__highlights strong {
  color: var(--color-text-primary);
  font-size: .75rem;
  font-weight: 680;
}

.email-hero__highlights small {
  color: var(--color-text-muted);
  font-size: .65rem;
  line-height: 1.35;
}

.email-hero__visual {
  position: relative;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background:
    radial-gradient(circle at 48% 48%, color-mix(in srgb, var(--color-primary) 14%, transparent), transparent 34%),
    linear-gradient(135deg, var(--color-surface-soft), color-mix(in srgb, var(--color-primary-soft) 70%, var(--color-surface)));
}

.email-visual-orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.email-visual-orbit--one {
  width: 290px;
  height: 190px;
}

.email-visual-orbit--two {
  width: 420px;
  height: 270px;
  border-style: dashed;
  border-color: color-mix(in srgb, var(--color-accent) 26%, var(--color-border));
}

.email-envelope {
  position: absolute;
  z-index: 3;
  top: 50%;
  left: 50%;
  width: 190px;
  height: 132px;
  transform: translate(-50%, -45%);
}

.email-envelope__body {
  position: absolute;
  inset: 28px 0 0;
  display: grid;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--color-primary-border);
  border-radius: 22px;
  background: linear-gradient(145deg, var(--color-primary), color-mix(in srgb, var(--color-primary) 62%, var(--color-accent)));
  color: white;
  box-shadow: 0 24px 50px color-mix(in srgb, var(--color-primary) 24%, transparent);
}

.email-envelope__body::before,
.email-envelope__body::after {
  position: absolute;
  bottom: -10px;
  width: 140px;
  height: 92px;
  background: color-mix(in srgb, white 11%, transparent);
  content: '';
}

.email-envelope__body::before {
  left: -40px;
  transform: rotate(32deg);
}

.email-envelope__body::after {
  right: -40px;
  transform: rotate(-32deg);
}

.email-envelope__paper {
  position: absolute;
  z-index: 4;
  top: 0;
  left: 50%;
  display: grid;
  width: 120px;
  height: 76px;
  place-items: center;
  align-content: center;
  gap: 3px;
  border: 1px solid var(--color-border);
  border-radius: 13px;
  background: var(--color-surface-raised);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
  transform: translateX(-50%);
}

.email-envelope__paper small {
  color: var(--color-text-muted);
  font-size: .58rem;
  font-weight: 700;
  letter-spacing: .12em;
}

.email-envelope__paper strong {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 1.2rem;
  letter-spacing: .04em;
}

.email-visual-bubble {
  position: absolute;
  z-index: 2;
  display: grid;
  place-items: center;
  border: 1px solid var(--glass-border);
  border-radius: 13px;
  background: var(--glass-bg);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
  -webkit-backdrop-filter: blur(12px);
  backdrop-filter: blur(12px);
}

.email-visual-bubble--left {
  top: 66px;
  left: 14%;
  width: 52px;
  height: 52px;
}

.email-visual-bubble--right {
  top: 62px;
  right: 13%;
  width: 48px;
  height: 48px;
}

.email-hero__slogan {
  position: absolute;
  right: 24px;
  bottom: 22px;
  color: var(--color-primary);
  font-size: .72rem;
  font-weight: 680;
  letter-spacing: .02em;
}

.email-tabs {
  display: flex;
  align-items: center;
  gap: 4px;
  min-height: 56px;
  margin-top: 12px;
  border-bottom: 1px solid var(--color-border);
}

.email-tabs > button:not(.email-tabs__refresh) {
  position: relative;
  min-height: 56px;
  padding: 0 16px;
  border: 0;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .8rem;
  font-weight: 620;
}

.email-tabs > button:not(.email-tabs__refresh)::after {
  position: absolute;
  right: 12px;
  bottom: -1px;
  left: 12px;
  height: 2px;
  border-radius: 999px;
  background: transparent;
  content: '';
}

.email-tabs > button.is-active {
  color: var(--color-primary);
}

.email-tabs > button.is-active::after {
  background: var(--color-primary);
}

.email-tabs__refresh {
  display: grid;
  width: 36px;
  height: 36px;
  margin-left: auto;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
}

.email-config,
.email-live-order,
.email-recent,
.email-orders {
  margin-top: 14px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.email-config {
  padding: 20px;
}

.email-config__header,
.email-live-order__header,
.email-recent__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.email-config__header h2,
.email-live-order__header h2,
.email-recent__header h2 {
  margin: 0;
  color: var(--color-text-primary);
  font-size: .98rem;
  font-weight: 690;
}

.email-config__header p,
.email-recent__header p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .7rem;
  line-height: 1.5;
}

.email-guide-chip {
  display: inline-flex;
  min-height: 34px;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  color: var(--color-text-secondary);
  font-size: .7rem;
  font-weight: 620;
}

.email-config__grid {
  display: grid;
  grid-template-columns: minmax(0, .95fr) minmax(0, 1.05fr) minmax(0, 1fr);
  gap: 12px;
  margin-top: 14px;
}

.email-step {
  min-width: 0;
  padding: 14px;
  border: 1px solid var(--color-border);
  border-radius: 13px;
  background: var(--color-surface-raised);
}

.email-step__header {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin-bottom: 13px;
}

.email-step__number {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .78rem;
  font-weight: 800;
}

.email-step__header h3 {
  margin: 1px 0 0;
  color: var(--color-text-primary);
  font-size: .8rem;
  font-weight: 670;
}

.email-step__header p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .64rem;
  line-height: 1.45;
}

.email-channel-card {
  display: grid;
  width: 100%;
  min-height: 70px;
  grid-template-columns: 18px 36px minmax(0, 1fr) auto;
  align-items: center;
  gap: 9px;
  padding: 10px 11px;
  border: 1px solid var(--color-border);
  border-radius: 11px;
  background: var(--color-surface);
  color: var(--color-text-primary);
  cursor: pointer;
  text-align: left;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

.email-channel-card + .email-channel-card {
  margin-top: 8px;
}

.email-channel-card:hover {
  border-color: var(--color-primary-border);
}

.email-channel-card.is-selected {
  border-color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 52%, var(--color-surface));
  box-shadow: 0 0 0 2px var(--color-primary-ring);
}

.email-channel-card__radio {
  width: 15px;
  height: 15px;
  border: 1.5px solid var(--color-border-strong);
  border-radius: 50%;
}

.email-channel-card.is-selected .email-channel-card__radio {
  border: 4px solid var(--color-primary);
  background: var(--color-surface);
}

.email-channel-card__icon {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.email-channel-card__copy {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.email-channel-card__copy strong {
  font-size: .76rem;
  font-weight: 670;
}

.email-channel-card__copy small {
  color: var(--color-text-muted);
  font-size: .62rem;
  line-height: 1.35;
}

.email-channel-card__badge {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  padding: 0 8px;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .62rem;
  font-weight: 700;
}

.email-channel-card__badge--free {
  background: color-mix(in srgb, var(--color-success) 10%, var(--color-surface));
  color: var(--color-success);
}

.email-type-select {
  position: relative;
}

.email-type-select__icon {
  position: absolute;
  z-index: 2;
  top: 50%;
  left: 12px;
  display: grid;
  place-items: center;
  color: var(--color-primary);
  transform: translateY(-50%);
  pointer-events: none;
}

.email-type-select :deep(button),
.email-type-select :deep(.select-trigger) {
  padding-left: 42px;
}

.email-type-note {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 11px;
  border: 1px solid var(--color-primary-border);
  border-radius: 10px;
  background: var(--color-primary-soft);
}

.email-type-note > span {
  display: inline-flex;
  flex: 0 0 auto;
  color: var(--color-primary);
}

.email-type-note strong {
  color: var(--color-primary);
  font-size: .68rem;
  font-weight: 680;
}

.email-type-note p {
  margin: 3px 0 0;
  color: var(--color-text-secondary);
  font-size: .63rem;
  line-height: 1.5;
}

.email-confirm-card {
  padding: 11px;
  border: 1px solid var(--color-border);
  border-radius: 11px;
  background: var(--color-surface);
}

.email-confirm-card dl {
  display: grid;
  gap: 0;
  margin: 0;
}

.email-confirm-card dl > div {
  display: flex;
  min-height: 32px;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.email-confirm-card dl > div:last-child {
  border-bottom: 0;
}

.email-confirm-card dt {
  color: var(--color-text-muted);
  font-size: .66rem;
}

.email-confirm-card dd {
  margin: 0;
  color: var(--color-text-primary);
  font-size: .7rem;
  font-weight: 670;
  text-align: right;
}

.email-primary-action,
.email-secondary-button,
.email-text-button,
.email-row-action,
.email-icon-button {
  font: inherit;
}

.email-primary-action {
  display: inline-flex;
  width: 100%;
  min-height: 40px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 10px;
  border: 1px solid var(--color-primary);
  border-radius: 9px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font-size: .72rem;
  font-weight: 670;
}

.email-primary-action--secondary {
  margin-top: 7px;
  background: color-mix(in srgb, var(--color-primary) 88%, var(--color-accent));
}

.email-primary-action:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.email-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid color-mix(in srgb, var(--color-surface) 38%, transparent);
  border-top-color: var(--color-surface);
  border-radius: 50%;
  animation: email-spin .75s linear infinite;
}

.email-quotes {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.email-quote-card {
  display: grid;
  min-width: 0;
  min-height: 72px;
  grid-template-columns: 36px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 11px 12px;
  border: 1px solid var(--color-border);
  border-radius: 12px;
  background: var(--color-surface);
  color: var(--color-text-primary);
  cursor: pointer;
  text-align: left;
}

.email-quote-card.is-selected {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 2px var(--color-primary-ring);
}

.email-quote-card__icon {
  display: grid;
  width: 36px;
  height: 36px;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.email-quote-card > span:nth-child(2) {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.email-quote-card strong {
  font-size: .73rem;
  font-weight: 680;
}

.email-quote-card small {
  color: var(--color-text-muted);
  font-size: .63rem;
}

.email-empty-inline {
  display: flex;
  min-height: 96px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 12px;
  border: 1px dashed var(--color-border-strong);
  border-radius: 12px;
  background: var(--color-surface);
  color: var(--color-text-muted);
  font-size: .72rem;
}

.email-live-order {
  padding: 18px;
}

.email-live-order__actions {
  display: flex;
  align-items: center;
  gap: 7px;
}

.email-status {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  padding: 0 8px;
  border-radius: 999px;
  font-size: .62rem;
  font-weight: 680;
}

.email-status.badge-success {
  background: color-mix(in srgb, var(--color-success) 11%, var(--color-surface));
  color: var(--color-success);
}

.email-status.badge-danger {
  background: color-mix(in srgb, var(--color-danger) 10%, var(--color-surface));
  color: var(--color-danger);
}

.email-status.badge-warning {
  background: color-mix(in srgb, var(--color-warning) 11%, var(--color-surface));
  color: var(--color-warning);
}

.email-status.badge-info {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.email-secondary-button {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font-size: .68rem;
  font-weight: 620;
}

.email-icon-button {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
}

.email-live-order__summary {
  display: grid;
  grid-template-columns: minmax(0, 1.2fr) minmax(120px, .45fr) minmax(180px, .65fr);
  gap: 10px;
  margin-top: 14px;
}

.email-live-order__summary > div {
  display: flex;
  min-width: 0;
  min-height: 72px;
  flex-wrap: wrap;
  align-content: center;
  align-items: center;
  gap: 5px 8px;
  padding: 10px 11px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.email-live-order__summary > div > span:first-child {
  width: 100%;
  color: var(--color-text-muted);
  font-size: .63rem;
}

.email-live-order__summary strong {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .78rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.email-live-order__summary small {
  color: var(--color-text-muted);
  font-size: .68rem;
}

.email-live-order__code {
  color: var(--color-primary) !important;
  font-size: 1.12rem !important;
  letter-spacing: .04em;
}

.email-inbox {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--color-border-subtle);
}

.email-inbox > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.email-inbox > header > div {
  display: flex;
  align-items: center;
  gap: 7px;
}

.email-inbox h3 {
  margin: 0;
  font-size: .78rem;
  font-weight: 670;
}

.email-inbox > header span {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 7px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .61rem;
}

.email-inbox > header small {
  color: var(--color-text-muted);
  font-size: .63rem;
}

.email-inbox__empty {
  display: flex;
  min-height: 100px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 10px;
  border-radius: 10px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .7rem;
}

.email-message-list {
  display: grid;
  gap: 8px;
  margin-top: 10px;
}

.email-message {
  padding: 11px 12px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.email-message > header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.email-message > header > div {
  display: grid;
  gap: 2px;
}

.email-message strong {
  font-size: .72rem;
  font-weight: 650;
}

.email-message small,
.email-message time {
  color: var(--color-text-muted);
  font-size: .61rem;
}

.email-message > p {
  margin: 7px 0 0;
  color: var(--color-text-secondary);
  font-size: .72rem;
}

.email-message__code {
  display: flex;
  align-items: center;
  gap: 7px;
  margin-top: 9px;
}

.email-message__code > span {
  color: var(--color-text-muted);
  font-size: .62rem;
}

.email-message__code code {
  padding: 3px 7px;
  border-radius: 7px;
  background: var(--color-surface);
  color: var(--color-primary);
  font-size: .88rem;
  font-weight: 700;
}

.email-message details {
  margin-top: 9px;
}

.email-message summary {
  color: var(--color-primary);
  cursor: pointer;
  font-size: .68rem;
  font-weight: 620;
}

.email-message pre {
  margin: 8px 0 0;
  overflow-x: auto;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  color: var(--color-text-secondary);
  font-family: inherit;
  font-size: .68rem;
  line-height: 1.55;
}

.email-recent {
  overflow: hidden;
}

.email-recent__header {
  align-items: center;
  padding: 14px 16px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.email-recent__header > div:last-child {
  display: flex;
  align-items: center;
  gap: 7px;
}

.email-text-button {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  border: 0;
  background: transparent;
  color: var(--color-primary);
  cursor: pointer;
  font-size: .68rem;
  font-weight: 650;
}

.email-records,
.email-orders__table {
  overflow-x: auto;
}

.email-records__head,
.email-record-row {
  display: grid;
  min-width: 1210px;
  grid-template-columns: 230px 110px minmax(280px, 1.5fr) 150px 110px 110px 90px 70px;
  align-items: center;
  gap: 8px;
}

.email-records__head {
  min-height: 38px;
  padding: 0 16px;
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .62rem;
  font-weight: 650;
}

.email-record-row {
  min-height: 48px;
  padding: 0 16px;
  border-bottom: 1px solid var(--color-border-subtle);
  color: var(--color-text-secondary);
  font-size: .68rem;
}

.email-record-row:last-child {
  border-bottom: 0;
}

.email-record-row__address {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 6px;
}

.email-record-row__address code {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text-primary);
  text-overflow: ellipsis;
  white-space: nowrap;
}

.email-record-row code {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

.email-row-action {
  min-height: 28px;
  padding: 0 9px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font-size: .64rem;
  font-weight: 620;
}

.email-recent__empty,
.email-orders__state {
  display: flex;
  min-height: 130px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-text-muted);
  font-size: .7rem;
}

.email-orders {
  overflow: hidden;
}

.email-orders__filters {
  display: grid;
  grid-template-columns: minmax(280px, 1fr) 230px auto auto;
  align-items: end;
  gap: 10px;
  padding: 16px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.email-orders__filters > label {
  display: grid;
  gap: 6px;
  color: var(--color-text-secondary);
  font-size: .68rem;
  font-weight: 620;
}

.email-filter-input {
  position: relative;
}

.email-filter-input svg {
  position: absolute;
  top: 50%;
  left: 11px;
  color: var(--color-text-muted);
  transform: translateY(-50%);
}

.email-filter-input input {
  width: 100%;
  min-height: 40px;
  padding: 0 11px 0 36px;
  border: 1px solid var(--color-border-strong);
  border-radius: 9px;
  outline: none;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .72rem;
}

.email-filter-input input:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.email-orders__search {
  width: auto;
  min-width: 86px;
  margin-top: 0;
}

.email-orders__reset {
  min-height: 40px;
}

.email-orders__pagination {
  border-top: 1px solid var(--color-border-subtle);
}

.email-tabs button:focus-visible,
.email-channel-card:focus-visible,
.email-quote-card:focus-visible,
.email-primary-action:focus-visible,
.email-secondary-button:focus-visible,
.email-icon-button:focus-visible,
.email-text-button:focus-visible,
.email-row-action:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

.is-spinning {
  animation: email-spin .8s linear infinite;
}

@keyframes email-spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 1180px) {
  .email-hero {
    grid-template-columns: minmax(0, 1fr) 340px;
  }

  .email-config__grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .email-step:last-child {
    grid-column: 1 / -1;
  }

  .email-orders__filters {
    grid-template-columns: minmax(260px, 1fr) 220px auto auto;
  }
}

@media (max-width: 900px) {
  .email-shell {
    width: min(100% - 28px, 820px);
  }

  .email-hero {
    grid-template-columns: 1fr;
  }

  .email-hero__visual {
    min-height: 230px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .email-config__grid,
  .email-quotes {
    grid-template-columns: 1fr;
  }

  .email-step:last-child {
    grid-column: auto;
  }

  .email-live-order__summary {
    grid-template-columns: 1fr 1fr;
  }

  .email-live-order__summary > div:first-child {
    grid-column: 1 / -1;
  }

  .email-orders__filters {
    grid-template-columns: 1fr 1fr;
  }

  .email-orders__search,
  .email-orders__reset {
    width: 100%;
  }
}

@media (max-width: 640px) {
  .email-shell {
    width: min(100% - 20px, 560px);
    padding-top: 16px;
  }

  .email-hero {
    min-height: auto;
    border-radius: 14px;
  }

  .email-hero__copy {
    padding: 24px 20px;
  }

  .email-hero h1 {
    font-size: 2rem;
  }

  .email-hero__highlights {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .email-hero__visual {
    display: none;
  }

  .email-tabs {
    overflow-x: auto;
  }

  .email-tabs > button:not(.email-tabs__refresh) {
    min-width: max-content;
  }

  .email-tabs__refresh {
    flex: 0 0 36px;
  }

  .email-config {
    padding: 14px;
  }

  .email-config__header {
    align-items: flex-start;
  }

  .email-guide-chip {
    display: none;
  }

  .email-live-order__summary {
    grid-template-columns: 1fr;
  }

  .email-live-order__summary > div:first-child {
    grid-column: auto;
  }

  .email-live-order__header,
  .email-recent__header {
    align-items: flex-start;
    flex-direction: column;
  }

  .email-live-order__actions,
  .email-recent__header > div:last-child {
    width: 100%;
  }

  .email-recent__header > div:last-child {
    justify-content: space-between;
  }

  .email-orders__filters {
    grid-template-columns: 1fr;
  }
}

@media (prefers-reduced-motion: reduce) {
  .email-channel-card,
  .email-primary-action,
  .email-quote-card {
    transition-duration: 1ms;
  }

  .email-spinner,
  .is-spinning {
    animation: none;
  }
}
</style>