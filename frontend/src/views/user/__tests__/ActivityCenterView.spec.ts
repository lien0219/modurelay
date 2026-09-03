import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Activity } from '@/types'
import ActivityCenterView from '../ActivityCenterView.vue'

const testDirectory = dirname(fileURLToPath(import.meta.url))
const viewSource = readFileSync(resolve(testDirectory, '../ActivityCenterView.vue'), 'utf8')
const appSource = readFileSync(resolve(testDirectory, '../../../App.vue'), 'utf8')

const { gsapTimeline, gsapSet, gsapMatchMedia, gsapPhases } = vi.hoisted(() => {
  const phases: Array<{
    target: unknown
    vars: Record<string, unknown>
    position?: string
  }> = []
  const timeline = vi.fn((options?: { onComplete?: () => void }) => {
    const instance = {
      to: vi.fn(),
      kill: vi.fn(),
    }
    instance.to.mockImplementation((target: unknown, vars: Record<string, unknown>, position?: string) => {
      phases.push({ target, vars, position })
      return instance
    })
    queueMicrotask(() => options?.onComplete?.())
    return instance
  })
  const set = vi.fn()
  const matchMedia = vi.fn(() => {
    let cleanup: (() => void) | undefined
    return {
      add: vi.fn((query: string, handler: () => void | (() => void)) => {
        if (!globalThis.matchMedia?.(query).matches) return
        const result = handler()
        if (typeof result === 'function') cleanup = result
      }),
      revert: vi.fn(() => cleanup?.()),
    }
  })
  return { gsapTimeline: timeline, gsapSet: set, gsapMatchMedia: matchMedia, gsapPhases: phases }
})

vi.mock('gsap', () => ({
  gsap: {
    timeline: gsapTimeline,
    set: gsapSet,
    matchMedia: gsapMatchMedia,
  },
}))

const { listActivities, listRewards, getActivity, drawLottery, claimBenefit, showWarning, showError } = vi.hoisted(() => ({
  listActivities: vi.fn(),
  listRewards: vi.fn(),
  getActivity: vi.fn(),
  drawLottery: vi.fn(),
  claimBenefit: vi.fn(),
  showWarning: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/activity', () => ({
  default: { listActivities, listRewards, getActivity, drawLottery, claimBenefit },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showWarning,
    showError,
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ refreshUser: vi.fn() }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => params
        ? `${key} ${Object.values(params).join(' ')}`
        : key,
    }),
  }
})

const activeLottery: Activity = {
  id: 1,
  slug: 'recharge-lottery',
  type: 'recharge_lottery',
  title: 'Lottery',
  description: 'Recharge and draw',
  status: 'published',
  enabled: true,
  sort_order: 10,
  current_config_version: 2,
  availability: 'active',
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  participation: {
    available_draws: 1,
    granted_draws: 1,
    used_draws: 0,
    drawn_today: 0,
    cumulative_recharge_amount: '30',
    recharge_progress_amount: '30',
    next_draw_recharge_amount: '70',
    reward_total: '12.34',
  },
  lottery: {
    id: 2,
    version: 2,
    currency: 'CNY',
    recharge_threshold: '100',
    draws_per_threshold: 1,
    max_chances_per_order: 10,
    per_user_draw_limit: 0,
    daily_draw_limit: 5,
    daily_limit_timezone: 'Asia/Shanghai',
    starts_at: null,
    ends_at: null,
    prizes: [
      { id: 1, name: 'Reward', amount: '10', probability_ppm: 100_000, sort_order: 10 },
      { id: 2, name: 'Try again', amount: '0', probability_ppm: 900_000, sort_order: 20 },
    ],
  },
}

const closedBenefit: Activity = {
  id: 2,
  slug: 'limited-time-benefit',
  type: 'limited_time_benefit',
  title: 'Benefit',
  description: 'Limited benefit',
  status: 'published',
  enabled: false,
  sort_order: 20,
  current_config_version: 1,
  availability: 'closed',
  closed_reason: 'disabled',
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  participation: { benefit_claims: 0, remaining_stock: 10 },
  benefit: {
    id: 3,
    version: 1,
    currency: 'CNY',
    reward_amount: '5',
    random_min_amount: '0',
    random_max_amount: '0',
    total_stock: 10,
    per_user_limit: 1,
    daily_claim_limit: 1,
    daily_limit_timezone: 'Asia/Shanghai',
    starts_at: null,
    ends_at: null,
    claimed_count: 0,
  },
}

function mountView() {
  return mount(ActivityCenterView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        RouterLink: { template: '<a><slot /></a>' },
      },
    },
  })
}

describe('ActivityCenterView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    gsapPhases.length = 0
    sessionStorage.clear()
    listActivities.mockResolvedValue([activeLottery, closedBenefit])
    listRewards.mockResolvedValue([])
    getActivity.mockResolvedValue(activeLottery)
    vi.stubGlobal('matchMedia', vi.fn(() => ({ matches: true })))
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  it('disables onboarding and clears inherited tour overlays on route entry', () => {
    expect(viewSource).toContain('<AppLayout :enable-onboarding="false">')
    expect(appSource).toContain("const ONBOARDING_DISABLED_PATHS = new Set([HOME_ROUTE_PATH, '/ai-learning', '/activities'])")
  })

  it('reconciles embedded feature flags with the public settings API on startup', () => {
    expect(appSource).toContain('await appStore.fetchPublicSettings(true)')
  })

  it('limits the dark marketing-image filter to the activity image', () => {
    expect(viewSource).toContain(':global(html.dark .activity-page .benefit-image)')
    expect(viewSource).not.toContain(':global(.dark) .benefit-image')
  })

  it('keeps a closed activity focusable and explains why it cannot be opened', async () => {
    const wrapper = mountView()
    await flushPromises()
    const closed = wrapper.findAll('button').find(button => button.text().includes('Benefit'))
    expect(closed?.attributes('aria-disabled')).toBe('true')
    expect(closed?.classes()).toContain('cursor-not-allowed')
    await closed?.trigger('click')
    expect(showWarning).toHaveBeenCalledWith('activityCenter.closedReason.disabled')
  })

  it('renders configured prizes and cumulative recharge progress without exposing probabilities', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('Reward')
    expect(wrapper.text()).toContain('Try again')
    expect(wrapper.text()).toContain('70.00')
    expect(wrapper.text()).not.toContain('100000')
    expect(wrapper.text()).not.toContain('900000')
    expect(wrapper.find('[role="progressbar"]').attributes('aria-valuenow')).toBe('30')
  })

  it('hides reward amounts before claim and reveals the actual random reward afterward', async () => {
    const randomBenefit: Activity = {
      ...closedBenefit,
      enabled: true,
      availability: 'active',
      closed_reason: undefined,
      benefit: {
        ...closedBenefit.benefit!,
        reward_amount: '0',
        random_min_amount: '0.01',
        random_max_amount: '8.88',
      },
    }
    listActivities.mockResolvedValue([randomBenefit])
    getActivity.mockResolvedValue(randomBenefit)
    claimBenefit.mockResolvedValue({
      claim_id: 9,
      reward_amount: '3.27',
      balance_after: '103.27',
      remaining_stock: 9,
      created_at: '2026-09-02T00:00:00Z',
    })
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="benefit-marketing-image"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="benefit-reward-message"]').text()).toContain('activityCenter.benefit.randomReveal')
    expect(wrapper.text()).not.toContain('0.01')
    expect(wrapper.text()).not.toContain('8.88')

    await wrapper.findAll('button').find(button => button.text().includes('activityCenter.benefit.claim'))!.trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="benefit-claim-result"]').text()).toContain('3.27')
  })

  it('does not disclose a fixed benefit amount before claim', async () => {
    const fixedBenefit: Activity = {
      ...closedBenefit,
      enabled: true,
      availability: 'active',
      closed_reason: undefined,
    }
    listActivities.mockResolvedValue([fixedBenefit])
    getActivity.mockResolvedValue(fixedBenefit)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="benefit-reward-message"]').text()).toContain('activityCenter.benefit.fixedReveal')
    expect(wrapper.text()).not.toContain('5.00')
  })

  it('disables a benefit after today\'s claim and explains the daily reset rule', async () => {
    const claimedBenefit: Activity = {
      ...closedBenefit,
      enabled: true,
      availability: 'active',
      closed_reason: undefined,
      participation: { benefit_claims: 4, benefit_claims_today: 1, remaining_stock: 6 },
    }
    listActivities.mockResolvedValue([claimedBenefit])
    getActivity.mockResolvedValue(claimedBenefit)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="benefit-daily-claim-rule"]').text()).toContain('activityCenter.benefit.dailyRule')
    const claimButton = wrapper.findAll('button').find(button => button.text().includes('activityCenter.benefit.claimedToday'))!
    expect(claimButton.attributes('disabled')).toBeDefined()
  })

  it.each([
    { perUserLimit: 0, dailyLimit: 5, totalVisible: false, dailyVisible: true },
    { perUserLimit: 10, dailyLimit: 5, totalVisible: true, dailyVisible: true },
    { perUserLimit: 0, dailyLimit: 0, totalVisible: false, dailyVisible: false },
  ])('shows only configured draw limits for $perUserLimit total and $dailyLimit daily', async ({ perUserLimit, dailyLimit, totalVisible, dailyVisible }) => {
    const configuredLottery: Activity = {
      ...activeLottery,
      lottery: {
        ...activeLottery.lottery!,
        per_user_draw_limit: perUserLimit,
        daily_draw_limit: dailyLimit,
      },
    }
    listActivities.mockResolvedValue([configuredLottery])
    getActivity.mockResolvedValue(configuredLottery)

    const wrapper = mountView()
    await flushPromises()
    const summaryRows = wrapper.findAll('.summary-row')
    const totalLimitRow = summaryRows.find(row => row.text().includes('activityCenter.lottery.totalDrawLimit'))
    const dailyLimitRow = summaryRows.find(row => row.text().includes('activityCenter.lottery.dailyDrawLimit'))

    expect(Boolean(totalLimitRow)).toBe(totalVisible)
    expect(Boolean(dailyLimitRow)).toBe(dailyVisible)
    if (totalLimitRow) expect(totalLimitRow.text()).toContain(String(perUserLimit))
    if (dailyLimitRow) expect(dailyLimitRow.text()).toContain(String(dailyLimit))
  })

  it.each([7, 12])('assigns %i wheel segments unique stable tones', async (prizeCount) => {
    const configuredLottery: Activity = {
      ...activeLottery,
      lottery: {
        ...activeLottery.lottery!,
        prizes: Array.from({ length: prizeCount }, (_, index) => ({
          id: index + 1,
          name: `Reward ${index + 1}`,
          amount: String(index + 1),
          probability_ppm: index === prizeCount - 1 ? 1_000_001 - prizeCount : 1,
          sort_order: (index + 1) * 10,
        })),
      },
    }
    listActivities.mockResolvedValue([configuredLottery])
    getActivity.mockResolvedValue(configuredLottery)

    const firstWrapper = mountView()
    await flushPromises()
    const firstBackground = firstWrapper.find('.wheel-disc').attributes('style') || ''
    const firstTones = [...firstBackground.matchAll(/--wheel-tone-(\d+)/g)].map(match => match[1])

    expect(firstTones).toHaveLength(prizeCount)
    expect(new Set(firstTones).size).toBe(prizeCount)

    firstWrapper.unmount()
    const secondWrapper = mountView()
    await flushPromises()
    const secondBackground = secondWrapper.find('.wheel-disc').attributes('style') || ''
    const secondTones = [...secondBackground.matchAll(/--wheel-tone-(\d+)/g)].map(match => match[1])

    expect(secondTones).toEqual(firstTones)
  })

  it('accelerates, cruises, and decelerates with continuous angular velocity', async () => {
    vi.stubGlobal('matchMedia', vi.fn((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })))
    drawLottery.mockResolvedValue({
      draw_id: 1,
      prize_id: 1,
      prize_name: 'Reward',
      reward_amount: '10',
      available_draws: 0,
      created_at: '2026-09-02T00:00:00Z',
    })

    const wrapper = mountView()
    await flushPromises()
    const spin = wrapper.findAll('button').find(button => button.text().includes('activityCenter.lottery.spin'))!
    await spin.trigger('click')
    await flushPromises()

    expect(gsapTimeline).toHaveBeenCalledTimes(1)
    expect(gsapPhases).toHaveLength(6)
    const wheelElement = wrapper.find('.wheel-disc').element
    const wheelPhases = gsapPhases.filter(phase => phase.target === wheelElement)
    const textPhases = gsapPhases.filter(phase => phase.target !== wheelElement)
    expect(wheelPhases).toHaveLength(3)
    expect(textPhases).toHaveLength(3)

    const [acceleration, cruise, deceleration] = wheelPhases.map(phase => phase.vars)
    expect(acceleration.ease).toBe('power1.in')
    expect(cruise.ease).toBe('none')
    expect(deceleration.ease).toBe('power1.out')

    textPhases.forEach((textPhase, index) => {
      const wheelPhase = wheelPhases[index]
      expect(textPhase.position).toBe('<')
      expect(textPhase.vars.rotation).toBe(-Number(wheelPhase.vars.rotation))
      expect(textPhase.vars.duration).toBe(wheelPhase.vars.duration)
      expect(textPhase.vars.ease).toBe(wheelPhase.vars.ease)
    })

    const accelerationEnd = Number(acceleration.rotation)
    const cruiseEnd = Number(cruise.rotation)
    const endRotation = Number(deceleration.rotation)
    const peakFromAcceleration = 2 * accelerationEnd / Number(acceleration.duration)
    const peakFromCruise = (cruiseEnd - accelerationEnd) / Number(cruise.duration)
    const peakFromDeceleration = 2 * (endRotation - cruiseEnd) / Number(deceleration.duration)

    expect(peakFromAcceleration).toBeCloseTo(peakFromCruise, 8)
    expect(peakFromDeceleration).toBeCloseTo(peakFromCruise, 8)
    expect(Number(acceleration.duration) + Number(cruise.duration) + Number(deceleration.duration)).toBeCloseTo(4.8, 8)
    expect(((endRotation % 360) + 360) % 360).toBeCloseTo(270, 8)
  })

  it('reuses the same request ID after an ambiguous draw failure', async () => {
    vi.spyOn(globalThis.crypto, 'randomUUID').mockReturnValue('11111111-1111-4111-8111-111111111111')
    drawLottery
      .mockRejectedValueOnce(new Error('network timeout'))
      .mockResolvedValueOnce({
        draw_id: 1,
        prize_id: 2,
        prize_name: 'Try again',
        reward_amount: '0',
        available_draws: 0,
        created_at: '2026-09-02T00:00:00Z',
      })

    const wrapper = mountView()
    await flushPromises()
    const spin = () => wrapper.findAll('button').find(button => button.text().includes('activityCenter.lottery.spin'))!
    await spin().trigger('click')
    await flushPromises()
    await spin().trigger('click')
    await flushPromises()

    expect(drawLottery).toHaveBeenCalledTimes(2)
    expect(drawLottery.mock.calls[0][1]).toBe('11111111-1111-4111-8111-111111111111')
    expect(drawLottery.mock.calls[1][1]).toBe(drawLottery.mock.calls[0][1])
    expect(sessionStorage.getItem('modurelay:activity:draw:recharge-lottery:request-id')).toBeNull()
    expect(showError).toHaveBeenCalledWith('network timeout')
  })
})
