<template>
  <AppLayout>
    <div class="mx-auto w-full min-w-0 max-w-[1800px] space-y-5">
      <header class="flex flex-wrap items-end justify-between gap-3">
        <div class="min-w-0">
          <p class="text-xs font-semibold uppercase tracking-[0.14em] text-cyan-600 dark:text-cyan-400">{{ t('email.admin.eyebrow') }}</p>
          <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ t('nav.emailManagement') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('email.admin.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" :aria-label="t('common.refresh')" @click="load">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" aria-hidden="true" />
        </button>
      </header>

      <section class="card flex flex-col gap-4 p-5 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0"><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('email.admin.enableTitle') }}</h2><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('email.admin.enableDescription') }}</p></div>
        <div class="flex min-h-11 shrink-0 items-center gap-3 self-start sm:self-center">
          <Toggle v-model="enabled" :label="t('email.admin.switchLabel')" :disabled="savingEnabled" @update:model-value="toggle" />
          <span class="min-w-12 text-sm font-medium text-gray-700 dark:text-gray-200">{{ enabled ? t('common.enabled') : t('common.disabled') }}</span>
        </div>
      </section>

      <section class="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        <div v-for="item in statItems" :key="item.key" class="card p-4"><div class="text-xs uppercase tracking-wide text-gray-500">{{ item.label }}</div><div class="mt-1 text-2xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ item.value }}</div></div>
      </section>

      <section class="card p-5">
        <div class="mb-4"><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('email.admin.billingTitle') }}</h2><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('email.admin.billingDescription') }}</p></div>
        <p class="mb-4 rounded-lg border border-cyan-200 bg-cyan-50 px-4 py-3 text-sm leading-6 text-cyan-900 dark:border-cyan-900/70 dark:bg-cyan-950/30 dark:text-cyan-100">{{ t('email.admin.billingGuide') }}</p>
        <div v-for="provider in providers" :key="provider.id" class="grid gap-3 border-t border-gray-100 py-4 first:border-t-0 dark:border-dark-700 sm:grid-cols-2 lg:grid-cols-4">
          <div class="sm:col-span-2 lg:col-span-4 text-sm font-medium text-gray-900 dark:text-white">{{ provider.name }}</div>
          <label class="input-label">{{ t('email.admin.costMode') }}<select v-model="billingDrafts[provider.id].cost_mode" class="input mt-1"><option value="request_based_plus_amortized">{{ t('email.admin.costModes.request_based_plus_amortized') }}</option><option value="request_based">{{ t('email.admin.costModes.request_based') }}</option><option value="monthly_amortized">{{ t('email.admin.costModes.monthly_amortized') }}</option><option value="fixed_per_order">{{ t('email.admin.costModes.fixed_per_order') }}</option><option value="manual">{{ t('email.admin.costModes.manual') }}</option></select><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.costMode') }}</span></label>
          <label class="input-label">{{ t('email.admin.monthlyFee') }}<input v-model.number="billingDrafts[provider.id].monthly_fee" type="number" min="0" step="0.01" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.monthlyFee') }}</span></label>
          <label class="input-label">{{ t('email.admin.includedMonthly') }}<input v-model.number="billingDrafts[provider.id].included_requests_monthly" type="number" min="0" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.includedMonthly') }}</span></label>
          <label class="input-label">{{ t('email.admin.includedDaily') }}<input v-model.number="billingDrafts[provider.id].included_requests_daily" type="number" min="0" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.includedDaily') }}</span></label>
          <label class="input-label">{{ t('email.admin.overagePrice') }}<input v-model.number="billingDrafts[provider.id].overage_price_per_request" type="number" min="0" step="0.0001" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.overagePrice') }}</span></label>
          <label class="input-label">{{ t('email.admin.warningPercent') }}<input v-model.number="billingDrafts[provider.id].quota_warning_percent" type="number" min="0" max="100" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.warningPercent') }}</span></label>
          <label class="input-label">{{ t('email.admin.stopPercent') }}<input v-model.number="billingDrafts[provider.id].quota_stop_percent" type="number" min="0" max="100" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.stopPercent') }}</span></label>
          <label class="input-label">{{ t('email.admin.reservePercent') }}<input v-model.number="billingDrafts[provider.id].quota_reserved_for_active_orders" type="number" min="0" max="100" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.reservePercent') }}</span></label>
          <label class="input-label">{{ t('email.admin.ratePerMinute') }}<input v-model.number="billingDrafts[provider.id].rate_limit_per_minute" type="number" min="0" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.ratePerMinute') }}</span></label>
          <label class="input-label">{{ t('email.admin.ratePerHour') }}<input v-model.number="billingDrafts[provider.id].rate_limit_per_hour" type="number" min="0" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.ratePerHour') }}</span></label>
          <label class="input-label">{{ t('email.admin.fixedOrderCost') }}<input v-model.number="billingDrafts[provider.id].fixed_cost_per_order" type="number" min="0" step="0.0001" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.fixedOrderCost') }}</span></label>
          <label class="input-label">{{ t('email.admin.estimatedRequests') }}<input v-model.number="billingDrafts[provider.id].estimated_requests_per_order" type="number" min="1" step="1" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.estimatedRequests') }}</span></label>
          <label class="input-label">{{ t('email.admin.providerConcurrency') }}<input v-model.number="billingDrafts[provider.id].provider_concurrency" type="number" min="1" max="32" step="1" class="input mt-1" /><span class="mt-1 block text-xs font-normal leading-5 text-gray-500 dark:text-gray-400">{{ t('email.admin.billingHints.providerConcurrency') }}</span></label>
          <div class="flex items-start pt-6"><button type="button" class="btn btn-secondary" :disabled="isProviderSaving(provider.id)" :aria-busy="isProviderSaving(provider.id)" @click="saveProvider(provider)">{{ isProviderSaving(provider.id) ? t('email.admin.saving') : t('email.admin.saveBilling') }}</button></div>
        </div>
      </section>

      <section class="card min-w-0 overflow-hidden">
        <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700"><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('email.admin.providers') }}</h2></div>
        <div class="max-w-full overflow-x-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500" tabindex="0" :aria-label="t('email.admin.providers')"><table class="w-full min-w-[1100px] text-left text-sm"><thead class="whitespace-nowrap bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">{{ t('email.admin.name') }}</th><th class="px-5 py-3">{{ t('email.admin.baseUrl') }}</th><th class="px-5 py-3">{{ t('email.admin.credential') }}</th><th class="px-5 py-3">{{ t('email.admin.health') }}</th><th class="px-5 py-3">{{ t('email.admin.enabled') }}</th><th class="px-5 py-3">{{ t('email.admin.action') }}</th></tr></thead><tbody>
          <tr v-for="provider in providers" :key="provider.id" class="border-t border-gray-100 dark:border-dark-700"><td class="px-5 py-3 font-medium text-gray-900 dark:text-white"><div class="flex min-w-48 items-center gap-2"><span>{{ provider.name }}</span><a v-if="providerPortalUrl(provider)" :href="providerPortalUrl(provider)" target="_blank" rel="noopener noreferrer" class="inline-flex min-h-8 items-center gap-1 rounded-md px-2 text-xs text-primary-600 hover:bg-primary-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:text-primary-300 dark:hover:bg-primary-950/30" :title="t('email.admin.openProvider')"><Icon name="externalLink" size="xs" aria-hidden="true" /><span>{{ t('email.admin.portal') }}</span></a></div></td><td class="px-5 py-3"><input v-model="provider.base_url" class="input min-w-56" :aria-label="`${t('email.admin.baseUrl')} - ${provider.name}`" /></td><td class="px-5 py-3">
  <input v-if="provider.requires_credential" v-model="credentialRefs[provider.id]" type="password" autocomplete="new-password" class="input min-w-52" :placeholder="t('email.admin.credentialPlaceholder')" :aria-label="`${t('email.admin.credential')} - ${provider.name}`" />
  <span v-else class="badge badge-success">{{ t('email.admin.noCredentialRequired') }}</span>
</td><td class="px-5 py-3"><span class="badge" :class="provider.health_status === 'healthy' ? 'badge-success' : 'badge-warning'">{{ t(`email.admin.status.${provider.health_status}`, provider.health_status) }}</span></td><td class="px-5 py-3"><div class="flex min-w-28 items-center gap-2"><Toggle v-model="provider.enabled" :label="t('email.admin.providerEnabled', { name: provider.name })" :disabled="isProviderSaving(provider.id)" @update:model-value="saveProvider(provider)" /><span class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ provider.enabled ? t('common.enabled') : t('common.disabled') }}</span></div></td><td class="px-5 py-3"><div class="flex flex-wrap gap-2"><button type="button" class="btn btn-secondary btn-sm" :disabled="isProviderSaving(provider.id)" :aria-busy="isProviderSaving(provider.id)" @click="saveProvider(provider)">{{ isProviderSaving(provider.id) ? t('email.admin.saving') : t('email.admin.save') }}</button><button type="button" class="btn btn-secondary btn-sm" :disabled="testingProviderId === provider.id || isProviderSaving(provider.id)" :aria-busy="testingProviderId === provider.id" :title="t('email.admin.testRequestNotice')" @click="testProvider(provider)">{{ testingProviderId === provider.id ? t('email.admin.testing') : t('email.admin.testConnection') }}</button></div></td></tr>
        </tbody></table></div>
      </section>

      <section class="card min-w-0 overflow-hidden">
        <div class="border-b border-gray-200 px-5 py-4 dark:border-dark-700"><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('email.admin.channels') }}</h2></div>
        <div class="max-w-full overflow-x-auto focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary-500" tabindex="0" :aria-label="t('email.admin.channels')"><table class="min-w-[2800px] text-left text-sm"><thead class="whitespace-nowrap bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-800"><tr><th class="px-5 py-3">{{ t('email.admin.channels') }}</th><th class="px-5 py-3">{{ t('email.admin.providers') }}</th><th class="px-5 py-3">{{ t('email.admin.salePrice') }}</th><th class="px-5 py-3">{{ t('email.admin.baseMarkup') }}</th><th class="px-5 py-3">{{ t('email.admin.fixedMarkup') }}</th><th class="px-5 py-3">{{ t('email.admin.minimumProfit') }}</th><th class="px-5 py-3">{{ t('email.admin.refundPolicy') }}</th><th class="px-5 py-3">{{ t('email.admin.capturePolicy') }}</th><th class="px-5 py-3">{{ t('email.admin.orderTtl') }}</th><th class="px-5 py-3">{{ t('email.admin.maxRequests') }}</th><th class="px-5 py-3">{{ t('email.admin.pollingBackoff') }}</th><th class="px-5 py-3">{{ t('email.admin.visible') }}</th><th class="px-5 py-3">{{ t('email.admin.health') }}</th><th class="px-5 py-3">{{ t('email.admin.enabled') }}</th><th class="px-5 py-3">{{ t('email.admin.action') }}</th></tr></thead><tbody>
          <tr v-for="channel in channels" :key="channel.id" class="border-t border-gray-100 dark:border-dark-700"><td class="px-5 py-3 font-medium text-gray-900 dark:text-white"><span>{{ channel.public_name }}</span> <span class="font-mono text-xs text-gray-500">{{ channel.code }}</span></td><td class="px-5 py-3">{{ channel.provider_name }}</td><td class="px-5 py-3"><input v-model.number="channel.sale_price" type="number" min="0" step="0.01" class="input w-28" :aria-label="`${t('email.admin.salePrice')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><input v-model.number="channel.base_markup" type="number" min="0" step="0.01" class="input w-24" :aria-label="`${t('email.admin.baseMarkup')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><input v-model.number="channel.fixed_markup" type="number" min="0" step="0.01" class="input w-24" :aria-label="`${t('email.admin.fixedMarkup')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><input v-model.number="channel.minimum_profit" type="number" min="0" step="0.01" class="input w-24" :aria-label="`${t('email.admin.minimumProfit')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><select v-model="channel.refund_policy" class="input min-w-56" :aria-label="`${t('email.admin.refundPolicy')} - ${channel.public_name}`"><option value="refund_if_no_message">{{ t('email.admin.refundPolicies.refund_if_no_message') }}</option><option value="no_refund_after_inbox_delivery">{{ t('email.admin.refundPolicies.no_refund_after_inbox_delivery') }}</option><option value="manual_review">{{ t('email.admin.refundPolicies.manual_review') }}</option></select></td><td class="px-5 py-3"><select v-model="channel.capture_policy" class="input min-w-56" :aria-label="`${t('email.admin.capturePolicy')} - ${channel.public_name}`"><option value="on_target_email_received">{{ t('email.admin.capturePolicies.on_target_email_received') }}</option><option value="on_verification_extracted">{{ t('email.admin.capturePolicies.on_verification_extracted') }}</option></select></td><td class="px-5 py-3"><input v-model.number="channel.order_ttl_seconds" type="number" min="60" max="86400" class="input w-28" :aria-label="`${t('email.admin.orderTtl')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><input v-model.number="channel.max_provider_requests_per_order" type="number" min="1" max="10000" class="input w-24" :aria-label="`${t('email.admin.maxRequests')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><input v-model="channelBackoffDrafts[channel.id]" class="input min-w-52 font-mono text-xs" :placeholder="t('email.admin.backoffPlaceholder')" :aria-label="`${t('email.admin.pollingBackoff')} - ${channel.public_name}`" /></td><td class="px-5 py-3"><Toggle v-model="channel.visible" :label="t('email.admin.channelVisible', { name: channel.public_name })" :disabled="isChannelSaving(channel.id)" @update:model-value="saveChannel(channel)" /></td><td class="px-5 py-3"><Toggle v-model="channel.healthy" :label="t('email.admin.channelHealthy', { name: channel.public_name })" :disabled="isChannelSaving(channel.id)" @update:model-value="saveChannel(channel)" /></td><td class="px-5 py-3"><div class="flex min-w-28 items-center gap-2"><Toggle v-model="channel.enabled" :label="t('email.admin.channelEnabled', { name: channel.public_name })" :disabled="isChannelSaving(channel.id)" @update:model-value="saveChannel(channel)" /><span class="text-xs font-medium text-gray-600 dark:text-gray-300">{{ channel.enabled ? t('common.enabled') : t('common.disabled') }}</span></div></td><td class="px-5 py-3"><button type="button" class="btn btn-secondary btn-sm" :disabled="isChannelSaving(channel.id)" :aria-busy="isChannelSaving(channel.id)" @click="saveChannel(channel)">{{ isChannelSaving(channel.id) ? t('email.admin.saving') : t('email.admin.save') }}</button></td></tr>
        </tbody></table></div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import adminEmail from '@/api/admin/email'
import type { EmailChannelAdmin, EmailProviderAdmin } from '@/api/admin/email'
import { useAppStore } from '@/stores'

const { t } = useI18n()
const appStore = useAppStore()
const providers = ref<EmailProviderAdmin[]>([])
const channels = ref<EmailChannelAdmin[]>([])
const stats = ref<Record<string, number | boolean | null>>({})
const enabled = ref(false)
const freeDailyLimit = ref(20)
const freeActiveLimit = ref(3)
const freeGenerationInterval = ref(5)
const loading = ref(false)
const savingEnabled = ref(false)
const testingProviderId = ref<number | null>(null)
const credentialRefs = ref<Record<number, string>>({})
const billingDrafts = ref<Record<number, Record<string, unknown>>>({})
const channelBackoffDrafts = ref<Record<number, string>>({})
const savingProviderIds = ref<Set<number>>(new Set())
const savingChannelIds = ref<Set<number>>(new Set())
const providerPortals: Record<string, string> = { emailnator: 'https://rapidapi.com/collection/gmailnator-api' }

const statItems = computed(() => ['orders', 'completed', 'refunded', 'today_requests', 'month_requests', 'failed_requests', 'rate_limited_requests'].filter(key => key in stats.value).map(key => ({ key, label: t(`email.admin.stats.${key}`), value: stats.value[key] })))
const providerPortalUrl = (provider: EmailProviderAdmin) => provider.portal_url || providerPortals[provider.code] || ''
const errorMessage = (error: unknown, fallback: string) => (error as { message?: string })?.message || fallback
const isProviderSaving = (id: number) => savingProviderIds.value.has(id)
const isChannelSaving = (id: number) => savingChannelIds.value.has(id)
function setSaving(target: typeof savingProviderIds, id: number, saving: boolean) {
  const next = new Set(target.value)
  if (saving) next.add(id); else next.delete(id)
  target.value = next
}

async function load() {
  loading.value = true
  try {
    const [p, c, s, settings] = await Promise.all([adminEmail.providers(), adminEmail.channels(), adminEmail.stats(), adminEmail.settings()])
    providers.value = p
    channels.value = c
    stats.value = s
    enabled.value = Boolean(settings.enabled)
    freeDailyLimit.value = settings.free_daily_limit
    freeActiveLimit.value = settings.free_active_limit
    freeGenerationInterval.value = settings.free_generation_interval_seconds
    credentialRefs.value = Object.fromEntries(p.map(item => [item.id, '']))
    billingDrafts.value = Object.fromEntries(p.map(item => [item.id, { ...(item.billing || {}) }]))
    channelBackoffDrafts.value = Object.fromEntries(c.map(item => [item.id, (item.polling_backoff || []).join(', ')]))
  } catch (error) {
    appStore.showError(errorMessage(error, t('email.admin.description')))
  } finally {
    loading.value = false
  }
}

async function saveSettings() {
  savingEnabled.value = true
  try {
    const result = await adminEmail.updateSettings({
      enabled: enabled.value,
      free_daily_limit: Math.max(0, Number(freeDailyLimit.value) || 0),
      free_active_limit: Math.max(0, Number(freeActiveLimit.value) || 0),
      free_generation_interval_seconds: Math.max(0, Number(freeGenerationInterval.value) || 0),
    })
    enabled.value = result.enabled
    freeDailyLimit.value = result.free_daily_limit
    freeActiveLimit.value = result.free_active_limit
    freeGenerationInterval.value = result.free_generation_interval_seconds
    appStore.showSuccess(t('email.admin.saved'))
  } catch (error) {
    appStore.showError(errorMessage(error, t('email.admin.enableTitle')))
  } finally {
    savingEnabled.value = false
  }
}
async function saveProvider(provider: EmailProviderAdmin) {
  const credential = (credentialRefs.value[provider.id] || '').trim()
  if (provider.requires_credential && !credential && !provider.credential_configured) {
    appStore.showError(t('email.admin.credentialRequired'))
    return
  }
  setSaving(savingProviderIds, provider.id, true)
  try {
    await adminEmail.updateProvider(provider.id, { enabled: provider.enabled, base_url: provider.base_url, credential_ref: credential || undefined, billing: billingDrafts.value[provider.id] })
    if (credential) provider.credential_configured = true
    credentialRefs.value[provider.id] = ''
    appStore.showSuccess(t('email.admin.saved'))
  } catch (error) { appStore.showError(errorMessage(error, t('email.admin.providers'))) }
  finally { setSaving(savingProviderIds, provider.id, false) }
}
async function testProvider(provider: EmailProviderAdmin) {
  testingProviderId.value = provider.id
  try { const result = await adminEmail.testProvider(provider.id); provider.health_status = result.health_status; appStore.showSuccess(`${t('email.admin.testSuccess')} (${result.latency_ms}ms)`) } catch (error) { appStore.showError(errorMessage(error, t('email.admin.testFailed'))) } finally { testingProviderId.value = null }
}
async function saveChannel(channel: EmailChannelAdmin) {
  const backoffTokens = (channelBackoffDrafts.value[channel.id] || '').split(',').map(value => value.trim()).filter(Boolean)
  const backoff = backoffTokens.map(value => Number(value))
  if (!backoff.length || backoff.some(value => !Number.isInteger(value) || value < 1 || value > 3600)) {
    appStore.showError(t('email.admin.invalidBackoff'))
    return
  }
  setSaving(savingChannelIds, channel.id, true)
  try {
    await adminEmail.updateChannel(channel.id, { enabled: channel.enabled, visible: channel.visible, healthy: channel.healthy, sale_price: channel.sale_price, base_markup: channel.base_markup, fixed_markup: channel.fixed_markup, minimum_profit: channel.minimum_profit, refund_policy: channel.refund_policy, capture_policy: channel.capture_policy, order_ttl_seconds: channel.order_ttl_seconds, max_provider_requests_per_order: channel.max_provider_requests_per_order, polling_backoff: backoff })
    appStore.showSuccess(t('email.admin.saved'))
  } catch (error) { appStore.showError(errorMessage(error, t('email.admin.channels'))) }
  finally { setSaving(savingChannelIds, channel.id, false) }
}
onMounted(load)
</script>
