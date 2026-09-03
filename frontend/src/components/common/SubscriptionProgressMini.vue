<template>
  <div v-if="hasActiveSubscriptions" class="relative" ref="containerRef">
    <!-- Mini Progress Display -->
    <button
      @click="toggleTooltip"
      class="subscription-progress-trigger flex cursor-pointer items-center gap-2 rounded-lg px-3 py-1.5"
      :title="t('subscriptionProgress.viewDetails')"
    >
      <Icon name="creditCard" size="sm" class="subscription-progress-icon" />
      <div class="flex items-center gap-1.5">
        <!-- Combined progress indicator -->
        <div class="flex items-center gap-0.5">
          <div
            v-for="(sub, index) in displaySubscriptions.slice(0, 3)"
            :key="index"
            class="h-2 w-2 rounded-full"
            :class="getProgressDotClass(sub)"
          ></div>
        </div>
        <span class="subscription-progress-count text-xs font-medium">
          {{ activeSubscriptions.length }}
        </span>
      </div>
    </button>

    <!-- Hover/Click Tooltip -->
    <transition name="dropdown">
      <div
        v-if="tooltipOpen"
        class="subscription-progress-popover absolute right-0 z-50 mt-2 w-[340px] overflow-hidden rounded-xl"
      >
        <div class="subscription-progress-header border-b p-3">
          <h3 class="subscription-progress-title text-sm font-semibold">
            {{ t('subscriptionProgress.title') }}
          </h3>
          <p class="subscription-progress-meta mt-0.5 text-xs">
            {{ t('subscriptionProgress.activeCount', { count: activeSubscriptions.length }) }}
          </p>
        </div>

        <div class="max-h-64 overflow-y-auto">
          <div
            v-for="subscription in displaySubscriptions"
            :key="subscription.id"
            class="border-b border-gray-50 p-3 last:border-b-0 dark:border-dark-700/50"
          >
            <div class="mb-2 flex items-center justify-between">
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                {{ subscription.group?.name || `Group #${subscription.group_id}` }}
              </span>
              <span
                v-if="subscription.expires_at"
                class="text-xs"
                :class="getDaysRemainingClass(subscription.expires_at)"
              >
                {{ formatDaysRemaining(subscription.expires_at) }}
              </span>
            </div>

            <!-- Progress bars or Unlimited badge -->
            <div class="space-y-1.5">
              <!-- Unlimited subscription badge -->
              <div
                v-if="isUnlimited(subscription)"
                class="subscription-progress-unlimited flex items-center gap-2 rounded-lg px-2.5 py-1.5"
              >
                <span class="text-lg">∞</span>
                <span class="text-xs font-medium">
                  {{ t('subscriptionProgress.unlimited') }}
                </span>
              </div>

              <!-- Progress bars for limited subscriptions -->
              <template v-else>
                <div v-if="subscription.group?.daily_limit_usd" class="flex items-center gap-2">
                  <span class="w-8 flex-shrink-0 text-[10px] text-gray-500">{{
                    t('subscriptionProgress.daily')
                  }}</span>
                  <div class="subscription-progress-track h-1.5 min-w-0 flex-1 rounded-full">
                    <div
                      class="subscription-progress-bar h-1.5 rounded-full"
                      :class="
                        getProgressBarClass(
                          subscription.daily_usage_usd,
                          subscription.group?.daily_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.daily_usage_usd,
                          subscription.group?.daily_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                  <span class="w-24 flex-shrink-0 text-right text-[10px] text-gray-500">
                    {{
                      formatUsage(subscription.daily_usage_usd, subscription.group?.daily_limit_usd)
                    }}
                  </span>
                </div>

                <div v-if="subscription.group?.weekly_limit_usd" class="flex items-center gap-2">
                  <span class="w-8 flex-shrink-0 text-[10px] text-gray-500">{{
                    t('subscriptionProgress.weekly')
                  }}</span>
                  <div class="subscription-progress-track h-1.5 min-w-0 flex-1 rounded-full">
                    <div
                      class="subscription-progress-bar h-1.5 rounded-full"
                      :class="
                        getProgressBarClass(
                          subscription.weekly_usage_usd,
                          subscription.group?.weekly_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.weekly_usage_usd,
                          subscription.group?.weekly_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                  <span class="w-24 flex-shrink-0 text-right text-[10px] text-gray-500">
                    {{
                      formatUsage(subscription.weekly_usage_usd, subscription.group?.weekly_limit_usd)
                    }}
                  </span>
                </div>

                <div v-if="subscription.group?.monthly_limit_usd" class="flex items-center gap-2">
                  <span class="w-8 flex-shrink-0 text-[10px] text-gray-500">{{
                    t('subscriptionProgress.monthly')
                  }}</span>
                  <div class="subscription-progress-track h-1.5 min-w-0 flex-1 rounded-full">
                    <div
                      class="subscription-progress-bar h-1.5 rounded-full"
                      :class="
                        getProgressBarClass(
                          subscription.monthly_usage_usd,
                          subscription.group?.monthly_limit_usd
                        )
                      "
                      :style="{
                        width: getProgressWidth(
                          subscription.monthly_usage_usd,
                          subscription.group?.monthly_limit_usd
                        )
                      }"
                    ></div>
                  </div>
                  <span class="w-24 flex-shrink-0 text-right text-[10px] text-gray-500">
                    {{
                      formatUsage(
                        subscription.monthly_usage_usd,
                        subscription.group?.monthly_limit_usd
                      )
                    }}
                  </span>
                </div>
              </template>
            </div>
          </div>
        </div>

        <div class="subscription-progress-footer border-t p-2">
          <router-link
            to="/subscriptions"
            @click="closeTooltip"
            class="subscription-progress-link block w-full py-1 text-center text-xs hover:underline"
          >
            {{ t('subscriptionProgress.viewAll') }}
          </router-link>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useSubscriptionStore } from '@/stores'
import type { UserSubscription } from '@/types'

const { t } = useI18n()

const subscriptionStore = useSubscriptionStore()

const containerRef = ref<HTMLElement | null>(null)
const tooltipOpen = ref(false)

// Use store data instead of local state
const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)
const hasActiveSubscriptions = computed(() => subscriptionStore.hasActiveSubscriptions)

const displaySubscriptions = computed(() => {
  // Sort by most usage (highest percentage first)
  return [...activeSubscriptions.value].sort((a, b) => {
    const aMax = getMaxUsagePercentage(a)
    const bMax = getMaxUsagePercentage(b)
    return bMax - aMax
  })
})

function getMaxUsagePercentage(sub: UserSubscription): number {
  const percentages: number[] = []
  if (sub.group?.daily_limit_usd) {
    percentages.push(((sub.daily_usage_usd || 0) / sub.group.daily_limit_usd) * 100)
  }
  if (sub.group?.weekly_limit_usd) {
    percentages.push(((sub.weekly_usage_usd || 0) / sub.group.weekly_limit_usd) * 100)
  }
  if (sub.group?.monthly_limit_usd) {
    percentages.push(((sub.monthly_usage_usd || 0) / sub.group.monthly_limit_usd) * 100)
  }
  return percentages.length > 0 ? Math.max(...percentages) : 0
}

function isUnlimited(sub: UserSubscription): boolean {
  return (
    !sub.group?.daily_limit_usd &&
    !sub.group?.weekly_limit_usd &&
    !sub.group?.monthly_limit_usd
  )
}

function getProgressDotClass(sub: UserSubscription): string {
  // Unlimited subscriptions get a special color
  if (isUnlimited(sub)) {
    return 'subscription-progress-dot-success'
  }
  const maxPercentage = getMaxUsagePercentage(sub)
  if (maxPercentage >= 90) return 'subscription-progress-dot-danger'
  if (maxPercentage >= 70) return 'subscription-progress-dot-warning'
  return 'subscription-progress-dot-success'
}

function getProgressBarClass(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return 'subscription-progress-bar-neutral'
  const percentage = ((used || 0) / limit) * 100
  if (percentage >= 90) return 'subscription-progress-bar-danger'
  if (percentage >= 70) return 'subscription-progress-bar-warning'
  return 'subscription-progress-bar-success'
}

function getProgressWidth(used: number | undefined, limit: number | null | undefined): string {
  if (!limit || limit === 0) return '0%'
  const percentage = Math.min(((used || 0) / limit) * 100, 100)
  return `${percentage}%`
}

function formatUsage(used: number | undefined, limit: number | null | undefined): string {
  const usedValue = (used || 0).toFixed(2)
  const limitValue = limit?.toFixed(2) || '∞'
  return `$${usedValue}/$${limitValue}`
}

function formatDaysRemaining(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  if (diff < 0) return t('subscriptionProgress.expired')
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days === 0) return t('subscriptionProgress.expiresToday')
  if (days === 1) return t('subscriptionProgress.expiresTomorrow')
  return t('subscriptionProgress.daysRemaining', { days })
}

function getDaysRemainingClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  if (days <= 3) return 'subscription-progress-expiry-danger'
  if (days <= 7) return 'subscription-progress-expiry-warning'
  return 'subscription-progress-expiry-neutral'
}

function toggleTooltip() {
  tooltipOpen.value = !tooltipOpen.value
}

function closeTooltip() {
  tooltipOpen.value = false
}

function handleClickOutside(event: MouseEvent) {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    closeTooltip()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  // Trigger initial fetch if not already loaded
  // The actual data loading is handled by App.vue globally
  subscriptionStore.fetchActiveSubscriptions().catch((error) => {
    console.error('Failed to load subscriptions in SubscriptionProgressMini:', error)
  })
})

onBeforeUnmount(() => {
  document.removeEventListener('click', handleClickOutside)
})
</script>

<style scoped>
.subscription-progress-trigger {
  color: var(--color-primary);
  background-color: var(--color-primary-soft);
  border: 1px solid transparent;
  transition: color 160ms ease, background-color 160ms ease, border-color 160ms ease;
}

.subscription-progress-trigger:hover {
  background-color: var(--color-surface-soft);
  border-color: var(--color-primary-border);
}

.subscription-progress-icon,
.subscription-progress-count,
.subscription-progress-link {
  color: var(--color-primary);
}

.subscription-progress-popover {
  background-color: var(--color-surface-overlay);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-overlay);
}

.subscription-progress-header,
.subscription-progress-footer {
  border-color: var(--color-border-subtle);
}

.subscription-progress-title {
  color: var(--color-text-primary);
}

.subscription-progress-meta,
.subscription-progress-expiry-neutral {
  color: var(--color-text-muted);
}

.subscription-progress-unlimited {
  color: var(--color-success);
  background-color: var(--color-accent-soft);
}

.subscription-progress-track {
  background-color: var(--color-bg-subtle);
}

.subscription-progress-bar {
  transition: width 300ms ease, background-color 180ms ease;
}

.subscription-progress-bar-neutral,
.subscription-progress-dot-neutral {
  background-color: var(--color-text-disabled);
}

.subscription-progress-bar-success,
.subscription-progress-dot-success {
  background-color: var(--color-success);
}

.subscription-progress-bar-warning,
.subscription-progress-dot-warning {
  background-color: var(--color-warning);
}

.subscription-progress-bar-danger,
.subscription-progress-dot-danger {
  background-color: var(--color-danger);
}

.subscription-progress-expiry-warning {
  color: var(--color-warning);
}

.subscription-progress-expiry-danger {
  color: var(--color-danger);
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 180ms var(--ease-standard), transform 180ms var(--ease-standard);
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
</style>
