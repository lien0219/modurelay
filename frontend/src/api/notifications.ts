import { apiClient } from './client'
import type { BasePaginationResponse, UserNotification } from '@/types'

export interface NotificationListQuery {
  page?: number
  page_size?: number
  category?: string
  unread?: boolean
  workspace_id?: number | null
  project_id?: number | null
  signal?: AbortSignal
}

export interface NotificationPage extends BasePaginationResponse<UserNotification> {
  items: UserNotification[]
}

export interface NotificationUnreadCount {
  count: number
  unread_count?: number
}

const queryParams = (query: NotificationListQuery = {}) => {
  const { signal, ...params } = query
  return { signal, params }
}

export async function listNotifications(query: NotificationListQuery = {}): Promise<NotificationPage> {
  const { data } = await apiClient.get<NotificationPage>('/notifications', queryParams({ page: 1, page_size: 20, ...query }))
  return data
}

export async function getUnreadNotificationCount(query: Omit<NotificationListQuery, 'page' | 'page_size'> = {}): Promise<number> {
  const { data } = await apiClient.get<NotificationUnreadCount | number>('/notifications/unread-count', queryParams(query))
  if (typeof data === 'number') return data
  return Number(data?.unread_count ?? data?.count ?? 0)
}

export async function markNotificationRead(id: number | string): Promise<void> {
  await apiClient.post(`/notifications/${encodeURIComponent(String(id))}/read`)
}

export async function markAllNotificationsRead(query: Omit<NotificationListQuery, 'page' | 'page_size' | 'signal'> = {}): Promise<void> {
  await apiClient.post('/notifications/read-all', undefined, { params: query })
}

export default {
  listNotifications,
  getUnreadNotificationCount,
  markNotificationRead,
  markAllNotificationsRead,
}
