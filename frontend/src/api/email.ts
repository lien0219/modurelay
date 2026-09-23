import { apiClient } from './client'

export interface EmailServiceItem { code: string; name: string }
export interface EmailCapabilities {
  supports_temporary_inbox: boolean
  supports_generate_single: boolean
  supports_generate_bulk?: boolean
  supports_inbox_list: boolean
  supports_message_read: boolean
  supports_message_delete?: boolean
  supports_polling: boolean
  supports_verification_code: boolean
  supports_verification_url: boolean
  supports_html_message: boolean
  supports_webhook?: boolean
  supports_private_inbox?: boolean
  supports_persistent_inbox?: boolean
}
export interface EmailQuote {
  channel_code: string
  public_name: string
  email_type: string
  privacy_level: string
  sale_price: number
  success_rate?: number
  success_rate_grade?: string
  success_rate_sample_count: number
  estimated_delivery_seconds: number
  retention_description: string
  refund_policy_description: string
  quote_id: string
  quote_expires_at: string
  capabilities: EmailCapabilities
}
export interface EmailMessage {
  id: string
  from_address: string
  from_name: string
  to_address: string
  subject: string
  text_body: string
  html_body?: string
  verification_code?: string
  verification_url?: string
  verification_confidence?: number
  verification_method?: string
  received_at: string
}
export interface EmailOrder {
  id: string
  order_no: string
  service_code: string
  channel_code: string
  channel_name: string
  email_address: string
  address_type: string
  price: number
  status: string
  success_rate?: number
  success_rate_grade?: string
  refund_policy: string
  capture_policy: string
  expires_at?: string
  created_at: string
  first_message_at?: string
  messages?: EmailMessage[]
  refund_status: string
  refund_reason?: string
  error_message?: string
}
export interface EmailOrderPage { items: EmailOrder[]; total: number; page: number; page_size: number; pages: number }

export const emailAPI = {
  services: () => apiClient.get<EmailServiceItem[]>('/email/services').then(r => r.data),
  quotes: (params: { service?: string; address_type?: string }) => apiClient.get<EmailQuote[]>('/email/quotes', { params }).then(r => r.data),
  orders: (params: { page: number; page_size: number; keyword?: string; status?: string }) => apiClient.get<EmailOrderPage>('/email/orders', { params }).then(r => r.data),
  order: (id: string) => apiClient.get<EmailOrder>(`/email/orders/${encodeURIComponent(id)}`).then(r => r.data),
  purchase: (payload: { channel_code: string; service_code?: string; address_type: string; expected_price: number; quote_id: string }, idempotencyKey: string) => apiClient.post<EmailOrder>('/email/orders', payload, { headers: { 'Idempotency-Key': idempotencyKey } }).then(r => r.data),
  cancel: (id: string) => apiClient.post<{ status: string }>(`/email/orders/${encodeURIComponent(id)}/cancel`).then(r => r.data),
  requestRefund: (id: string) => apiClient.post<{ status: string }>(`/email/orders/${encodeURIComponent(id)}/refund`).then(r => r.data),
  refundStatus: (id: string) => apiClient.get<{ status: string; reason?: string }>(`/email/orders/${encodeURIComponent(id)}/refund-status`).then(r => r.data),
}
