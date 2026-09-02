import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { Activity } from '@/types'
import ActivityAdminView from '../ActivityAdminView.vue'

const {
  getActivityCenter,
  updateCenterSettings,
  updateActivity,
  publishLotteryConfig,
  publishBenefitConfig,
  stepUpRun,
  fetchPublicSettings,
} = vi.hoisted(() => ({
  getActivityCenter: vi.fn(),
  updateCenterSettings: vi.fn(),
  updateActivity: vi.fn(),
  publishLotteryConfig: vi.fn(),
  publishBenefitConfig: vi.fn(),
  stepUpRun: vi.fn((action: () => Promise<unknown>) => action()),
  fetchPublicSettings: vi.fn(),
}))

vi.mock('@/api/admin/activity', () => ({
  default: { getActivityCenter, updateCenterSettings, updateActivity, publishLotteryConfig, publishBenefitConfig },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings,
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/composables/useStepUp', () => ({
  useStepUp: () => ({ run: stepUpRun, visible: { value: false } }),
  isStepUpBlocked: () => false,
  isStepUpCancelled: () => false,
  stepUpBlockReason: () => '',
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

const lotteryActivity: Activity = {
  id: 1,
  slug: 'recharge-lottery',
  type: 'recharge_lottery',
  title: 'Lottery',
  description: 'Recharge and draw',
  status: 'published',
  enabled: false,
  sort_order: 10,
  current_config_version: 1,
  availability: 'closed',
  closed_reason: 'disabled',
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:00:00Z',
  lottery: {
    id: 1,
    version: 1,
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

const benefitActivity: Activity = {
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
  benefit: {
    id: 2,
    version: 1,
    currency: 'CNY',
    reward_amount: '5',
    random_min_amount: '0',
    random_max_amount: '0',
    total_stock: 100,
    per_user_limit: 1,
    daily_claim_limit: 1,
    daily_limit_timezone: 'Asia/Shanghai',
    starts_at: null,
    ends_at: null,
    claimed_count: 0,
  },
}

function mountView() {
  return mount(ActivityAdminView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        TotpStepUpDialog: true,
      },
    },
  })
}

describe('ActivityAdminView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getActivityCenter.mockResolvedValue({ enabled: false, activities: [lotteryActivity] })
    updateCenterSettings.mockResolvedValue({ enabled: true, activities: [lotteryActivity] })
  })

  it('rejects a probability total other than 100 percent before publishing', async () => {
    const wrapper = mountView()
    await flushPromises()
    const probabilityInputs = wrapper.findAll('input[type="number"][max="100"]')
    expect(probabilityInputs).toHaveLength(2)
    await probabilityInputs[0].setValue('20')
    await probabilityInputs[1].setValue('20')
    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()

    expect(publishLotteryConfig).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.activities.probabilityInvalid')
  })

  it('publishes the recharge threshold as a string after editing the number input', async () => {
    publishLotteryConfig.mockResolvedValue(lotteryActivity)
    const wrapper = mountView()
    await flushPromises()
    const thresholdInput = wrapper.find('input[type="number"][min="0.00000001"]')

    await thresholdInput.setValue('100')
    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()

    expect(publishLotteryConfig).toHaveBeenCalledOnce()
    expect(publishLotteryConfig).toHaveBeenCalledWith(
      'recharge-lottery',
      expect.objectContaining({ recharge_threshold: '100' }),
    )
    expect(typeof publishLotteryConfig.mock.calls[0][1].recharge_threshold).toBe('string')
  })

  it('runs the global navigation switch through the step-up controller', async () => {
    const wrapper = mountView()
    await flushPromises()
    const globalSwitch = wrapper.find('input[type="checkbox"]')
    await globalSwitch.setValue(true)
    await flushPromises()

    expect(stepUpRun).toHaveBeenCalledOnce()
    expect(updateCenterSettings).toHaveBeenCalledWith(true)
    expect(fetchPublicSettings).toHaveBeenCalledWith(true)
  })

  it('toggles the lottery and benefit independently from the global navigation switch', async () => {
    getActivityCenter.mockResolvedValue({ enabled: false, activities: [lotteryActivity, benefitActivity] })
    updateActivity.mockImplementation(async (slug: string, payload: { enabled: boolean }) => ({
      ...(slug === lotteryActivity.slug ? lotteryActivity : benefitActivity),
      enabled: payload.enabled,
    }))
    const wrapper = mountView()
    await flushPromises()

    const lotterySwitch = wrapper.get('[data-testid="activity-enabled-toggle-recharge-lottery"]')
    const benefitSwitch = wrapper.get('[data-testid="activity-enabled-toggle-limited-time-benefit"]')
    await lotterySwitch.setValue(true)
    await flushPromises()
    await benefitSwitch.setValue(true)
    await flushPromises()

    expect(updateCenterSettings).not.toHaveBeenCalled()
    expect(updateActivity).toHaveBeenNthCalledWith(1, 'recharge-lottery', { enabled: true })
    expect(updateActivity).toHaveBeenNthCalledWith(2, 'limited-time-benefit', { enabled: true })
    expect(stepUpRun).toHaveBeenCalledTimes(2)
    expect((lotterySwitch.element as HTMLInputElement).checked).toBe(true)
    expect((benefitSwitch.element as HTMLInputElement).checked).toBe(true)
  })

  it('publishes a random benefit range when reward amount is zero', async () => {
    const randomBenefit: Activity = {
      ...benefitActivity,
      benefit: {
        ...benefitActivity.benefit!,
        reward_amount: '0',
        random_min_amount: '0.01',
        random_max_amount: '1.00',
      },
    }
    getActivityCenter.mockResolvedValue({ enabled: true, activities: [randomBenefit] })
    publishBenefitConfig.mockResolvedValue(randomBenefit)
    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.get('[data-testid="benefit-random-min-limited-time-benefit"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="benefit-daily-claim-rule"]').text()).toContain('admin.activities.dailyBenefitRule')
    await wrapper.get('[data-testid="benefit-random-min-limited-time-benefit"]').setValue('0.25')
    await wrapper.get('[data-testid="benefit-random-max-limited-time-benefit"]').setValue('3.75')
    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()

    expect(publishBenefitConfig).toHaveBeenCalledWith('limited-time-benefit', expect.objectContaining({
      reward_amount: '0',
      random_min_amount: '0.25',
      random_max_amount: '3.75',
      per_user_limit: 1,
    }))
  })

  it('rejects an invalid random range before publishing', async () => {
    const randomBenefit: Activity = {
      ...benefitActivity,
      benefit: {
        ...benefitActivity.benefit!,
        reward_amount: '0',
        random_min_amount: '0.01',
        random_max_amount: '1.00',
      },
    }
    getActivityCenter.mockResolvedValue({ enabled: true, activities: [randomBenefit] })
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-testid="benefit-random-min-limited-time-benefit"]').setValue('2.00')
    await wrapper.get('[data-testid="benefit-random-max-limited-time-benefit"]').setValue('1.00')
    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()

    expect(publishBenefitConfig).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('admin.activities.invalidRandomRange')
  })

  it('clears random bounds when publishing a fixed benefit', async () => {
    getActivityCenter.mockResolvedValue({ enabled: true, activities: [benefitActivity] })
    publishBenefitConfig.mockResolvedValue(benefitActivity)
    const wrapper = mountView()
    await flushPromises()

    await wrapper.findAll('form')[1].trigger('submit')
    await flushPromises()

    expect(publishBenefitConfig).toHaveBeenCalledWith('limited-time-benefit', expect.objectContaining({
      reward_amount: '5',
      random_min_amount: '0',
      random_max_amount: '0',
    }))
  })
})
