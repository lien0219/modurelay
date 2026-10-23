import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminInvoicesAPI, invoicesAPI } from '../invoices'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { get, post } }))

describe('invoice document API', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses authenticated API requests for private preview and download, without tokens in URLs', async () => {
    await invoicesAPI.document(5)
    expect(get).toHaveBeenLastCalledWith('/invoices/5/document', { responseType: 'blob', params: undefined })
    await adminInvoicesAPI.document(5, true)
    expect(get).toHaveBeenLastCalledWith('/admin/invoices/5/document', { responseType: 'blob', params: { download: 1 } })
  })

  it('uploads the PDF as a multipart file', async () => {
    const file = new File(['%PDF-1.4'], 'invoice.pdf', { type: 'application/pdf' })
    await adminInvoicesAPI.uploadDocument(5, file)
    const [url, form, config] = post.mock.calls[0]
    expect(url).toBe('/admin/invoices/5/document')
    expect(form).toBeInstanceOf(FormData)
    expect(form.get('file')).toBe(file)
    expect(config.headers['Content-Type']).toBe('multipart/form-data')
  })

  it('posts explicit export IDs and columns without silently adding filters', async () => {
    post.mockResolvedValue({ status: 200, data: new Blob(['CSV']) })
    await adminInvoicesAPI.export({ ids: [5, 6], columns: ['user_email', 'amount'] })
    expect(post).toHaveBeenCalledWith('/admin/invoices/export', { ids: [5, 6], columns: ['user_email', 'amount'] }, expect.objectContaining({ responseType: 'blob' }))
  })
})
