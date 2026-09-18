import { apiClient } from '../client'
import type { SMSCapabilities, SMSOrder } from '../sms'
export interface SMSPricingSettings { cost_multiplier: number; fixed_markup: number; unknown_grade_multiplier: number; unknown_grade_fixed_markup: number; temporary_expiry_minutes: number; self_service_cancel_after_minutes: number; grade_multipliers: Record<string, number>; grade_fixed_markups: Record<string, number> }

export interface SMSProviderAdmin { id: number; code: string; name: string; base_url: string; health_status: string; enabled: boolean; credential_configured: boolean; credential_ref?: string; capabilities: SMSCapabilities }
export interface SMSChannelAdmin { id: number; code: string; public_name: string; role: string; provider_code: string; provider_id?: number; enabled: boolean; visible: boolean; healthy: boolean; sort_order: number }
export interface SMSProviderMappingAdmin { kind: 'service' | 'country'; target_id: number; internal_code: string; internal_name: string; provider_code: string; provider_name?: string; temporary_supported?: boolean; rental_supported?: boolean; enabled: boolean }
export interface SMSProviderMappingPage { items: SMSProviderMappingAdmin[]; total: number; page: number; page_size: number; pages: number }
const smsAdminAPI = {
  providers: () => apiClient.get<SMSProviderAdmin[]>('/admin/sms/providers').then(r => r.data),
  updateProvider: (id: number, payload: { enabled: boolean; base_url?: string; credential_ref?: string }) => apiClient.put(`/admin/sms/providers/${id}`, payload).then(r => r.data),
  testProvider: (id: number) => apiClient.post<{ healthy: boolean; health_status: string; latency_ms: number }>(`/admin/sms/providers/${id}/test`).then(r => r.data),
  providerMappings: (id: number, params: { kind: 'service' | 'country'; page: number; page_size: number; keyword?: string }) => apiClient.get<SMSProviderMappingPage>(`/admin/sms/providers/${id}/mappings`, { params }).then(r => r.data),
  updateProviderServiceMapping: (providerId: number, serviceId: number, payload: { provider_code: string; provider_name?: string; temporary_supported: boolean; rental_supported: boolean; enabled: boolean }) => apiClient.put(`/admin/sms/providers/${providerId}/service-mappings/${serviceId}`, payload).then(r => r.data),
  updateProviderCountryMapping: (providerId: number, countryId: number, payload: { provider_country_id: string; provider_country_code?: string; enabled: boolean }) => apiClient.put(`/admin/sms/providers/${providerId}/country-mappings/${countryId}`, payload).then(r => r.data),
  channels: () => apiClient.get<SMSChannelAdmin[]>('/admin/sms/channels').then(r => r.data),
  updateChannel: (id: number, payload: { enabled: boolean; visible: boolean; healthy: boolean; provider_id?: number }) => apiClient.put(`/admin/sms/channels/${id}`, payload).then(r => r.data),
  stats: () => apiClient.get<Record<string, number | boolean>>('/admin/sms/stats').then(r => r.data),
  orders: () => apiClient.get<SMSOrder[]>('/admin/sms/orders').then(r => r.data),
  setEnabled: (enabled: boolean) => apiClient.put('/admin/sms/settings', { enabled }).then(r => r.data as { enabled: boolean }),
  pricing: () => apiClient.get<SMSPricingSettings>('/admin/sms/pricing').then(r => r.data),
  updatePricing: (payload: SMSPricingSettings) => apiClient.put<SMSPricingSettings>('/admin/sms/pricing', payload).then(r => r.data),
}
export default smsAdminAPI
