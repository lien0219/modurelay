<template>
  <AppLayout>
    <main class="mx-auto max-w-6xl space-y-6">
      <header class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div class="min-w-0">
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.activities.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.activities.description') }}</p>
        </div>
        <button type="button" class="btn btn-secondary inline-flex items-center gap-2 self-start" :disabled="loading" @click="load">
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          {{ t('activityCenter.refresh') }}
        </button>
      </header>

      <section class="glass-panel flex flex-col gap-4 rounded-xl border p-5 sm:flex-row sm:items-center sm:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('admin.activities.globalSwitch') }}</h2>
            <span class="rounded-full border px-2.5 py-1 text-xs font-medium" :class="centerEnabled ? 'border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-300' : 'border-gray-200 bg-gray-100 text-gray-600 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300'">
              {{ centerEnabled ? t('admin.activities.centerEnabled') : t('admin.activities.centerDisabled') }}
            </span>
          </div>
          <p class="mt-1 max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ t('admin.activities.globalSwitchHint') }}</p>
        </div>
        <label class="inline-flex min-h-11 shrink-0 cursor-pointer items-center gap-3 self-start sm:self-center">
          <input v-model="centerEnabled" type="checkbox" class="peer sr-only" :disabled="savingCenter" @change="saveCenter" />
          <span class="relative h-6 w-11 rounded-full bg-gray-300 transition-[background-color] duration-200 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow-sm after:transition-transform after:duration-200 peer-checked:bg-primary-600 peer-checked:after:translate-x-5 peer-focus-visible:ring-2 peer-focus-visible:ring-primary-500 peer-focus-visible:ring-offset-2 dark:bg-dark-600 dark:peer-focus-visible:ring-offset-dark-900" />
          <span class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ centerEnabled ? t('common.enabled') : t('common.disabled') }}</span>
        </label>
      </section>

      <div v-if="loading" class="space-y-4" aria-busy="true">
        <div v-for="index in 2" :key="index" class="h-80 animate-pulse rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800" />
      </div>

      <section v-for="activity in activities" v-else :key="activity.slug" class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="flex flex-col gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex min-w-0 items-center gap-3">
            <span class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-700 dark:bg-primary-950/30 dark:text-primary-300">
              <Icon :name="activity.type === 'recharge_lottery' ? 'trophy' : 'gift'" size="md" />
            </span>
            <div class="min-w-0">
              <h2 class="truncate text-base font-semibold text-gray-900 dark:text-white">{{ activity.title }}</h2>
              <p class="mt-0.5 font-mono text-xs text-gray-500 dark:text-gray-400">{{ activity.slug }}</p>
            </div>
          </div>
          <div class="flex flex-wrap items-center gap-3 sm:justify-end">
            <span class="rounded-md border border-gray-200 bg-gray-50 px-2.5 py-1 text-xs font-medium text-gray-600 dark:border-dark-600 dark:bg-dark-900 dark:text-gray-300">
              {{ t('admin.activities.currentVersion', { version: activity.current_config_version }) }}
            </span>
            <label
              class="inline-flex min-h-10 items-center gap-2.5"
              :class="saving[activity.slug] ? 'cursor-wait opacity-70' : 'cursor-pointer'"
            >
              <span class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.activities.activitySwitch') }}</span>
              <input
                type="checkbox"
                class="peer sr-only"
                :checked="activityEnabled(activity)"
                :disabled="!editors[activity.slug] || Boolean(saving[activity.slug])"
                :aria-label="t('admin.activities.activitySwitchLabel', { title: activity.title })"
                :data-testid="`activity-enabled-toggle-${activity.slug}`"
                @change="saveActivityEnabled(activity, $event)"
              />
              <span class="relative h-6 w-11 rounded-full bg-gray-300 transition-[background-color] duration-200 after:absolute after:left-0.5 after:top-0.5 after:h-5 after:w-5 after:rounded-full after:bg-white after:shadow-sm after:transition-transform after:duration-200 peer-checked:bg-primary-600 peer-checked:after:translate-x-5 peer-focus-visible:ring-2 peer-focus-visible:ring-primary-500 peer-focus-visible:ring-offset-2 peer-disabled:opacity-60 dark:bg-dark-600 dark:peer-focus-visible:ring-offset-dark-800" />
              <span class="min-w-12 text-sm font-medium text-gray-700 dark:text-gray-200">
                {{ activityEnabled(activity) ? t('admin.activities.activityEnabled') : t('admin.activities.activityDisabled') }}
              </span>
            </label>
          </div>
        </div>

        <div v-if="editors[activity.slug]" class="divide-y divide-gray-100 dark:divide-dark-700">
          <form class="space-y-4 p-5" @submit.prevent="saveBase(activity)">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.activities.baseSettings') }}</h3>
            <div class="grid gap-4 lg:grid-cols-[minmax(0,1.3fr)_minmax(180px,0.7fr)_120px]">
              <label class="block min-w-0">
                <span class="input-label">{{ t('admin.activities.titleLabel') }}</span>
                <input v-model.trim="editors[activity.slug].base.title" type="text" maxlength="120" required class="form-input mt-1 w-full" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.statusLabel') }}</span>
                <select v-model="editors[activity.slug].base.status" class="form-input mt-1 w-full">
                  <option value="draft">{{ t('admin.activities.status.draft') }}</option>
                  <option value="published">{{ t('admin.activities.status.published') }}</option>
                  <option value="archived">{{ t('admin.activities.status.archived') }}</option>
                </select>
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.sortOrder') }}</span>
                <input v-model.number="editors[activity.slug].base.sort_order" type="number" step="1" class="form-input mt-1 w-full" />
              </label>
            </div>
            <label class="block">
              <span class="input-label">{{ t('admin.activities.descriptionLabel') }}</span>
              <textarea v-model.trim="editors[activity.slug].base.description" rows="2" maxlength="1000" class="form-input mt-1 w-full resize-y" />
            </label>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary min-w-40" :disabled="saving[activity.slug] === 'base'">
                {{ saving[activity.slug] === 'base' ? t('common.saving') : t('admin.activities.saveBase') }}
              </button>
            </div>
          </form>

          <form v-if="activity.type === 'recharge_lottery' && editors[activity.slug].lottery" class="space-y-5 p-5" @submit.prevent="publishLottery(activity)">
            <div>
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.activities.lotteryConfig') }}</h3>
              <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.activities.prizeMinimum') }}</p>
            </div>

            <div v-if="errors[activity.slug]" ref="errorSummaries" role="alert" tabindex="-1" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">
              {{ errors[activity.slug] }}
            </div>

            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <label class="block">
                <span class="input-label">{{ t('admin.activities.currency') }}</span>
                <input value="CNY" disabled class="form-input mt-1 w-full" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.threshold') }}</span>
                <input v-model="editors[activity.slug].lottery!.recharge_threshold" type="number" min="0.00000001" max="1000000" step="0.00000001" required class="form-input mt-1 w-full" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.drawsPerThreshold') }}</span>
                <input v-model.number="editors[activity.slug].lottery!.draws_per_threshold" type="number" min="1" max="1000000" step="1" required class="form-input mt-1 w-full" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.maxPerOrder') }}</span>
                <input v-model.number="editors[activity.slug].lottery!.max_chances_per_order" type="number" min="0" max="1000000" step="1" class="form-input mt-1 w-full" />
                <span class="input-hint">{{ t('admin.activities.noLimitHint') }}</span>
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.perUserDrawLimit') }}</span>
                <input v-model.number="editors[activity.slug].lottery!.per_user_draw_limit" type="number" min="0" max="1000000" step="1" class="form-input mt-1 w-full" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.dailyDrawLimit') }}</span>
                <input v-model.number="editors[activity.slug].lottery!.daily_draw_limit" type="number" min="0" max="1000000" step="1" class="form-input mt-1 w-full" />
              </label>
              <label class="block sm:col-span-2">
                <span class="input-label">{{ t('admin.activities.timezone') }}</span>
                <input v-model.trim="editors[activity.slug].lottery!.daily_limit_timezone" type="text" required class="form-input mt-1 w-full font-mono" />
              </label>
              <label class="block sm:col-span-1 lg:col-span-2">
                <span class="input-label">{{ t('admin.activities.startsAt') }}</span>
                <input v-model="editors[activity.slug].lottery!.starts_at" type="datetime-local" class="form-input mt-1 w-full" />
              </label>
              <label class="block sm:col-span-1 lg:col-span-2">
                <span class="input-label">{{ t('admin.activities.endsAt') }}</span>
                <input v-model="editors[activity.slug].lottery!.ends_at" type="datetime-local" class="form-input mt-1 w-full" />
              </label>
            </div>

            <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
              <div class="flex flex-col gap-2 border-b border-gray-100 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-900 sm:flex-row sm:items-center sm:justify-between">
                <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.activities.prizeList') }}</h4>
                <div class="flex flex-wrap items-center gap-2">
                  <span class="text-xs font-medium" :class="probabilityTotal(activity.slug) === 100 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">
                    {{ t('admin.activities.probabilityTotal', { value: formatPercent(probabilityTotal(activity.slug)) }) }}
                  </span>
                  <button type="button" class="btn btn-secondary inline-flex h-9 items-center gap-1.5 px-3" :disabled="editors[activity.slug].lottery!.prizes.length >= 12" @click="addPrize(activity.slug)">
                    <Icon name="plus" size="sm" />{{ t('admin.activities.addPrize') }}
                  </button>
                </div>
              </div>
              <div class="overflow-x-auto">
                <table class="w-full min-w-[680px] text-left text-sm">
                  <thead class="text-xs text-gray-500 dark:text-gray-400">
                    <tr class="border-b border-gray-100 dark:border-dark-700">
                      <th class="px-4 py-3 font-medium">{{ t('admin.activities.prizeName') }}</th>
                      <th class="w-40 px-4 py-3 font-medium">{{ t('admin.activities.prizeAmount') }}</th>
                      <th class="w-40 px-4 py-3 font-medium">{{ t('admin.activities.probability') }}</th>
                      <th class="w-16 px-4 py-3" :aria-label="t('common.actions')"></th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="(prize, index) in editors[activity.slug].lottery!.prizes" :key="prize.key" class="border-b border-gray-100 last:border-b-0 dark:border-dark-700">
                      <td class="px-4 py-2.5"><input v-model.trim="prize.name" type="text" maxlength="120" required class="form-input h-9 w-full" /></td>
                      <td class="px-4 py-2.5"><input v-model="prize.amount" type="number" min="0" max="1000000" step="0.00000001" required class="form-input h-9 w-full tabular-nums" /></td>
                      <td class="px-4 py-2.5"><input v-model.number="prize.probability" type="number" min="0.0001" max="100" step="0.0001" required class="form-input h-9 w-full tabular-nums" /></td>
                      <td class="px-4 py-2.5 text-right">
                        <button type="button" class="inline-flex h-9 w-9 items-center justify-center rounded-md text-gray-400 transition-[color,background-color] duration-150 hover:bg-red-50 hover:text-red-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 dark:hover:bg-red-950/30" :title="t('admin.activities.removePrize')" :aria-label="t('admin.activities.removePrize')" :disabled="editors[activity.slug].lottery!.prizes.length <= 2" @click="removePrize(activity.slug, index)">
                          <Icon name="trash" size="sm" />
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary min-w-48" :disabled="saving[activity.slug] === 'config'">
                {{ saving[activity.slug] === 'config' ? t('common.saving') : t('admin.activities.publishLottery') }}
              </button>
            </div>
          </form>

          <form v-else-if="activity.type === 'limited_time_benefit' && editors[activity.slug].benefit" class="space-y-5 p-5" @submit.prevent="publishBenefit(activity)">
            <div class="flex flex-wrap items-center justify-between gap-3">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.activities.benefitConfig') }}</h3>
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.activities.claimedCount', { count: activity.benefit?.claimed_count || 0 }) }}</span>
            </div>
            <div v-if="errors[activity.slug]" ref="errorSummaries" role="alert" tabindex="-1" class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900 dark:bg-red-950/30 dark:text-red-300">
              {{ errors[activity.slug] }}
            </div>
            <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <label class="block">
                <span class="input-label">{{ t('admin.activities.currency') }}</span>
                <input value="CNY" disabled class="form-input mt-1 w-full" />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.rewardAmount') }}</span>
                <input
                  v-model="editors[activity.slug].benefit!.reward_amount"
                  type="number"
                  min="0"
                  max="1000000"
                  step="0.00000001"
                  required
                  class="form-input mt-1 w-full"
                  :data-testid="`benefit-reward-amount-${activity.slug}`"
                />
                <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.activities.rewardModeHint') }}</span>
              </label>
              <label v-if="isRandomBenefit(activity.slug)" class="block">
                <span class="input-label">{{ t('admin.activities.randomMinAmount') }}</span>
                <input
                  v-model="editors[activity.slug].benefit!.random_min_amount"
                  type="number"
                  min="0.01"
                  max="1000000"
                  step="0.01"
                  required
                  class="form-input mt-1 w-full"
                  :data-testid="`benefit-random-min-${activity.slug}`"
                />
              </label>
              <label v-if="isRandomBenefit(activity.slug)" class="block">
                <span class="input-label">{{ t('admin.activities.randomMaxAmount') }}</span>
                <input
                  v-model="editors[activity.slug].benefit!.random_max_amount"
                  type="number"
                  min="0.01"
                  max="1000000"
                  step="0.01"
                  required
                  class="form-input mt-1 w-full"
                  :data-testid="`benefit-random-max-${activity.slug}`"
                />
              </label>
              <label class="block">
                <span class="input-label">{{ t('admin.activities.totalStock') }}</span>
                <input v-model.number="editors[activity.slug].benefit!.total_stock" type="number" min="1" max="1000000" step="1" required class="form-input mt-1 w-full" />
              </label>
              <div class="block">
                <span class="input-label">{{ t('admin.activities.claimRule') }}</span>
                <p class="mt-1 flex min-h-10 items-center rounded-lg border border-gray-200 bg-gray-50 px-3 text-sm text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200" data-testid="benefit-daily-claim-rule">
                  {{ t('admin.activities.dailyBenefitRule') }}
                </p>
              </div>
              <label class="block sm:col-span-1 lg:col-span-2">
                <span class="input-label">{{ t('admin.activities.startsAt') }}</span>
                <input v-model="editors[activity.slug].benefit!.starts_at" type="datetime-local" class="form-input mt-1 w-full" />
              </label>
              <label class="block sm:col-span-1 lg:col-span-2">
                <span class="input-label">{{ t('admin.activities.endsAt') }}</span>
                <input v-model="editors[activity.slug].benefit!.ends_at" type="datetime-local" class="form-input mt-1 w-full" />
              </label>
            </div>
            <p class="flex min-h-9 items-center rounded-lg bg-gray-50 px-3 text-sm text-gray-600 dark:bg-dark-900 dark:text-gray-300" aria-live="polite">
              {{ t('admin.activities.maximumBudget', { amount: benefitMaximumBudget(activity.slug) }) }}
            </p>
            <div class="flex justify-end">
              <button type="submit" class="btn btn-primary min-w-48" :disabled="saving[activity.slug] === 'config'">
                {{ saving[activity.slug] === 'config' ? t('common.saving') : t('admin.activities.publishBenefit') }}
              </button>
            </div>
          </form>
        </div>
      </section>
    </main>
    <TotpStepUpDialog :controller="activityStepUp" />
  </AppLayout>
</template>

<script setup lang="ts">
import { nextTick, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import activityAdminAPI, { type BenefitConfigPayload, type LotteryConfigPayload } from '@/api/admin/activity'
import { isStepUpBlocked, isStepUpCancelled, stepUpBlockReason, useStepUp } from '@/composables/useStepUp'
import { ACTIVITY_PROBABILITY_SCALE, isActivityLimit, isActivityMoney, probabilityToPPM, probabilityTotalPPM } from '@/features/activity/validation'
import { useAppStore } from '@/stores/app'
import type { Activity, ActivityStatus } from '@/types'

interface PrizeEditor {
  key: string
  name: string
  amount: string
  probability: number
}

interface ActivityEditor {
  base: {
    title: string
    description: string
    status: ActivityStatus
    enabled: boolean
    sort_order: number
  }
  lottery?: {
    recharge_threshold: string
    draws_per_threshold: number
    max_chances_per_order: number
    per_user_draw_limit: number
    daily_draw_limit: number
    daily_limit_timezone: string
    starts_at: string
    ends_at: string
    prizes: PrizeEditor[]
  }
  benefit?: {
    reward_amount: string
    random_min_amount: string
    random_max_amount: string
    total_stock: number
    starts_at: string
    ends_at: string
  }
}

const { t } = useI18n()
const appStore = useAppStore()
const activityStepUp = useStepUp()
const loading = ref(true)
const centerEnabled = ref(false)
const savingCenter = ref(false)
const activities = ref<Activity[]>([])
const editors = reactive<Record<string, ActivityEditor>>({})
const saving = reactive<Record<string, 'base' | 'config' | 'enabled' | undefined>>({})
const errors = reactive<Record<string, string>>({})
const errorSummaries = ref<HTMLElement[]>([])

function apiMessage(error: unknown): string {
  const details = error as { code?: string; reason?: string }
  const code = String(details?.reason || details?.code || '')
  if (code === 'ACTIVITY_CONFIG_INVALID') return t('admin.activities.configRequired')
  return String((error as { message?: string })?.message || t('common.unknownError'))
}

function handleStepUpError(error: unknown): boolean {
  if (isStepUpCancelled(error)) return true
  if (!isStepUpBlocked(error)) return false
  const key = stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
    ? 'stepUp.adminApiKeyForbidden'
    : 'stepUp.notEnabled'
  appStore.showError(t(key))
  return true
}

function toLocalInput(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  const local = new Date(date.getTime() - date.getTimezoneOffset() * 60_000)
  return local.toISOString().slice(0, 16)
}

function toISO(value: string): string | null {
  return value ? new Date(value).toISOString() : null
}

function compactMoney(value: string): string {
  const normalized = String(value).trim()
  const [whole, fraction] = normalized.split('.')
  if (!fraction || !/^\d+$/.test(whole) || !/^\d+$/.test(fraction)) return normalized
  const compactFraction = fraction.replace(/0+$/, '')
  return compactFraction ? `${whole}.${compactFraction}` : whole
}

function editorFromActivity(activity: Activity): ActivityEditor {
  const editor: ActivityEditor = {
    base: {
      title: activity.title,
      description: activity.description,
      status: activity.status,
      enabled: activity.enabled,
      sort_order: activity.sort_order,
    },
  }
  if (activity.lottery) {
    editor.lottery = {
      recharge_threshold: activity.lottery.recharge_threshold,
      draws_per_threshold: activity.lottery.draws_per_threshold,
      max_chances_per_order: activity.lottery.max_chances_per_order,
      per_user_draw_limit: activity.lottery.per_user_draw_limit,
      daily_draw_limit: activity.lottery.daily_draw_limit,
      daily_limit_timezone: activity.lottery.daily_limit_timezone || 'Asia/Shanghai',
      starts_at: toLocalInput(activity.lottery.starts_at),
      ends_at: toLocalInput(activity.lottery.ends_at),
      prizes: activity.lottery.prizes.map(prize => ({
        key: `prize-${prize.id}`,
        name: prize.name,
        amount: prize.amount,
        probability: (prize.probability_ppm ?? 0) / 10_000,
      })),
    }
  }
  if (activity.benefit) {
    const isRandom = Number(activity.benefit.reward_amount) === 0
    editor.benefit = {
      reward_amount: compactMoney(activity.benefit.reward_amount),
      random_min_amount: isRandom && Number(activity.benefit.random_min_amount) <= 0
        ? '0.01'
        : compactMoney(activity.benefit.random_min_amount),
      random_max_amount: isRandom && Number(activity.benefit.random_max_amount) <= 0
        ? '1.00'
        : compactMoney(activity.benefit.random_max_amount),
      total_stock: activity.benefit.total_stock,
      starts_at: toLocalInput(activity.benefit.starts_at),
      ends_at: toLocalInput(activity.benefit.ends_at),
    }
  }
  return editor
}

function setActivities(items: Activity[]): void {
  activities.value = items
  for (const activity of items) editors[activity.slug] = editorFromActivity(activity)
}

async function load(): Promise<void> {
  loading.value = true
  try {
    const data = await activityAdminAPI.getActivityCenter()
    centerEnabled.value = data.enabled
    setActivities(data.activities)
  } catch (error) {
    appStore.showError(apiMessage(error) || t('admin.activities.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function saveCenter(): Promise<void> {
  savingCenter.value = true
  try {
    const data = await activityStepUp.run(() => activityAdminAPI.updateCenterSettings(centerEnabled.value))
    centerEnabled.value = data.enabled
    await appStore.fetchPublicSettings(true)
    appStore.showSuccess(t('admin.activities.saved'))
  } catch (error) {
    centerEnabled.value = !centerEnabled.value
    if (!handleStepUpError(error)) appStore.showError(apiMessage(error))
  } finally {
    savingCenter.value = false
  }
}

function replaceActivity(updated: Activity): void {
  const index = activities.value.findIndex(item => item.slug === updated.slug)
  if (index >= 0) activities.value[index] = updated
  editors[updated.slug] = editorFromActivity(updated)
}

function activityEnabled(activity: Activity): boolean {
  return editors[activity.slug]?.base.enabled ?? activity.enabled
}

async function saveActivityEnabled(activity: Activity, event: Event): Promise<void> {
  const editor = editors[activity.slug]
  if (!editor) return
  const enabled = (event.currentTarget as HTMLInputElement).checked
  const previous = activity.enabled
  editor.base.enabled = enabled
  saving[activity.slug] = 'enabled'
  try {
    const updated = await activityStepUp.run(() => activityAdminAPI.updateActivity(activity.slug, { enabled }))
    const index = activities.value.findIndex(item => item.slug === updated.slug)
    if (index >= 0) activities.value[index] = updated
    editor.base.enabled = updated.enabled
    appStore.showSuccess(t('admin.activities.saved'))
  } catch (error) {
    editor.base.enabled = previous
    if (!handleStepUpError(error)) appStore.showError(apiMessage(error))
  } finally {
    saving[activity.slug] = undefined
  }
}

async function saveBase(activity: Activity): Promise<void> {
  const editor = editors[activity.slug]
  errors[activity.slug] = ''
  saving[activity.slug] = 'base'
  try {
    const updated = await activityStepUp.run(() => activityAdminAPI.updateActivity(activity.slug, editor.base))
    replaceActivity(updated)
    appStore.showSuccess(t('admin.activities.saved'))
  } catch (error) {
    if (handleStepUpError(error)) return
    errors[activity.slug] = apiMessage(error)
    appStore.showError(errors[activity.slug])
  } finally {
    saving[activity.slug] = undefined
  }
}

function probabilityTotal(slug: string): number {
  const total = probabilityTotalPPM(editors[slug]?.lottery?.prizes || [])
  return total === null ? Number.NaN : total / 10_000
}

function formatPercent(value: number): string {
  return Number.isFinite(value) ? value.toFixed(4).replace(/\.?0+$/, '') : '--'
}

function addPrize(slug: string): void {
  editors[slug].lottery?.prizes.push({ key: `new-${Date.now()}-${Math.random()}`, name: '', amount: '0', probability: 0.0001 })
}

function removePrize(slug: string, index: number): void {
  editors[slug].lottery?.prizes.splice(index, 1)
}

function validWindow(startsAt: string, endsAt: string): boolean {
  return !startsAt || !endsAt || new Date(startsAt).getTime() < new Date(endsAt).getTime()
}

function isRandomBenefit(slug: string): boolean {
  return Number(editors[slug]?.benefit?.reward_amount) === 0
}

function isCentMoney(value: string): boolean {
  return /^\d+(?:\.\d{1,2})?$/.test(String(value).trim()) && isActivityMoney(value, false)
}

function benefitMaximumBudget(slug: string): string {
  const editor = editors[slug]?.benefit
  if (!editor) return '--'
  const unitAmount = isRandomBenefit(slug)
    ? Number(editor.random_max_amount)
    : Number(editor.reward_amount)
  const amount = unitAmount * Number(editor.total_stock)
  if (!Number.isFinite(amount) || amount < 0) return '--'
  return amount.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

async function focusError(): Promise<void> {
  await nextTick()
  errorSummaries.value.at(-1)?.focus()
}

async function publishLottery(activity: Activity): Promise<void> {
  const editor = editors[activity.slug].lottery
  if (!editor) return
  errors[activity.slug] = ''
  if (probabilityTotalPPM(editor.prizes) !== ACTIVITY_PROBABILITY_SCALE) {
    errors[activity.slug] = t('admin.activities.probabilityInvalid')
    await focusError()
    return
  }
  if (editor.prizes.length < 2 || !editor.prizes.some(prize => Number(prize.amount) > 0)) {
    errors[activity.slug] = t('admin.activities.prizeMinimum')
    await focusError()
    return
  }
  if (!isActivityMoney(editor.recharge_threshold, false) ||
      !isActivityLimit(Number(editor.draws_per_threshold), false) ||
      !isActivityLimit(Number(editor.max_chances_per_order), true) ||
      !isActivityLimit(Number(editor.per_user_draw_limit), true) ||
      !isActivityLimit(Number(editor.daily_draw_limit), true) ||
      !editor.daily_limit_timezone.trim() ||
      editor.prizes.some(prize => !prize.name.trim() || !isActivityMoney(prize.amount, true) || probabilityToPPM(Number(prize.probability)) === null)) {
    errors[activity.slug] = t('admin.activities.invalidNumber')
    await focusError()
    return
  }
  if (!validWindow(editor.starts_at, editor.ends_at)) {
    errors[activity.slug] = t('admin.activities.invalidWindow')
    await focusError()
    return
  }
  const payload: LotteryConfigPayload = {
    currency: 'CNY',
    recharge_threshold: String(editor.recharge_threshold),
    draws_per_threshold: Number(editor.draws_per_threshold),
    max_chances_per_order: Number(editor.max_chances_per_order),
    per_user_draw_limit: Number(editor.per_user_draw_limit),
    daily_draw_limit: Number(editor.daily_draw_limit),
    daily_limit_timezone: editor.daily_limit_timezone,
    starts_at: toISO(editor.starts_at),
    ends_at: toISO(editor.ends_at),
    prizes: editor.prizes.map((prize, index) => ({
      name: prize.name.trim(),
      amount: String(prize.amount),
      probability_ppm: probabilityToPPM(Number(prize.probability))!,
      sort_order: (index + 1) * 10,
    })),
  }
  saving[activity.slug] = 'config'
  try {
    replaceActivity(await activityStepUp.run(() => activityAdminAPI.publishLotteryConfig(activity.slug, payload)))
    appStore.showSuccess(t('admin.activities.published'))
  } catch (error) {
    if (handleStepUpError(error)) return
    errors[activity.slug] = apiMessage(error)
    appStore.showError(errors[activity.slug])
    await focusError()
  } finally {
    saving[activity.slug] = undefined
  }
}

async function publishBenefit(activity: Activity): Promise<void> {
  const editor = editors[activity.slug].benefit
  if (!editor) return
  errors[activity.slug] = ''
  const randomMode = isRandomBenefit(activity.slug)
  if (!isActivityMoney(editor.reward_amount, true) ||
      !isActivityLimit(Number(editor.total_stock), false)) {
    errors[activity.slug] = t('admin.activities.invalidNumber')
    await focusError()
    return
  }
  if (randomMode && (!isCentMoney(editor.random_min_amount) ||
      !isCentMoney(editor.random_max_amount) ||
      Number(editor.random_min_amount) > Number(editor.random_max_amount))) {
    errors[activity.slug] = t('admin.activities.invalidRandomRange')
    await focusError()
    return
  }
  if (!validWindow(editor.starts_at, editor.ends_at)) {
    errors[activity.slug] = t('admin.activities.invalidWindow')
    await focusError()
    return
  }
  const payload: BenefitConfigPayload = {
    currency: 'CNY',
    reward_amount: String(editor.reward_amount),
    random_min_amount: randomMode ? String(editor.random_min_amount) : '0',
    random_max_amount: randomMode ? String(editor.random_max_amount) : '0',
    total_stock: Number(editor.total_stock),
    per_user_limit: 1,
    starts_at: toISO(editor.starts_at),
    ends_at: toISO(editor.ends_at),
  }
  saving[activity.slug] = 'config'
  try {
    replaceActivity(await activityStepUp.run(() => activityAdminAPI.publishBenefitConfig(activity.slug, payload)))
    appStore.showSuccess(t('admin.activities.published'))
  } catch (error) {
    if (handleStepUpError(error)) return
    errors[activity.slug] = apiMessage(error)
    appStore.showError(errors[activity.slug])
    await focusError()
  } finally {
    saving[activity.slug] = undefined
  }
}

onMounted(() => { void load() })
</script>
