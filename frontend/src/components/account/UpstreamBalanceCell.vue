<template>
  <div v-if="eligible" class="flex min-h-10 min-w-[8rem] items-center gap-1.5 sm:min-h-9">
    <HelpTooltip
      class="-ml-1 min-w-0"
      trigger="click"
      width-class="w-max max-w-[calc(100vw-2rem)]"
      data-testid="upstream-balance-details"
    >
      <template #trigger>
        <button
          type="button"
          class="inline-flex h-10 min-w-0 cursor-help items-center truncate rounded-sm border-b border-dotted border-gray-300 text-left text-sm font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40 sm:h-9 dark:border-dark-600"
          :class="hasCurrentValue ? 'font-mono text-gray-800 dark:text-gray-200' : statusClass"
          :aria-label="t('admin.accounts.upstreamBalance.details', { value: primaryValue })"
          data-testid="upstream-balance-value"
        >
          {{ primaryValue }}
        </button>
      </template>

      <div class="space-y-1">
        <template v-if="lastKnownValue && data">
          <p>{{ t('admin.accounts.upstreamBalance.mode', { value: modeLabel }) }}</p>
          <p v-if="data.mode === 'wallet'">
            {{ t('admin.accounts.upstreamBalance.walletBalance', { value: lastKnownValue }) }}
          </p>
          <p v-else-if="data.unlimited">
            {{ t('admin.accounts.upstreamBalance.remaining', { value: t('admin.accounts.upstreamBalance.unlimited') }) }}
          </p>
          <p v-else>
            {{ t('admin.accounts.upstreamBalance.remaining', { value: lastKnownValue }) }}
          </p>
          <p v-if="limitUsageLabel">
            {{ t('admin.accounts.upstreamBalance.usedLimit', { value: limitUsageLabel }) }}
          </p>
          <p>{{ t('admin.accounts.upstreamBalance.updatedAt', { value: formatDate(balanceSnapshot?.received_at) }) }}</p>
          <p>{{ t('admin.accounts.upstreamBalance.source', { value: sourceLabel }) }}</p>
          <p
            v-if="walletProbeDetail"
            :class="walletProbeStatus === 'failed' ? 'text-amber-300' : ''"
            data-testid="upstream-wallet-probe-detail"
          >
            {{ walletProbeDetail }}
          </p>
        </template>
        <p v-else>{{ statusLabel || '-' }}</p>
        <p v-if="failureDetail" class="text-red-300" data-testid="upstream-balance-error">
          {{ failureDetail }}
        </p>
        <p
          v-if="probeEnabled && globalProbeEnabled !== false && nextProbeAt"
          data-testid="upstream-balance-next-probe"
        >
          {{ t('admin.accounts.upstreamBalance.nextProbeAt', { value: formatDate(nextProbeAt) }) }}
        </p>
      </div>
    </HelpTooltip>

    <span
      v-if="hasCurrentValue && statusLabel"
      class="whitespace-nowrap text-xs font-medium"
      :class="statusClass"
      data-testid="upstream-balance-status"
    >
      {{ statusLabel }}
    </span>

    <button
      type="button"
      class="inline-flex h-10 w-10 flex-shrink-0 items-center justify-center rounded text-blue-600 transition-colors hover:bg-blue-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/40 disabled:cursor-not-allowed disabled:opacity-50 sm:h-9 sm:w-9 dark:text-blue-400 dark:hover:bg-blue-900/30"
      :disabled="probing"
      :aria-busy="probing"
      :aria-label="probing ? t('admin.accounts.upstreamBalance.refreshing') : t('admin.accounts.upstreamBalance.manualProbe')"
      :title="probing ? t('admin.accounts.upstreamBalance.refreshing') : t('admin.accounts.upstreamBalance.manualProbe')"
      data-testid="upstream-balance-probe"
      @click="$emit('probe')"
    >
      <Icon name="refresh" size="xs" :class="{ 'animate-spin': probing }" aria-hidden="true" />
    </button>
  </div>
  <span v-else class="text-sm text-gray-400 dark:text-dark-500">-</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatCurrency } from '@/utils/format'
import { isUpstreamBillingProbeAccount } from '@/utils/upstreamBilling'
import type { Account, UpstreamBalanceData, UpstreamBalanceProbeSnapshot } from '@/types'

const props = withDefaults(defineProps<{
  account: Account
  now: number
  probing?: boolean
  globalProbeEnabled?: boolean
}>(), {
  probing: false,
  globalProbeEnabled: true
})

defineEmits<{
  (event: 'probe'): void
}>()

const { t, te } = useI18n()
const CLOCK_SKEW_TOLERANCE_MS = 5 * 60 * 1000

const eligible = computed(() => isUpstreamBillingProbeAccount(props.account))
const billingSnapshot = computed(() => props.account.extra?.upstream_billing_probe)
const balanceSnapshot = computed<UpstreamBalanceProbeSnapshot | undefined>(() => billingSnapshot.value?.balance)
const data = computed<UpstreamBalanceData | undefined>(() => balanceSnapshot.value?.data)
const walletProbeStatus = computed(() => data.value?.wallet_probe_status)
const probeEnabled = computed(() => props.account.extra?.upstream_billing_probe_enabled !== false)
const autoUnschedulable = computed(() => props.account.extra?.upstream_billing_auto_unschedulable === true)
const nextProbeAt = computed(() => {
  const value = balanceSnapshot.value?.next_probe_at || billingSnapshot.value?.next_probe_at
  return typeof value === 'string' && Number.isFinite(Date.parse(value)) ? value : ''
})
const receivedAt = computed(() => {
  const value = balanceSnapshot.value?.received_at
  return typeof value === 'string' ? Date.parse(value) : Number.NaN
})
const freshUntil = computed(() => {
  const value = balanceSnapshot.value?.fresh_until
  if (typeof value === 'string') return Date.parse(value)
  const next = balanceSnapshot.value?.next_probe_at
  if (balanceSnapshot.value?.status !== 'ok' || typeof next !== 'string') return Number.NaN
  const nextTimestamp = Date.parse(next)
  return Number.isFinite(nextTimestamp) && nextTimestamp > receivedAt.value
    ? receivedAt.value + 2 * (nextTimestamp - receivedAt.value)
    : Number.NaN
})
const validTimestamps = computed(() => (
  Number.isFinite(receivedAt.value) &&
  receivedAt.value <= props.now + CLOCK_SKEW_TOLERANCE_MS &&
  Number.isFinite(freshUntil.value) &&
  freshUntil.value > receivedAt.value
))
const stale = computed(() => {
  if (!balanceSnapshot.value || !data.value) return false
  if (!validTimestamps.value) return true
  return props.now > freshUntil.value
})

const validNumber = (value: unknown): value is number => typeof value === 'number' && Number.isFinite(value)
const effectiveAmount = computed(() => {
  const value = data.value
  if (!value || value.unlimited) return null
  if (value.mode === 'wallet' && validNumber(value.balance)) return value.balance
  return validNumber(value.remaining) ? value.remaining : null
})
const formatAmount = (value: number, unit: string) => {
  if (/^[A-Z]{3}$/.test(unit)) {
    try {
      return formatCurrency(value, unit)
    } catch {
      // Fall through for non-ISO units reported by a compatible upstream.
    }
  }
  return `${unit} ${value.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 })}`
}
const formattedAmount = computed(() => {
  const value = data.value
  if (!value || typeof value.unit !== 'string' || value.unit === '') return ''
  if (value.unlimited) {
    return balanceSnapshot.value?.source === 'new_api_token'
      ? t('admin.accounts.upstreamBalance.tokenUnlimited')
      : t('admin.accounts.upstreamBalance.unlimited')
  }
  return effectiveAmount.value == null ? '' : formatAmount(effectiveAmount.value, value.unit)
})
const lastKnownValue = computed(() => formattedAmount.value || '')
const hasCurrentValue = computed(() => (
  Boolean(lastKnownValue.value) &&
  validTimestamps.value &&
  !stale.value &&
  ['ok', 'failed'].includes(balanceSnapshot.value?.status ?? '')
))
const exhausted = computed(() => (
  hasCurrentValue.value &&
  data.value?.unlimited !== true &&
  effectiveAmount.value != null &&
  effectiveAmount.value <= 0
))

const modeLabel = computed(() => {
  const mode = data.value?.mode
  if (mode === 'wallet') return t('admin.accounts.upstreamBalance.modes.wallet')
  if (mode === 'key_quota') return t('admin.accounts.upstreamBalance.modes.keyQuota')
  if (mode === 'subscription') return t('admin.accounts.upstreamBalance.modes.subscription')
  return '-'
})
const sourceLabel = computed(() => {
  if (balanceSnapshot.value?.source === 'billing') return t('admin.accounts.upstreamBalance.sources.billing')
  if (balanceSnapshot.value?.source === 'usage') return t('admin.accounts.upstreamBalance.sources.usage')
  if (balanceSnapshot.value?.source === 'new_api_token') return t('admin.accounts.upstreamBalance.sources.newAPIToken')
  if (balanceSnapshot.value?.source === 'new_api_wallet') return t('admin.accounts.upstreamBalance.sources.newAPIWallet')
  return '-'
})
const limitUsageLabel = computed(() => {
  const value = data.value
  if (!value || !validNumber(value.limit) || !validNumber(value.used) || !value.unit) return ''
  return `${formatAmount(value.used, value.unit)} / ${formatAmount(value.limit, value.unit)}`
})
const statusLabel = computed(() => {
  const snapshot = balanceSnapshot.value
  if (!snapshot) {
    if (autoUnschedulable.value) return t('admin.accounts.upstreamBalance.autoPaused')
    if (billingSnapshot.value?.status === 'unsupported') return t('admin.accounts.upstreamBalance.unsupported')
    if (billingSnapshot.value?.status === 'failed') return t('admin.accounts.upstreamBalance.failed')
    return t('admin.accounts.upstreamBalance.notProbed')
  }
  if (autoUnschedulable.value) {
    return exhausted.value
      ? t('admin.accounts.upstreamBalance.exhaustedAutoPaused')
      : t('admin.accounts.upstreamBalance.autoPaused')
  }
  if (snapshot.status === 'unsupported') return t('admin.accounts.upstreamBalance.unsupported')
  if (snapshot.status === 'failed' || balanceSnapshot.value?.status === 'failed') {
    return t('admin.accounts.upstreamBalance.failed')
  }
  if (balanceSnapshot.value?.status === 'unsupported') return t('admin.accounts.upstreamBalance.unsupported')
  if (stale.value) return t('admin.accounts.upstreamBalance.stale')
  if (data.value?.is_valid === false) return t('admin.accounts.upstreamBalance.invalid')
  if (exhausted.value) return t('admin.accounts.upstreamBalance.exhausted')
  if (walletProbeStatus.value === 'failed') return t('admin.accounts.upstreamBalance.walletProbeFailed')
  if (walletProbeStatus.value === 'not_configured') return t('admin.accounts.upstreamBalance.walletNotConfigured')
  return ''
})
const statusClass = computed(() => {
  if (
    balanceSnapshot.value?.status === 'failed' ||
    billingSnapshot.value?.status === 'failed' ||
    autoUnschedulable.value ||
    data.value?.is_valid === false ||
    exhausted.value
  ) {
    return 'text-red-600 dark:text-red-400'
  }
  if (stale.value) return 'text-amber-600 dark:text-amber-400'
  if (walletProbeStatus.value === 'failed') return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-500 dark:text-gray-400'
})
const primaryValue = computed(() => hasCurrentValue.value ? lastKnownValue.value : statusLabel.value || '-')
const failureDetail = computed(() => {
  const snapshot = balanceSnapshot.value
  const topLevel = billingSnapshot.value
  const failure = snapshot?.status === 'failed' ? snapshot : topLevel?.status === 'failed' ? topLevel : null
  if (!failure) return ''
  const key = `admin.accounts.upstreamBalance.errors.${failure.last_error || ''}`
  const message = te(key) ? t(key) : t('admin.accounts.upstreamBalance.failed')
  return failure.http_status ? `${message} (HTTP ${failure.http_status})` : message
})
const walletProbeDetail = computed(() => {
  if (walletProbeStatus.value === 'not_configured') {
    return t('admin.accounts.upstreamBalance.walletNotConfiguredDetail')
  }
  if (walletProbeStatus.value !== 'failed') return ''
  const reason = data.value?.wallet_probe_error || ''
  const key = `admin.accounts.upstreamBalance.errors.${reason}`
  const message = te(key) ? t(key) : t('admin.accounts.upstreamBalance.failed')
  const status = data.value?.wallet_probe_http_status
  const detail = typeof status === 'number' && Number.isFinite(status)
    ? `${message} (HTTP ${status})`
    : message
  return t('admin.accounts.upstreamBalance.walletProbeError', { value: detail })
})
const formatDate = (value?: string) => {
  if (!value || !Number.isFinite(Date.parse(value))) return '-'
  return new Date(value).toLocaleString(undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit'
  })
}
</script>
