import { apiClient } from '../client'
import type { EmailCapabilities } from '../email'

export interface EmailProviderAdmin {
  id: number; code: string; name: string; base_url: string; health_status: string; enabled: boolean
  credential_configured: boolean; masked_hint?: string; billing?: Record<string, unknown>
  portal_url?: string; today_requests?: number; month_requests?: number
}
export interface EmailChannelAdmin {
  id: number; code: string; public_name: string; provider_name: string; enabled: boolean; visible: boolean; healthy: boolean
  email_type: string; privacy_level: string; sale_price: number; refund_policy: string; capture_policy: string; order_ttl_seconds: number
  base_markup: number; fixed_markup: number; minimum_profit: number
  polling_backoff?: number[]; max_provider_requests_per_order?: number
}
export interface EmailOrderAdmin {
  id: string; order_no: string; user_id: number; service_code: string; channel_code: string; channel_name: string
  provider_code: string; provider_inbox_id: string; email_address: string; address_type: string; status: string
  sale_price: number; provider_request_count: number; refund_status: string; created_at: string; expires_at?: string
}
const emailAdminAPI = {
  providers: () => apiClient.get<EmailProviderAdmin[]>('/admin/email/providers').then(r => r.data),
  updateProvider: (id: number, payload: { enabled: boolean; base_url?: string; credential_ref?: string; billing?: Record<string, unknown> }) => apiClient.put(`/admin/email/providers/${id}`, payload).then(r => r.data),
  testProvider: (id: number) => apiClient.post<{ healthy: boolean; health_status: string; latency_ms: number }>(`/admin/email/providers/${id}/test`).then(r => r.data),
  channels: () => apiClient.get<EmailChannelAdmin[]>('/admin/email/channels').then(r => r.data),
  orders: () => apiClient.get<EmailOrderAdmin[]>('/admin/email/orders').then(r => r.data),
  updateChannel: (id: number, payload: { enabled: boolean; visible: boolean; healthy: boolean; sale_price: number; refund_policy: string; capture_policy: string; base_markup?: number; fixed_markup?: number; minimum_profit?: number; order_ttl_seconds?: number; max_provider_requests_per_order?: number; polling_backoff?: number[] }) => apiClient.put(`/admin/email/channels/${id}`, payload).then(r => r.data),
  stats: () => apiClient.get<Record<string, number | boolean | null>>('/admin/email/stats').then(r => r.data),
  setEnabled: (enabled: boolean) => apiClient.put('/admin/email/settings', { enabled }).then(r => r.data as { enabled: boolean }),
}
export default emailAdminAPI
export type { EmailCapabilities }
