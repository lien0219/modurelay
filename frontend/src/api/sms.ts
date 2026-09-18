import { apiClient } from './client'

export interface SMSServiceItem { code: string; name: string; icon?: string; category?: string; description?: string; provider_code?: string; stock?: number; provider_cost?: number; available?: boolean }
export interface SMSCatalogPage<T> { items: T[]; total: number; page: number; page_size: number; pages: number; has_more: boolean; catalog_stale?: boolean; catalog_observed_at?: string }
export interface SMSProviderItem { code: string; name: string; beta: boolean; selectable: boolean; capabilities: SMSCapabilities }
export interface SMSCountryItem { iso2: string; iso3?: string; calling_code?: string; name_zh?: string; name_en?: string; provider_code?: string; stock?: number; provider_cost?: number; conversion_rate?: number; available?: boolean }
export interface SMSCapabilities { supports_temporary: boolean; supports_rental: boolean; supports_rental_cancel: boolean; supports_webhook: boolean; supports_polling: boolean; supports_cancel: boolean; supports_refund: boolean; supports_refund_status: boolean; supports_finish: boolean; supports_ban: boolean; supports_extend: boolean; supports_resend: boolean; supports_voice: boolean; supports_voice_sms: boolean; supports_voice_caller_id: boolean; supports_voice_call: boolean; supports_operator_selection: boolean; supports_service_selection: boolean; supports_conversion_stats: boolean }
export interface SMSQuote { channel_code: string; public_name: string; channel_role: string; sale_price: number; stock: number; success_rate?: number; success_rate_grade?: string; success_rate_source: string; estimated_delivery_seconds: number; capabilities: SMSCapabilities; quote_id: string; quote_expires_at: string }
export interface SMSOperatorItem { code: string; name: string; stock?: number; provider_cost?: number; provider_rate?: number; available: boolean }
export interface SMSMessage { id: number; message_text: string; verification_code?: string; received_at: string }
export interface SMSOrder { id: string; product_type: 'temporary' | 'rental'; status: string; channel_code: string; channel_name: string; service_code: string; country_code: string; phone_number?: string; operator_code?: string; voice_mode?: number; price: number; success_rate?: number; success_rate_grade?: string; success_rate_source: string; refund_status: string; refund_reason?: string; capabilities?: SMSCapabilities; messages?: SMSMessage[]; expires_at?: string; remaining_seconds?: number; created_at: string }
export interface SMSOrderPage { items: SMSOrder[]; total: number; page: number; page_size: number; pages: number }

export const smsAPI = {
  providers: () => apiClient.get<SMSProviderItem[]>('/sms/providers').then(r => r.data),
  providerServices: (provider: string, params?: { product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSServiceItem[]>(`/sms/providers/${encodeURIComponent(provider)}/services`, { params }).then(r => r.data),
  providerServicesPage: (provider: string, params: { page: number; page_size: number; keyword?: string; product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSCatalogPage<SMSServiceItem>>(`/sms/providers/${encodeURIComponent(provider)}/services`, { params }).then(r => r.data),
  services: () => apiClient.get<SMSServiceItem[]>('/sms/services').then(r => r.data),
  countries: () => apiClient.get<SMSCountryItem[]>('/sms/countries').then(r => r.data),
  serviceCountries: (provider: string, service: string, params?: { product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSCountryItem[]>(`/sms/providers/${encodeURIComponent(provider)}/services/${encodeURIComponent(service)}/countries`, { params }).then(r => r.data),
  serviceCountriesPage: (provider: string, service: string, params: { page: number; page_size: number; keyword?: string }) => apiClient.get<SMSCatalogPage<SMSCountryItem>>(`/sms/providers/${encodeURIComponent(provider)}/services/${encodeURIComponent(service)}/countries`, { params }).then(r => r.data),
  operators: (provider: string, service: string, country: string, params?: { voice_mode?: number; product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSOperatorItem[]>(`/sms/providers/${encodeURIComponent(provider)}/services/${encodeURIComponent(service)}/countries/${encodeURIComponent(country)}/operators`, { params }).then(r => r.data),
  quotes: (params: { provider?: string; service: string; country: string; product_type: 'temporary' | 'rental'; operator?: string; voice_mode?: number; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSQuote[]>('/sms/quotes', { params }).then(r => r.data),
  orders: (params: { page: number; page_size: number; keyword?: string; status?: string }) => apiClient.get<SMSOrderPage>('/sms/orders', { params }).then(r => r.data),
  order: (id: string) => apiClient.get<SMSOrder>(`/sms/orders/${encodeURIComponent(id)}`).then(r => r.data),
  purchase: (payload: { channel_code: string; service_code: string; country_code: string; product_type: 'temporary' | 'rental'; operator_code?: string; voice_mode?: number; duration_value?: number; duration_unit?: string; quote_id: string; expected_price: number }, idempotencyKey: string) => apiClient.post<SMSOrder>('/sms/orders', payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  purchaseBatch: (payload: { items: Array<{ channel_code: string; service_code: string; country_code: string; product_type: 'temporary' | 'rental'; operator_code?: string; voice_mode?: number; duration_value?: number; duration_unit?: string; quote_id: string; expected_price: number }> }, idempotencyKey: string) => apiClient.post<{ items: SMSOrder[]; partial_error?: string }>('/sms/orders/batch', payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  cancel: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/cancel`).then(r => r.data),
  finish: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/finish`).then(r => r.data),
  resend: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/resend`).then(r => r.data),
  ban: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/ban`).then(r => r.data),
  refund: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/refund`).then(r => r.data),
  refundStatus: (id: string) => apiClient.get<{ status: string; reason?: string }>(`/sms/orders/${encodeURIComponent(id)}/refund-status`).then(r => r.data),
  extendRental: (id: string, payload: { duration_value: number; duration_unit: string }, idempotencyKey: string) => apiClient.post<SMSOrder>(`/sms/rentals/${encodeURIComponent(id)}/extend`, payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
}
