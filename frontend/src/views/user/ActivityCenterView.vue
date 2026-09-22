<template>
  <AppLayout :enable-onboarding="false">
    <main class="activity-page">
      <div class="activity-shell">
        <div v-if="loading" class="activity-loading" aria-busy="true">
          <span class="activity-loading__hero"></span>
          <span class="activity-loading__tabs"></span>
          <span class="activity-loading__content"></span>
        </div>

        <template v-else>
          <section class="activity-hero">
            <div class="activity-hero__copy">
              <p class="activity-eyebrow">{{ t('activityCenter.heroEyebrow') }}</p>
              <h1>{{ t('activityCenter.heroTitle') }}</h1>
              <p class="activity-hero__description">{{ t('activityCenter.heroDescription') }}</p>

              <div class="activity-hero__highlights">
                <div class="activity-highlight">
                  <span class="activity-highlight__icon"><Icon name="gift" size="md" aria-hidden="true" /></span>
                  <span>
                    <strong>{{ t('activityCenter.heroHighlightRewards') }}</strong>
                    <small>{{ t('activityCenter.heroHighlightRewardsDesc') }}</small>
                  </span>
                </div>
                <div class="activity-highlight">
                  <span class="activity-highlight__icon activity-highlight__icon--accent"><Icon name="sparkles" size="md" aria-hidden="true" /></span>
                  <span>
                    <strong>{{ t('activityCenter.heroHighlightEasy') }}</strong>
                    <small>{{ t('activityCenter.heroHighlightEasyDesc') }}</small>
                  </span>
                </div>
                <div class="activity-highlight">
                  <span class="activity-highlight__icon"><Icon name="trophy" size="md" aria-hidden="true" /></span>
                  <span>
                    <strong>{{ t('activityCenter.heroHighlightFresh') }}</strong>
                    <small>{{ t('activityCenter.heroHighlightFreshDesc') }}</small>
                  </span>
                </div>
              </div>
            </div>

            <div class="activity-hero__visual" aria-hidden="true">
              <span class="activity-orbit activity-orbit--one"></span>
              <span class="activity-orbit activity-orbit--two"></span>
              <span class="activity-confetti activity-confetti--one"></span>
              <span class="activity-confetti activity-confetti--two"></span>
              <span class="activity-confetti activity-confetti--three"></span>
              <img :src="benefitGiftImage" alt="" class="activity-hero__gift" />
              <span class="activity-hero__note">{{ t('activityCenter.heroNote') }}</span>
            </div>
          </section>

          <section v-if="activities.length" class="activity-tabs">
            <nav class="activity-switcher" :aria-label="t('activityCenter.activityList')">
              <button
                v-for="activity in activities"
                :key="activity.id"
                type="button"
                class="activity-switch"
                :class="activitySwitchClass(activity)"
                :aria-disabled="activity.availability !== 'active'"
                :aria-pressed="selectedSlug === activity.slug"
                @click="selectActivity(activity)"
              >
                <Icon :name="activity.type === 'recharge_lottery' ? 'trophy' : 'gift'" size="sm" />
                <span>{{ activity.title }}</span>
                <span v-if="activity.availability !== 'active'" class="activity-switch__status">
                  {{ t('activityCenter.status.' + activity.availability) }}
                </span>
              </button>
            </nav>
            <span class="activity-tabs__hint">{{ t('activityCenter.currentActivityHint') }}</span>
          </section>

          <section v-if="!activities.length" class="activity-empty">
            <span class="activity-empty__icon"><Icon name="gift" size="xl" /></span>
            <strong>{{ t('activityCenter.empty') }}</strong>
          </section>

          <template v-if="selectedActivity?.type === 'recharge_lottery' && selectedActivity.lottery">
            <section class="lottery-stage">
              <div class="wheel-panel">
                <div class="activity-section-heading">
                  <div>
                    <p class="activity-section-kicker">{{ t('activityCenter.lottery.sectionEyebrow') }}</p>
                    <h2>{{ selectedActivity.title }}</h2>
                    <p>
                      {{ t('activityCenter.lottery.rechargeRuleImmediate', {
                        threshold: money(selectedActivity.lottery.recharge_threshold),
                        draws: selectedActivity.lottery.draws_per_threshold,
                      }) }}
                    </p>
                  </div>
                  <span class="activity-status-pill">
                    <span></span>
                    {{ t('activityCenter.status.active') }}
                  </span>
                </div>

                <div class="wheel-stage">
                  <div class="wheel-frame" :class="{ 'wheel-frame--spinning': spinning }">
                    <div class="wheel-pointer" aria-hidden="true" />
                    <div
                      ref="wheelDisc"
                      class="wheel-disc"
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
                      <span class="wheel-center">
                        <Icon name="gift" size="lg" />
                      </span>
                    </div>
                  </div>

                  <button
                    type="button"
                    class="wheel-action"
                    :disabled="spinning || !(selectedActivity.participation?.available_draws || 0)"
                    :aria-busy="spinning"
                    @click="spinLottery"
                  >
                    <span v-if="spinning" class="wheel-action__spinner"></span>
                    <Icon v-else name="sparkles" size="sm" />
                    {{ spinning ? t('activityCenter.lottery.spinning') : spinButtonLabel }}
                    <small v-if="!spinning">{{ t('activityCenter.lottery.remainingDraws', { count: selectedActivity.participation?.available_draws || 0 }) }}</small>
                  </button>
                </div>
              </div>

              <aside class="lottery-console" :aria-label="t('activityCenter.lottery.summary')">
                <section class="lottery-console__metrics">
                  <div class="lottery-console__heading">
                    <div>
                      <p class="activity-section-kicker">{{ t('activityCenter.lottery.summaryEyebrow') }}</p>
                      <h2>{{ t('activityCenter.lottery.summary') }}</h2>
                    </div>
                    <span class="lottery-console__available">{{ selectedActivity.participation?.available_draws || 0 }} {{ t('activityCenter.lottery.times') }}</span>
                  </div>

                  <dl class="lottery-metrics">
                    <div class="summary-row summary-row--primary">
                      <dt>{{ t('activityCenter.lottery.available') }}</dt>
                      <dd>{{ selectedActivity.participation?.available_draws || 0 }}</dd>
                    </div>
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
                </section>

                <section class="lottery-progress-card">
                  <div class="lottery-progress-card__topline">
                    <div>
                      <p class="activity-section-kicker">{{ t('activityCenter.lottery.nextChanceEyebrow') }}</p>
                      <h3>{{ t('activityCenter.lottery.nextChance') }}</h3>
                    </div>
                    <strong>{{ t('activityCenter.lottery.amountRemaining', { amount: money(selectedActivity.participation?.next_draw_recharge_amount) }) }}</strong>
                  </div>

                  <div
                    class="progress-track"
                    role="progressbar"
                    :aria-label="t('activityCenter.lottery.rechargeProgress')"
                    :aria-valuenow="progressPercent"
                    aria-valuemin="0"
                    aria-valuemax="100"
                  >
                    <span class="progress-value" :style="{ width: progressPercent + '%' }"></span>
                  </div>

                  <div class="lottery-progress-card__bottom">
                    <span>
                      {{ t('activityCenter.lottery.progressDetail', {
                        current: money(selectedActivity.participation?.recharge_progress_amount),
                        threshold: money(selectedActivity.lottery.recharge_threshold),
                      }) }}
                    </span>
                    <RouterLink to="/recharge">
                      <Icon name="creditCard" size="sm" />
                      {{ t('activityCenter.lottery.recharge') }}
                    </RouterLink>
                  </div>
                </section>

                <section class="lottery-prizes">
                  <div class="lottery-prizes__heading">
                    <div>
                      <p class="activity-section-kicker">{{ t('activityCenter.lottery.prizeOverviewEyebrow') }}</p>
                      <h3>{{ t('activityCenter.lottery.availablePrizes') }}</h3>
                    </div>
                    <span>{{ selectedActivity.lottery.prizes.length }}</span>
                  </div>
                  <ul :aria-label="t('activityCenter.lottery.availablePrizes')">
                    <li v-for="(prize, index) in selectedActivity.lottery.prizes" :key="prize.id" class="prize-chip">
                      <span class="prize-chip__icon"><Icon name="gift" size="sm" /></span>
                      <span>
                        <strong>{{ prize.name }}</strong>
                        <small>{{ t('activityCenter.lottery.prizeLabel', { index: index + 1 }) }}</small>
                      </span>
                    </li>
                  </ul>
                </section>
              </aside>
            </section>
          </template>

          <section
            v-if="selectedActivity?.type === 'limited_time_benefit' && selectedActivity.benefit"
            class="benefit-stage"
          >
            <div class="benefit-content">
              <div class="activity-section-heading">
                <div>
                  <p class="activity-section-kicker">{{ t('activityCenter.benefit.eyebrow') }}</p>
                  <h2>{{ selectedActivity.title }}</h2>
                  <p>{{ selectedActivity.description || t('activityCenter.benefit.defaultDescription') }}</p>
                </div>
                <span class="activity-status-pill">
                  <span></span>
                  {{ (selectedActivity.participation?.benefit_claims_today || 0) > 0
                    ? t('activityCenter.benefit.claimedToday')
                    : t('activityCenter.benefit.availableToday') }}
                </span>
              </div>

              <div class="benefit-reward-card">
                <span class="benefit-reward-card__icon"><Icon name="gift" size="lg" /></span>
                <span>
                  <small>{{ t('activityCenter.benefit.rewardEyebrow') }}</small>
                  <strong data-testid="benefit-reward-message">
                    {{ isRandomBenefit ? t('activityCenter.benefit.randomReveal') : t('activityCenter.benefit.fixedReveal') }}
                  </strong>
                  <p>{{ t('activityCenter.benefit.rewardDescription') }}</p>
                </span>
              </div>

              <div class="benefit-facts">
                <div>
                  <span class="benefit-fact__icon"><Icon name="gift" size="sm" /></span>
                  <span>
                    <strong>{{ t('activityCenter.benefit.remaining', { count: selectedActivity.participation?.remaining_stock || 0 }) }}</strong>
                    <small>{{ t('activityCenter.benefit.remainingHint') }}</small>
                  </span>
                </div>
                <div>
                  <span class="benefit-fact__icon"><Icon name="sparkles" size="sm" /></span>
                  <span>
                    <strong data-testid="benefit-daily-claim-rule">{{ t('activityCenter.benefit.dailyRule') }}</strong>
                    <small>{{ t('activityCenter.benefit.dailyHint') }}</small>
                  </span>
                </div>
                <div>
                  <span class="benefit-fact__icon benefit-fact__icon--success"><Icon name="trophy" size="sm" /></span>
                  <span>
                    <strong>
                      {{ (selectedActivity.participation?.benefit_claims_today || 0) > 0
                        ? t('activityCenter.benefit.claimedToday')
                        : t('activityCenter.benefit.availableToday') }}
                    </strong>
                    <small>{{ t('activityCenter.benefit.statusHint') }}</small>
                  </span>
                </div>
              </div>

              <div v-if="selectedBenefitClaim" class="benefit-result" role="status" data-testid="benefit-claim-result">
                <span>{{ t('activityCenter.benefit.received') }}</span>
                <strong>+¥{{ money(selectedBenefitClaim.reward_amount) }}</strong>
              </div>

              <button
                type="button"
                class="benefit-action"
                :disabled="claiming || !canClaimBenefit"
                :aria-busy="claiming"
                @click="claimBenefit"
              >
                <span v-if="claiming" class="wheel-action__spinner"></span>
                <Icon v-else name="gift" size="sm" />
                {{ benefitClaimButtonLabel }}
              </button>
            </div>

            <figure class="benefit-visual" aria-hidden="true">
              <span class="benefit-visual__orbit benefit-visual__orbit--one"></span>
              <span class="benefit-visual__orbit benefit-visual__orbit--two"></span>
              <img :src="benefitGiftImage" alt="" class="benefit-image" data-testid="benefit-marketing-image" />
              <span class="benefit-visual__note">{{ t('activityCenter.benefit.visualNote') }}</span>
            </figure>
          </section>

          <section v-if="selectedActivity" class="reward-history">
            <header class="reward-history__header">
              <div>
                <p class="activity-section-kicker">{{ t('activityCenter.rewards.eyebrow') }}</p>
                <h2>{{ t('activityCenter.rewards.title') }}</h2>
                <p>{{ t('activityCenter.rewards.description') }}</p>
              </div>
              <button
                type="button"
                class="refresh-button"
                :disabled="loading"
                :title="t('activityCenter.refresh')"
                :aria-label="t('activityCenter.refresh')"
                @click="loadAll"
              >
                <Icon name="refresh" size="sm" />
              </button>
            </header>

            <div v-if="selectedRewards.length" class="reward-history__list">
              <div v-for="reward in selectedRewards" :key="reward.id" class="reward-row">
                <span class="reward-row__icon"><Icon name="gift" size="sm" /></span>
                <span class="reward-row__copy">
                  <strong>{{ reward.activity_title }}</strong>
                  <small>{{ t('activityCenter.rewards.' + reward.source_type) }} · {{ formatDate(reward.created_at) }}</small>
                </span>
                <strong class="reward-row__amount">+¥{{ money(reward.amount) }}</strong>
              </div>
            </div>

            <div v-else class="reward-history__empty">
              <span><Icon name="gift" size="lg" /></span>
              <strong>{{ t('activityCenter.rewards.empty') }}</strong>
              <small>{{ t('activityCenter.rewards.emptyHint') }}</small>
            </div>
          </section>
        </template>
      </div>
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
.activity-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.activity-shell {
  width: min(1480px, calc(100% - 40px));
  margin-inline: auto;
  padding: 24px 0 52px;
}

.activity-loading {
  display: grid;
  gap: 14px;
}

.activity-loading > span {
  display: block;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  animation: activity-pulse 1.45s ease-in-out infinite;
}

.activity-loading__hero { height: 310px; }
.activity-loading__tabs { height: 62px; }
.activity-loading__content { height: 560px; }

.activity-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.08fr) minmax(430px, .92fr);
  min-height: 310px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background:
    radial-gradient(circle at 15% 20%, color-mix(in srgb, var(--color-primary) 8%, transparent), transparent 34%),
    var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.activity-hero__copy {
  position: relative;
  z-index: 2;
  min-width: 0;
  padding: 36px 40px;
}

.activity-eyebrow,
.activity-section-kicker {
  margin: 0;
  color: var(--color-primary);
  font-size: .7rem;
  font-weight: 760;
  letter-spacing: .09em;
  text-transform: uppercase;
}

.activity-hero h1 {
  max-width: 760px;
  margin: .8rem 0 0;
  font-size: clamp(2.15rem, 3.3vw, 3.2rem);
  font-weight: 740;
  line-height: 1.12;
  letter-spacing: -.048em;
}

.activity-hero__description {
  max-width: 700px;
  margin: .85rem 0 0;
  color: var(--color-text-secondary);
  font-size: .92rem;
  line-height: 1.7;
}

.activity-hero__highlights {
  display: grid;
  max-width: 760px;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  margin-top: 1.55rem;
}

.activity-highlight {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 9px;
}

.activity-highlight__icon {
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

.activity-highlight__icon--accent {
  border-color: color-mix(in srgb, var(--color-accent) 28%, var(--color-border));
  background: var(--color-accent-soft);
  color: var(--color-accent);
}

.activity-highlight > span:last-child {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.activity-highlight strong {
  font-size: .76rem;
  font-weight: 680;
}

.activity-highlight small {
  overflow: hidden;
  color: var(--color-text-muted);
  font-size: .66rem;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.activity-hero__visual {
  position: relative;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background:
    radial-gradient(circle at 55% 48%, color-mix(in srgb, var(--color-primary) 14%, transparent), transparent 36%),
    linear-gradient(135deg, color-mix(in srgb, var(--color-primary-soft) 78%, var(--color-surface-soft)), var(--color-surface-soft));
}

.activity-hero__gift {
  position: absolute;
  z-index: 3;
  top: 50%;
  left: 50%;
  width: min(66%, 350px);
  max-height: 82%;
  object-fit: cover;
  object-position: 50% 46%;
  border-radius: 34px;
  box-shadow: var(--shadow-lg);
  transform: translate(-50%, -50%) rotate(2deg);
  filter: saturate(.82) contrast(.96);
}

.activity-orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.activity-orbit--one { width: 330px; height: 210px; }
.activity-orbit--two {
  width: 470px;
  height: 290px;
  border-style: dashed;
  border-color: color-mix(in srgb, var(--color-accent) 24%, var(--color-border));
}

.activity-confetti {
  position: absolute;
  z-index: 4;
  width: 12px;
  height: 28px;
  border-radius: 4px;
  background: var(--color-primary);
  transform: rotate(26deg);
}

.activity-confetti--one { top: 46px; left: 18%; }
.activity-confetti--two { top: 72px; right: 13%; background: var(--color-warning); transform: rotate(-24deg); }
.activity-confetti--three { right: 24%; bottom: 36px; background: var(--color-accent); transform: rotate(52deg); }

.activity-hero__note {
  position: absolute;
  z-index: 5;
  right: 18px;
  bottom: 16px;
  padding: 7px 10px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg);
  color: var(--color-primary);
  font-size: .67rem;
  font-weight: 650;
  box-shadow: var(--shadow-xs);
  -webkit-backdrop-filter: blur(12px);
  backdrop-filter: blur(12px);
}

.activity-tabs {
  display: flex;
  min-height: 62px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 14px;
  padding: 10px 12px;
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  background: var(--glass-bg);
  box-shadow: var(--glass-shadow);
  -webkit-backdrop-filter: blur(16px);
  backdrop-filter: blur(16px);
}

.activity-switcher {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.activity-switch {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 8px;
  padding: 0 14px;
  border-radius: 10px;
  font-size: .78rem;
  font-weight: 620;
  transition-property: color, background-color, border-color, box-shadow, filter, transform;
  transition-duration: var(--motion-fast);
  transition-timing-function: var(--ease-standard);
}

.activity-switch:hover:not([aria-disabled='true']) {
  transform: translateY(-1px);
}

.activity-switch__status {
  font-size: .65rem;
  font-weight: 500;
}

.activity-tabs__hint {
  flex: 0 0 auto;
  color: var(--color-text-muted);
  font-size: .69rem;
}

.activity-empty {
  display: grid;
  min-height: 360px;
  place-items: center;
  align-content: center;
  gap: 8px;
  margin-top: 16px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
}

.activity-empty__icon {
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  border-radius: 17px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.lottery-stage {
  display: grid;
  grid-template-columns: minmax(520px, .98fr) minmax(0, 1.02fr);
  gap: 14px;
  margin-top: 14px;
}

.wheel-panel,
.lottery-console,
.benefit-stage,
.reward-history {
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
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
  min-width: 0;
  padding: 22px;
  overflow: hidden;
}

.activity-section-heading {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}

.activity-section-heading > div {
  min-width: 0;
}

.activity-section-heading h2 {
  margin: 5px 0 0;
  color: var(--color-text-primary);
  font-size: 1.15rem;
  font-weight: 700;
  line-height: 1.35;
}

.activity-section-heading p:not(.activity-section-kicker) {
  max-width: 660px;
  margin: 5px 0 0;
  color: var(--color-text-muted);
  font-size: .74rem;
  line-height: 1.55;
}

.activity-status-pill {
  display: inline-flex;
  min-height: 30px;
  flex: 0 0 auto;
  align-items: center;
  gap: 7px;
  padding: 0 9px;
  border: 1px solid color-mix(in srgb, var(--color-success) 22%, var(--color-border));
  border-radius: 999px;
  background: color-mix(in srgb, var(--color-success) 7%, var(--color-surface));
  color: var(--color-success);
  font-size: .66rem;
  font-weight: 650;
}

.activity-status-pill > span {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.wheel-stage {
  display: grid;
  justify-items: center;
  margin-top: 16px;
}

.wheel-frame {
  position: relative;
  width: min(100%, 480px);
  aspect-ratio: 1;
  isolation: isolate;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  background: var(--color-primary-soft);
  box-shadow:
    inset 0 0 0 8px var(--color-surface-raised),
    inset 0 0 0 10px var(--color-primary-border),
    0 22px 60px color-mix(in srgb, var(--color-primary) 16%, transparent);
  transition:
    border-color var(--motion-base) var(--ease-standard),
    box-shadow var(--motion-base) var(--ease-standard);
}

.wheel-frame::before {
  position: absolute;
  z-index: 0;
  inset: 12px;
  border: 1px solid var(--color-border-strong);
  border-radius: 50%;
  box-shadow: inset 0 0 0 2px color-mix(in srgb, var(--color-surface) 74%, transparent);
  content: '';
  pointer-events: none;
}

.wheel-frame::after {
  position: absolute;
  z-index: 5;
  inset: 2px;
  border: 4px dotted color-mix(in srgb, var(--color-primary) 32%, var(--color-surface));
  border-radius: 50%;
  content: '';
  pointer-events: none;
}

.wheel-frame--spinning {
  box-shadow:
    inset 0 0 0 8px var(--color-surface-raised),
    inset 0 0 0 10px var(--color-primary-border),
    0 0 0 4px var(--color-primary-ring),
    0 26px 70px color-mix(in srgb, var(--color-primary) 22%, transparent);
}

.wheel-disc {
  position: absolute;
  z-index: 1;
  inset: 20px;
  overflow: hidden;
  border: 9px solid var(--color-surface-raised);
  border-radius: 50%;
  box-shadow:
    0 3px 10px color-mix(in srgb, var(--color-bg-deep) 16%, transparent),
    inset 0 0 0 1px color-mix(in srgb, var(--color-text-primary) 13%, transparent);
  transform-origin: 50% 50%;
}

.wheel-disc--spinning {
  will-change: transform;
}

.wheel-pointer {
  position: absolute;
  z-index: 8;
  top: -2px;
  left: 50%;
  width: 32px;
  height: 42px;
  transform: translateX(-50%);
  clip-path: polygon(8% 0, 92% 0, 50% 100%);
  background: var(--color-primary);
  filter: drop-shadow(0 3px 3px color-mix(in srgb, var(--color-bg-deep) 28%, transparent));
}

.wheel-pointer::after {
  position: absolute;
  top: 7px;
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
  max-width: 88px;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 13px;
  font-weight: 700;
  line-height: 1.2;
  text-align: center;
  text-overflow: ellipsis;
  text-shadow: 0 1px 1px color-mix(in srgb, var(--color-surface) 74%, transparent);
  white-space: nowrap;
}

.wheel-prize-label--dense .wheel-prize-text {
  max-width: 60px;
  font-size: 11px;
}

.wheel-center {
  position: absolute;
  z-index: 4;
  top: 50%;
  left: 50%;
  display: grid;
  width: 78px;
  height: 78px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  background: color-mix(in srgb, var(--color-primary) 10%, var(--color-surface));
  color: var(--color-primary-hover);
  box-shadow:
    0 0 0 7px var(--color-surface-raised),
    0 0 0 8px var(--color-border),
    var(--shadow-md);
  transform: translate(-50%, -50%);
}

.wheel-action,
.benefit-action {
  display: inline-flex;
  min-height: 46px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  margin-top: 16px;
  padding: 0 18px;
  border: 1px solid var(--color-primary);
  border-radius: 11px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .8rem;
  font-weight: 680;
  box-shadow: var(--shadow-sm);
  transition:
    filter var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.wheel-action small {
  margin-left: 3px;
  opacity: .75;
  font-size: .64rem;
  font-weight: 550;
}

.wheel-action:hover:not(:disabled),
.benefit-action:hover:not(:disabled) {
  filter: brightness(.96);
  transform: translateY(-1px);
}

.wheel-action:disabled,
.benefit-action:disabled {
  cursor: not-allowed;
  opacity: .55;
}

.wheel-action__spinner {
  width: 15px;
  height: 15px;
  border: 2px solid color-mix(in srgb, var(--color-surface) 42%, transparent);
  border-top-color: var(--color-surface);
  border-radius: 50%;
  animation: activity-spin .8s linear infinite;
}

.lottery-console {
  display: grid;
  min-width: 0;
  align-content: start;
  gap: 12px;
  padding: 16px;
}

.lottery-console__metrics,
.lottery-progress-card,
.lottery-prizes {
  border: 1px solid var(--color-border);
  border-radius: 13px;
  background: var(--color-surface-raised);
}

.lottery-console__metrics {
  padding: 14px;
}

.lottery-console__heading,
.lottery-progress-card__topline,
.lottery-prizes__heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.lottery-console__heading h2,
.lottery-progress-card__topline h3,
.lottery-prizes__heading h3 {
  margin: 4px 0 0;
  color: var(--color-text-primary);
  font-size: .88rem;
  font-weight: 680;
}

.lottery-console__available {
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  padding: 0 9px;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .68rem;
  font-weight: 680;
}

.lottery-metrics {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.summary-row {
  display: grid;
  min-height: 72px;
  align-content: center;
  gap: 5px;
  padding: 11px 12px;
  border: 1px solid var(--color-border-subtle);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.summary-row dt {
  color: var(--color-text-muted);
  font-size: .66rem;
}

.summary-row dd {
  margin: 0;
  color: var(--color-text-primary);
  font-size: .82rem;
  font-weight: 680;
  font-variant-numeric: tabular-nums;
}

.summary-row--primary dd {
  color: var(--color-primary);
  font-size: 1.55rem;
  line-height: 1;
}

.lottery-progress-card {
  padding: 14px;
}

.lottery-progress-card__topline > strong {
  color: var(--color-text-primary);
  font-size: .76rem;
  font-weight: 680;
}

.progress-track {
  height: 8px;
  margin-top: 14px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--color-surface-soft);
}

.progress-value {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: linear-gradient(90deg, var(--color-primary), var(--color-accent));
  transition: width var(--motion-base) var(--ease-standard);
}

.lottery-progress-card__bottom {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 9px;
}

.lottery-progress-card__bottom > span {
  color: var(--color-text-muted);
  font-size: .65rem;
  line-height: 1.45;
}

.lottery-progress-card__bottom > a {
  display: inline-flex;
  min-height: 34px;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
  padding: 0 11px;
  border-radius: 9px;
  background: var(--color-primary);
  color: var(--color-surface);
  font-size: .7rem;
  font-weight: 650;
  text-decoration: none;
}

.lottery-prizes {
  padding: 14px;
}

.lottery-prizes__heading > span {
  display: grid;
  min-width: 26px;
  height: 26px;
  place-items: center;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .65rem;
  font-weight: 700;
}

.lottery-prizes ul {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 7px;
  margin: 12px 0 0;
  padding: 0;
  list-style: none;
}

.prize-chip {
  display: flex;
  min-width: 0;
  min-height: 54px;
  align-items: center;
  gap: 8px;
  padding: 8px 9px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.prize-chip__icon {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.prize-chip > span:last-child {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.prize-chip strong {
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: .7rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.prize-chip small {
  color: var(--color-text-muted);
  font-size: .6rem;
}

.benefit-stage {
  display: grid;
  grid-template-columns: minmax(0, 1.12fr) minmax(380px, .88fr);
  min-height: 420px;
  margin-top: 14px;
  overflow: hidden;
}

.benefit-content {
  min-width: 0;
  align-self: center;
  padding: 30px 34px;
}

.benefit-reward-card {
  display: flex;
  align-items: center;
  gap: 13px;
  margin-top: 22px;
  padding: 16px;
  border: 1px solid var(--color-primary-border);
  border-radius: 13px;
  background: linear-gradient(100deg, var(--color-primary-soft), color-mix(in srgb, var(--color-accent-soft) 70%, var(--color-surface)));
}

.benefit-reward-card__icon {
  display: grid;
  width: 52px;
  height: 52px;
  flex: 0 0 52px;
  place-items: center;
  border-radius: 15px;
  background: var(--color-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-xs);
}

.benefit-reward-card > span:last-child {
  min-width: 0;
}

.benefit-reward-card small {
  display: block;
  color: var(--color-primary);
  font-size: .65rem;
  font-weight: 700;
}

.benefit-reward-card strong {
  display: block;
  margin-top: 3px;
  color: var(--color-text-primary);
  font-size: 1rem;
  font-weight: 700;
}

.benefit-reward-card p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .68rem;
  line-height: 1.45;
}

.benefit-facts {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 9px;
  margin-top: 14px;
}

.benefit-facts > div {
  display: flex;
  min-width: 0;
  align-items: flex-start;
  gap: 8px;
  padding: 10px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface-soft);
}

.benefit-fact__icon {
  display: grid;
  width: 30px;
  height: 30px;
  flex: 0 0 30px;
  place-items: center;
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.benefit-fact__icon--success {
  background: color-mix(in srgb, var(--color-success) 9%, var(--color-surface));
  color: var(--color-success);
}

.benefit-facts > div > span:last-child {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.benefit-facts strong {
  color: var(--color-text-primary);
  font-size: .7rem;
  font-weight: 650;
  line-height: 1.4;
}

.benefit-facts small {
  color: var(--color-text-muted);
  font-size: .6rem;
  line-height: 1.4;
}

.benefit-result {
  display: flex;
  max-width: 420px;
  min-height: 46px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 14px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--color-success) 22%, var(--color-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-success) 6%, var(--color-surface));
}

.benefit-result span {
  color: var(--color-text-secondary);
  font-size: .72rem;
}

.benefit-result strong {
  color: var(--color-success);
  font-size: .95rem;
}

.benefit-action {
  min-width: 160px;
  margin-top: 18px;
}

.benefit-visual {
  position: relative;
  min-width: 0;
  min-height: 420px;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background:
    radial-gradient(circle at 50% 50%, color-mix(in srgb, var(--color-primary) 10%, transparent), transparent 40%),
    var(--color-surface-soft);
}

.benefit-image {
  position: absolute;
  z-index: 3;
  top: 50%;
  left: 50%;
  width: min(74%, 390px);
  max-height: 76%;
  object-fit: cover;
  object-position: 50% 48%;
  border-radius: 26px;
  box-shadow: var(--shadow-lg);
  transform: translate(-50%, -50%) rotate(2deg);
  filter: saturate(0.82) contrast(0.96);
}

:global(html.dark .activity-page .benefit-image) {
  filter: brightness(0.72) saturate(0.72) contrast(0.94);
}

.benefit-visual__orbit {
  position: absolute;
  top: 50%;
  left: 50%;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  transform: translate(-50%, -50%);
}

.benefit-visual__orbit--one { width: 320px; height: 220px; }
.benefit-visual__orbit--two {
  width: 440px;
  height: 310px;
  border-style: dashed;
  border-color: color-mix(in srgb, var(--color-accent) 23%, var(--color-border));
}

.benefit-visual__note {
  position: absolute;
  z-index: 4;
  right: 20px;
  bottom: 18px;
  max-width: 180px;
  padding: 8px 10px;
  border: 1px solid var(--glass-border);
  border-radius: 10px;
  background: var(--glass-bg);
  color: var(--color-primary);
  font-size: .67rem;
  font-weight: 650;
  line-height: 1.45;
  box-shadow: var(--shadow-xs);
  -webkit-backdrop-filter: blur(12px);
  backdrop-filter: blur(12px);
}

.reward-history {
  margin-top: 14px;
  overflow: hidden;
}

.reward-history__header {
  display: flex;
  min-height: 78px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 18px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.reward-history__header h2 {
  margin: 4px 0 0;
  color: var(--color-text-primary);
  font-size: .9rem;
  font-weight: 680;
}

.reward-history__header p:not(.activity-section-kicker) {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .66rem;
}

.refresh-button {
  display: inline-flex;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    border-color var(--motion-fast) var(--ease-standard);
}

.refresh-button:hover {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.reward-history__list {
  display: grid;
}

.reward-row {
  display: grid;
  min-height: 64px;
  grid-template-columns: 36px minmax(0, 1fr) auto;
  align-items: center;
  gap: 11px;
  padding: 10px 18px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.reward-row:last-child {
  border-bottom: 0;
}

.reward-row__icon {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 10px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.reward-row__copy {
  display: grid;
  min-width: 0;
  gap: 2px;
}

.reward-row__copy strong {
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: .75rem;
  font-weight: 650;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.reward-row__copy small {
  color: var(--color-text-muted);
  font-size: .64rem;
}

.reward-row__amount {
  color: var(--color-success);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .78rem;
  font-weight: 700;
}

.reward-history__empty {
  display: grid;
  min-height: 180px;
  justify-items: center;
  align-content: center;
  gap: 5px;
  padding: 24px;
  text-align: center;
}

.reward-history__empty > span {
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  margin-bottom: 3px;
  border-radius: 15px;
  background: var(--color-surface-soft);
  color: var(--color-text-disabled);
}

.reward-history__empty strong {
  color: var(--color-text-secondary);
  font-size: .78rem;
  font-weight: 650;
}

.reward-history__empty small {
  color: var(--color-text-muted);
  font-size: .65rem;
}

.activity-switch:focus-visible,
.refresh-button:focus-visible,
.wheel-action:focus-visible,
.benefit-action:focus-visible,
.lottery-progress-card__bottom > a:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

@keyframes activity-spin {
  to { transform: rotate(360deg); }
}

@keyframes activity-pulse {
  0%, 100% { opacity: .55; }
  50% { opacity: 1; }
}

@media (max-width: 1220px) {
  .activity-hero {
    grid-template-columns: minmax(0, 1fr) 360px;
  }

  .lottery-stage {
    grid-template-columns: minmax(470px, .9fr) minmax(0, 1.1fr);
  }

  .benefit-stage {
    grid-template-columns: minmax(0, 1fr) 360px;
  }
}

@media (max-width: 980px) {
  .activity-shell {
    width: min(100% - 28px, 860px);
  }

  .activity-hero,
  .lottery-stage,
  .benefit-stage {
    grid-template-columns: 1fr;
  }

  .activity-hero__visual {
    min-height: 240px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .lottery-console {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .lottery-prizes {
    grid-column: 1 / -1;
  }

  .benefit-visual {
    min-height: 300px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }
}

@media (max-width: 700px) {
  .activity-shell {
    width: min(100% - 20px, 560px);
    padding-top: 16px;
  }

  .activity-hero {
    min-height: auto;
    border-radius: 14px;
  }

  .activity-hero__copy {
    padding: 24px 20px;
  }

  .activity-hero h1 {
    font-size: 2rem;
  }

  .activity-hero__highlights {
    grid-template-columns: 1fr;
    gap: 10px;
  }

  .activity-highlight small {
    white-space: normal;
  }

  .activity-hero__visual {
    display: none;
  }

  .activity-tabs {
    align-items: flex-start;
    flex-direction: column;
  }

  .activity-tabs__hint {
    white-space: normal;
  }

  .activity-switch {
    min-height: 42px;
  }

  .wheel-panel,
  .lottery-console,
  .benefit-content {
    padding: 16px;
  }

  .activity-section-heading {
    align-items: flex-start;
    flex-direction: column;
  }

  .wheel-frame {
    width: min(100%, 380px);
  }

  .wheel-prize-text {
    max-width: 66px;
    font-size: 11px;
  }

  .wheel-prize-label--dense .wheel-prize-text {
    max-width: 44px;
    font-size: 10px;
  }

  .lottery-console {
    grid-template-columns: 1fr;
  }

  .lottery-prizes {
    grid-column: auto;
  }

  .lottery-progress-card__bottom {
    align-items: flex-start;
    flex-direction: column;
  }

  .lottery-prizes ul,
  .lottery-metrics {
    grid-template-columns: 1fr 1fr;
  }

  .benefit-facts {
    grid-template-columns: 1fr;
  }

  .benefit-visual {
    display: none;
  }

  .benefit-action {
    width: 100%;
  }
}

@media (max-width: 430px) {
  .lottery-prizes ul,
  .lottery-metrics {
    grid-template-columns: 1fr;
  }

  .wheel-frame {
    width: min(100%, 330px);
  }

  .wheel-disc {
    inset: 16px;
  }

  .wheel-center {
    width: 66px;
    height: 66px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .activity-switch,
  .wheel-frame,
  .progress-value,
  .refresh-button,
  .wheel-action,
  .benefit-action {
    transition-duration: 1ms;
  }

  .activity-switch:hover:not([aria-disabled='true']),
  .wheel-action:hover:not(:disabled),
  .benefit-action:hover:not(:disabled) {
    transform: none;
  }

  .activity-loading > span,
  .wheel-action__spinner {
    animation: none;
  }
}
</style>