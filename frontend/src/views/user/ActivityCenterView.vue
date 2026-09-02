<template>
  <AppLayout :enable-onboarding="false">
    <main class="activity-page mx-auto max-w-5xl space-y-5">
      <div v-if="loading" class="space-y-4" aria-busy="true">
        <div class="h-20 animate-pulse rounded-xl bg-gray-100 dark:bg-dark-800" />
        <div class="h-[460px] animate-pulse rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800" />
      </div>

      <template v-else>
        <header class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
          <div class="min-w-0">
            <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ t('activityCenter.title') }}</p>
            <h1 class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ selectedActivity?.title || t('activityCenter.title') }}
            </h1>
            <p class="mt-1.5 max-w-2xl text-sm leading-6 text-gray-500 dark:text-gray-400">
              <template v-if="selectedActivity?.type === 'recharge_lottery' && selectedActivity.lottery">
                {{ t('activityCenter.lottery.rechargeRuleImmediate', { threshold: money(selectedActivity.lottery.recharge_threshold), draws: selectedActivity.lottery.draws_per_threshold }) }}
              </template>
              <template v-else>{{ selectedActivity?.description || t('activityCenter.description') }}</template>
            </p>
          </div>
          <RouterLink
            v-if="selectedActivity?.type === 'recharge_lottery'"
            to="/recharge"
            class="btn btn-primary inline-flex h-11 shrink-0 items-center justify-center gap-2 self-start px-5"
          >
            <Icon name="creditCard" size="sm" />
            {{ t('activityCenter.lottery.recharge') }}
          </RouterLink>
        </header>

        <nav v-if="activities.length > 1" class="activity-switcher flex flex-wrap gap-2" :aria-label="t('activityCenter.activityList')">
          <button
            v-for="activity in activities"
            :key="activity.id"
            type="button"
            class="activity-switch min-h-10 rounded-lg border px-3.5 py-2 text-sm font-medium"
            :class="activitySwitchClass(activity)"
            :aria-disabled="activity.availability !== 'active'"
            :aria-pressed="selectedSlug === activity.slug"
            @click="selectActivity(activity)"
          >
            <Icon :name="activity.type === 'recharge_lottery' ? 'trophy' : 'gift'" size="sm" />
            <span>{{ activity.title }}</span>
            <span v-if="activity.availability !== 'active'" class="text-xs font-normal">
              {{ t(`activityCenter.status.${activity.availability}`) }}
            </span>
          </button>
        </nav>

        <section v-if="!activities.length" class="rounded-xl border border-gray-200 bg-white p-10 text-center dark:border-dark-700 dark:bg-dark-800">
          <Icon name="gift" size="xl" class="mx-auto text-gray-400" />
          <p class="mt-3 text-sm text-gray-500 dark:text-gray-400">{{ t('activityCenter.empty') }}</p>
        </section>

        <template v-if="selectedActivity?.type === 'recharge_lottery' && selectedActivity.lottery">
          <div class="lottery-layout grid gap-4 lg:grid-cols-[minmax(0,1fr)_288px]">
            <section class="wheel-panel min-w-0 rounded-xl border border-gray-200 bg-white p-5 dark:border-dark-700 dark:bg-dark-800 sm:p-6">
              <div class="flex items-center justify-between gap-3">
                <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('activityCenter.lottery.wheel') }}</h2>
                <span class="text-sm font-semibold tabular-nums text-gray-700 dark:text-gray-200">
                  {{ t('activityCenter.lottery.drawOrdinal', { count: (selectedActivity.participation?.used_draws || 0) + 1 }) }}
                </span>
              </div>

              <div class="mx-auto mt-5 w-full max-w-[360px]">
                <div class="wheel-frame relative aspect-square w-full" :class="{ 'wheel-frame--spinning': spinning }">
                  <div class="wheel-pointer" aria-hidden="true" />
                  <div
                    ref="wheelDisc"
                    class="wheel-disc absolute rounded-full"
                    :class="{ 'wheel-disc--spinning': spinning }"
                    :style="{ background: wheelBackground }"
                    :aria-label="prizeSummary"
                    role="img"
                  >
                    <span
                      v-for="(prize, index) in selectedActivity.lottery.prizes"
                      :key="prize.id"
                      class="wheel-prize-label"
                      :class="{ 'wheel-prize-label--dense': selectedActivity.lottery.prizes.length > 8 }"
                      :style="wheelPrizeStyle(index, selectedActivity.lottery.prizes.length)"
                      aria-hidden="true"
                    >
                      <span class="wheel-prize-anchor">
                        <span class="wheel-prize-text">{{ prize.name }}</span>
                      </span>
                    </span>
                    <span class="wheel-center absolute left-1/2 top-1/2 flex -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full">
                      <Icon name="gift" size="lg" />
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  class="btn btn-primary mx-auto mt-4 flex h-11 min-w-40 items-center justify-center gap-2"
                  :disabled="spinning || !(selectedActivity.participation?.available_draws || 0)"
                  :aria-busy="spinning"
                  @click="spinLottery"
                >
                  <span v-if="spinning" class="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white" />
                  <Icon v-else name="sparkles" size="sm" />
                  {{ spinning ? t('activityCenter.lottery.spinning') : spinButtonLabel }}
                </button>
              </div>
            </section>

            <aside class="lottery-summary overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800" :aria-label="t('activityCenter.lottery.summary')">
              <div class="px-5 py-5">
                <div class="flex items-start justify-between gap-4">
                  <span class="text-sm text-gray-500 dark:text-gray-400">{{ t('activityCenter.lottery.available') }}</span>
                  <strong class="text-3xl font-semibold tabular-nums text-primary-600 dark:text-primary-300">
                    {{ selectedActivity.participation?.available_draws || 0 }}
                  </strong>
                </div>
              </div>
              <dl class="divide-y divide-gray-100 border-y border-gray-100 dark:divide-dark-700 dark:border-dark-700">
                <div class="summary-row">
                  <dt>{{ t('activityCenter.lottery.totalDraws') }}</dt>
                  <dd>{{ selectedActivity.participation?.used_draws || 0 }} {{ t('activityCenter.lottery.times') }}</dd>
                </div>
                <div v-if="selectedActivity.lottery.per_user_draw_limit > 0" class="summary-row">
                  <dt>{{ t('activityCenter.lottery.totalDrawLimit') }}</dt>
                  <dd>{{ selectedActivity.lottery.per_user_draw_limit }} {{ t('activityCenter.lottery.times') }}</dd>
                </div>
                <div v-if="selectedActivity.lottery.daily_draw_limit > 0" class="summary-row">
                  <dt>{{ t('activityCenter.lottery.dailyDrawLimit') }}</dt>
                  <dd>{{ selectedActivity.lottery.daily_draw_limit }} {{ t('activityCenter.lottery.times') }}</dd>
                </div>
                <div class="summary-row">
                  <dt>{{ t('activityCenter.lottery.totalRewards') }}</dt>
                  <dd>¥{{ money(selectedActivity.participation?.reward_total) }}</dd>
                </div>
              </dl>
              <div class="px-5 py-5">
                <div class="flex items-center justify-between gap-3 text-sm">
                  <span class="text-gray-500 dark:text-gray-400">{{ t('activityCenter.lottery.nextChance') }}</span>
                  <strong class="tabular-nums text-gray-900 dark:text-white">
                    {{ t('activityCenter.lottery.amountRemaining', { amount: money(selectedActivity.participation?.next_draw_recharge_amount) }) }}
                  </strong>
                </div>
                <div
                  class="progress-track mt-3 h-1.5 overflow-hidden rounded-full"
                  role="progressbar"
                  :aria-label="t('activityCenter.lottery.rechargeProgress')"
                  :aria-valuenow="progressPercent"
                  aria-valuemin="0"
                  aria-valuemax="100"
                >
                  <span class="progress-value block h-full rounded-full" :style="{ width: `${progressPercent}%` }" />
                </div>
                <p class="mt-2 text-xs tabular-nums text-gray-500 dark:text-gray-400">
                  {{ t('activityCenter.lottery.progressDetail', {
                    current: money(selectedActivity.participation?.recharge_progress_amount),
                    threshold: money(selectedActivity.lottery.recharge_threshold),
                  }) }}
                </p>
              </div>
              <div class="border-t border-gray-100 px-5 py-5 dark:border-dark-700">
                <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('activityCenter.lottery.availablePrizes') }}</h3>
                <ul class="mt-3 flex flex-wrap gap-2" :aria-label="t('activityCenter.lottery.availablePrizes')">
                  <li v-for="prize in selectedActivity.lottery.prizes" :key="prize.id" class="prize-chip">
                    {{ prize.name }}
                  </li>
                </ul>
              </div>
            </aside>
          </div>
        </template>

        <section v-if="selectedActivity?.type === 'limited_time_benefit' && selectedActivity.benefit" class="benefit-panel overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="benefit-layout">
            <div class="benefit-content min-w-0 p-5 sm:p-7">
              <p class="text-xs font-semibold text-primary-600 dark:text-primary-300">{{ t('activityCenter.benefit.eyebrow') }}</p>
              <h2 class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ selectedActivity.title }}</h2>
              <p class="mt-1 max-w-xl text-sm leading-6 text-gray-500 dark:text-gray-400">{{ selectedActivity.description }}</p>
              <p class="mt-5 text-base font-semibold text-gray-900 dark:text-white" data-testid="benefit-reward-message">
                {{ isRandomBenefit ? t('activityCenter.benefit.randomReveal') : t('activityCenter.benefit.fixedReveal') }}
              </p>
              <div class="mt-3 flex flex-wrap gap-x-5 gap-y-2 text-sm text-gray-600 dark:text-gray-300">
                <span>{{ t('activityCenter.benefit.remaining', { count: selectedActivity.participation?.remaining_stock || 0 }) }}</span>
                <span data-testid="benefit-daily-claim-rule">{{ t('activityCenter.benefit.dailyRule') }}</span>
                <span :class="(selectedActivity.participation?.benefit_claims_today || 0) > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-emerald-600 dark:text-emerald-400'">
                  {{ (selectedActivity.participation?.benefit_claims_today || 0) > 0 ? t('activityCenter.benefit.claimedToday') : t('activityCenter.benefit.availableToday') }}
                </span>
              </div>
              <div v-if="selectedBenefitClaim" class="benefit-result mt-5" role="status" data-testid="benefit-claim-result">
                <span class="text-sm text-gray-600 dark:text-gray-300">{{ t('activityCenter.benefit.received') }}</span>
                <strong class="tabular-nums text-lg text-emerald-600 dark:text-emerald-400">+¥{{ money(selectedBenefitClaim.reward_amount) }}</strong>
              </div>
              <button type="button" class="btn btn-primary mt-6 flex h-11 w-full items-center justify-center gap-2 sm:w-auto sm:min-w-40" :disabled="claiming || !canClaimBenefit" :aria-busy="claiming" @click="claimBenefit">
                <span v-if="claiming" class="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white" />
                <Icon v-else name="gift" size="sm" />
                {{ benefitClaimButtonLabel }}
              </button>
            </div>
            <figure class="benefit-visual" aria-hidden="true">
              <img :src="benefitGiftImage" alt="" class="benefit-image" data-testid="benefit-marketing-image" />
            </figure>
          </div>
        </section>

        <section v-if="selectedActivity" class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="flex min-h-14 items-center justify-between gap-3 border-b border-gray-100 px-5 dark:border-dark-700">
            <div>
              <p class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('activityCenter.rewards.eyebrow') }}</p>
              <h2 class="mt-0.5 text-base font-semibold text-gray-900 dark:text-white">{{ t('activityCenter.rewards.title') }}</h2>
            </div>
            <button type="button" class="refresh-button" :disabled="loading" :title="t('activityCenter.refresh')" :aria-label="t('activityCenter.refresh')" @click="loadAll">
              <Icon name="refresh" size="sm" />
            </button>
          </div>
          <div v-if="selectedRewards.length" class="divide-y divide-gray-100 dark:divide-dark-700">
            <div v-for="reward in selectedRewards" :key="reward.id" class="flex min-h-14 items-center justify-between gap-4 px-5 py-3">
              <div class="min-w-0">
                <p class="truncate text-sm font-medium text-gray-800 dark:text-gray-100">{{ reward.activity_title }}</p>
                <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{{ t(`activityCenter.rewards.${reward.source_type}`) }} · {{ formatDate(reward.created_at) }}</p>
              </div>
              <span class="shrink-0 font-mono text-sm font-semibold text-emerald-600 dark:text-emerald-400">+¥{{ money(reward.amount) }}</span>
            </div>
          </div>
          <p v-else class="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('activityCenter.rewards.empty') }}</p>
        </section>
      </template>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, type CSSProperties } from 'vue'
import { gsap } from 'gsap'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import activityAPI from '@/api/activity'
import benefitGiftImage from '@/assets/activity/limited-benefit-gift.jpg'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { Activity, ActivityReward, BenefitClaimResult } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const loading = ref(true)
const spinning = ref(false)
const claiming = ref(false)
const lastBenefitClaim = ref<(BenefitClaimResult & { activity_id: number }) | null>(null)
const activities = ref<Activity[]>([])
const rewards = ref<ActivityReward[]>([])
const selectedSlug = ref('')
const wheelDisc = ref<HTMLElement | null>(null)
const wheelRotation = ref(0)
const reduceWheelMotion = ref(false)
const pendingRequestIDs = new Map<string, string>()
const WHEEL_TONE_COUNT = 12
const WHEEL_BASE_TONE_COUNT = 6
const WHEEL_FULL_TURNS = 5
const WHEEL_ACCELERATION_DURATION = 1
const WHEEL_CRUISE_DURATION = 0.45
const WHEEL_DECELERATION_DURATION = 3.35
let wheelTimeline: gsap.core.Timeline | null = null
let wheelAnimationResolve: (() => void) | null = null
let wheelMotionMedia: ReturnType<typeof gsap.matchMedia> | null = null

const selectedActivity = computed(() => activities.value.find(item => item.slug === selectedSlug.value) || null)
const selectedRewards = computed(() => rewards.value.filter(reward => reward.activity_id === selectedActivity.value?.id))
const isRandomBenefit = computed(() => Number(selectedActivity.value?.benefit?.reward_amount) === 0)
const selectedBenefitClaim = computed(() => lastBenefitClaim.value?.activity_id === selectedActivity.value?.id
  ? lastBenefitClaim.value
  : null)
const wheelBackground = computed(() => {
  const count = Math.max(1, selectedActivity.value?.lottery?.prizes.length || 1)
  const colors = wheelTones(selectedActivity.value, count).map(tone => `var(--wheel-tone-${tone})`)
  const slices = colors.flatMap((color, index) => {
    const start = (index / count) * 360
    const end = ((index + 1) / count) * 360
    const dividerStart = Math.max(start, end - 0.8)
    return [
      `${color} ${start}deg ${dividerStart}deg`,
      `var(--wheel-divider) ${dividerStart}deg ${end}deg`,
    ]
  })
  return `conic-gradient(${slices.join(', ')})`
})
const progressPercent = computed(() => {
  const activity = selectedActivity.value
  const threshold = Number(activity?.lottery?.recharge_threshold || 0)
  const progress = Number(activity?.participation?.recharge_progress_amount || 0)
  if (!Number.isFinite(threshold) || threshold <= 0 || !Number.isFinite(progress)) return 0
  return Math.min(100, Math.max(0, Math.round((progress / threshold) * 100)))
})
const prizeSummary = computed(() => t('activityCenter.lottery.wheelAria', {
  prizes: selectedActivity.value?.lottery?.prizes.map(prize => prize.name).join('、') || '',
}))
const spinButtonLabel = computed(() => (selectedActivity.value?.participation?.available_draws || 0) > 0
  ? t('activityCenter.lottery.spin')
  : t('activityCenter.lottery.noChanceShort'))
const canClaimBenefit = computed(() => {
  const activity = selectedActivity.value
  if (!activity?.benefit || activity.availability !== 'active') return false
  const claimsToday = activity.participation?.benefit_claims_today || 0
  const remaining = activity.participation?.remaining_stock || 0
  return claimsToday < activity.benefit.daily_claim_limit && remaining > 0
})
const benefitClaimButtonLabel = computed(() => {
  if (claiming.value) return t('activityCenter.benefit.claiming')
  if ((selectedActivity.value?.participation?.benefit_claims_today || 0) > 0) return t('activityCenter.benefit.claimedToday')
  return t('activityCenter.benefit.claim')
})

function wheelHash(value: string): number {
  let hash = 2166136261
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

function wheelTones(activity: Activity | null, count: number): number[] {
  const configSeed = activity?.lottery
    ? `${activity.slug}:${activity.lottery.version}:${activity.lottery.prizes.map(prize => prize.id).join(',')}`
    : 'empty-wheel'

  const baseTones = Array.from({ length: WHEEL_BASE_TONE_COUNT }, (_, index) => index + 1)
    .sort((left, right) => wheelHash(`${configSeed}:${left}`) - wheelHash(`${configSeed}:${right}`) || left - right)
  if (count <= WHEEL_BASE_TONE_COUNT) return baseTones.slice(0, count)

  // Keep related light/dark variants apart so adjacent segments remain visibly distinct.
  const variantOffset = count === WHEEL_TONE_COUNT ? 2 : 1
  const variantTones = baseTones.map((_, index) => baseTones[(index + variantOffset) % WHEEL_BASE_TONE_COUNT] + WHEEL_BASE_TONE_COUNT)
  return [...baseTones, ...variantTones].slice(0, count)
}

function cancelWheelAnimation(): void {
  const resolve = wheelAnimationResolve
  wheelAnimationResolve = null
  wheelTimeline?.kill()
  wheelTimeline = null
  resolve?.()
}

function nextWheelRotation(target: number): number {
  const current = wheelRotation.value
  const normalizedCurrent = ((current % 360) + 360) % 360
  const normalizedTarget = ((target % 360) + 360) % 360
  const landingDelta = (normalizedTarget - normalizedCurrent + 360) % 360
  return current + WHEEL_FULL_TURNS * 360 + landingDelta
}

function setWheelRotation(element: HTMLElement, rotation: number): void {
  gsap.set(element, { rotation, force3D: true })
  gsap.set(element.querySelectorAll('.wheel-prize-text'), {
    rotation: -rotation,
    force3D: true,
  })
}

function animateWheel(endRotation: number): Promise<void> {
  const element = wheelDisc.value
  const startRotation = wheelRotation.value
  cancelWheelAnimation()
  wheelRotation.value = endRotation

  if (!element || reduceWheelMotion.value) {
    if (element) setWheelRotation(element, endRotation)
    return Promise.resolve()
  }

  const prizeTexts = element.querySelectorAll('.wheel-prize-text')
  const rotationDelta = endRotation - startRotation
  const weightedDuration = WHEEL_ACCELERATION_DURATION / 2
    + WHEEL_CRUISE_DURATION
    + WHEEL_DECELERATION_DURATION / 2
  const peakVelocity = rotationDelta / weightedDuration
  const accelerationEnd = startRotation + peakVelocity * WHEEL_ACCELERATION_DURATION / 2
  const cruiseEnd = accelerationEnd + peakVelocity * WHEEL_CRUISE_DURATION

  return new Promise((resolve) => {
    wheelAnimationResolve = resolve
    const timeline = gsap.timeline({
      onComplete: () => {
        if (wheelTimeline !== timeline) return
        wheelTimeline = null
        wheelAnimationResolve = null
        setWheelRotation(element, endRotation)
        resolve()
      },
    })
    wheelTimeline = timeline
    timeline
      .to(element, {
        rotation: accelerationEnd,
        duration: WHEEL_ACCELERATION_DURATION,
        ease: 'power1.in',
        force3D: true,
        overwrite: 'auto',
      })
      .to(prizeTexts, {
        rotation: -accelerationEnd,
        duration: WHEEL_ACCELERATION_DURATION,
        ease: 'power1.in',
        force3D: true,
        overwrite: 'auto',
      }, '<')
      .to(element, {
        rotation: cruiseEnd,
        duration: WHEEL_CRUISE_DURATION,
        ease: 'none',
        force3D: true,
      })
      .to(prizeTexts, {
        rotation: -cruiseEnd,
        duration: WHEEL_CRUISE_DURATION,
        ease: 'none',
        force3D: true,
      }, '<')
      .to(element, {
        rotation: endRotation,
        duration: WHEEL_DECELERATION_DURATION,
        ease: 'power1.out',
        force3D: true,
      })
      .to(prizeTexts, {
        rotation: -endRotation,
        duration: WHEEL_DECELERATION_DURATION,
        ease: 'power1.out',
        force3D: true,
      }, '<')
  })
}

function activitySwitchClass(activity: Activity): string[] {
  if (activity.availability !== 'active') {
    return ['cursor-not-allowed border-gray-200 bg-gray-50 text-gray-400 grayscale-[0.3] focus-visible:ring-2 focus-visible:ring-primary-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-500']
  }
  return selectedSlug.value === activity.slug
    ? ['border-primary-300 bg-primary-50 text-primary-700 focus-visible:ring-2 focus-visible:ring-primary-500 dark:border-primary-800 dark:bg-primary-950/30 dark:text-primary-300']
    : ['border-gray-200 bg-white text-gray-600 hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-primary-500 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700']
}

function selectActivity(activity: Activity): void {
  if (activity.availability !== 'active') {
    const key = activity.closed_reason && ['disabled', 'not_started', 'ended', 'configuration_required', 'unpublished'].includes(activity.closed_reason)
      ? activity.closed_reason
      : 'fallback'
    appStore.showWarning(t(`activityCenter.closedReason.${key}`))
    return
  }
  selectedSlug.value = activity.slug
}

function wheelPrizeStyle(index: number, count: number): CSSProperties {
  const angle = (index + 0.5) * (360 / Math.max(1, count))
  return { '--prize-angle': `${angle}deg` } as CSSProperties
}

function requestID(): string {
  return globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(36).slice(2)}-activity`
}

function requestStorageKey(action: 'draw' | 'claim', slug: string): string {
  return `modurelay:activity:${action}:${slug}:request-id`
}

function pendingRequestID(action: 'draw' | 'claim', slug: string): string {
  const key = requestStorageKey(action, slug)
  const inMemory = pendingRequestIDs.get(key)
  if (inMemory) return inMemory
  try {
    const stored = globalThis.sessionStorage?.getItem(key)
    if (stored) {
      pendingRequestIDs.set(key, stored)
      return stored
    }
  } catch {
    // In-memory idempotency still protects retries when session storage is unavailable.
  }
  const created = requestID()
  pendingRequestIDs.set(key, created)
  try {
    globalThis.sessionStorage?.setItem(key, created)
  } catch {
    // Ignore storage restrictions and retain the in-memory key.
  }
  return created
}

function clearPendingRequest(action: 'draw' | 'claim', slug: string): void {
  const key = requestStorageKey(action, slug)
  pendingRequestIDs.delete(key)
  try {
    globalThis.sessionStorage?.removeItem(key)
  } catch {
    // Nothing else is required when storage is unavailable.
  }
}

function isTerminalActionError(error: unknown): boolean {
  const status = Number((error as { status?: number })?.status || 0)
  return status >= 400 && status < 500
}

function money(value?: string | null): string {
  const amount = Number(value || 0)
  return Number.isFinite(amount)
    ? new Intl.NumberFormat(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 8 }).format(amount)
    : '0.00'
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function errorCode(error: unknown): string {
  return String((error as { code?: string })?.code || '')
}

function errorMessage(error: unknown): string {
  return String((error as { message?: string })?.message || t('common.unknownError'))
}

async function loadAll(): Promise<void> {
  loading.value = true
  try {
    const [activityItems, rewardItems] = await Promise.all([activityAPI.listActivities(), activityAPI.listRewards()])
    activities.value = activityItems
    rewards.value = rewardItems
    if (!selectedSlug.value || !activityItems.some(item => item.slug === selectedSlug.value && item.availability === 'active')) {
      selectedSlug.value = activityItems.find(item => item.availability === 'active')?.slug || ''
    }
  } catch (error) {
    appStore.showError(errorMessage(error) || t('activityCenter.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function refreshSelected(): Promise<void> {
  if (!selectedSlug.value) return
  const activity = await activityAPI.getActivity(selectedSlug.value)
  const index = activities.value.findIndex(item => item.slug === activity.slug)
  if (index >= 0) activities.value[index] = activity
  rewards.value = await activityAPI.listRewards()
}

async function spinLottery(): Promise<void> {
  const activity = selectedActivity.value
  if (!activity?.lottery || spinning.value) return
  if (!(activity.participation?.available_draws || 0)) {
    appStore.showWarning(t('activityCenter.lottery.noChance'))
    return
  }
  spinning.value = true
  const idempotencyKey = pendingRequestID('draw', activity.slug)
  try {
    const result = await activityAPI.drawLottery(activity.slug, idempotencyKey)
    clearPendingRequest('draw', activity.slug)
    const prizeIndex = Math.max(0, activity.lottery.prizes.findIndex(prize => prize.id === result.prize_id))
    const segment = 360 / Math.max(1, activity.lottery.prizes.length)
    const target = 360 - (prizeIndex + 0.5) * segment
    await animateWheel(nextWheelRotation(target))
    if (Number(result.reward_amount) > 0) {
      appStore.showSuccess(t('activityCenter.lottery.won', { name: result.prize_name, amount: money(result.reward_amount) }))
      await authStore.refreshUser()
    } else {
      appStore.showInfo(t('activityCenter.lottery.noReward', { name: result.prize_name }))
    }
    await refreshSelected()
  } catch (error) {
    if (isTerminalActionError(error)) clearPendingRequest('draw', activity.slug)
    const code = errorCode(error)
    appStore.showError(code === 'LOTTERY_NO_CHANCE' ? t('activityCenter.lottery.noChance') : errorMessage(error))
  } finally {
    spinning.value = false
  }
}

async function claimBenefit(): Promise<void> {
  const activity = selectedActivity.value
  if (!activity?.benefit || claiming.value || !canClaimBenefit.value) return
  claiming.value = true
  const idempotencyKey = pendingRequestID('claim', activity.slug)
  try {
    const result = await activityAPI.claimBenefit(activity.slug, idempotencyKey)
    clearPendingRequest('claim', activity.slug)
    lastBenefitClaim.value = { ...result, activity_id: activity.id }
    appStore.showSuccess(t('activityCenter.benefit.success', { amount: money(result.reward_amount), balance: money(result.balance_after) }))
    await Promise.all([authStore.refreshUser(), refreshSelected()])
  } catch (error) {
    if (isTerminalActionError(error)) clearPendingRequest('claim', activity.slug)
    const code = errorCode(error)
    if (code === 'BENEFIT_LIMIT_REACHED') appStore.showWarning(t('activityCenter.benefit.limitReached'))
    else if (code === 'BENEFIT_EXHAUSTED') appStore.showWarning(t('activityCenter.benefit.exhausted'))
    else appStore.showError(errorMessage(error))
  } finally {
    claiming.value = false
  }
}

watch(wheelDisc, (element, previousElement) => {
  if (!element && previousElement) {
    cancelWheelAnimation()
    return
  }
  if (element) setWheelRotation(element, wheelRotation.value)
})

onMounted(() => {
  const motionQuery = globalThis.matchMedia?.('(prefers-reduced-motion: reduce)')
  if (motionQuery?.matches || typeof motionQuery?.addEventListener !== 'function') {
    reduceWheelMotion.value = Boolean(motionQuery?.matches)
  } else {
    wheelMotionMedia = gsap.matchMedia()
    wheelMotionMedia.add('(prefers-reduced-motion: reduce)', () => {
      reduceWheelMotion.value = true
      cancelWheelAnimation()
      if (wheelDisc.value) setWheelRotation(wheelDisc.value, wheelRotation.value)
      return () => { reduceWheelMotion.value = false }
    })
  }
  void loadAll()
})

onUnmounted(() => {
  cancelWheelAnimation()
  wheelMotionMedia?.revert()
  wheelMotionMedia = null
})
</script>

<style scoped>
.activity-switch {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  transition-property: color, background-color, border-color, box-shadow, filter;
  transition-duration: var(--motion-fast);
  transition-timing-function: var(--ease-standard);
}

.activity-switch:focus-visible,
.refresh-button:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

.benefit-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(240px, 34%);
  min-height: 300px;
}

.benefit-content {
  align-self: center;
}

.benefit-visual {
  min-width: 0;
  min-height: 240px;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}

.benefit-image {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: cover;
  object-position: 48% 50%;
  filter: saturate(0.82) contrast(0.96);
}

.benefit-result {
  display: flex;
  max-width: 360px;
  min-height: 44px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  border-top: 1px solid var(--color-border-subtle);
  border-bottom: 1px solid var(--color-border-subtle);
  padding: 10px 0;
}

:global(html.dark .activity-page .benefit-image) {
  filter: brightness(0.72) saturate(0.72) contrast(0.94);
}

.wheel-panel {
  --wheel-divider: color-mix(in srgb, var(--color-surface-raised) 76%, var(--color-border));
  --wheel-tone-1: color-mix(in srgb, var(--color-primary) 52%, var(--color-surface));
  --wheel-tone-2: color-mix(in srgb, var(--color-accent) 45%, var(--color-surface));
  --wheel-tone-3: color-mix(in srgb, var(--color-warning) 39%, var(--color-surface));
  --wheel-tone-4: color-mix(in srgb, var(--color-text-secondary) 35%, var(--color-surface));
  --wheel-tone-5: color-mix(in srgb, var(--color-success) 39%, var(--color-surface));
  --wheel-tone-6: color-mix(in srgb, var(--color-info) 43%, var(--color-surface));
  --wheel-tone-7: color-mix(in srgb, var(--color-primary-hover) 31%, var(--color-surface));
  --wheel-tone-8: color-mix(in srgb, var(--color-accent) 29%, var(--color-surface));
  --wheel-tone-9: color-mix(in srgb, var(--color-warning) 25%, var(--color-surface));
  --wheel-tone-10: color-mix(in srgb, var(--color-text-muted) 25%, var(--color-surface));
  --wheel-tone-11: color-mix(in srgb, var(--color-success) 26%, var(--color-surface));
  --wheel-tone-12: color-mix(in srgb, var(--color-info) 28%, var(--color-surface));
}

.wheel-frame {
  isolation: isolate;
  border: 1px solid var(--color-border);
  border-radius: 50%;
  background: var(--color-surface-soft);
  box-shadow:
    inset 0 0 0 5px var(--color-surface-raised),
    inset 0 0 0 7px var(--color-border-subtle),
    var(--shadow-sm);
  transition-property: border-color, box-shadow;
  transition-duration: var(--motion-base);
  transition-timing-function: var(--ease-standard);
}

.wheel-frame::before {
  position: absolute;
  z-index: 0;
  inset: 7px;
  border: 1px solid var(--color-border-strong);
  border-radius: 50%;
  box-shadow:
    inset 0 0 0 2px color-mix(in srgb, var(--color-surface) 74%, transparent),
    0 1px 0 color-mix(in srgb, var(--color-surface) 88%, transparent);
  content: '';
  pointer-events: none;
}

.wheel-frame--spinning {
  border-color: var(--color-primary-border);
  box-shadow:
    inset 0 0 0 5px var(--color-surface-raised),
    inset 0 0 0 7px var(--color-primary-border),
    0 0 0 3px var(--color-primary-ring),
    var(--shadow-md);
}

.wheel-disc {
  z-index: 1;
  inset: 14px;
  overflow: hidden;
  border: 8px solid var(--color-surface-raised);
  box-shadow:
    0 2px 7px color-mix(in srgb, var(--color-bg-deep) 16%, transparent),
    inset 0 0 0 1px color-mix(in srgb, var(--color-text-primary) 13%, transparent),
    inset 0 0 18px color-mix(in srgb, var(--color-bg-deep) 9%, transparent);
  transform-origin: 50% 50%;
}

.wheel-disc::after {
  position: absolute;
  z-index: 2;
  inset: 0;
  border: 1px solid color-mix(in srgb, var(--color-surface) 62%, transparent);
  border-radius: 50%;
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-bg-deep) 8%, transparent);
  content: '';
  pointer-events: none;
}

.wheel-disc--spinning {
  will-change: transform;
}

.wheel-pointer {
  position: absolute;
  z-index: 4;
  top: -1px;
  left: 50%;
  width: 30px;
  height: 38px;
  transform: translateX(-50%);
  clip-path: polygon(8% 0, 92% 0, 50% 100%);
  background: var(--color-primary);
  filter: drop-shadow(0 2px 2px color-mix(in srgb, var(--color-bg-deep) 28%, transparent));
}

.wheel-pointer::after {
  position: absolute;
  top: 6px;
  left: 50%;
  width: 8px;
  height: 8px;
  transform: translateX(-50%);
  border: 2px solid color-mix(in srgb, var(--color-primary-active) 62%, transparent);
  border-radius: 50%;
  background: var(--color-surface-raised);
  content: '';
}

.wheel-prize-label {
  position: absolute;
  z-index: 1;
  inset: 0;
  transform: rotate(var(--prize-angle));
  pointer-events: none;
}

.wheel-prize-anchor {
  position: absolute;
  top: 17%;
  left: 50%;
  transform: translateX(-50%) rotate(calc(-1 * var(--prize-angle)));
}

.wheel-prize-text {
  display: block;
  max-width: 84px;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 14px;
  font-weight: 700;
  line-height: 1.2;
  text-align: center;
  text-overflow: ellipsis;
  text-shadow: 0 1px 1px color-mix(in srgb, var(--color-surface) 74%, transparent);
  white-space: nowrap;
}

.wheel-prize-label--dense .wheel-prize-text {
  max-width: 58px;
  font-size: 12px;
}

.wheel-center {
  z-index: 3;
  width: 74px;
  height: 74px;
  border: 1px solid var(--color-primary-border);
  color: var(--color-primary-hover);
  background: color-mix(in srgb, var(--color-primary) 10%, var(--color-surface));
  box-shadow:
    0 0 0 6px var(--color-surface-raised),
    0 0 0 7px var(--color-border),
    var(--shadow-md);
}

.summary-row {
  display: flex;
  min-height: 54px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 12px 20px;
  color: var(--color-text-secondary);
  font-size: 14px;
}

.summary-row dd {
  color: var(--color-text-primary);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.progress-track {
  background: var(--color-surface-soft);
}

.progress-value {
  background: var(--color-primary);
  transition-property: width;
  transition-duration: var(--motion-base);
  transition-timing-function: var(--ease-standard);
}

.prize-chip {
  border: 1px solid var(--color-border);
  border-radius: 999px;
  background: var(--color-surface-soft);
  padding: 5px 9px;
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.25;
  white-space: nowrap;
}

.refresh-button {
  display: inline-flex;
  width: 36px;
  height: 36px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  color: var(--color-text-secondary);
  background: var(--color-surface);
  transition-property: color, background-color, border-color;
  transition-duration: var(--motion-fast);
}

.refresh-button:hover {
  border-color: var(--color-border-strong);
  color: var(--color-text-primary);
  background: var(--color-surface-soft);
}

@media (prefers-reduced-motion: reduce) {
  .activity-switch,
  .wheel-frame,
  .progress-value,
  .refresh-button {
    transition-duration: 1ms;
  }
}

@media (max-width: 767px) {
  .benefit-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .benefit-content {
    order: 2;
  }

  .benefit-visual {
    order: 1;
    height: 180px;
    min-height: 180px;
    border-bottom: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .benefit-image {
    object-position: 50% 48%;
  }
}

@media (max-width: 430px) {
  .activity-page {
    min-width: 0;
  }

  .wheel-panel {
    padding: 16px;
  }

  .wheel-prize-text {
    max-width: 66px;
    font-size: 12px;
  }

  .wheel-prize-label--dense .wheel-prize-text {
    max-width: 46px;
    font-size: 11px;
  }
}
</style>
