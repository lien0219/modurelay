export type TokenMarketCategoryId = 'mall' | 'delivery' | 'digital' | 'ai_credit' | 'services'
export type TokenExchangeDirection = 'balance_to_token' | 'token_to_balance'
export type TokenMarketOrderStatus = 'pending_payment' | 'fulfilling' | 'completed' | 'after_sale' | 'cancelled'
export type TokenMarketExchangeStatus = 'pending' | 'success' | 'failed'
export type TokenMarketMoneyUnit = 'balance' | 'token'

export interface TokenMarketPageResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
  hasMore: boolean
}

export interface TokenMarketWalletSnapshot {
  platformBalance: number
  tokenBalance: number
  frozenToken: number
  pendingToken: number
  todaySpentToken: number
  yesterdayIncomeToken: number
  updatedAt: string
}

export interface TokenMarketCategory {
  id: TokenMarketCategoryId
  label: string
  description: string
}

export interface TokenMarketProduct {
  id: string
  name: string
  categoryId: TokenMarketCategoryId
  priceToken: number
  merchantId: string
  badge?: string
  modelId?: string
  tokenTypes?: Array<'input' | 'output' | 'cache'>
  modelTokenQuantity?: string
  packageCombination?: string
}

export interface TokenMarketMerchant {
  id: string
  name: string
  categoryId: TokenMarketCategoryId
  rating: number
  soldCount: number
}

export interface TokenMarketActivity {
  id: string
  title: string
  detail: string
  tokenAmount?: number
  occurredAt: string
}

export interface TokenMarketOrder {
  id: string
  categoryId: TokenMarketCategoryId
  productId?: string
  merchantId?: string
  title: string
  amountToken: number
  status: TokenMarketOrderStatus
  statusText: string
  createdAt: string
}

export interface TokenMarketExchangeRecord {
  id: string
  direction: TokenExchangeDirection
  sourceAmount: number
  sourceUnit: TokenMarketMoneyUnit
  destinationAmount: number
  destinationUnit: TokenMarketMoneyUnit
  tokenPerBalanceUnit: number
  status: TokenMarketExchangeStatus
  createdAt: string
}

export interface TokenMarketProductQuery {
  page?: number
  pageSize?: number
  merchantId?: string
  categoryId?: TokenMarketCategoryId
  query?: string
}

export interface TokenMarketOrderQuery {
  page?: number
  pageSize?: number
  status?: TokenMarketOrderStatus
  query?: string
}

export interface TokenMarketExchangeHistoryQuery {
  page?: number
  pageSize?: number
  direction?: TokenExchangeDirection
}

export interface TokenMarketBootstrap {
  wallet: TokenMarketWalletSnapshot
  categories: TokenMarketCategory[]
  products: TokenMarketProduct[]
  merchants: TokenMarketMerchant[]
  activities: TokenMarketActivity[]
  exchangeRate: number
}

export interface TokenExchangeQuoteRequest {
  direction: TokenExchangeDirection
  sourceAmount: number
}

export interface TokenExchangeQuote {
  direction: TokenExchangeDirection
  sourceAmount: number
  destinationAmount: number
  tokenPerBalanceUnit: number
  quotedAt: string
  quoteId: string
  basis?: string
  applicableModels?: string[]
  tokenTypes?: string[]
  updatedAt?: string
  expiresAt?: string
}

export interface TokenExchangeExecuteRequest {
  quoteId: string
  direction: TokenExchangeDirection
  sourceAmount: number
  idempotencyKey: string
}

export interface TokenExchangeExecuteResult {
  transactionId: string
  wallet: TokenMarketWalletSnapshot
}

export interface TokenMarketSearchResult {
  products: TokenMarketProduct[]
  merchants: TokenMarketMerchant[]
}

export interface TokenMarketService {
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
