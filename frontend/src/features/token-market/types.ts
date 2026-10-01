export type TokenMarketCategoryId = 'mall' | 'delivery' | 'digital' | 'ai_credit' | 'services'
export type TokenExchangeDirection = 'balance_to_token' | 'token_to_balance'

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
}

export interface TokenExchangeExecuteRequest {
  quoteId: string
  direction: TokenExchangeDirection
  sourceAmount: number
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
  quoteExchange(request: TokenExchangeQuoteRequest): Promise<TokenExchangeQuote>
  executeExchange(request: TokenExchangeExecuteRequest): Promise<TokenExchangeExecuteResult>
  search(query: string): Promise<TokenMarketSearchResult>
}
