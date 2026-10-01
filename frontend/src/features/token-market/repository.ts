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
  TokenMarketWalletSnapshot,
} from './types'

export interface TokenMarketRepository {
  getBootstrap(): Promise<TokenMarketBootstrap>
  listProducts(query?: TokenMarketProductQuery): Promise<TokenMarketPageResult<TokenMarketProduct>>
  getProduct(id: string): Promise<TokenMarketProduct | null>
  listOrders(query?: TokenMarketOrderQuery): Promise<TokenMarketPageResult<TokenMarketOrder>>
  getWallet(): Promise<TokenMarketWalletSnapshot>
  listExchangeHistory(query?: TokenMarketExchangeHistoryQuery): Promise<TokenMarketPageResult<TokenMarketExchangeRecord>>
  quoteExchange(request: TokenExchangeQuoteRequest): Promise<TokenExchangeQuote>
  executeExchange(request: TokenExchangeExecuteRequest): Promise<TokenExchangeExecuteResult>
  search(query: string): Promise<TokenMarketSearchResult>
}
