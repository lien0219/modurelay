import type {
  TokenMarketBootstrapDTO,
  TokenMarketExchangeRecordDTO,
  TokenMarketOrderDTO,
  TokenMarketPageDTO,
  TokenMarketProductDTO,
  TokenMarketQuoteDTO,
  TokenMarketWalletDTO,
} from './contracts'
import type {
  TokenExchangeQuote,
  TokenMarketBootstrap,
  TokenMarketExchangeRecord,
  TokenMarketOrder,
  TokenMarketPageResult,
  TokenMarketProduct,
  TokenMarketWalletSnapshot,
} from './types'

const asNumber = (value: string | number | null | undefined): number => {
  const numeric = Number(value ?? 0)
  if (!Number.isFinite(numeric)) throw new Error('Token Market API returned an invalid numeric value')
  return numeric
}

export function mapWallet(dto: TokenMarketWalletDTO): TokenMarketWalletSnapshot {
  return {
    platformBalance: asNumber(dto.platform_balance),
    tokenBalance: asNumber(dto.token_balance),
    frozenToken: asNumber(dto.frozen_token),
    pendingToken: asNumber(dto.pending_token),
    todaySpentToken: asNumber(dto.today_spent_token),
    yesterdayIncomeToken: asNumber(dto.yesterday_income_token),
    updatedAt: dto.updated_at,
  }
}

export function mapProduct(dto: TokenMarketProductDTO): TokenMarketProduct {
  return {
    id: dto.id,
    name: dto.name,
    categoryId: dto.category_id,
    priceToken: asNumber(dto.price_token),
    merchantId: dto.merchant_id,
    badge: dto.badge ?? undefined,
  }
}

export function mapOrder(dto: TokenMarketOrderDTO): TokenMarketOrder {
  return {
    id: dto.id,
    categoryId: dto.category_id,
    productId: dto.product_id ?? undefined,
    merchantId: dto.merchant_id ?? undefined,
    title: dto.title,
    amountToken: asNumber(dto.amount_token),
    status: dto.status,
    statusText: dto.status_text,
    createdAt: dto.created_at,
  }
}

export function mapExchangeRecord(dto: TokenMarketExchangeRecordDTO): TokenMarketExchangeRecord {
  return {
    id: dto.id,
    direction: dto.direction,
    sourceAmount: asNumber(dto.source_amount),
    sourceUnit: dto.source_unit,
    destinationAmount: asNumber(dto.destination_amount),
    destinationUnit: dto.destination_unit,
    tokenPerBalanceUnit: asNumber(dto.token_per_balance_unit),
    status: dto.status,
    createdAt: dto.created_at,
  }
}

export function mapQuote(dto: TokenMarketQuoteDTO): TokenExchangeQuote {
  return {
    quoteId: dto.quote_id,
    direction: dto.direction,
    sourceAmount: asNumber(dto.source_amount),
    destinationAmount: asNumber(dto.destination_amount),
    tokenPerBalanceUnit: asNumber(dto.token_per_balance_unit),
    quotedAt: dto.quoted_at,
  }
}

export function mapPage<TDTO, TDomain>(dto: TokenMarketPageDTO<TDTO>, mapper: (item: TDTO) => TDomain): TokenMarketPageResult<TDomain> {
  return {
    items: dto.items.map(mapper),
    total: dto.total,
    page: dto.page,
    pageSize: dto.page_size,
    hasMore: dto.has_more ?? dto.page * dto.page_size < dto.total,
  }
}

export function mapBootstrap(dto: TokenMarketBootstrapDTO): TokenMarketBootstrap {
  return {
    wallet: mapWallet(dto.wallet),
    categories: dto.categories.map(item => ({ id: item.id, label: item.label, description: item.description })),
    products: dto.products.map(mapProduct),
    merchants: dto.merchants.map(item => ({ id: item.id, name: item.name, categoryId: item.category_id, rating: item.rating, soldCount: item.sold_count })),
    activities: dto.activities.map(item => ({ id: item.id, title: item.title, detail: item.detail, tokenAmount: item.token_amount == null ? undefined : asNumber(item.token_amount), occurredAt: item.occurred_at })),
    exchangeRate: asNumber(dto.exchange_rate),
  }
}
