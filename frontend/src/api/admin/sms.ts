import { apiClient } from '../client'
import type { SMSCapabilities, SMSOrder } from '../sms'

export interface SMSProviderAdmin { id: number; code: string; name: string; base_url: string; health_status: string; enabled: boolean; credential_configured: boolean; credential_ref?: string; capabilities: SMSCapabilities }
export interface SMSChannelAdmin { id: number; code: string; public_name: string; role: string; provider_code: string; provider_id?: number; enabled: boolean; visible: boolean; healthy: boolean; sort_order: number }
const smsAdminAPI = {
  providers: () => apiClient.get<SMSProviderAdmin[]>('/admin/sms/providers').then(r => r.data),
  updateProvider: (id: number, payload: { enabled: boolean; base_url?: string; credential_ref?: string }) => apiClient.put(`/admin/sms/providers/${id}`, payload).then(r => r.data),
  testProvider: (id: number) => apiClient.post<{ healthy: boolean; health_status: string; latency_ms: number }>(`/admin/sms/providers/${id}/test`).then(r => r.data),
  channels: () => apiClient.get<SMSChannelAdmin[]>('/admin/sms/channels').then(r => r.data),
  updateChannel: (id: number, payload: { enabled: boolean; visible: boolean; healthy: boolean; provider_id?: number }) => apiClient.put(`/admin/sms/channels/${id}`, payload).then(r => r.data),
  stats: () => apiClient.get<Record<string, number | boolean>>('/admin/sms/stats').then(r => r.data),
  orders: () => apiClient.get<SMSOrder[]>('/admin/sms/orders').then(r => r.data),
  setEnabled: (enabled: boolean) => apiClient.put('/admin/sms/settings', { enabled }).then(r => r.data as { enabled: boolean }),
}
export default smsAdminAPI
