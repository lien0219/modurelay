import { beforeEach, describe, expect, it, vi } from 'vitest'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))

import { HttpTokenMarketRepository } from '../adapters/httpRepository'
import { MockTokenMarketRepository, resetTokenMarketMockRepository } from '../adapters/mockRepository'
import { mapOrder, mapWallet } from '../mappers'

describe('token market commerce repository', () => {
  beforeEach(() => resetTokenMarketMockRepository())

  it('maps API numeric strings into stable domain numbers', () => {
    const wallet = mapWallet({
      platform_balance: '12.5', token_balance: '1000', frozen_token: '5', pending_token: '8',
      today_spent_token: '20', yesterday_income_token: '30', updated_at: '2026-10-01T00:00:00Z',
    })
    expect(wallet.platformBalance).toBe(12.5)
    expect(wallet.tokenBalance).toBe(1000)
  })

  it('maps order wire contracts without leaking snake_case into pages', () => {
    const order = mapOrder({
      id: 'o1', category_id: 'mall', product_id: 'p1', merchant_id: 'm1', title: '商品',
      amount_token: '1880', status: 'completed', status_text: '已完成', created_at: '2026-10-01T00:00:00Z',
    })
    expect(order.amountToken).toBe(1880)
    expect(order.productId).toBe('p1')
  })

  it('supports product, order, wallet and exchange use cases through one repository contract', async () => {
    const repository = new MockTokenMarketRepository()
    expect((await repository.listProducts()).items.length).toBeGreaterThan(0)
    expect((await repository.listOrders({ status: 'completed' })).items.every(item => item.status === 'completed')).toBe(true)
    expect((await repository.getWallet()).tokenBalance).toBeGreaterThan(0)
    expect((await repository.listExchangeHistory()).items.length).toBeGreaterThan(0)
  })

  it('writes exchange execution back to wallet and exchange history atomically in the adapter', async () => {
    const repository = new MockTokenMarketRepository()
    const before = await repository.getWallet()
    const quote = await repository.quoteExchange({ direction: 'balance_to_token', sourceAmount: 10 })
    const request = { quoteId: quote.quoteId, direction: quote.direction, sourceAmount: quote.sourceAmount, idempotencyKey: 'exchange-test-operation' }
    const result = await repository.executeExchange(request)
    const repeated = await repository.executeExchange(request)
    const history = await repository.listExchangeHistory()
    expect(result.wallet.platformBalance).toBe(before.platformBalance - 10)
    expect(repeated.transactionId).toBe(result.transactionId)
    expect((await repository.getWallet()).platformBalance).toBe(before.platformBalance - 10)
    expect(history.items[0]?.sourceAmount).toBe(10)
    expect(history.items.filter(item => item.id === result.transactionId)).toHaveLength(1)
  })

  it('sends the same operation key in the execute request header', async () => {
    post.mockResolvedValue({ data: { transaction_id: 'tx-1', wallet: {
      platform_balance: '90', token_balance: '11000', frozen_token: '0', pending_token: '0',
      today_spent_token: '0', yesterday_income_token: '0', updated_at: '2026-10-01T00:00:00Z',
    } } })
    await new HttpTokenMarketRepository().executeExchange({ quoteId: 'quote-1', direction: 'balance_to_token', sourceAmount: 10, idempotencyKey: 'exchange-test-operation' })
    expect(post).toHaveBeenCalledWith('/token-market/exchange/execute', {
      quote_id: 'quote-1', direction: 'balance_to_token', source_amount: 10,
    }, { headers: { 'Idempotency-Key': 'exchange-test-operation' } })
  })
})
