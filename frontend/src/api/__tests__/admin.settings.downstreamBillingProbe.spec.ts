import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get, put }
}))

import {
  getDownstreamBillingProbeSettings,
  updateDownstreamBillingProbeSettings
} from '@/api/admin/settings'

describe('admin downstream billing probe settings API', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
  })

  it('reads and updates the disclosure switch', async () => {
    const settings = { enabled: false }
    get.mockResolvedValueOnce({ data: settings })
    put.mockResolvedValueOnce({ data: settings })

    await expect(getDownstreamBillingProbeSettings()).resolves.toEqual(settings)
    await expect(updateDownstreamBillingProbeSettings(settings)).resolves.toEqual(settings)
    expect(get).toHaveBeenCalledWith('/admin/settings/downstream-billing-probe')
    expect(put).toHaveBeenCalledWith('/admin/settings/downstream-billing-probe', settings)
  })
})
