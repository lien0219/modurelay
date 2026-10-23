import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import InvoiceDocument from '../InvoiceDocument.vue'

const { saveAs } = vi.hoisted(() => ({ saveAs: vi.fn() }))
vi.mock('file-saver', () => ({ saveAs }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
const document = { filename: 'invoice.pdf', size_bytes: 2048, uploaded_at: '2026-10-23T04:00:00Z', sha256: 'pdf-hash' }
const createObjectURL = vi.fn(() => 'blob:private-invoice')
const revokeObjectURL = vi.fn()
const wrappers: ReturnType<typeof mount>[] = []
function render(loadDocument = vi.fn().mockResolvedValue({ data: new Blob(['%PDF-1.4'], { type: 'application/pdf' }) })) {
  const wrapper = mount(InvoiceDocument, { props: { document, loadDocument } })
  wrappers.push(wrapper)
  return { wrapper, loadDocument }
}

describe('private invoice PDF', () => {
  beforeEach(() => { vi.clearAllMocks(); vi.stubGlobal('URL', { createObjectURL, revokeObjectURL }) })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()); vi.unstubAllGlobals() })

  it('fetches an authenticated blob for preview and revokes its URL on close', async () => {
    const { wrapper, loadDocument } = render()
    expect(loadDocument).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="preview-invoice-pdf"]').trigger('click')
    await flushPromises()
    expect(loadDocument).toHaveBeenCalledWith()
    expect(wrapper.get('iframe').attributes('src')).toBe('blob:private-invoice')
    expect(createObjectURL).toHaveBeenCalledWith(expect.objectContaining({ type: 'application/pdf' }))
    await wrapper.get('[data-testid="preview-invoice-pdf"]').trigger('click')
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:private-invoice')
    expect(wrapper.find('iframe').exists()).toBe(false)
  })

  it('downloads the protected file with its original PDF filename', async () => {
    const { wrapper, loadDocument } = render()
    await wrapper.get('[data-testid="download-invoice-pdf"]').trigger('click')
    await flushPromises()
    expect(loadDocument).toHaveBeenCalledWith(true)
    expect(saveAs).toHaveBeenCalledWith(expect.objectContaining({ type: 'application/pdf' }), 'invoice.pdf')
  })

  it('revokes a displayed blob URL on unmount', async () => {
    const { wrapper } = render()
    await wrapper.get('[data-testid="preview-invoice-pdf"]').trigger('click')
    await flushPromises()
    wrapper.unmount()
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:private-invoice')
  })

  it('does not allocate an object URL when a request completes after the dialog closes', async () => {
    let resolve!: (value: { data: Blob }) => void
    const loadDocument = vi.fn(() => new Promise<{ data: Blob }>(done => { resolve = done }))
    const { wrapper } = render(loadDocument)
    await wrapper.get('[data-testid="preview-invoice-pdf"]').trigger('click')
    wrapper.unmount()
    resolve({ data: new Blob(['%PDF-1.4']) })
    await flushPromises()
    expect(createObjectURL).not.toHaveBeenCalled()
  })

  it('shows denied access without creating a preview or a download', async () => {
    const { wrapper } = render(vi.fn().mockRejectedValue({ status: 403, message: 'Access denied' }))
    await wrapper.get('[data-testid="preview-invoice-pdf"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Access denied')
    expect(wrapper.find('iframe').exists()).toBe(false)
    expect(createObjectURL).not.toHaveBeenCalled()
    expect(saveAs).not.toHaveBeenCalled()
  })
})
