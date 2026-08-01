import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import UsageTable from '../UsageTable.vue'

vi.mock('@/utils/ipGeoLookup', () => ({
  useIpGeoLookup: () => ({
    getEntry: vi.fn(() => ({ status: 'idle' })),
    fetchOne: vi.fn(),
    fetchBatch: vi.fn(),
  }),
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.original': 'Original',
  'usage.userBilled': 'User billed',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div>
      <div v-for="row in data" :key="row.request_id">
        <slot name="cell-cost" :row="row" />
      </div>
    </div>
  `,
}

describe('UsageTable billing detail visibility', () => {
  it('keeps actual user billing visible while hiding unit prices, multiplier and original cost', async () => {
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    const wrapper = mount(UsageTable, {
      props: {
        data: [{
          request_id: 'req-user-hidden-pricing',
          model: 'gpt-5.6',
          actual_cost: 0.038786,
          total_cost: 1.29288,
          rate_multiplier: 0.03,
          account_rate_multiplier: 1,
          service_tier: 'standard',
          input_cost: 0.4252,
          output_cost: 0.0792,
          cache_creation_cost: 0,
          cache_read_cost: 0.788488,
          input_tokens: 2126,
          output_tokens: 66,
          cache_creation_tokens: 0,
          cache_read_tokens: 394244,
        }],
        loading: false,
        columns: [],
        showAccountBilling: false,
        showUnitPrices: false,
        showRateMultiplier: false,
        showOriginalCost: false,
      },
      global: {
        stubs: {
          DataTable: DataTableStub,
          EmptyState: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await wrapper.get('.group.relative').trigger('mouseenter')
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('User billed')
    expect(text).toContain('$0.038786')
    expect(text).not.toContain('Input Cost')
    expect(text).not.toContain('Output Cost')
    expect(text).not.toContain('Input price')
    expect(text).not.toContain('Output price')
    expect(text).not.toContain('Rate')
    expect(text).not.toContain('Original')
    expect(text).not.toContain('0.03x')
    expect(text).not.toContain('$1.292880')
  })
})
