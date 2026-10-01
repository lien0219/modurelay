export type SMSOrderPollBucket = 'baseline' | 'channel-2-fast' | 'channel-2-medium' | 'channel-2-slow' | 'recovery'

export interface SMSOrderPollingInput {
  channel_code?: string
  created_at?: string
  product_type?: string
  status?: string
  latest_verification_code?: string
}

/**
 * Keep polling provider-neutral at the browser boundary. The API exposes
 * opaque channel identifiers, so the schedule must not inspect provider names.
 */
export function smsOrderPollBucket(order: SMSOrderPollingInput, now = Date.now()): SMSOrderPollBucket {
  if (
    order.product_type === 'temporary'
    && String(order.status || '').toLowerCase() === 'completed'
    && !String(order.latest_verification_code || '').trim()
  ) return 'recovery'
  if (order.channel_code !== 'channel_2') return 'baseline'
  const createdAt = Date.parse(String(order.created_at || ''))
  const age = Number.isFinite(createdAt) ? Math.max(0, now - createdAt) : 0
  if (age < 60_000) return 'channel-2-fast'
  if (age < 300_000) return 'channel-2-medium'
  return 'channel-2-slow'
}

export function smsOrderPollDelay(bucket: SMSOrderPollBucket): number {
  switch (bucket) {
    case 'channel-2-fast': return 5_000
    case 'channel-2-medium': return 10_000
    case 'channel-2-slow': return 25_000
    case 'recovery': return 15_000
    default: return 3_000
  }
}
