import { apiClient } from './client'
import type { Activity, ActivityReward, BenefitClaimResult, LotteryDrawResult } from '@/types'

export async function listActivities(): Promise<Activity[]> {
  const { data } = await apiClient.get<Activity[]>('/activities')
  return data
}

export async function getActivity(slug: string): Promise<Activity> {
  const { data } = await apiClient.get<Activity>(`/activities/${encodeURIComponent(slug)}`)
  return data
}

export async function drawLottery(slug: string, requestId: string): Promise<LotteryDrawResult> {
  const { data } = await apiClient.post<LotteryDrawResult>(
    `/activities/${encodeURIComponent(slug)}/draw`,
    { request_id: requestId },
  )
  return data
}

export async function claimBenefit(slug: string, requestId: string): Promise<BenefitClaimResult> {
  const { data } = await apiClient.post<BenefitClaimResult>(
    `/activities/${encodeURIComponent(slug)}/claim`,
    { request_id: requestId },
  )
  return data
}

export async function listRewards(): Promise<ActivityReward[]> {
  const { data } = await apiClient.get<ActivityReward[]>('/activities/rewards')
  return data
}

export default { listActivities, getActivity, drawLottery, claimBenefit, listRewards }
