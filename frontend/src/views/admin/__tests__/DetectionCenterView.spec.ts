import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount, flushPromises } from '@vue/test-utils'

import type {
  DetectionDiscoveryResponse,
  DetectionProbeResult,
  DetectionReport,
  DetectionStatus,
} from '@/api/admin/detectionCenter'
import DetectionCenterView from '../DetectionCenterView.vue'

const { discoverMock, runMock } = vi.hoisted(() => ({
  discoverMock: vi.fn(),
  runMock: vi.fn(),
}))

vi.mock('@/api/admin/detectionCenter', () => ({
  default: { discover: discoverMock, run: runMock },
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const labels: Record<string, string> = {
        'admin.detectionCenter.discover': 'Fetch models',
        'admin.detectionCenter.start': 'Start detection',
        'admin.detectionCenter.success': 'Passed',
        'admin.detectionCenter.failed': 'Failed',
        'admin.detectionCenter.partialAvailable': 'Partially available',
        'admin.detectionCenter.inconclusive': '无法确认',
        'admin.detectionCenter.unavailableSimple': 'Unavailable',
      }
      return labels[key] ?? key
    },
  }),
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: { template: '<div data-testid="app-layout"><slot /></div>' },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: { props: ['name'], template: '<span data-testid="icon" :data-name="name" />' },
}))

enableAutoUnmount(afterEach)

const discovery: DetectionDiscoveryResponse = {
  suggested_protocol: 'anthropic',
  protocols: ['anthropic'],
  models: [{ id: 'test-model', name: 'Test model', protocols: ['anthropic'] }],
  attempts: [],
}

function probe(id: string, status: DetectionStatus): DetectionProbeResult {
  return { id, name: id, category: 'Protocol', status, confidence: 0.9, summary: id }
}

function report(probes: DetectionProbeResult[]): DetectionReport {
  const count = (status: DetectionStatus) => probes.filter(item => item.status === status).length
  return {
    report_id: 'test-report',
    started_at: '2026-09-30T00:00:00Z',
    completed_at: '2026-09-30T00:00:01Z',
    base_url: 'https://api.example.com',
    protocol: 'anthropic',
    model: 'test-model',
    mode: 'standard',
    summary: {
      total: probes.length,
      success: count('success'),
      failed: count('failed'),
      partial: count('partial'),
      inconclusive: count('inconclusive'),
      not_applicable: count('not_applicable'),
      unavailable: count('unavailable'),
    },
    probes,
  }
}

async function configureAndRun(wrapper: ReturnType<typeof mount>, result: DetectionReport) {
  runMock.mockResolvedValueOnce(result)
  await wrapper.get('.config-actions .btn-primary').trigger('click')
  await flushPromises()
}

async function configureView() {
  const wrapper = mount(DetectionCenterView)
  await wrapper.get('input[type="url"]').setValue('https://api.example.com')
  await wrapper.get('input[type="password"]').setValue('test-key')
  await wrapper.get('.config-grid select').setValue('anthropic')
  await wrapper.get('.config-actions .btn-secondary').trigger('click')
  await flushPromises()
  return wrapper
}

beforeEach(() => {
  discoverMock.mockReset().mockResolvedValue(discovery)
  runMock.mockReset()
})

describe('DetectionCenterView summary and selection', () => {
  it('shows inconclusive separately and selects it before partial or unavailable results', async () => {
    const wrapper = await configureView()
    await configureAndRun(wrapper, report([
      probe('success', 'success'),
      probe('partial', 'partial'),
      probe('not-applicable', 'not_applicable'),
      probe('unavailable', 'unavailable'),
      probe('policy-refusal', 'inconclusive'),
    ]))

    expect(wrapper.get('.summary-success strong').text()).toBe('1')
    expect(wrapper.get('.summary-failed strong').text()).toBe('0')
    expect(wrapper.get('.summary-partial strong').text()).toBe('1')
    expect(wrapper.get('.summary-inconclusive strong').text()).toBe('1')
    expect(wrapper.get('.summary-inconclusive').text()).toContain('无法确认')
    expect(wrapper.get('.summary-unavailable strong').text()).toBe('2')
    expect(wrapper.get('.capability-row.active .capability-name').text()).toBe('policy-refusal')
  })

  it('keeps failed results ahead of inconclusive, partial, and unavailable results', async () => {
    const wrapper = await configureView()
    await configureAndRun(wrapper, report([
      probe('policy-refusal', 'inconclusive'),
      probe('partial', 'partial'),
      probe('unavailable', 'unavailable'),
      probe('failed', 'failed'),
    ]))

    expect(wrapper.get('.capability-row.active .capability-name').text()).toBe('failed')
  })
})
