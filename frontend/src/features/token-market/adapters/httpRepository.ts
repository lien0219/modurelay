import { apiClient } from '@/api/client'
import type {
  TokenMarketBootstrapDTO,
  TokenMarketExchangeRecordDTO,
  TokenMarketOrderDTO,
  TokenMarketPageDTO,
  TokenMarketProductDTO,
  TokenMarketQuoteDTO,
  TokenMarketExecuteDTO,
  TokenMarketWalletDTO,
} from '../contracts'
import { mapBootstrap, mapExchangeRecord, mapOrder, mapPage, mapProduct, mapQuote, mapWallet } from '../mappers'
import type { TokenMarketRepository } from '../repository'
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
} from '../types'

const BASE = '/token-market'

export class HttpTokenMarketRepository implements TokenMarketRepository {
  async getBootstrap(): Promise<TokenMarketBootstrap> {
    const { data } = await apiClient.get<TokenMarketBootstrapDTO>(`${BASE}/bootstrap`)
    return mapBootstrap(data)
  }

  async listProducts(query: TokenMarketProductQuery = {}): Promise<TokenMarketPageResult<TokenMarketProduct>> {
    const { data } = await apiClient.get<TokenMarketPageDTO<TokenMarketProductDTO>>(`${BASE}/products`, {
      params: {
        page: query.page,
        page_size: query.pageSize,
        merchant_id: query.merchantId,
        category_id: query.categoryId,
        q: query.query,
      },
    })
    return mapPage(data, mapProduct)
  }

  async getProduct(id: string): Promise<TokenMarketProduct | null> {
    try {
      const { data } = await apiClient.get<TokenMarketProductDTO>(`${BASE}/products/${encodeURIComponent(id)}`)
      return mapProduct(data)
    } catch (error: any) {
      if (error?.status === 404) return null
      throw error
    }
  }

  async listOrders(query: TokenMarketOrderQuery = {}): Promise<TokenMarketPageResult<TokenMarketOrder>> {
    const { data } = await apiClient.get<TokenMarketPageDTO<TokenMarketOrderDTO>>(`${BASE}/orders`, {
      params: { page: query.page, page_size: query.pageSize, status: query.status, q: query.query },
    })
    return mapPage(data, mapOrder)
  }

  async getWallet(): Promise<TokenMarketWalletSnapshot> {
    const { data } = await apiClient.get<TokenMarketWalletDTO>(`${BASE}/wallet`)
    return mapWallet(data)
  }

  async listExchangeHistory(query: TokenMarketExchangeHistoryQuery = {}): Promise<TokenMarketPageResult<TokenMarketExchangeRecord>> {
    const { data } = await apiClient.get<TokenMarketPageDTO<TokenMarketExchangeRecordDTO>>(`${BASE}/exchange/history`, {
      params: { page: query.page, page_size: query.pageSize, direction: query.direction },
    })
    return mapPage(data, mapExchangeRecord)
  }

  async quoteExchange(request: TokenExchangeQuoteRequest): Promise<TokenExchangeQuote> {
    const { data } = await apiClient.post<TokenMarketQuoteDTO>(`${BASE}/exchange/quote`, {
      direction: request.direction,
      source_amount: request.sourceAmount,
    })
    return mapQuote(data)
  }

  async executeExchange(request: TokenExchangeExecuteRequest): Promise<TokenExchangeExecuteResult> {
    const { data } = await apiClient.post<TokenMarketExecuteDTO>(`${BASE}/exchange/execute`, {
      quote_id: request.quoteId,
      direction: request.direction,
      source_amount: request.sourceAmount,
    }, { headers: { 'Idempotency-Key': request.idempotencyKey } })
    return { transactionId: data.transaction_id, wallet: mapWallet(data.wallet) }
  }

  async search(query: string): Promise<TokenMarketSearchResult> {
    const { data } = await apiClient.get<{ products: TokenMarketProductDTO[]; merchants: TokenMarketBootstrapDTO['merchants'] }>(`${BASE}/search`, { params: { q: query } })
    return {
      products: data.products.map(mapProduct),
      merchants: data.merchants.map(item => ({ id:item.id, name:item.name, categoryId:item.category_id, rating:item.rating, soldCount:item.sold_count })),
    }
  }
}
