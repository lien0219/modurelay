import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import type { OpsBillingHealth as BillingHealth } from '@/api/admin/ops'
import OpsBillingHealth from '../OpsBillingHealth.vue'

const { getBillingHealth, recheckBillingOperation, acknowledgeBillingRecordingGap } = vi.hoisted(() => ({ getBillingHealth: vi.fn(), recheckBillingOperation: vi.fn(), acknowledgeBillingRecordingGap: vi.fn() }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/ops', () => ({ opsAPI: { getBillingHealth, recheckBillingOperation, acknowledgeBillingRecordingGap } }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.resetAllMocks() })

const resolvedAt = '2026-10-22T01:05:00Z'
function health(resolved: boolean): BillingHealth {
  return {
    state: 'Available', checked_at: resolvedAt, window_start: '2026-10-21T01:00:00Z',
    window_end: '2026-10-22T01:03:00Z', truncated: false,
    missing_usage_logs: resolved ? 0 : 1, amount_mismatches: 0,
    findings: [{ id: 1, settlement_id: 42, request_id: 'request-with-history', kind: 'missing_usage_log',
      expected_cost: '1.12345678', actual_cost: resolved ? '1.12345678' : null,
      first_seen_at: '2026-10-22T01:00:00Z', last_seen_at: resolvedAt, resolved_at: resolved ? resolvedAt : null }]
  }
}

describe('billing reconciliation history', () => {
  it('retains recording gaps and requires a review note to acknowledge them', async () => {
    getBillingHealth.mockResolvedValue({ ...health(true), findings: [], operational_state: 'Unknown', observation_unpersisted: 1, recording_gaps: [{ instance_id: 'pod-a', missed_count: 3, acknowledged_count: 1, first_missed_at: resolvedAt, last_missed_at: resolvedAt }] })
    acknowledgeBillingRecordingGap.mockResolvedValue(undefined)
    const wrapper = mount(OpsBillingHealth)
    await flushPromises()
    expect(wrapper.get('[data-testid="recording-gap-form"] button').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="recording-gap-form"] input').setValue('checked ledger for the affected period')
    await wrapper.get('[data-testid="recording-gap-form"]').trigger('submit')
    await flushPromises()
    expect(acknowledgeBillingRecordingGap).toHaveBeenCalledWith('pod-a', 'checked ledger for the affected period')
    expect(wrapper.text()).toContain('admin.ops.monitoring.billing.gapHint')
    expect(wrapper.find('tbody tr').text()).toContain('pod-a')
    expect(getBillingHealth).toHaveBeenCalledTimes(2)
  })
  it('keeps resolved history distinct from zero current unresolved counts', async () => {
    getBillingHealth.mockResolvedValue(health(true))
    const wrapper = mount(OpsBillingHealth)
    await flushPromises()

    expect(wrapper.findAll('dd').map(cell => cell.text())).toEqual(['0', '0'])
    expect(wrapper.find('caption').text()).toBe('admin.ops.monitoring.billing.history')
    const row = wrapper.find('tbody tr')
    expect(row.text()).toContain('request-with-history')
    expect(row.text()).toContain('admin.ops.monitoring.billing.resolved')
    expect(row.text()).not.toContain('admin.ops.monitoring.billing.active')
    expect(row.findAll('td')[7].find('time').attributes('datetime')).toBe(resolvedAt)
    expect(row.findAll('td')[7].text()).toBe(new Date(resolvedAt).toLocaleString())
    expect(row.text()).toContain('1.12345678')
  })

  it('marks unresolved findings explicitly without inventing a recovery timestamp', async () => {
    getBillingHealth.mockResolvedValue(health(false))
    const wrapper = mount(OpsBillingHealth)
    await flushPromises()

    const row = wrapper.find('tbody tr')
    expect(row.text()).toContain('admin.ops.monitoring.billing.active')
    expect(row.text()).not.toContain('admin.ops.monitoring.billing.resolved')
    expect(row.findAll('td')[7].find('time').exists()).toBe(false)
    expect(row.findAll('td')[7].text()).toBe('—')
    expect(wrapper.findAll('dd').map(cell => cell.text())).toEqual(['1', '0'])
  })
  it('requires an audit note to recheck failed settlement evidence and refreshes the result', async () => {
    getBillingHealth.mockResolvedValue({ ...health(true), findings: [], operational_state: 'Available', failed_settlements: 1, compensation_backlog: 1, ledger_mismatches: 0, operations: [{ id: 8, request_id: 'failed-request', operation: 'usage', state: 'manual_required', error_code: 'settlement_failed' }] })
    recheckBillingOperation.mockResolvedValue(undefined)
    const wrapper = mount(OpsBillingHealth)
    await flushPromises()
    expect(wrapper.get('form button').attributes('disabled')).toBeDefined()
    await wrapper.get('form input').setValue('verified existing receipt')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(recheckBillingOperation).toHaveBeenCalledWith(8, 'verified existing receipt')
    expect(getBillingHealth).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('admin.ops.monitoring.billing.recheckHint')
  })
  it('does not present unknown operational coverage as zero failed settlements', async () => {
    getBillingHealth.mockResolvedValue({ ...health(true), operational_state: 'Unknown', failed_settlements: null, compensation_backlog: null, ledger_mismatches: null })
    const wrapper = mount(OpsBillingHealth)
    await flushPromises()
    expect(wrapper.findAll('dd').slice(2).map(cell => cell.text())).toEqual(Array(3).fill('admin.ops.monitoring.states.Unknown'))
  })
})
