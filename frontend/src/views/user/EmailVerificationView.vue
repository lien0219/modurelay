<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-[1800px] space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0"><p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">ModuRelay</p><h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.emailService') }}</h1><p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('email.user.description') }}</p></div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="loadAll"><Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" /></button>
      </header>

      <div class="flex gap-2 border-b border-gray-200 dark:border-dark-700" role="tablist" :aria-label="t('nav.emailService')">
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'inbox' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="activeTab === 'inbox'" @click="activeTab = 'inbox'">{{ t('email.user.temporary') }}</button>
        <button type="button" class="border-b-2 px-3 py-2 text-sm font-medium" :class="activeTab === 'orders' ? 'border-primary-600 text-primary-600' : 'border-transparent text-gray-500'" role="tab" :aria-selected="activeTab === 'orders'" @click="openOrders">{{ t('email.user.orders') }}</button>
      </div>

      <section v-if="activeTab === 'inbox'" class="card space-y-4 p-5">
        <div class="grid gap-4 sm:grid-cols-2">
          <Select v-model="serviceCode" :label="t('email.user.service')" :options="serviceOptions" :placeholder="t('email.user.selectService')" searchable>
            <template #selected="{ option }"><VerificationIdentity v-if="option" kind="platform" :code="String(option.value)" :label="String(option.label)" :show-code="false" /><span v-else>{{ t('email.user.selectService') }}</span></template>
            <template #option="{ option }"><VerificationIdentity kind="platform" :code="String(option.value)" :label="String(option.label)" /></template>
          </Select>
          <Select v-model="addressType" :label="t('email.user.addressType')" :options="addressTypeOptions" />
        </div>
        <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm leading-6 text-amber-900 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-200">{{ t('email.user.privacyWarning') }}</div>
        <div class="flex flex-wrap items-center justify-between gap-3"><span class="text-xs text-gray-500 dark:text-gray-400">{{ quotes.length ? t('email.user.quoteHint') : t('email.user.chooseForQuote') }}</span><button type="button" class="btn btn-secondary" :disabled="!serviceCode || quoting" @click="loadQuotes">{{ quoting ? t('email.user.checking') : t('sms.user.getQuote') }}</button></div>
      </section>

      <section v-if="activeTab === 'inbox'" class="space-y-3">
        <div v-if="quoting" class="card p-8 text-center text-sm text-gray-500">{{ t('email.user.checking') }}</div><div v-else-if="!quotes.length" class="card p-8 text-center text-sm text-gray-500">{{ serviceCode ? t('email.user.noChannel') : t('email.user.chooseForQuote') }}</div>
        <article v-for="quote in quotes" :key="quote.quote_id" class="card flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
          <div class="min-w-0"><div class="flex flex-wrap items-center gap-2"><h2 class="font-semibold text-gray-900 dark:text-white">{{ quote.public_name }}</h2><span class="badge badge-info">{{ t('email.user.gmail') }}</span><span v-if="quote.success_rate_grade" class="badge badge-success">{{ quote.success_rate_grade }}</span></div><div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm text-gray-500 dark:text-gray-400"><span>{{ t('email.user.eta') }}: {{ quote.estimated_delivery_seconds }}{{ t('email.user.seconds') }}</span><span>{{ t('email.user.retention') }}: {{ quote.retention_description }}</span><span v-if="quote.success_rate != null">{{ t('email.user.successRate') }}: {{ (quote.success_rate * 100).toFixed(1) }}%</span><span v-else>{{ t('email.user.insufficientSuccessData') }}</span></div><p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ refundPolicyLabel(quote.refund_policy_description) }}</p></div>
          <div class="flex shrink-0 items-center gap-3"><strong class="text-lg tabular-nums text-gray-900 dark:text-white">¥{{ quote.sale_price.toFixed(2) }}</strong><button type="button" class="btn btn-primary" :disabled="purchasing" @click="purchase(quote)">{{ purchasing ? t('email.user.processing') : t('email.user.purchase') }}</button></div>
        </article>
      </section>

      <section v-else class="space-y-3">
        <form class="glass-panel grid items-end gap-3 rounded-xl p-4 md:grid-cols-[minmax(240px,1fr)_220px_auto]" @submit.prevent="applyOrderFilters">
          <label class="block min-w-0"><span class="input-label">{{ t('verificationRecords.filters.keyword') }}</span><input v-model.trim="orderDraft.keyword" class="input h-[42px]" :placeholder="t('verificationRecords.filters.keywordPlaceholder')" /></label>
          <Select v-model="orderDraft.status" :label="t('verificationRecords.filters.outcome')" :options="orderStatusOptions" :placeholder="t('verificationRecords.filters.allOutcomes')" clearable searchable />
          <div class="flex h-[42px] items-center gap-2 self-end"><button type="submit" class="btn btn-primary h-[42px]" :disabled="ordersLoading"><Icon name="search" size="sm" aria-hidden="true" />{{ t('common.search') }}</button><button type="button" class="btn btn-secondary h-[42px]" :disabled="ordersLoading" @click="resetOrderFilters"><Icon name="eraser" size="sm" aria-hidden="true" />{{ t('common.reset') }}</button></div>
        </form>
        <div v-if="ordersLoading" class="card p-8 text-center text-sm text-gray-500">{{ t('email.user.loading') }}</div><div v-else-if="!orders.length" class="card p-8 text-center text-sm text-gray-500">{{ t('email.user.noOrders') }}</div>
        <article v-for="order in orders" :key="order.id" class="card space-y-4 p-5">
          <div class="flex flex-wrap items-start justify-between gap-3"><div class="min-w-0"><div class="flex flex-wrap items-center gap-2"><VerificationIdentity kind="platform" :code="order.service_code" :label="serviceLabel(order.service_code)" /><span class="badge" :class="statusClass(order.status)">{{ statusLabel(order.status) }}</span></div><p class="mt-1 break-all font-mono text-sm text-gray-700 dark:text-gray-200">{{ order.email_address || '...' }}</p><p class="mt-1 text-xs text-gray-500">{{ order.order_no }}</p><p v-if="order.error_message" class="mt-2 text-sm text-red-600 dark:text-red-300" role="status">{{ order.error_message }}</p></div><div class="flex flex-wrap items-center gap-2"><button v-if="canCancel(order)" type="button" class="btn btn-secondary btn-sm" :disabled="actionId === order.id" @click="cancelOrder(order)">{{ t('email.user.cancel') }}</button><button v-if="canRefund(order)" type="button" class="btn btn-secondary btn-sm" :disabled="actionId === order.id" @click="requestRefund(order)">{{ t('email.user.requestRefund') }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="refreshingId === order.id" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="refreshOrder(order.id)"><Icon name="refresh" size="sm" :class="refreshingId === order.id ? 'animate-spin' : ''" aria-hidden="true" /></button></div></div>
          <div v-if="order.messages?.length" class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700"><div v-for="message in order.messages" :key="message.id" class="rounded-lg bg-gray-50 p-4 dark:bg-dark-800"><div class="flex flex-wrap justify-between gap-2 text-sm"><span class="font-medium text-gray-900 dark:text-white">{{ message.from_name || message.from_address }}</span><time class="text-xs text-gray-500">{{ new Date(message.received_at).toLocaleString() }}</time></div><p class="mt-1 text-sm text-gray-700 dark:text-gray-200">{{ message.subject }}</p><div v-if="message.verification_code" class="mt-3 flex flex-wrap items-center gap-2"><span class="text-xs text-gray-500">{{ t('email.user.code') }}</span><code class="rounded bg-white px-2 py-1 font-mono text-lg font-semibold text-gray-900 dark:bg-dark-700 dark:text-white">{{ message.verification_code }}</code><button type="button" class="btn btn-secondary btn-sm" @click="copy(message.verification_code)">{{ t('email.user.copy') }}</button></div><details class="mt-3"><summary class="cursor-pointer text-sm font-medium text-primary-600 dark:text-primary-300">{{ t('email.user.viewMessage') }}</summary><pre class="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-gray-700 dark:text-gray-200">{{ message.text_body }}</pre></details></div></div>
          <p v-else class="border-t border-gray-100 pt-4 text-sm text-gray-500 dark:border-dark-700">{{ order.status === 'waiting_email' ? t('email.user.waiting') : t('email.user.loading') }}</p>
        </article>
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
import { emailAPI, type EmailOrder, type EmailOrderPage, type EmailQuote, type EmailServiceItem } from '@/api/email'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useAppStore } from '@/stores'

const { t } = useI18n(); const appStore = useAppStore()
const activeTab = ref<'inbox' | 'orders'>('inbox'); const services = ref<EmailServiceItem[]>([]); const quotes = ref<EmailQuote[]>([]); const orders = ref<EmailOrder[]>([])
const serviceCode = ref(''); const addressType = ref('gmail'); const loading = ref(false); const quoting = ref(false); const ordersLoading = ref(false); const purchasing = ref(false); const refreshingId = ref(''); const actionId = ref(''); let timer: number | undefined
const orderPagination = reactive({ page: 1, pageSize: getPersistedPageSize(20), total: 0 }); const orderDraft = reactive({ keyword: '', status: '' }); const orderFilters = reactive({ keyword: '', status: '' })
const serviceOptions = computed(() => services.value.map(item => ({ value: item.code, label: item.name }))); const addressTypeOptions = computed(() => [{ value: 'gmail', label: t('email.user.gmail') }])
const emailStatuses = ['reserved', 'generating_inbox', 'waiting_email', 'email_received', 'verification_extracted', 'completed', 'reconciling', 'expired', 'refunded', 'failed', 'cancelled']
const orderStatusOptions = computed(() => emailStatuses.map(value => ({ value, label: statusLabel(value) })))
const serviceLabel = (code: string) => services.value.find(item => item.code === code)?.name || code
const statusLabel = (status: string) => t(`email.user.statuses.${status}`, status); const statusClass = (status: string) => ['completed', 'refunded'].includes(status) ? 'badge-success' : status === 'failed' ? 'badge-danger' : ['expired', 'cancelled'].includes(status) ? 'badge-warning' : 'badge-info'
const refundPolicyLabel = (policy: string) => t(`email.user.refundPolicies.${policy}`, policy); const errorMessage = (error: unknown, fallback: string) => (error as { message?: string })?.message || fallback
async function loadAll() { loading.value = true; try { services.value = await emailAPI.services(); if (!serviceCode.value) serviceCode.value = services.value[0]?.code || ''; await loadQuotes(); if (activeTab.value === 'orders') await loadOrders() } catch (error) { appStore.showError(errorMessage(error, t('email.user.errors.unavailable'))) } finally { loading.value = false } }
async function loadQuotes() { if (!serviceCode.value) return; quoting.value = true; try { quotes.value = await emailAPI.quotes({ service: serviceCode.value, address_type: addressType.value }) } catch (error) { quotes.value = []; appStore.showError(errorMessage(error, t('email.user.errors.quote'))) } finally { quoting.value = false } }
async function loadOrders() { ordersLoading.value = true; try { const result = await emailAPI.orders({ page: orderPagination.page, page_size: orderPagination.pageSize, ...orderFilters }) as EmailOrderPage | EmailOrder[]; if (Array.isArray(result)) { orders.value = result; orderPagination.total = result.length } else { orders.value = result.items; orderPagination.total = result.total; orderPagination.page = result.page; orderPagination.pageSize = result.page_size } } catch (error) { appStore.showError(errorMessage(error, t('email.user.errors.orders'))) } finally { ordersLoading.value = false } }
function openOrders() { activeTab.value = 'orders'; void loadOrders() } function applyOrderFilters() { Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() } function resetOrderFilters() { Object.assign(orderDraft, { keyword: '', status: '' }); Object.assign(orderFilters, orderDraft); orderPagination.page = 1; void loadOrders() } function changeOrderPage(page: number) { orderPagination.page = page; void loadOrders() } function changeOrderPageSize(pageSize: number) { orderPagination.pageSize = pageSize; orderPagination.page = 1; void loadOrders() }
async function purchase(quote: EmailQuote) { purchasing.value = true; try { await emailAPI.purchase({ channel_code: quote.channel_code, service_code: serviceCode.value, address_type: addressType.value, expected_price: quote.sale_price, quote_id: quote.quote_id }, `email-${quote.quote_id}`); activeTab.value = 'orders'; orderPagination.page = 1; await loadOrders() } catch (error) { appStore.showError(errorMessage(error, t('email.user.errors.purchase'))) } finally { purchasing.value = false } }
const canCancel = (order: EmailOrder) => ['reserved', 'generating_inbox', 'reconciling', 'waiting_email', 'email_received', 'verification_extracted'].includes(order.status); const canRefund = (order: EmailOrder) => order.refund_policy === 'refund_if_no_message' && !order.first_message_at && ['waiting_email', 'email_received'].includes(order.status)
async function cancelOrder(order: EmailOrder) { if (!window.confirm(t('common.confirm'))) return; actionId.value = order.id; try { await emailAPI.cancel(order.id); await refreshOrder(order.id) } catch (error) { appStore.showError(errorMessage(error, t('email.user.errors.cancel'))) } finally { actionId.value = '' } }
async function requestRefund(order: EmailOrder) { if (!window.confirm(t('common.confirm'))) return; actionId.value = order.id; try { await emailAPI.requestRefund(order.id); await refreshOrder(order.id) } catch (error) { appStore.showError(errorMessage(error, t('email.user.errors.refund'))) } finally { actionId.value = '' } }
async function refreshOrder(id: string) { refreshingId.value = id; try { const updated = await emailAPI.order(id); const index = orders.value.findIndex(item => item.id === id); if (index >= 0) orders.value[index] = updated } catch (error) { appStore.showError(errorMessage(error, t('email.user.errors.orders'))) } finally { refreshingId.value = '' } }
async function copy(value: string) { try { await navigator.clipboard.writeText(value); appStore.showSuccess(t('email.user.copied')) } catch { appStore.showError(t('email.user.copy')) } }
onMounted(() => { void loadAll(); timer = window.setInterval(() => { if (activeTab.value === 'orders') void loadOrders() }, 10000) }); onBeforeUnmount(() => { if (timer) window.clearInterval(timer) })
</script>
