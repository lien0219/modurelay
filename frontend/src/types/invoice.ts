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
