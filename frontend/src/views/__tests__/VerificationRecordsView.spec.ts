import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import Select from '@/components/common/Select.vue'
import VerificationAnalytics from '@/components/verification/VerificationAnalytics.vue'
import VerificationIdentity from '@/components/verification/VerificationIdentity.vue'
import VerificationRecordsView from '../VerificationRecordsView.vue'

const { list, locale, options, showError, route } = vi.hoisted(() => ({
  list: vi.fn(),
  locale: { value: 'zh-CN' },
  options: vi.fn(),
  showError: vi.fn(),
  route: { meta: { requiresAdmin: false } },
}))

vi.mock('@/api/verificationRecords', () => ({
  default: { list, options },
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('vue-router', () => ({ useRoute: () => route }))
vi.mock('vue-chartjs', () => ({ Bar: { template: '<div data-test="bar-chart" />' } }))
vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return { ...actual, useI18n: () => ({ t: (key: string) => key, locale }) }
})

const record = {
  id: 'record-1',
  order_no: 'EML-1',
  verification_type: 'email',
  product_type: 'gmail',
  user_id: 42,
  user_email: 'operator-visible@example.com',
  service_code: 'openai',
  channel_code: 'email_channel_1',
  channel_name: 'Email channel 1',
  provider_code: 'emailnator',
  target: 'mail@example.com',
  status: 'failed',
  outcome: 'failed',
  refund_status: 'released',
  refund_reason: 'provider rejected request',
  sale_amount: 3.5,
  provider_cost: 0.4,
  user_debit_amount: 3.5,
  reserved_amount: 0,
  captured_amount: 0,
  released_amount: 3.5,
  refunded_amount: 0,
  currency: 'CNY',
  provider_request_count: 2,
  error_code: 'EMAIL_GENERATION_FAILED',
  error_message: 'internal provider response',
  public_error_message: 'public failure message',
  created_at: '2026-09-16T08:00:00Z',
  updated_at: '2026-09-16T08:00:01Z',
}

const response = {
  items: [record],
  total: 1,
  page: 1,
  page_size: 20,
  pages: 1,
  summary: {
    total: 1,
    processing: 0,
    success: 0,
    failed: 1,
    refunded: 0,
    cancelled: 0,
    expired: 0,
    sale_amount: 3.5,
    user_debit_amount: 3.5,
    reserved_amount: 0,
    captured_amount: 0,
    released_amount: 3.5,
    refunded_amount: 0,
    provider_cost: 0.4,
    estimated_profit: -0.4,
  },
}

function mountView() {
  return mount(VerificationRecordsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        Icon: true,
        Pagination: true,
      },
    },
  })
}

describe('VerificationRecordsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    locale.value = 'zh-CN'
    route.meta.requiresAdmin = false
    list.mockResolvedValue(response)
    options.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20, pages: 1 })
  })

  it('renders a user-scoped record table without admin-only fields', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }), false)
    expect(wrapper.text()).toContain('public failure message')
    expect(wrapper.text()).not.toContain('operator-visible@example.com')
    expect(wrapper.text()).not.toContain('emailnator')
    expect(wrapper.text()).not.toContain('internal provider response')
    expect(wrapper.text()).not.toContain('verificationRecords.columns.providerCost')
    const tableScroller = wrapper.get('[aria-label="verificationRecords.userTableLabel"]')
    expect(tableScroller.classes()).toContain('overflow-x-auto')
    expect(tableScroller.classes()).toContain('min-w-0')
    expect(wrapper.get('table').classes()).toContain('min-w-[1750px]')
    expect(wrapper.get('table').classes()).toContain('whitespace-nowrap')
    expect(wrapper.text()).toContain('verificationRecords.columns.platform')
    expect(wrapper.text()).toContain('verificationRecords.columns.country')
  })

  it('keeps filter actions readable before the six-column desktop breakpoint', async () => {
    const wrapper = mountView()
    await flushPromises()

    const filterGrid = wrapper.get('form > div')
    expect(filterGrid.classes()).toContain('grid-cols-1')
    expect(filterGrid.classes()).toContain('items-end')
    expect(filterGrid.classes()).toContain('xl:grid-cols-4')
    expect(filterGrid.classes().some(className => className.startsWith('2xl:grid-cols-'))).toBe(false)
    expect(filterGrid.classes()).toContain('min-[2000px]:grid-cols-[150px_170px_minmax(190px,1fr)_minmax(190px,1fr)_minmax(230px,1.2fr)_160px_160px_auto]')
    for (const button of wrapper.findAll('form .self-end button')) {
      expect(button.classes()).toContain('whitespace-nowrap')
      expect(button.classes()).toContain('shrink-0')
      expect(button.classes()).toContain('h-[42px]')
    }
    expect(wrapper.find('form .self-end').classes()).toContain('flex-wrap')
    for (const input of wrapper.findAll('form input.input')) {
      expect(input.classes()).toContain('h-[42px]')
    }
  })

  it('localizes country names outside the current catalog page', async () => {
    list.mockResolvedValue({
      ...response,
      items: [{ ...record, verification_type: 'sms', region: 'GB' }],
      analytics: {
        by_platform: [],
        by_country: [
          { key: 'VN', total: 1, success: 1, success_rate: 1 },
          { key: 'United States', total: 1, success: 1, success_rate: 1 },
        ],
        by_type: [],
        financial: [],
      },
    })
    options.mockImplementation(({ kind }: { kind: string }) => Promise.resolve({
      items: kind === 'country'
        ? [{ value: 'US', label: 'United States', label_en: 'United States' }]
        : [],
      total: kind === 'country' ? 1 : 0,
      page: 1,
      page_size: 20,
      pages: 1,
    }))

    const wrapper = mountView()
    await flushPromises()

    const countrySelect = wrapper.findAllComponents(Select)[3]
    expect(countrySelect?.props('options')).toEqual([
      expect.objectContaining({ value: 'US', label: '美国' }),
    ])
    const countryIdentity = wrapper.findAllComponents(VerificationIdentity)
      .find(component => component.props('kind') === 'country')
    expect(countryIdentity?.props('label')).toBe('英国')
    expect(wrapper.getComponent(VerificationAnalytics).props('countryLabels')).toMatchObject({
      GB: '英国',
      VN: '越南',
      'United States': '美国',
    })
  })

  it('renders detailed operational and financial fields for admins', async () => {
    route.meta.requiresAdmin = true
    const wrapper = mountView()
    await flushPromises()

    expect(list).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }), true)
    expect(wrapper.text()).toContain('operator-visible@example.com')
    expect(wrapper.text()).toContain('emailnator')
    expect(wrapper.text()).toContain('EMAIL_GENERATION_FAILED')
    expect(wrapper.text()).toContain('internal provider response')
    expect(wrapper.text()).toContain('verificationRecords.columns.providerCost')
    expect(wrapper.text()).toContain('verificationRecords.refundStatuses.released')
    const tableScroller = wrapper.get('[aria-label="verificationRecords.adminTableLabel"]')
    expect(tableScroller.classes()).toContain('overflow-x-auto')
    expect(tableScroller.classes()).toContain('min-w-0')
    expect(wrapper.get('table').classes()).toContain('min-w-[3300px]')
    expect(wrapper.get('table').classes()).toContain('whitespace-nowrap')
    expect(wrapper.text()).toContain('verificationRecords.summary.totalAmount')
    expect(wrapper.text()).toContain('verificationRecords.summary.totalCost')

    const adminRow = wrapper.get('[aria-label="verificationRecords.adminTableLabel"] tbody tr')
    const cells = adminRow.findAll('td')
    expect(cells).toHaveLength(21)
    const refundCell = cells[11]
    const refundFields = refundCell.findAll('div')
    expect(refundFields[0].classes()).toEqual(expect.arrayContaining(['block', 'max-w-52', 'truncate']))
    expect(refundFields[0].attributes('title')).toBe('verificationRecords.refundStatuses.released')
    expect(refundFields[1].classes()).toEqual(expect.arrayContaining(['block', 'max-w-52', 'truncate']))
    expect(refundFields[1].attributes('title')).toBe('provider rejected request')
    expect(cells[12].classes()).toContain('whitespace-nowrap')
    expect(cells[12].text()).toContain('3.50')
  })
})
