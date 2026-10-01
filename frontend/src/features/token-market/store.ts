import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { tokenMarketService } from './service'
import type {
  TokenExchangeDirection,
  TokenExchangeQuote,
  TokenMarketBootstrap,
  TokenMarketExchangeHistoryQuery,
  TokenMarketExchangeRecord,
  TokenMarketMerchant,
  TokenMarketOrder,
  TokenMarketOrderQuery,
  TokenMarketProduct,
  TokenMarketProductQuery,
  TokenMarketWalletSnapshot,
} from './types'

export const useTokenMarketStore = defineStore('token-market', () => {
  const bootstrap = ref<TokenMarketBootstrap | null>(null)
  const loading = ref(false)
  const exchanging = ref(false)
  const searchLoading = ref(false)
  const resourceLoading = ref(false)
  const activeQuote = ref<TokenExchangeQuote | null>(null)
  const activeProduct = ref<TokenMarketProduct | null>(null)
  const orderItems = ref<TokenMarketOrder[]>([])
  const exchangeHistory = ref<TokenMarketExchangeRecord[]>([])
  const walletSnapshot = ref<TokenMarketWalletSnapshot | null>(null)
  const searchProducts = ref<TokenMarketProduct[]>([])
  const searchMerchants = ref<TokenMarketMerchant[]>([])
  const lastError = ref<string | null>(null)

  const wallet = computed(() => walletSnapshot.value ?? bootstrap.value?.wallet ?? null)
  const categories = computed(() => bootstrap.value?.categories ?? [])
  const products = computed(() => bootstrap.value?.products ?? [])
  const merchants = computed(() => bootstrap.value?.merchants ?? [])
  const activities = computed(() => bootstrap.value?.activities ?? [])
  const exchangeRate = computed(() => bootstrap.value?.exchangeRate ?? 100)

  async function initialize(force = false): Promise<void> {
    if (bootstrap.value && !force) return
    loading.value = true
    lastError.value = null
    try {
      bootstrap.value = await tokenMarketService.getBootstrap()
      walletSnapshot.value = bootstrap.value.wallet
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : 'Token 市场加载失败'
      throw error
    } finally {
      loading.value = false
    }
  }

  async function loadProduct(id: string): Promise<TokenMarketProduct | null> {
    resourceLoading.value = true
    lastError.value = null
    try {
      activeProduct.value = await tokenMarketService.getProduct(id)
      return activeProduct.value
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : '商品加载失败'
      throw error
    } finally { resourceLoading.value = false }
  }

  async function loadProducts(query?: TokenMarketProductQuery): Promise<TokenMarketProduct[]> {
    resourceLoading.value = true
    lastError.value = null
    try { return (await tokenMarketService.listProducts(query)).items }
    catch (error) { lastError.value = error instanceof Error ? error.message : '商品加载失败'; throw error }
    finally { resourceLoading.value = false }
  }

  async function loadOrders(query?: TokenMarketOrderQuery): Promise<TokenMarketOrder[]> {
    resourceLoading.value = true
    lastError.value = null
    try { orderItems.value = (await tokenMarketService.listOrders(query)).items; return orderItems.value }
    catch (error) { lastError.value = error instanceof Error ? error.message : '订单加载失败'; throw error }
    finally { resourceLoading.value = false }
  }

  async function loadWallet(): Promise<TokenMarketWalletSnapshot> {
    resourceLoading.value = true
    lastError.value = null
    try { walletSnapshot.value = await tokenMarketService.getWallet(); return walletSnapshot.value }
    catch (error) { lastError.value = error instanceof Error ? error.message : '钱包加载失败'; throw error }
    finally { resourceLoading.value = false }
  }

  async function loadExchangeHistory(query?: TokenMarketExchangeHistoryQuery): Promise<TokenMarketExchangeRecord[]> {
    resourceLoading.value = true
    lastError.value = null
    try { exchangeHistory.value = (await tokenMarketService.listExchangeHistory(query)).items; return exchangeHistory.value }
    catch (error) { lastError.value = error instanceof Error ? error.message : '兑换记录加载失败'; throw error }
    finally { resourceLoading.value = false }
  }

  async function quoteExchange(direction: TokenExchangeDirection, sourceAmount: number): Promise<TokenExchangeQuote> {
    lastError.value = null
    try {
      activeQuote.value = await tokenMarketService.quoteExchange({ direction, sourceAmount })
      return activeQuote.value
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : '兑换报价失败'
      throw error
    }
  }

  async function executeExchange(): Promise<void> {
    if (!activeQuote.value) throw new Error('兑换报价不存在或已失效')
    exchanging.value = true
    lastError.value = null
    try {
      const result = await tokenMarketService.executeExchange({ quoteId: activeQuote.value.quoteId, direction: activeQuote.value.direction, sourceAmount: activeQuote.value.sourceAmount })
      walletSnapshot.value = result.wallet
      if (bootstrap.value) bootstrap.value.wallet = result.wallet
      activeQuote.value = null
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : '兑换失败'
      throw error
    } finally { exchanging.value = false }
  }

  async function search(query: string): Promise<void> {
    searchLoading.value = true
    lastError.value = null
    try {
      const result = await tokenMarketService.search(query)
      searchProducts.value = result.products
      searchMerchants.value = result.merchants
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : '搜索失败'
      throw error
    } finally { searchLoading.value = false }
  }

  function clearQuote(): void { activeQuote.value = null }

  return {
    bootstrap, loading, exchanging, searchLoading, resourceLoading, activeQuote, activeProduct, orderItems, exchangeHistory, walletSnapshot,
    searchProducts, searchMerchants, lastError, wallet, categories, products, merchants, activities, exchangeRate,
    initialize, loadProduct, loadProducts, loadOrders, loadWallet, loadExchangeHistory, quoteExchange, executeExchange, search, clearQuote,
  }
})
