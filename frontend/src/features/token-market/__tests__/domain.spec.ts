import { describe, expect, it } from 'vitest'
import {
  applyMockExchange,
  calculateExchangeDestination,
  normalizeAmount,
  validateExchangeBalance,
} from '../domain'
import type { TokenExchangeQuote, TokenMarketWalletSnapshot } from '../types'

const wallet: TokenMarketWalletSnapshot = {
  platformBalance: 1250,
  tokenBalance: 128520,
  frozenToken: 0,
  pendingToken: 4500,
  todaySpentToken: 2380,
  yesterdayIncomeToken: 12600,
  updatedAt: '2026-10-01T00:00:00.000Z',
}

describe('token-market exchange domain', () => {
  it('normalizes invalid and high precision amounts', () => {
    expect(normalizeAmount(Number.NaN)).toBe(0)
    expect(normalizeAmount(-1)).toBe(0)
    expect(normalizeAmount(1.123456789)).toBe(1.123457)
  })

  it('calculates both exchange directions using the configured rate', () => {
    expect(calculateExchangeDestination('balance_to_token', 100, 100)).toBe(10000)
    expect(calculateExchangeDestination('token_to_balance', 10000, 100)).toBe(100)
  })

  it('rejects zero and insufficient balances', () => {
    expect(validateExchangeBalance(wallet, 'balance_to_token', 0)).toBe('请输入有效兑换数量')
    expect(validateExchangeBalance(wallet, 'balance_to_token', 1250.01)).toBe('平台余额不足')
    expect(validateExchangeBalance(wallet, 'token_to_balance', 128521)).toBe('Token 余额不足')
    expect(validateExchangeBalance(wallet, 'token_to_balance', 10000)).toBeNull()
  })

  it('applies balance-to-token exchange without mutating the source wallet', () => {
    const quote: TokenExchangeQuote = {
      direction: 'balance_to_token',
      sourceAmount: 100,
      destinationAmount: 10000,
      tokenPerBalanceUnit: 100,
      quotedAt: '2026-10-01T00:00:00.000Z',
      quoteId: 'q1',
    }
    const next = applyMockExchange(wallet, quote)
    expect(next.platformBalance).toBe(1150)
    expect(next.tokenBalance).toBe(138520)
    expect(wallet.platformBalance).toBe(1250)
    expect(wallet.tokenBalance).toBe(128520)
  })

  it('applies token-to-balance exchange symmetrically', () => {
    const quote: TokenExchangeQuote = {
      direction: 'token_to_balance',
      sourceAmount: 10000,
      destinationAmount: 100,
      tokenPerBalanceUnit: 100,
      quotedAt: '2026-10-01T00:00:00.000Z',
      quoteId: 'q2',
    }
    const next = applyMockExchange(wallet, quote)
    expect(next.tokenBalance).toBe(118520)
    expect(next.platformBalance).toBe(1350)
  })
})
