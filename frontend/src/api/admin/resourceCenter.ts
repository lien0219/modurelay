import { apiClient } from '../client'
import type { ResourceCenterConfig, ResourceCommentPage, ResourcePostPage, ResourcePostDetail } from '@/types'

export async function getConfig(): Promise<ResourceCenterConfig> {
  const { data } = await apiClient.get<ResourceCenterConfig>('/admin/resource-center/config')
  return data
}

export async function updateConfig(config: ResourceCenterConfig): Promise<ResourceCenterConfig> {
  const { data } = await apiClient.put<ResourceCenterConfig>('/admin/resource-center/config', config)
  return data
}

export interface ResourceModerationQuery {
  q?: string
  sort?: 'latest' | 'popular'
  start_at?: string
  end_at?: string
  page?: number
  page_size?: number
}

export async function listPosts(query: ResourceModerationQuery = {}): Promise<ResourcePostPage> {
	const { data } = await apiClient.get<ResourcePostPage>('/admin/resource-center/posts', { params: query })
	return data
}

export async function getPost(id: number): Promise<ResourcePostDetail> {
  const { data } = await apiClient.get<ResourcePostDetail>(`/admin/resource-center/posts/${id}`)
  return data
}

export async function listComments(id: number, query: { page?: number; page_size?: number } = {}): Promise<ResourceCommentPage> {
	const { data } = await apiClient.get<ResourceCommentPage>(`/admin/resource-center/posts/${id}/comments`, { params: query })
	return data
}

export async function listAllComments(query: ResourceModerationQuery = {}): Promise<ResourceCommentPage> {
	const { data } = await apiClient.get<ResourceCommentPage>('/admin/resource-center/comments', { params: query })
	return data
}

export async function deletePost(id: number): Promise<void> {
  await apiClient.delete(`/admin/resource-center/posts/${id}`)
}

export async function deleteComment(id: number): Promise<void> {
  await apiClient.delete(`/admin/resource-center/comments/${id}`)
}

export async function batchDeletePosts(ids: number[]): Promise<{ deleted: number }> {
  const { data } = await apiClient.post<{ deleted: number }>('/admin/resource-center/posts/batch-delete', { ids })
  return data
}

export async function batchDeleteComments(ids: number[]): Promise<{ deleted: number }> {
  const { data } = await apiClient.post<{ deleted: number }>('/admin/resource-center/comments/batch-delete', { ids })
  return data
}

export default { getConfig, updateConfig, listPosts, getPost, listComments, listAllComments, deletePost, deleteComment, batchDeletePosts, batchDeleteComments }
