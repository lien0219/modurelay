import { apiClient } from './client'

export interface SMSServiceItem { code: string; name: string; icon?: string; category?: string; description?: string }
export interface SMSCountryItem { iso2: string; iso3?: string; calling_code?: string; name_zh?: string; name_en?: string }
export interface SMSCapabilities { supports_temporary: boolean; supports_rental: boolean; supports_webhook: boolean; supports_polling: boolean; supports_cancel: boolean; supports_refund: boolean; supports_refund_status: boolean; supports_extend: boolean; supports_resend: boolean; supports_voice: boolean; supports_operator_selection: boolean; supports_service_selection: boolean }
export interface SMSQuote { channel_code: string; public_name: string; channel_role: string; sale_price: number; stock: number; success_rate?: number; success_rate_grade?: string; success_rate_source: string; estimated_delivery_seconds: number; capabilities: SMSCapabilities; quote_id: string; quote_expires_at: string }
export interface SMSMessage { id: number; message_text: string; verification_code?: string; received_at: string }
export interface SMSOrder { id: string; product_type: 'temporary' | 'rental'; status: string; channel_code: string; channel_name: string; service_code: string; country_code: string; phone_number?: string; price: number; success_rate?: number; success_rate_grade?: string; success_rate_source: string; refund_status: string; refund_reason?: string; messages?: SMSMessage[]; expires_at?: string; created_at: string }
export interface SMSOrderPage { items: SMSOrder[]; total: number; page: number; page_size: number; pages: number }

export const smsAPI = {
  services: () => apiClient.get<SMSServiceItem[]>('/sms/services').then(r => r.data),
  countries: () => apiClient.get<SMSCountryItem[]>('/sms/countries').then(r => r.data),
  quotes: (params: { service: string; country: string; product_type: 'temporary' | 'rental' }) => apiClient.get<SMSQuote[]>('/sms/quotes', { params }).then(r => r.data),
  orders: (params: { page: number; page_size: number; keyword?: string; status?: string }) => apiClient.get<SMSOrderPage>('/sms/orders', { params }).then(r => r.data),
  order: (id: string) => apiClient.get<SMSOrder>(`/sms/orders/${encodeURIComponent(id)}`).then(r => r.data),
  purchase: (payload: { channel_code: string; service_code: string; country_code: string; product_type: 'temporary' | 'rental'; duration_value?: number; duration_unit?: string; quote_id: string; expected_price: number }, idempotencyKey: string) => apiClient.post<SMSOrder>('/sms/orders', payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  cancel: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/cancel`).then(r => r.data),
  refund: (id: string) => apiClient.post(`/sms/orders/${encodeURIComponent(id)}/refund`).then(r => r.data),
  refundStatus: (id: string) => apiClient.get<{ status: string; reason?: string }>(`/sms/orders/${encodeURIComponent(id)}/refund-status`).then(r => r.data),
  extendRental: (id: string, payload: { duration_value: number; duration_unit: string }, idempotencyKey: string) => apiClient.post<SMSOrder>(`/sms/rentals/${encodeURIComponent(id)}/extend`, payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
}
