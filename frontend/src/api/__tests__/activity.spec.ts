import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, put, patch } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  patch: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, post, put, patch },
}))

import activityAPI from '@/api/activity'
import activityAdminAPI from '@/api/admin/activity'

describe('activity APIs', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    get.mockResolvedValue({ data: [] })
    post.mockResolvedValue({ data: {} })
    put.mockResolvedValue({ data: {} })
    patch.mockResolvedValue({ data: {} })
  })

  it('uses encoded user routes and sends request IDs in action bodies', async () => {
    await activityAPI.listActivities()
    await activityAPI.listRewards()
    await activityAPI.getActivity('limited benefit')
    await activityAPI.drawLottery('recharge lottery', 'request-12345678')
    await activityAPI.claimBenefit('limited benefit', 'request-87654321')

    expect(get).toHaveBeenNthCalledWith(1, '/activities')
    expect(get).toHaveBeenNthCalledWith(2, '/activities/rewards')
    expect(get).toHaveBeenNthCalledWith(3, '/activities/limited%20benefit')
    expect(post).toHaveBeenNthCalledWith(1, '/activities/recharge%20lottery/draw', { request_id: 'request-12345678' })
    expect(post).toHaveBeenNthCalledWith(2, '/activities/limited%20benefit/claim', { request_id: 'request-87654321' })
  })

  it('preserves admin configuration payloads', async () => {
    const lottery = {
      currency: 'CNY' as const,
      recharge_threshold: '100',
      draws_per_threshold: 1,
      max_chances_per_order: 10,
      per_user_draw_limit: 0,
      daily_draw_limit: 5,
      daily_limit_timezone: 'Asia/Shanghai',
      starts_at: null,
      ends_at: null,
      prizes: [
        { name: 'Reward', amount: '10', probability_ppm: 100_000, sort_order: 10 },
        { name: 'Try again', amount: '0', probability_ppm: 900_000, sort_order: 20 },
      ],
    }
    await activityAdminAPI.updateCenterSettings(true)
    await activityAdminAPI.updateActivity('recharge lottery', { enabled: true })
    await activityAdminAPI.publishLotteryConfig('recharge lottery', lottery)
    const benefit = {
      currency: 'CNY' as const,
      reward_amount: '0',
      random_min_amount: '0.25',
      random_max_amount: '3.75',
      total_stock: 100,
      per_user_limit: 1,
      starts_at: null,
      ends_at: null,
    }
    await activityAdminAPI.publishBenefitConfig('limited benefit', benefit)

    expect(put).toHaveBeenCalledWith('/admin/activities/settings', { enabled: true })
    expect(patch).toHaveBeenCalledWith('/admin/activities/recharge%20lottery', { enabled: true })
    expect(post).toHaveBeenCalledWith('/admin/activities/recharge%20lottery/lottery-config', lottery)
    expect(post).toHaveBeenCalledWith('/admin/activities/limited%20benefit/benefit-config', benefit)
  })
})
