import { HttpTokenMarketRepository } from './adapters/httpRepository'
import { MockTokenMarketRepository, resetTokenMarketMockRepository } from './adapters/mockRepository'
import type { TokenMarketRepository } from './repository'
import type {
  TokenExchangeExecuteRequest,
  TokenExchangeExecuteResult,
  TokenExchangeQuote,
  TokenExchangeQuoteRequest,
  TokenMarketBootstrap,
  TokenMarketExchangeHistoryQuery,
  TokenMarketExchangeRecord,
  TokenMarketOrder,
  TokenMarketOrderQuery,
  TokenMarketPageResult,
  TokenMarketProduct,
  TokenMarketProductQuery,
  TokenMarketSearchResult,
  TokenMarketService,
  TokenMarketWalletSnapshot,
} from './types'

export class CommerceTokenMarketService implements TokenMarketService {
  constructor(private readonly repository: TokenMarketRepository) {}

  getBootstrap(): Promise<TokenMarketBootstrap> { return this.repository.getBootstrap() }
  listProducts(query?: TokenMarketProductQuery): Promise<TokenMarketPageResult<TokenMarketProduct>> { return this.repository.listProducts(query) }
  getProduct(id: string): Promise<TokenMarketProduct | null> { return this.repository.getProduct(id) }
  listOrders(query?: TokenMarketOrderQuery): Promise<TokenMarketPageResult<TokenMarketOrder>> { return this.repository.listOrders(query) }
  getWallet(): Promise<TokenMarketWalletSnapshot> { return this.repository.getWallet() }
  listExchangeHistory(query?: TokenMarketExchangeHistoryQuery): Promise<TokenMarketPageResult<TokenMarketExchangeRecord>> { return this.repository.listExchangeHistory(query) }
  quoteExchange(request: TokenExchangeQuoteRequest): Promise<TokenExchangeQuote> { return this.repository.quoteExchange(request) }
  executeExchange(request: TokenExchangeExecuteRequest): Promise<TokenExchangeExecuteResult> { return this.repository.executeExchange(request) }
  search(query: string): Promise<TokenMarketSearchResult> { return this.repository.search(query) }
}

export type TokenMarketDataSource = 'mock' | 'http'

export function createTokenMarketRepository(source: TokenMarketDataSource = resolveDataSource()): TokenMarketRepository {
  return source === 'http' ? new HttpTokenMarketRepository() : new MockTokenMarketRepository()
}

export function createTokenMarketService(source?: TokenMarketDataSource): TokenMarketService {
  return new CommerceTokenMarketService(createTokenMarketRepository(source))
}

function resolveDataSource(): TokenMarketDataSource {
  return import.meta.env.VITE_TOKEN_MARKET_DATA_SOURCE === 'http' ? 'http' : 'mock'
}

export const tokenMarketService: TokenMarketService = createTokenMarketService()

export function resetTokenMarketMockState(): void {
  resetTokenMarketMockRepository()
}
