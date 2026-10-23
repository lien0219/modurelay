import { Blob } from 'node:buffer'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { adminInvoicesAPI } from '../invoices'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('../client', () => ({ apiClient: { post } }))

describe('invoice CSV export errors', () => {
  beforeEach(() => vi.clearAllMocks())

  it('preserves the binary CSV response on success', async () => {
    const response = { status: 200, data: new Blob(['\uFEFFApplication,Amount\nINV001,300'], { type: 'text/csv' }) }
    post.mockResolvedValue(response)
    expect(await adminInvoicesAPI.export({ status: 'pending' })).toBe(response)
  })

  it('decodes a blob JSON limit error for a localized actionable message', async () => {
    post.mockResolvedValue({ status: 400, data: new Blob([JSON.stringify({ code: 400, reason: 'INVOICE_EXPORT_TOO_LARGE', message: 'Use narrower filters' })], { type: 'application/json' }) })
    await expect(adminInvoicesAPI.export({})).rejects.toMatchObject({ status: 400, reason: 'INVOICE_EXPORT_TOO_LARGE', message: 'Use narrower filters' })
    const options = post.mock.calls[0][2]
    expect(options.validateStatus(400)).toBe(true)
    for (const status of [401, 403, 500]) expect(options.validateStatus(status)).toBe(false)
  })

  it('rejects malformed error responses so they cannot be downloaded as CSV', async () => {
    post.mockResolvedValue({ status: 400, data: new Blob(['Invalid export']) })
    await expect(adminInvoicesAPI.export({})).rejects.toMatchObject({ status: 400, message: undefined })
  })
})
