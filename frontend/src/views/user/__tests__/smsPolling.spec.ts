import { describe, expect, it } from 'vitest'

import { smsOrderPollBucket, smsOrderPollDelay } from '../smsPolling'

describe('SMS order polling schedule', () => {
  const now = Date.parse('2026-09-21T00:00:00.000Z')

  it('keeps channel 1 at the existing three-second cadence', () => {
    expect(smsOrderPollBucket({ channel_code: 'channel_1', created_at: '2026-09-20T00:00:00.000Z' }, now)).toBe('baseline')
    expect(smsOrderPollDelay('baseline')).toBe(3000)
  })

  it('backs off channel 2 by opaque order age', () => {
    expect(smsOrderPollDelay(smsOrderPollBucket({ channel_code: 'channel_2', created_at: '2026-09-20T23:59:30.000Z' }, now))).toBe(5000)
    expect(smsOrderPollDelay(smsOrderPollBucket({ channel_code: 'channel_2', created_at: '2026-09-20T23:58:00.000Z' }, now))).toBe(10000)
    expect(smsOrderPollDelay(smsOrderPollBucket({ channel_code: 'channel_2', created_at: '2026-09-20T23:50:00.000Z' }, now))).toBe(25000)
  })

  it('uses the conservative fast bucket when the timestamp is missing', () => {
    expect(smsOrderPollBucket({ channel_code: 'channel_2' }, now)).toBe('channel-2-fast')
  })

  it('backs off completed orders recovering a missing code', () => {
    const input = { channel_code: 'channel_1', product_type: 'temporary', status: 'completed', latest_verification_code: '' }
    expect(smsOrderPollBucket(input, now)).toBe('recovery')
    expect(smsOrderPollDelay('recovery')).toBe(15_000)
  })
})
