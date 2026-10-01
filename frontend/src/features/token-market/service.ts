import { applyMockExchange, calculateExchangeDestination, validateExchangeBalance } from './domain'
import { tokenMarketMockBootstrap } from './mockData'
import type {
  TokenExchangeExecuteRequest,
  TokenExchangeExecuteResult,
  TokenExchangeQuote,
  TokenExchangeQuoteRequest,
  TokenMarketBootstrap,
  TokenMarketSearchResult,
  TokenMarketService,
} from './types'

const delay = (ms = 80) => new Promise<void>((resolve) => window.setTimeout(resolve, ms))

let state: TokenMarketBootstrap = structuredClone(tokenMarketMockBootstrap)

function createId(prefix: string): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`
}

class MockTokenMarketService implements TokenMarketService {
  async getBootstrap(): Promise<TokenMarketBootstrap> {
    await delay()
    return structuredClone(state)
  }

  async quoteExchange(request: TokenExchangeQuoteRequest): Promise<TokenExchangeQuote> {
    await delay(60)
    const error = validateExchangeBalance(state.wallet, request.direction, request.sourceAmount)
    if (error) throw new Error(error)

    return {
      direction: request.direction,
      sourceAmount: request.sourceAmount,
      destinationAmount: calculateExchangeDestination(request.direction, request.sourceAmount, state.exchangeRate),
      tokenPerBalanceUnit: state.exchangeRate,
      quotedAt: new Date().toISOString(),
      quoteId: createId('quote'),
    }
  }

  async executeExchange(request: TokenExchangeExecuteRequest): Promise<TokenExchangeExecuteResult> {
    await delay(100)
    const quote = await this.quoteExchange({
      direction: request.direction,
      sourceAmount: request.sourceAmount,
    })
    state.wallet = applyMockExchange(state.wallet, quote)
    return {
      transactionId: createId('tx'),
      wallet: structuredClone(state.wallet),
    }
  }

  async search(query: string): Promise<TokenMarketSearchResult> {
    await delay(70)
    const q = query.trim().toLocaleLowerCase()
    if (!q) return { products: [], merchants: [] }
    return {
      products: state.products.filter((item) => item.name.toLocaleLowerCase().includes(q)),
      merchants: state.merchants.filter((item) => item.name.toLocaleLowerCase().includes(q)),
    }
  }
}

export const tokenMarketService: TokenMarketService = new MockTokenMarketService()

/**
 * Swap this export to an HTTP-backed implementation when the backend contracts
 * are ready. UI/store code should depend only on TokenMarketService.
 */
export function resetTokenMarketMockState(): void {
  state = structuredClone(tokenMarketMockBootstrap)
}
