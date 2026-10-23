import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type {
  InvoiceApplication, InvoiceApplicationInput, InvoiceEligibility,
  InvoiceListParams, InvoiceUpdateInput,
} from '@/types/invoice'

export const invoicesAPI = {
  eligibility: () => apiClient.get<InvoiceEligibility>('/invoices/eligibility'),
  list: (params: Pick<InvoiceListParams, 'page' | 'page_size'>) =>
    apiClient.get<BasePaginationResponse<InvoiceApplication>>('/invoices', { params }),
  create: (data: InvoiceApplicationInput) => apiClient.post<InvoiceApplication>('/invoices', data),
  detail: (id: number) => apiClient.get<InvoiceApplication>(`/invoices/${id}`),
  cancel: (id: number) => apiClient.post<InvoiceApplication>(`/invoices/${id}/cancel`),
}

export const adminInvoicesAPI = {
  config: () => apiClient.get<{ enabled: boolean }>('/admin/invoices/config'),
  updateConfig: (enabled: boolean) => apiClient.put<{ enabled: boolean }>('/admin/invoices/config', { enabled }),
  list: (params: InvoiceListParams) => apiClient.get<BasePaginationResponse<InvoiceApplication>>('/admin/invoices', { params }),
  detail: (id: number) => apiClient.get<InvoiceApplication>(`/admin/invoices/${id}`),
  update: (id: number, data: InvoiceUpdateInput) => apiClient.patch<InvoiceApplication>(`/admin/invoices/${id}`, data),
  async export(params: Pick<InvoiceListParams, 'status' | 'search'>) {
    const response = await apiClient.get<Blob>('/admin/invoices/export', {
      params,
      responseType: 'blob',
      // Decode export-limit JSON ourselves; keep authentication and other failures
      // on the shared client's normal error/refresh path.
      validateStatus: status => (status >= 200 && status < 300) || status === 400,
    })
    if (response.status === 400) {
      let payload: Record<string, unknown> = {}
      try {
        const value: unknown = JSON.parse(await response.data.text())
        if (value && typeof value === 'object' && !Array.isArray(value)) payload = value as Record<string, unknown>
      } catch { /* Fall back to the view's localized export error. */ }
      throw { status: response.status, code: payload.code, reason: payload.reason, message: typeof payload.message === 'string' ? payload.message : undefined }
    }
    return response
  },
}
