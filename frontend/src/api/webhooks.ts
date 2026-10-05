import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export interface WorkspaceWebhook {
  id: number
  workspace_id: number
  name: string
  url: string
  enabled: boolean
  event_types: string[]
  created_by_user_id?: number
  created_at?: string
  updated_at?: string
  previous_secret_until?: string | null
  disabled_at?: string | null
}

export interface WorkspaceWebhookDelivery {
  id: number
  webhook_id: number
  event_id: string
  event_type: string
  status: string
  attempts: number
  next_attempt_at?: string
  response_status?: number | null
  response_preview?: string
  last_error?: string
  error_code?: string
  last_attempt_at?: string | null
  created_at?: string
  delivered_at?: string | null
}

export interface WorkspaceWebhookInput {
  name: string
  url: string
  event_types: string[]
}

export interface WorkspaceWebhookUpdateInput {
  name?: string
  url?: string
  enabled?: boolean
  event_types?: string[]
}

export interface WorkspaceWebhookSecretResponse {
  webhook: WorkspaceWebhook
  secret: string
}

export interface WorkspaceWebhookPage extends BasePaginationResponse<WorkspaceWebhook> {
  items: WorkspaceWebhook[]
}

export interface WorkspaceWebhookDeliveryPage extends BasePaginationResponse<WorkspaceWebhookDelivery> {
  items: WorkspaceWebhookDelivery[]
}

const path = (workspaceId: number, webhookId?: number) => webhookId === undefined
  ? `/workspaces/${workspaceId}/webhooks`
  : `/workspaces/${workspaceId}/webhooks/${webhookId}`

export const webhooksAPI = {
  list: async (workspaceId: number, params: { signal?: AbortSignal } = {}) =>
    (await apiClient.get<WorkspaceWebhookPage | WorkspaceWebhook[]>(path(workspaceId), { signal: params.signal, params: { page: 1, page_size: 50 } })).data,
  create: async (workspaceId: number, payload: WorkspaceWebhookInput) =>
    (await apiClient.post<WorkspaceWebhookSecretResponse | { webhook: WorkspaceWebhook; secret?: string }>(path(workspaceId), payload)).data,
  update: async (workspaceId: number, webhookId: number, payload: WorkspaceWebhookUpdateInput) =>
    (await apiClient.patch<WorkspaceWebhook>(path(workspaceId, webhookId), payload)).data,
  remove: async (workspaceId: number, webhookId: number) => {
    await apiClient.delete(path(workspaceId, webhookId))
  },
  rotateSecret: async (workspaceId: number, webhookId: number) =>
    (await apiClient.post<WorkspaceWebhookSecretResponse | { webhook: WorkspaceWebhook; secret?: string }>(`${path(workspaceId, webhookId)}/rotate`)).data,
  test: async (workspaceId: number, webhookId: number) =>
    (await apiClient.post<WorkspaceWebhookDelivery>(`${path(workspaceId, webhookId)}/test`)).data,
  listDeliveries: async (workspaceId: number, webhookId: number, params: { page?: number; page_size?: number; signal?: AbortSignal } = {}) =>
    (await apiClient.get<WorkspaceWebhookDeliveryPage>(`${path(workspaceId, webhookId)}/deliveries`, {
      signal: params.signal,
      params: { page: params.page ?? 1, page_size: params.page_size ?? 20 },
    })).data,
  retry: async (workspaceId: number, webhookId: number, deliveryId: number) =>
    (await apiClient.post<WorkspaceWebhookDelivery>(`${path(workspaceId, webhookId)}/deliveries/${deliveryId}/retry`)).data,
}

export default webhooksAPI
