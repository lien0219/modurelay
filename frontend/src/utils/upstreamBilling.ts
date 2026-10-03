import type { Account, AccountPlatform } from '@/types'

const upstreamBillingProbePlatforms = new Set<AccountPlatform>([
  'openai',
  'anthropic',
  'gemini',
  'antigravity',
  'grok',
  'kimi',
  'zhipu',
  'deepseek',
  'minimax',
  'opencode_go'
])

export const isUpstreamBillingProbeAccount = (
  account: Pick<Account, 'platform' | 'type'>
): boolean => (
  upstreamBillingProbePlatforms.has(account.platform) &&
  (
    account.type === 'apikey' ||
    (account.type === 'upstream' && account.platform === 'antigravity')
  )
)
