import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import InvoicesView from '../InvoicesView.vue'
import { INVOICE_EXPORT_COLUMNS, type InvoiceApplication } from '@/types/invoice'

const { api, appStore, saveAs } = vi.hoisted(() => ({
  api: { config: vi.fn(), updateConfig: vi.fn(), list: vi.fn(), detail: vi.fn(), update: vi.fn(), export: vi.fn(), account: vi.fn(), uploadDocument: vi.fn(), document: vi.fn() },
  appStore: { showSuccess: vi.fn(), showError: vi.fn(), fetchPublicSettings: vi.fn(), cachedPublicSettings: { invoice_enabled: false } },
  saveAs: vi.fn(),
}))
vi.mock('@/api/invoices', () => ({ adminInvoicesAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('file-saver', () => ({ saveAs }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

const application: InvoiceApplication = {
  id: 5, application_no: 'INV202610230005', user_id: 9, user_email: 'user@example.test', user_name: 'Alice',
  amount_cents: 30000, invoice_type: 'vat_special', title_type: 'enterprise', title: 'Example Enterprise',
  tax_number: '91310000123456789A', email: 'finance@example.test', address: 'Example address', phone: '01012345678', bank_name: 'Example Bank', bank_account: '123456789012345678', remark: 'Finance note',
  status: 'pending', admin_remark: '', invoice_number: '', created_at: '2026-10-23T04:00:00Z', updated_at: '2026-10-23T04:00:00Z', processed_at: null,
  contact: 'wechat-finance', requested_issue_date: '2026-10-25', document: null,
}
const document = { filename: 'invoice.pdf', size_bytes: 2048, uploaded_at: '2026-10-23T04:00:00Z', sha256: 'invoice-hash' }

function render() {
  return mount(InvoicesView, {
    global: { stubs: {
      AppLayout: { template: '<main><slot /></main>' },
      BaseDialog: { props: ['show', 'title'], template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>' },
    } },
  })
}
type Wrapper = ReturnType<typeof render>
const wrappers: Wrapper[] = []
async function setup() { const wrapper = render(); wrappers.push(wrapper); await flushPromises(); return wrapper }
function button(wrapper: Wrapper, text: string) { return wrapper.findAll('button').find(item => item.text() === text)! }
async function chooseFile(wrapper: Wrapper, file: File) {
  Object.defineProperty(wrapper.get('#invoice-pdf-file').element, 'files', { value: [file], configurable: true })
  await wrapper.get('#invoice-pdf-file').trigger('change')
}

describe('administrator invoice workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    appStore.cachedPublicSettings.invoice_enabled = false
    appStore.fetchPublicSettings.mockResolvedValue({ invoice_enabled: true })
    api.config.mockResolvedValue({ data: { enabled: false } })
    api.updateConfig.mockResolvedValue({ data: { enabled: true } })
    api.list.mockResolvedValue({ data: { items: [application], total: 1, page: 1, page_size: 20, pages: 1 } })
    api.detail.mockResolvedValue({ data: application })
    api.update.mockResolvedValue({ data: { ...application, status: 'issued', invoice_number: 'INV-123' } })
    api.export.mockResolvedValue({ data: new Blob(['\uFEFFApplication,Amount\nINV,300'], { type: 'text/csv' }) })
    api.uploadDocument.mockResolvedValue({ data: { ...application, document } })
    api.account.mockResolvedValue({ data: { user: { id: 9, email: 'user@example.test', username: 'Alice', balance: 42.5 }, orders: { items: [], total: 0, page: 1, page_size: 10, pages: 0 } } })
  })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

  it('saves the visibility switch and refreshes the shared navigation setting', async () => {
    const wrapper = await setup()
    expect(wrapper.get('[data-testid="save-invoice-config"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="invoice-enabled"]').setValue(true)
    await wrapper.get('[data-testid="save-invoice-config"]').trigger('click')
    await flushPromises()
    expect(api.updateConfig).toHaveBeenCalledWith(true)
    expect(appStore.cachedPublicSettings.invoice_enabled).toBe(true)
    expect(appStore.fetchPublicSettings).toHaveBeenCalledWith(true)
  })

  it('does not change shared settings if the server rejects the config update', async () => {
    api.updateConfig.mockRejectedValue({ message: 'Save failed' })
    const wrapper = await setup()
    await wrapper.get('[data-testid="invoice-enabled"]').setValue(true)
    await wrapper.get('[data-testid="save-invoice-config"]').trigger('click')
    await flushPromises()
    expect(appStore.cachedPublicSettings.invoice_enabled).toBe(false)
    expect(appStore.showError).toHaveBeenCalledWith('Save failed')
  })

  it('keeps the saved switch when a reused public-settings request returns older data', async () => {
    appStore.fetchPublicSettings.mockImplementation(async () => {
      appStore.cachedPublicSettings.invoice_enabled = false
      return { invoice_enabled: false }
    })
    const wrapper = await setup()
    await wrapper.get('[data-testid="invoice-enabled"]').setValue(true)
    await wrapper.get('[data-testid="save-invoice-config"]').trigger('click')
    await flushPromises()
    expect(appStore.cachedPublicSettings.invoice_enabled).toBe(true)
  })

  it('filters server results and exports all matching rows rather than only the current page', async () => {
    api.list.mockResolvedValue({ data: { items: [application], total: 41, page: 1, page_size: 20, pages: 3 } })
    const wrapper = await setup()
    await wrapper.get('#invoice-search').setValue(' Example Enterprise ')
    await wrapper.get('#invoice-status-filter').setValue('pending')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith({ search: 'Example Enterprise', status: 'pending', page: 1, page_size: 20 })
    await button(wrapper, 'pagination.next').trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith({ search: 'Example Enterprise', status: 'pending', page: 2, page_size: 20 })
    await wrapper.get('[data-testid="export-invoices"]').trigger('click')
    await wrapper.get('#invoice-export').trigger('submit')
    await flushPromises()
    expect(api.export).toHaveBeenCalledWith({ search: 'Example Enterprise', status: 'pending', columns: [...INVOICE_EXPORT_COLUMNS] })
    expect(saveAs).toHaveBeenCalledWith(expect.any(Blob), expect.stringMatching(/^invoice-applications-.*\.csv$/))
  })

  it('shows all financial details and requires an invoice number to mark issued', async () => {
    api.detail.mockResolvedValue({ data: { ...application, document } })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    for (const value of ['finance@example.test', '91310000123456789A', 'Example Bank', '123456789012345678', 'Finance note']) {
      expect(wrapper.text()).toContain(value)
    }
    await wrapper.get('#invoice-next-status').setValue('issued')
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).not.toHaveBeenCalled()
    await wrapper.get('#invoice-number').setValue(' INV-123 ')
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(5, { status: 'issued', invoice_number: 'INV-123', admin_remark: '' })
    expect(wrapper.find('#invoice-processing').exists()).toBe(false)
    expect(wrapper.text()).toContain('INV-123')
  })

  it('requires a user-visible rejection reason', async () => {
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    await wrapper.get('#invoice-next-status').setValue('rejected')
    await wrapper.get('#invoice-admin-remark').setValue('  ')
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).not.toHaveBeenCalled()
    await wrapper.get('#invoice-admin-remark').setValue(' Please correct the tax number ')
    api.update.mockResolvedValue({ data: { ...application, status: 'rejected', admin_remark: 'Please correct the tax number' } })
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(5, { status: 'rejected', invoice_number: '', admin_remark: 'Please correct the tax number' })
    expect(wrapper.text()).toContain('Please correct the tax number')
  })

  it('offers only issued or rejected for an application already being processed', async () => {
    api.detail.mockResolvedValue({ data: { ...application, status: 'processing', document } })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    expect((wrapper.get('#invoice-next-status').element as HTMLSelectElement).value).toBe('issued')
    expect(wrapper.findAll('#invoice-next-status option').map(option => option.attributes('value'))).toEqual(['issued', 'rejected'])
    expect(wrapper.get('#invoice-number').attributes('required')).toBeDefined()
    await wrapper.get('#invoice-number').setValue('INV-123')
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(5, { status: 'issued', invoice_number: 'INV-123', admin_remark: '' })
  })

  it('advances the available action after marking a pending application as processing', async () => {
    api.update.mockResolvedValue({ data: { ...application, status: 'processing' } })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(5, { status: 'processing', invoice_number: '', admin_remark: '' })
    expect((wrapper.get('#invoice-next-status').element as HTMLSelectElement).value).toBe('issued')
    expect(wrapper.findAll('#invoice-next-status option').map(option => option.attributes('value'))).toEqual(['issued', 'rejected'])
  })

  it.each(['issued', 'rejected', 'cancelled'])('keeps %s status final', async status => {
    api.detail.mockResolvedValue({ data: { ...application, status } })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    expect(wrapper.find('#invoice-processing').exists()).toBe(false)
    expect(wrapper.find('[data-testid="update-invoice"]').exists()).toBe(false)
  })

  it('shows a backend state conflict without claiming success', async () => {
    api.update.mockRejectedValue({ message: 'Application already cancelled' })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('Application already cancelled')
    expect(appStore.showSuccess).not.toHaveBeenCalled()
  })

  it('requires a PDF before issuing, then accepts a validated upload', async () => {
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    await wrapper.get('#invoice-next-status').setValue('issued')
    await wrapper.get('#invoice-number').setValue('INV-123')
    expect(wrapper.get('[data-testid="update-invoice"]').attributes('disabled')).toBeDefined()
    await wrapper.get('#invoice-processing').trigger('submit')
    expect(api.update).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('invoices.pdfRequired')
    await chooseFile(wrapper, new File(['not a pdf'], 'invoice.txt'))
    expect(wrapper.text()).toContain('invoices.pdfInvalid')
    expect(wrapper.get('[data-testid="upload-invoice-pdf"]').attributes('disabled')).toBeDefined()
    const file = new File(['%PDF-1.4\ninvoice'], 'invoice.pdf', { type: 'application/pdf' })
    await chooseFile(wrapper, file)
    await wrapper.get('[data-testid="upload-invoice-pdf"]').trigger('click')
    await flushPromises()
    expect(api.uploadDocument).toHaveBeenCalledWith(5, file)
    expect(api.update).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="update-invoice"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('#invoice-processing').trigger('submit')
    await flushPromises()
    expect(api.update).toHaveBeenCalledWith(5, { status: 'issued', invoice_number: 'INV-123', admin_remark: '' })
  })

  it('can backfill a historical issued PDF without changing its status', async () => {
    api.detail.mockResolvedValue({ data: { ...application, status: 'issued' } })
    api.uploadDocument.mockResolvedValue({ data: { ...application, status: 'issued', document } })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('invoices.backfillHelp')
    await chooseFile(wrapper, new File(['%PDF-1.4'], 'invoice.pdf', { type: 'application/pdf' }))
    await wrapper.get('[data-testid="upload-invoice-pdf"]').trigger('click')
    await flushPromises()
    expect(api.update).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="invoice-pdf-file"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="preview-invoice-pdf"]').exists()).toBe(true)
  })

  it('keeps selections across pages and exports only chosen columns in stable order', async () => {
    api.list.mockResolvedValue({ data: { items: [application], total: 41, page: 1, page_size: 20, pages: 3 } })
    const wrapper = await setup()
    await wrapper.get('[data-test="select-row"]').setValue(true)
    expect(api.detail).not.toHaveBeenCalled()
    api.list.mockResolvedValue({ data: { items: [{ ...application, id: 6 }], total: 41, page: 2, page_size: 20, pages: 3 } })
    await button(wrapper, 'pagination.next').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="select-row"]').setValue(true)
    await wrapper.get('[data-testid="export-invoices"]').trigger('click')
    await button(wrapper, 'invoices.clearColumns').trigger('click')
    expect(wrapper.get('[data-testid="confirm-export-invoices"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="export-column-amount"]').setValue(true)
    await wrapper.get('[data-testid="export-column-user_email"]').setValue(true)
    await wrapper.get('#invoice-export').trigger('submit')
    await flushPromises()
    expect(api.export).toHaveBeenCalledWith({ ids: [5, 6], columns: ['user_email', 'amount'] })
    expect(saveAs).toHaveBeenCalledTimes(1)
  })

  it('loads the applicant balance and recharge records only when requested', async () => {
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    expect(api.account).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="invoice-account-toggle"]').trigger('click')
    await flushPromises()
    expect(api.account).toHaveBeenCalledWith(5, { page: 1, page_size: 10 })
    expect(wrapper.text()).toContain('42.50')
    expect(wrapper.text()).toContain('USD')
  })

  it('pages recharge history independently from the invoice application list', async () => {
    api.account.mockResolvedValue({ data: { user: { id: 9, email: 'user@example.test', username: 'Alice', balance: 42.5 }, orders: { items: [], total: 21, page: 1, page_size: 10, pages: 3 } } })
    const wrapper = await setup()
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="invoice-account-toggle"]').trigger('click')
    await flushPromises()
    await wrapper.find('[id="invoice-account-records"]').findAll('button').find(item => item.text() === 'pagination.next')!.trigger('click')
    await flushPromises()
    expect(api.account).toHaveBeenLastCalledWith(5, { page: 2, page_size: 10 })
    expect(api.list).toHaveBeenCalledTimes(1)
  })
})
