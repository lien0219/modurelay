import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import InvoicesView from '../InvoicesView.vue'
import type { InvoiceApplication, InvoiceEligibility } from '@/types/invoice'

const { api, showSuccess, showError } = vi.hoisted(() => ({
  api: { eligibility: vi.fn(), list: vi.fn(), create: vi.fn(), detail: vi.fn(), cancel: vi.fn() },
  showSuccess: vi.fn(), showError: vi.fn(),
}))
vi.mock('@/api/invoices', () => ({ invoicesAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

const eligibility: InvoiceEligibility = { enabled: true, minimum_amount_cents: 30000, total_recharged_cents: 100000, reserved_amount_cents: 30000, available_amount_cents: 70000 }
const application: InvoiceApplication = {
  id: 5, application_no: 'INV202610230005', user_id: 9, user_email: 'user@example.test', user_name: 'Alice',
  amount_cents: 30000, invoice_type: 'vat_normal', title_type: 'enterprise', title: 'Example Enterprise',
  tax_number: '91310000123456789A', email: 'finance@example.test', address: '', phone: '', bank_name: '', bank_account: '', remark: 'Finance note',
  status: 'pending', admin_remark: '', invoice_number: '', created_at: '2026-10-23T04:00:00Z', updated_at: '2026-10-23T04:00:00Z', processed_at: null,
}

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
async function fillForm(wrapper: Wrapper, value = '300.01') {
  await wrapper.get('[data-testid="new-invoice"]').trigger('click')
  await wrapper.get('#invoice-amount').setValue(value)
  await wrapper.get('#invoice-title').setValue(' Example Enterprise ')
  await wrapper.get('#invoice-tax').setValue('91310000123456789A')
  await wrapper.get('#invoice-email').setValue('finance@example.test')
}

describe('user invoice workflow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    api.eligibility.mockResolvedValue({ data: { ...eligibility } })
    api.list.mockResolvedValue({ data: { items: [application], total: 1, page: 1, page_size: 20, pages: 1 } })
    api.create.mockResolvedValue({ data: application })
    api.detail.mockResolvedValue({ data: application })
    api.cancel.mockResolvedValue({ data: { ...application, status: 'cancelled' } })
  })
  afterEach(() => { wrappers.splice(0).forEach(wrapper => wrapper.unmount()) })

  it.each([
    [{ total_recharged_cents: 29999, available_amount_cents: 29999 }, 'invoices.insufficientRecharge'],
    [{ available_amount_cents: 29999 }, 'invoices.insufficientAvailable'],
    [{ enabled: false }, 'invoices.disabled'],
  ])('disables new applications when ineligible and preserves historical detail access', async (change, notice) => {
    api.eligibility.mockResolvedValue({ data: { ...eligibility, ...change } })
    const wrapper = await setup()
    expect(wrapper.get('[data-testid="new-invoice"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain(notice)
    expect(wrapper.text()).toContain(application.application_no)
    await button(wrapper, 'invoices.details').trigger('click')
    await flushPromises()
    expect(api.detail).toHaveBeenCalledWith(5)
    expect(wrapper.text()).toContain('finance@example.test')
    expect(wrapper.text()).toContain('Finance note')
    expect(api.create).not.toHaveBeenCalled()
  })

  it('accepts the exact 300 yuan recharge threshold', async () => {
    api.eligibility.mockResolvedValue({ data: { ...eligibility, total_recharged_cents: 30000, reserved_amount_cents: 0, available_amount_cents: 30000 } })
    const wrapper = await setup()
    expect(wrapper.get('[data-testid="new-invoice"]').attributes('disabled')).toBeUndefined()
    await fillForm(wrapper, '300')
    await wrapper.get('#invoice-application').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ amount_cents: 30000 }))
  })

  it.each(['299.99', '700.01', '300.001', '3e2'])('blocks an invalid requested amount %s before sending', async amount => {
    const wrapper = await setup()
    await fillForm(wrapper, amount)
    await wrapper.get('#invoice-application').trigger('submit')
    expect(api.create).not.toHaveBeenCalled()
    expect(wrapper.get('#invoice-amount').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('#invoice-amount-error').text()).toBe('invoices.amountInvalid')
  })

  it('submits integer cents and refreshes allowance and history after success', async () => {
    const wrapper = await setup()
    await fillForm(wrapper)
    await wrapper.get('#invoice-application').trigger('submit')
    await flushPromises()
    expect(api.create).toHaveBeenCalledWith(expect.objectContaining({ amount_cents: 30001, title: 'Example Enterprise', email: 'finance@example.test' }))
    expect(api.eligibility).toHaveBeenCalledTimes(2)
    expect(api.list).toHaveBeenCalledTimes(2)
    expect(wrapper.find('#invoice-application').exists()).toBe(false)
  })

  it('requires enterprise tax and bank details for special VAT invoices', async () => {
    const wrapper = await setup()
    await fillForm(wrapper)
    await wrapper.get('#invoice-title-type').setValue('personal')
    expect(wrapper.find('#invoice-tax').exists()).toBe(false)
    await wrapper.get('#invoice-type').setValue('vat_special')
    expect((wrapper.get('#invoice-title-type').element as HTMLSelectElement).value).toBe('enterprise')
    expect(wrapper.get('#invoice-title-type').attributes('disabled')).toBeDefined()
    for (const selector of ['#invoice-tax', '#invoice-address', '#invoice-phone', '#invoice-bank_name', '#invoice-bank_account']) {
      expect(wrapper.get(selector).attributes('required')).toBeDefined()
    }
  })

  it('surfaces a backend rejection and reloads eligibility to prevent stale allowance reuse', async () => {
    api.create.mockRejectedValue({ message: 'Allowance already reserved' })
    const wrapper = await setup()
    await fillForm(wrapper)
    api.eligibility.mockResolvedValue({ data: { ...eligibility, available_amount_cents: 0 } })
    await wrapper.get('#invoice-application').trigger('submit')
    await flushPromises()
    expect(wrapper.text()).toContain('Allowance already reserved')
    expect(wrapper.get('[data-testid="submit-invoice"]').attributes('disabled')).toBeDefined()
  })

  it('keeps applying disabled on eligibility failure but still loads history', async () => {
    api.eligibility.mockRejectedValue({ message: 'Temporarily unavailable' })
    const wrapper = await setup()
    expect(wrapper.text()).toContain('Temporarily unavailable')
    expect(wrapper.get('[data-testid="new-invoice"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain(application.application_no)
  })

  it('cancels only a pending record and reloads the released allowance', async () => {
    const wrapper = await setup()
    await button(wrapper, 'invoices.cancel').trigger('click')
    expect(api.cancel).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="confirm-cancel-invoice"]').trigger('click')
    await flushPromises()
    expect(api.cancel).toHaveBeenCalledWith(5)
    expect(api.eligibility).toHaveBeenCalledTimes(2)
    api.list.mockResolvedValue({ data: { items: [{ ...application, status: 'issued' }], total: 1 } })
    await button(wrapper, 'common.refresh').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('button').some(item => item.text() === 'invoices.cancel')).toBe(false)
  })

  it('loads record pages from the server', async () => {
    api.list.mockResolvedValue({ data: { items: [application], total: 41, page: 1, page_size: 20, pages: 3 } })
    const wrapper = await setup()
    await button(wrapper, 'pagination.next').trigger('click')
    await flushPromises()
    expect(api.list).toHaveBeenLastCalledWith({ page: 2, page_size: 20 })
  })
})
