import type { TokenExchangeDirection, TokenMarketCategoryId, TokenMarketExchangeStatus, TokenMarketOrderStatus } from './types'

export interface TokenMarketProductDTO {
  id: string
  name: string
  category_id: TokenMarketCategoryId
  price_token: string | number
  merchant_id: string
  badge?: string | null
}

export interface TokenMarketOrderDTO {
  id: string
  category_id: TokenMarketCategoryId
  product_id?: string | null
  merchant_id?: string | null
  title: string
  amount_token: string | number
  status: TokenMarketOrderStatus
  status_text: string
  created_at: string
}

export interface TokenMarketWalletDTO {
  platform_balance: string | number
  token_balance: string | number
  frozen_token: string | number
  pending_token: string | number
  today_spent_token: string | number
  yesterday_income_token: string | number
  updated_at: string
}

export interface TokenMarketExchangeRecordDTO {
  id: string
  direction: TokenExchangeDirection
  source_amount: string | number
  source_unit: 'balance' | 'token'
  destination_amount: string | number
  destination_unit: 'balance' | 'token'
  token_per_balance_unit: string | number
  status: TokenMarketExchangeStatus
  created_at: string
}

export interface TokenMarketQuoteDTO {
  quote_id: string
  direction: TokenExchangeDirection
  source_amount: string | number
  destination_amount: string | number
  token_per_balance_unit: string | number
  quoted_at: string
}

export interface TokenMarketExecuteDTO {
  transaction_id: string
  wallet: TokenMarketWalletDTO
}

export interface TokenMarketPageDTO<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  has_more?: boolean
}

export interface TokenMarketBootstrapDTO {
  wallet: TokenMarketWalletDTO
  categories: Array<{ id: TokenMarketCategoryId; label: string; description: string }>
  products: TokenMarketProductDTO[]
  merchants: Array<{ id: string; name: string; category_id: TokenMarketCategoryId; rating: number; sold_count: number }>
  activities: Array<{ id: string; title: string; detail: string; token_amount?: string | number | null; occurred_at: string }>
  exchange_rate: string | number
}
