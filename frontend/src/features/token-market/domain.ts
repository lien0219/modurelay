import type {
  TokenExchangeDirection,
  TokenExchangeQuote,
  TokenMarketWalletSnapshot,
} from './types'

export const DEFAULT_TOKEN_PER_BALANCE_UNIT = 100

export function normalizeAmount(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0
  return Math.round(value * 1000000) / 1000000
}

export function calculateExchangeDestination(
  direction: TokenExchangeDirection,
  sourceAmount: number,
  tokenPerBalanceUnit = DEFAULT_TOKEN_PER_BALANCE_UNIT,
): number {
  const amount = normalizeAmount(sourceAmount)
  if (amount === 0) return 0
  return direction === 'balance_to_token'
    ? amount * tokenPerBalanceUnit
    : amount / tokenPerBalanceUnit
}

export function validateExchangeBalance(
  wallet: TokenMarketWalletSnapshot,
  direction: TokenExchangeDirection,
  sourceAmount: number,
): string | null {
  const amount = normalizeAmount(sourceAmount)
  if (amount <= 0) return '请输入有效兑换数量'
  if (direction === 'balance_to_token' && amount > wallet.platformBalance) return '平台余额不足'
  if (direction === 'token_to_balance' && amount > wallet.tokenBalance) return 'Token 余额不足'
  return null
}

export function applyMockExchange(
  wallet: TokenMarketWalletSnapshot,
  quote: TokenExchangeQuote,
): TokenMarketWalletSnapshot {
  const next = { ...wallet, updatedAt: new Date().toISOString() }
  if (quote.direction === 'balance_to_token') {
    next.platformBalance = normalizeAmount(next.platformBalance - quote.sourceAmount)
    next.tokenBalance = normalizeAmount(next.tokenBalance + quote.destinationAmount)
  } else {
    next.tokenBalance = normalizeAmount(next.tokenBalance - quote.sourceAmount)
    next.platformBalance = normalizeAmount(next.platformBalance + quote.destinationAmount)
  }
  return next
}
