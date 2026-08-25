import { apiClient } from './client'
import type {
  ResourceCategory,
  ResourceCenterConfig,
  ResourceComment,
  ResourceCommentPage,
  ResourceNotification,
  ResourcePost,
  ResourcePostPage,
  ResourcePostDetail,
} from '@/types'

export interface ResourcePostQuery {
  category_id?: number
  q?: string
  sort?: 'latest' | 'popular'
  page?: number
  page_size?: number
}

export async function getConfig(): Promise<ResourceCenterConfig> {
  const { data } = await apiClient.get<ResourceCenterConfig>('/resource-center/config')
  return data
}

export async function listCategories(): Promise<ResourceCategory[]> {
  const { data } = await apiClient.get<ResourceCategory[]>('/resource-center/categories')
  return data
}

export async function listPosts(query: ResourcePostQuery = {}): Promise<ResourcePostPage> {
	const { data } = await apiClient.get<ResourcePostPage>('/resource-center/posts', { params: query })
	return data
}

export async function getPost(id: number): Promise<ResourcePostDetail> {
  const { data } = await apiClient.get<ResourcePostDetail>(`/resource-center/posts/${id}`)
  return data
}

export async function listComments(postId: number, query: { page?: number; page_size?: number } = {}): Promise<ResourceCommentPage> {
	const { data } = await apiClient.get<ResourceCommentPage>(`/resource-center/posts/${postId}/comments`, { params: query })
	return data
}

export async function createPost(payload: { category_id: number; title: string; content: string }): Promise<ResourcePost> {
  const { data } = await apiClient.post<ResourcePost>('/resource-center/posts', payload)
  return data
}

export async function deletePost(postId: number): Promise<void> {
  await apiClient.delete(`/resource-center/posts/${postId}`)
}

export async function createComment(postId: number, payload: { parent_id?: number; content: string }): Promise<ResourceComment> {
  const { data } = await apiClient.post<ResourceComment>(`/resource-center/posts/${postId}/comments`, payload)
  return data
}

export async function deleteComment(commentId: number): Promise<void> {
  await apiClient.delete(`/resource-center/comments/${commentId}`)
}

export async function togglePostLike(postId: number): Promise<{ liked: boolean }> {
  const { data } = await apiClient.post<{ liked: boolean }>(`/resource-center/posts/${postId}/like`)
  return data
}

export async function toggleCommentLike(commentId: number): Promise<{ liked: boolean }> {
  const { data } = await apiClient.post<{ liked: boolean }>(`/resource-center/comments/${commentId}/like`)
  return data
}

export async function listNotifications(): Promise<ResourceNotification[]> {
  const { data } = await apiClient.get<ResourceNotification[]>('/resource-center/notifications')
  return data
}

export async function markNotificationRead(id: number): Promise<void> {
  await apiClient.post(`/resource-center/notifications/${id}/read`)
}

export async function markAllNotificationsRead(): Promise<void> {
  await apiClient.post('/resource-center/notifications/read-all')
}

export default {
  getConfig,
  listCategories,
  listPosts,
  getPost,
  listComments,
  createPost,
  deletePost,
  createComment,
  deleteComment,
  togglePostLike,
  toggleCommentLike,
  listNotifications,
  markNotificationRead,
  markAllNotificationsRead,
}
