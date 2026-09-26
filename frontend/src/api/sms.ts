import { apiClient } from './client'

export interface SMSServiceItem { code: string; name: string; icon?: string; category?: string; description?: string; stock?: number; starting_price?: number; available?: boolean }
export interface SMSCatalogPage<T> { items: T[]; total: number; page: number; page_size: number; pages: number; has_more: boolean; catalog_stale?: boolean; catalog_observed_at?: string }
export interface SMSProviderItem { code: string; name: string; beta: boolean; selectable: boolean; capabilities: SMSCapabilities }
export interface SMSCountryItem { iso2: string; iso3?: string; calling_code?: string; name_zh?: string; name_en?: string; stock?: number; starting_price?: number; conversion_rate?: number; platform_30d_success_rate?: number; platform_30d_sample_size?: number; platform_30d_successes?: number; platform_30d_failures?: number; recommended_operator?: string; recommended_operator_stock?: number; recommended_starting_price?: number; available?: boolean }
export interface SMSCapabilities { supports_temporary: boolean; supports_rental: boolean; supports_rental_constraints?: boolean; supports_rental_restore?: boolean; supports_rental_multi_service?: boolean; supports_rental_add_service?: boolean; supports_rental_cancel: boolean; supports_webhook: boolean; supports_polling: boolean; supports_cancel: boolean; supports_refund: boolean; supports_refund_status: boolean; supports_finish: boolean; supports_ban: boolean; supports_extend: boolean; supports_resend: boolean; supports_voice: boolean; supports_voice_sms: boolean; supports_voice_caller_id: boolean; supports_voice_call: boolean; supports_operator_selection: boolean; supports_service_selection: boolean; supports_conversion_stats: boolean }
export interface SMSQuote { channel_code: string; public_name: string; channel_role: string; sale_price: number; stock: number; success_rate?: number; success_rate_grade?: string; success_rate_source: string; success_rate_sample_size?: number; estimated_delivery_seconds: number; capabilities: SMSCapabilities; quote_id: string; quote_expires_at: string }
export interface SMSOperatorItem { code: string; name: string; stock?: number; source_type?: 'native' | 'donor' | 'unknown'; provider_rate?: number; platform_30d_success_rate?: number; platform_30d_sample_size?: number; platform_30d_successes?: number; platform_30d_failures?: number; available: boolean }
export interface SMSMessage { id: number; message_text: string; verification_code?: string; sender?: string; provider_received_at?: string; message_type?: string; service_code?: string; other_sms?: boolean; received_at: string }
export interface SMSOrder {
  id: string
  product_type: 'temporary' | 'rental'
  status: string
  reconciliation_action?: string
  channel_code: string
  channel_name: string
  service_code: string
  country_code: string
  calling_code?: string
  phone_number?: string
  operator_code?: string
  voice_mode?: number
  price: number
  success_rate?: number
  success_rate_grade?: string
  success_rate_source: string
  refund_status: string
  refund_reason?: string
  capabilities?: SMSCapabilities
  messages?: SMSMessage[]
  latest_verification_code?: string
  expires_at?: string
  remaining_seconds?: number
  /** Explicit cancellation policy fields, supported by newer API builds. */
  cancel_available_at?: string
  cancellation_available_at?: string
  cancel_after?: string
  cancel_after_seconds?: number
  cancel_remaining_seconds?: number
  cancellation_remaining_seconds?: number
  can_cancel?: boolean
  cancellation_available?: boolean
  created_at: string
}
export interface SMSOrderPage { items: SMSOrder[]; total: number; page: number; page_size: number; pages: number }
export interface SMSRecentSuccessItem { username: string; country_code: string; phone: string }
export interface SMSRecentSuccessFeed { source: 'mock' | 'real'; real_success_count: number; items: SMSRecentSuccessItem[] }
export interface SMSRentalConstraints { can_extend: boolean; can_prolong_max?: number; current_until?: string; can_prolong_until?: string; last_online?: string }
export interface SMSRentalServiceOption { code: string; name: string; icon?: string; stock: number; rent_days: number; sale_price: number }
export interface SMSRentalServiceQuote { quote_id: string; service_code: string; rent_days: number; sale_price: number; expires_at: string }
export interface SMSRentalRestoreQuote { quote_id: string; service_code: string; country_code: string; phone_number: string; duration_days: number; sale_price: number; expires_at: string }
export interface SMSSettings { batch_purchase_limit: number }

export const smsAPI = {
  settings: () => apiClient.get<SMSSettings>('/sms/settings').then(r => r.data),
  providers: () => apiClient.get<SMSProviderItem[]>('/sms/providers').then(r => r.data),
  recentSuccesses: () => apiClient.get<SMSRecentSuccessFeed>('/sms/recent-successes').then(r => r.data),
  providerServices: (provider: string, params?: { product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSServiceItem[]>(`/sms/providers/${encodeURIComponent(provider)}/services`, { params }).then(r => r.data),
  providerServicesPage: (provider: string, params: { page: number; page_size: number; keyword?: string; product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSCatalogPage<SMSServiceItem>>(`/sms/providers/${encodeURIComponent(provider)}/services`, { params }).then(r => r.data),
  services: () => apiClient.get<SMSServiceItem[]>('/sms/services').then(r => r.data),
  countries: () => apiClient.get<SMSCountryItem[]>('/sms/countries').then(r => r.data),
  serviceCountries: (provider: string, service: string, params?: { product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSCountryItem[]>(`/sms/providers/${encodeURIComponent(provider)}/services/${encodeURIComponent(service)}/countries`, { params }).then(r => r.data),
  serviceCountriesPage: (provider: string, service: string, params: { page: number; page_size: number; keyword?: string; sort?: 'recommended' | 'platform' | 'rate' | 'price' | 'stock' | 'name'; product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSCatalogPage<SMSCountryItem>>(`/sms/providers/${encodeURIComponent(provider)}/services/${encodeURIComponent(service)}/countries`, { params }).then(r => r.data),
  operators: (provider: string, service: string, country: string, params?: { voice_mode?: number; product_type?: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSOperatorItem[]>(`/sms/providers/${encodeURIComponent(provider)}/services/${encodeURIComponent(service)}/countries/${encodeURIComponent(country)}/operators`, { params }).then(r => r.data),
  quotes: (params: { provider?: string; service: string; services?: string; country: string; product_type: 'temporary' | 'rental'; operator?: string; voice_mode?: number; duration_value?: number; duration_unit?: string }) => apiClient.get<SMSQuote[]>('/sms/quotes', { params }).then(r => r.data),
  orders: (params: { page: number; page_size: number; keyword?: string; status?: string }) => apiClient.get<SMSOrderPage>('/sms/orders', { params }).then(r => r.data),
  order: (id: string) => apiClient.get<SMSOrder>(`/sms/orders/${encodeURIComponent(id)}`).then(r => r.data),
  purchase: (payload: { channel_code: string; service_code: string; service_codes?: string[]; country_code: string; product_type: 'temporary' | 'rental'; operator_code?: string; voice_mode?: number; duration_value?: number; duration_unit?: string; quote_id: string; expected_price: number }, idempotencyKey: string) => apiClient.post<SMSOrder>('/sms/orders', payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  purchaseBatch: (payload: { items: Array<{ channel_code: string; service_code: string; service_codes?: string[]; country_code: string; product_type: 'temporary' | 'rental'; operator_code?: string; voice_mode?: number; duration_value?: number; duration_unit?: string; quote_id: string; expected_price: number }> }, idempotencyKey: string) => apiClient.post<{ items: SMSOrder[]; partial_error?: string }>('/sms/orders/batch', payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  cancel: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/cancel`).then(r => r.data),
  resend: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/resend`).then(r => r.data),
  refund: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/refund`).then(r => r.data),
  refundStatus: (id: string) => apiClient.get<{ status: string; reason?: string }>(`/sms/orders/${encodeURIComponent(id)}/refund-status`).then(r => r.data),
  rentalConstraints: (id: string) => apiClient.get<SMSRentalConstraints>(`/sms/rentals/${encodeURIComponent(id)}/constraints`).then(r => r.data),
  rentalServiceOptions: (id: string, rentDays = 7) => apiClient.get<SMSRentalServiceOption[]>(`/sms/rentals/${encodeURIComponent(id)}/services`, { params: { rent_days: rentDays } }).then(r => r.data),
  rentalServiceQuote: (id: string, payload: { service_code: string; rent_days: number }) => apiClient.post<SMSRentalServiceQuote>(`/sms/rentals/${encodeURIComponent(id)}/services/quotes`, payload).then(r => r.data),
  addRentalService: (id: string, payload: { quote_id: string; expected_price: number }, idempotencyKey: string) => apiClient.post<SMSOrder>(`/sms/rentals/${encodeURIComponent(id)}/services`, payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  rentalRestoreQuote: (id: string) => apiClient.post<SMSRentalRestoreQuote>(`/sms/rentals/${encodeURIComponent(id)}/restore-quote`).then(r => r.data),
  restoreRental: (id: string, payload: { quote_id: string; expected_price: number }, idempotencyKey: string) => apiClient.post<SMSOrder>(`/sms/rentals/${encodeURIComponent(id)}/restore`, payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  extendRental: (id: string, payload: { duration_value: number; duration_unit: string }, idempotencyKey: string) => apiClient.post<SMSOrder>(`/sms/rentals/${encodeURIComponent(id)}/extend`, payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
}
