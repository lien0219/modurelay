import { apiClient } from './client'

export type VerificationType = 'sms' | 'email'
export type VerificationOutcome = 'processing' | 'success' | 'failed' | 'refunded' | 'cancelled' | 'expired'

export interface VerificationRecord {
  id: string
  order_no: string
  verification_type: VerificationType
  product_type: string
  user_id?: number
  user_email?: string
  service_code: string
  channel_code: string
  channel_name: string
  provider_code?: string
  target: string
  region?: string
  status: string
  outcome: VerificationOutcome
  refund_status: string
  refund_reason?: string
  sale_amount: number
  provider_cost?: number
  provider_cost_estimated?: boolean
  settlement_estimated?: boolean
  user_debit_amount: number
  reserved_amount: number
  captured_amount: number
  released_amount: number
  refunded_amount: number
  currency: string
  provider_request_count?: number
  error_code?: string
  error_message?: string
  public_error_message?: string
  created_at: string
  updated_at: string
  completed_at?: string
  expires_at?: string
}

export interface VerificationRecordSummary {
  total: number
  processing: number
  success: number
  failed: number
  refunded: number
  cancelled: number
  expired: number
  sale_amount: number
  user_debit_amount: number
  reserved_amount: number
  captured_amount: number
  released_amount: number
  refunded_amount: number
  provider_cost?: number
  net_revenue: number
  estimated_profit?: number
}

export interface VerificationRecordBreakdown {
  key: string
  total: number
  success: number
  success_rate: number
}

export interface VerificationRecordFinancialTotal {
  currency: string
  sale_amount: number
  provider_cost: number
  captured_amount: number
  refunded_amount: number
  net_revenue: number
  estimated_profit: number
  estimated: boolean
}

export interface VerificationRecordAnalytics {
  by_platform: VerificationRecordBreakdown[]
  by_country: VerificationRecordBreakdown[]
  by_type: VerificationRecordBreakdown[]
  financial: VerificationRecordFinancialTotal[]
}

export interface VerificationRecordResponse {
  items: VerificationRecord[]
  total: number
  page: number
  page_size: number
  pages: number
  summary: VerificationRecordSummary
  analytics?: VerificationRecordAnalytics
}

export interface VerificationRecordQuery {
  page?: number
  page_size?: number
  type?: VerificationType | ''
  outcome?: VerificationOutcome | ''
  platform?: string
  country?: string
  keyword?: string
  created_from?: string
  created_to?: string
}

export interface VerificationRecordOption {
  value: string
  label: string
  label_en?: string
  icon?: string
}

export interface VerificationRecordOptionPage {
  items: VerificationRecordOption[]
  total: number
  page: number
  page_size: number
  pages: number
}

export async function listVerificationRecords(
  params: VerificationRecordQuery,
  admin: boolean,
): Promise<VerificationRecordResponse> {
  const endpoint = admin ? '/admin/verification-records' : '/verification-records'
  const { data } = await apiClient.get<VerificationRecordResponse>(endpoint, { params })
  return data
}

export async function listVerificationRecordOptions(
  params: { kind: 'platform' | 'country'; type?: VerificationType | ''; query?: string; page?: number; page_size?: number },
  admin: boolean,
): Promise<VerificationRecordOptionPage> {
  const endpoint = admin ? '/admin/verification-records/options' : '/verification-records/options'
  const { data } = await apiClient.get<VerificationRecordOptionPage>(endpoint, { params })
  return data
}

export const verificationRecordsAPI = { list: listVerificationRecords, options: listVerificationRecordOptions }

export default verificationRecordsAPI
