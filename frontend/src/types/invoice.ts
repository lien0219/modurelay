import type { BasePaginationResponse } from '@/types'
import type { PaymentOrder } from '@/types/payment'

export type InvoiceStatus = 'pending' | 'processing' | 'issued' | 'rejected' | 'cancelled'

export interface InvoiceApplicationInput {
  amount_cents: number
  invoice_type: 'vat_normal' | 'vat_special'
  title_type: 'personal' | 'enterprise'
  title: string
  tax_number: string
  email: string
  address: string
  phone: string
  bank_name: string
  bank_account: string
  remark: string
  contact: string
  requested_issue_date: string
}

export interface InvoiceDocument {
  filename: string
  size_bytes: number
  uploaded_at: string
  sha256: string
}

export interface InvoiceApplication extends InvoiceApplicationInput {
  id: number
  application_no: string
  user_id: number
  user_email: string
  user_name: string
  status: InvoiceStatus
  admin_remark: string
  invoice_number: string
  created_at: string
  updated_at: string
  processed_at: string | null
  document: InvoiceDocument | null
}

export interface InvoiceEligibility {
  enabled: boolean
  minimum_amount_cents: number
  total_recharged_cents: number
  reserved_amount_cents: number
  available_amount_cents: number
}

export interface InvoiceListParams {
  page?: number
  page_size?: number
  status?: InvoiceStatus | ''
  search?: string
}

export interface InvoiceUpdateInput {
  status: 'processing' | 'issued' | 'rejected'
  admin_remark: string
  invoice_number: string
}

export const INVOICE_EXPORT_COLUMNS = [
  'application_no', 'user_id', 'user_email', 'user_name', 'contact', 'amount',
  'invoice_type', 'title_type', 'title', 'tax_number', 'email', 'address', 'phone',
  'bank_name', 'bank_account', 'remark', 'status', 'admin_remark', 'invoice_number',
  'requested_issue_date', 'created_at', 'processed_at',
] as const
export type InvoiceExportColumn = typeof INVOICE_EXPORT_COLUMNS[number]
export interface InvoiceExportInput extends Pick<InvoiceListParams, 'status' | 'search'> {
  ids?: number[]
  columns?: InvoiceExportColumn[]
}
export interface InvoiceAccount {
  user: { id: number; email: string; username: string; balance: number }
  orders: BasePaginationResponse<PaymentOrder>
}
