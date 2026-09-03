import { apiClient } from '../client'
import type { Activity, ActivityCenterAdminView, ActivityStatus } from '@/types'

export interface ActivityUpdatePayload {
  title?: string
  description?: string
  status?: ActivityStatus
  enabled?: boolean
  sort_order?: number
}

export interface LotteryPrizePayload {
  name: string
  amount: string
  probability_ppm: number
  sort_order: number
}

export interface LotteryConfigPayload {
  currency: 'CNY'
  recharge_threshold: string
  draws_per_threshold: number
  max_chances_per_order: number
  per_user_draw_limit: number
  daily_draw_limit: number
  daily_limit_timezone: string
  starts_at: string | null
  ends_at: string | null
  prizes: LotteryPrizePayload[]
}

export interface BenefitConfigPayload {
  currency: 'CNY'
  reward_amount: string
  random_min_amount: string
  random_max_amount: string
  total_stock: number
  per_user_limit: number
  starts_at: string | null
  ends_at: string | null
}

export async function getActivityCenter(): Promise<ActivityCenterAdminView> {
  const { data } = await apiClient.get<ActivityCenterAdminView>('/admin/activities')
  return data
}

export async function updateCenterSettings(enabled: boolean): Promise<ActivityCenterAdminView> {
  const { data } = await apiClient.put<ActivityCenterAdminView>('/admin/activities/settings', { enabled })
  return data
}

export async function updateActivity(slug: string, payload: ActivityUpdatePayload): Promise<Activity> {
  const { data } = await apiClient.patch<Activity>(`/admin/activities/${encodeURIComponent(slug)}`, payload)
  return data
}

export async function publishLotteryConfig(slug: string, payload: LotteryConfigPayload): Promise<Activity> {
  const { data } = await apiClient.post<Activity>(`/admin/activities/${encodeURIComponent(slug)}/lottery-config`, payload)
  return data
}

export async function publishBenefitConfig(slug: string, payload: BenefitConfigPayload): Promise<Activity> {
  const { data } = await apiClient.post<Activity>(`/admin/activities/${encodeURIComponent(slug)}/benefit-config`, payload)
  return data
}

export default { getActivityCenter, updateCenterSettings, updateActivity, publishLotteryConfig, publishBenefitConfig }
