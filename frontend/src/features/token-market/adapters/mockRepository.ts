import { applyMockExchange, calculateExchangeDestination, validateExchangeBalance } from '../domain'
import { tokenMarketMockBootstrap } from '../mockData'
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

const delay = (ms = 70) => new Promise<void>(resolve => window.setTimeout(resolve, ms))
const createId = (prefix: string) => `${prefix}_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`

const initialOrders: TokenMarketOrder[] = [
  { id:'TM202610010001', categoryId:'mall', productId:'p-headset', merchantId:'m-star', title:'星芒无线降噪耳机', amountToken:2880, status:'completed', statusText:'已确认收货', createdAt:'2026-10-01T14:32:00+08:00' },
  { id:'TM202610010002', categoryId:'delivery', productId:'p-night', merchantId:'m-night', title:'深夜限定套餐', amountToken:680, status:'fulfilling', statusText:'配送中', createdAt:'2026-10-01T12:08:00+08:00' },
  { id:'TM202609300087', categoryId:'digital', productId:'p-ai-art', merchantId:'m-studio', title:'AI 灵感创作包', amountToken:980, status:'completed', statusText:'自动交付完成', createdAt:'2026-09-30T22:16:00+08:00' },
  { id:'TM202609290041', categoryId:'services', merchantId:'m-studio', title:'AI 工作流部署服务', amountToken:8600, status:'after_sale', statusText:'售后处理中', createdAt:'2026-09-29T16:20:00+08:00' },
]
const initialExchanges: TokenMarketExchangeRecord[] = [
  { id:'EX202610010001', direction:'balance_to_token', sourceAmount:100, sourceUnit:'balance', destinationAmount:10000, destinationUnit:'token', tokenPerBalanceUnit:100, status:'success', createdAt:'2026-10-01T15:20:00+08:00' },
  { id:'EX202609300088', direction:'token_to_balance', sourceAmount:25000, sourceUnit:'token', destinationAmount:250, destinationUnit:'balance', tokenPerBalanceUnit:100, status:'success', createdAt:'2026-09-30T20:18:00+08:00' },
]

let state: TokenMarketBootstrap = structuredClone(tokenMarketMockBootstrap)
let orders: TokenMarketOrder[] = structuredClone(initialOrders)
let exchanges: TokenMarketExchangeRecord[] = structuredClone(initialExchanges)
let quotes = new Map<string, TokenExchangeQuote>()
let executions = new Map<string, { request: TokenExchangeExecuteRequest; result: TokenExchangeExecuteResult }>()

function page<T>(items: T[], pageNumber = 1, pageSize = 20): TokenMarketPageResult<T> {
  const page = Math.max(1, pageNumber)
  const size = Math.max(1, Math.min(100, pageSize))
  const start = (page - 1) * size
  return { items: structuredClone(items.slice(start, start + size)), total: items.length, page, pageSize: size, hasMore: start + size < items.length }
}

export class MockTokenMarketRepository implements TokenMarketRepository {
  async getBootstrap(): Promise<TokenMarketBootstrap> { await delay(); return structuredClone(state) }
  async listProducts(query: TokenMarketProductQuery = {}): Promise<TokenMarketPageResult<TokenMarketProduct>> {
    await delay()
    const q = query.query?.trim().toLocaleLowerCase()
    let items = state.products.filter(item => (!query.merchantId || item.merchantId === query.merchantId) && (!query.categoryId || item.categoryId === query.categoryId))
    if (q) items = items.filter(item => item.name.toLocaleLowerCase().includes(q))
    return page(items, query.page, query.pageSize)
  }
  async getProduct(id: string): Promise<TokenMarketProduct | null> { await delay(40); return structuredClone(state.products.find(item => item.id === id) ?? null) }
  async listOrders(query: TokenMarketOrderQuery = {}): Promise<TokenMarketPageResult<TokenMarketOrder>> {
    await delay()
    let items = query.status ? orders.filter(item => item.status === query.status) : orders
    if (query.query?.trim()) {
      const term = query.query.trim().toLocaleLowerCase()
      items = items.filter(item => item.id.toLocaleLowerCase().includes(term) || item.title.toLocaleLowerCase().includes(term))
    }
    return page(items, query.page, query.pageSize)
  }
  async getWallet(): Promise<TokenMarketWalletSnapshot> { await delay(45); return structuredClone(state.wallet) }
  async listExchangeHistory(query: TokenMarketExchangeHistoryQuery = {}): Promise<TokenMarketPageResult<TokenMarketExchangeRecord>> {
    await delay()
    return page(query.direction ? exchanges.filter(item => item.direction === query.direction) : exchanges, query.page, query.pageSize)
  }
  async quoteExchange(request: TokenExchangeQuoteRequest): Promise<TokenExchangeQuote> {
    await delay(50)
    const error = validateExchangeBalance(state.wallet, request.direction, request.sourceAmount)
    if (error) throw new Error(error)
    const quote = { quoteId:createId('quote'), direction:request.direction, sourceAmount:request.sourceAmount, destinationAmount:calculateExchangeDestination(request.direction, request.sourceAmount, state.exchangeRate), tokenPerBalanceUnit:state.exchangeRate, quotedAt:new Date().toISOString(), expiresAt:new Date(Date.now() + 60_000).toISOString(), basis:'演示比率，仅供界面预览', applicableModels:[], tokenTypes:[] }
    quotes.set(quote.quoteId, quote)
    return quote
  }
  async executeExchange(request: TokenExchangeExecuteRequest): Promise<TokenExchangeExecuteResult> {
    const previous = executions.get(request.idempotencyKey)
    if (previous) {
      if (previous.request.quoteId !== request.quoteId || previous.request.direction !== request.direction || previous.request.sourceAmount !== request.sourceAmount) throw new Error('幂等键已用于其他兑换')
      return structuredClone(previous.result)
    }
    const quote = quotes.get(request.quoteId)
    if (!quote || quote.direction !== request.direction || quote.sourceAmount !== request.sourceAmount || !quote.expiresAt || Date.parse(quote.expiresAt) <= Date.now()) throw new Error('演示报价不存在或已过期')
    const error = validateExchangeBalance(state.wallet, request.direction, request.sourceAmount)
    if (error) throw new Error(error)
    state.wallet = applyMockExchange(state.wallet, quote)
    const transactionId = createId('tx')
    exchanges = [{ id:transactionId, direction:quote.direction, sourceAmount:quote.sourceAmount, sourceUnit:quote.direction === 'balance_to_token' ? 'balance' : 'token', destinationAmount:quote.destinationAmount, destinationUnit:quote.direction === 'balance_to_token' ? 'token' : 'balance', tokenPerBalanceUnit:quote.tokenPerBalanceUnit, status:'success', createdAt:new Date().toISOString() }, ...exchanges]
    const result = { transactionId, wallet:structuredClone(state.wallet) }
    executions.set(request.idempotencyKey, { request: { ...request }, result })
    quotes.delete(quote.quoteId)
    return structuredClone(result)
  }
  async search(query: string): Promise<TokenMarketSearchResult> {
    await delay()
    const q = query.trim().toLocaleLowerCase()
    if (!q) return { products:[], merchants:[] }
    return { products:state.products.filter(item => item.name.toLocaleLowerCase().includes(q)), merchants:state.merchants.filter(item => item.name.toLocaleLowerCase().includes(q)) }
  }
}

export function resetTokenMarketMockRepository(): void {
  state = structuredClone(tokenMarketMockBootstrap)
  orders = structuredClone(initialOrders)
  exchanges = structuredClone(initialExchanges)
  quotes = new Map()
  executions = new Map()
}
