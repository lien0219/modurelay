import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { updateProfile } from '@/api/user'
import { useAuthStore } from '@/stores/auth'
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
  const auth = useAuthStore()
  let accountVersion = 0
  let quoteVersion = 0
  const bootstrap = ref<TokenMarketBootstrap | null>(null)
  const loading = ref(false)
  const exchanging = ref(false)
  const searchLoading = ref(false)
  const resourceLoading = ref(false)
  const activeQuote = ref<TokenExchangeQuote | null>(null)
  const executionKey = ref<string | null>(null)
  const activeProduct = ref<TokenMarketProduct | null>(null)
  const orderItems = ref<TokenMarketOrder[]>([])
  const orderTotal = ref(0)
  const orderHasMore = ref(false)
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

  watch(() => auth.user?.id, () => {
    accountVersion++
    quoteVersion++
    bootstrap.value = null
    walletSnapshot.value = null
    activeQuote.value = null
    executionKey.value = null
    activeProduct.value = null
    orderItems.value = []
    orderTotal.value = 0
    orderHasMore.value = false
    exchangeHistory.value = []
    searchProducts.value = []
    searchMerchants.value = []
    lastError.value = null
    loading.value = false
    resourceLoading.value = false
    searchLoading.value = false
    exchanging.value = false
  }, { flush: 'sync' })

  async function initialize(force = false): Promise<void> {
    if (bootstrap.value && !force) return
    const version = accountVersion
    loading.value = true
    lastError.value = null
    try {
      const result = await tokenMarketService.getBootstrap()
      if (version !== accountVersion) return
      bootstrap.value = result
      walletSnapshot.value = bootstrap.value.wallet
    } catch (error) {
      if (version === accountVersion) lastError.value = error instanceof Error ? error.message : 'Token 市场加载失败'
      throw error
    } finally {
      if (version === accountVersion) loading.value = false
    }
  }

  async function loadProduct(id: string): Promise<TokenMarketProduct | null> {
    const version = accountVersion
    resourceLoading.value = true
    lastError.value = null
    try {
      const result = await tokenMarketService.getProduct(id)
      if (version !== accountVersion) return null
      activeProduct.value = result
      return activeProduct.value
    } catch (error) {
      if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '商品加载失败'
      throw error
    } finally { if (version === accountVersion) resourceLoading.value = false }
  }

  async function loadProducts(query?: TokenMarketProductQuery): Promise<TokenMarketProduct[]> {
    const version = accountVersion
    resourceLoading.value = true
    lastError.value = null
    try { const result = await tokenMarketService.listProducts(query); return version === accountVersion ? result.items : [] }
    catch (error) { if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '商品加载失败'; throw error }
    finally { if (version === accountVersion) resourceLoading.value = false }
  }

  async function loadOrders(query?: TokenMarketOrderQuery): Promise<TokenMarketOrder[]> {
    const version = accountVersion
    resourceLoading.value = true
    lastError.value = null
    try {
      const result = await tokenMarketService.listOrders(query)
      if (version !== accountVersion) return []
      orderItems.value = result.items
      orderTotal.value = result.total
      orderHasMore.value = result.hasMore
      return orderItems.value
    }
    catch (error) { if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '订单加载失败'; throw error }
    finally { if (version === accountVersion) resourceLoading.value = false }
  }

  async function loadWallet(): Promise<TokenMarketWalletSnapshot> {
    const version = accountVersion
    resourceLoading.value = true
    lastError.value = null
    try { const result = await tokenMarketService.getWallet(); if (version !== accountVersion) throw new Error('登录账户已变更'); walletSnapshot.value = result; return result }
    catch (error) { if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '钱包加载失败'; throw error }
    finally { if (version === accountVersion) resourceLoading.value = false }
  }

  async function loadExchangeHistory(query?: TokenMarketExchangeHistoryQuery): Promise<TokenMarketExchangeRecord[]> {
    const version = accountVersion
    resourceLoading.value = true
    lastError.value = null
    try { const result = await tokenMarketService.listExchangeHistory(query); if (version !== accountVersion) return []; exchangeHistory.value = result.items; return result.items }
    catch (error) { if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '兑换记录加载失败'; throw error }
    finally { if (version === accountVersion) resourceLoading.value = false }
  }

  async function quoteExchange(direction: TokenExchangeDirection, sourceAmount: number): Promise<TokenExchangeQuote> {
    const version = accountVersion
    const requestVersion = ++quoteVersion
    lastError.value = null
    activeQuote.value = null
    executionKey.value = null
    try {
      const result = await tokenMarketService.quoteExchange({ direction, sourceAmount })
      if (version !== accountVersion || requestVersion !== quoteVersion) throw new Error('报价条件已变更，请重新获取')
      activeQuote.value = result
      executionKey.value = crypto.randomUUID()
      return activeQuote.value
    } catch (error) {
      if (version === accountVersion && requestVersion === quoteVersion) lastError.value = error instanceof Error ? error.message : '兑换报价失败'
      throw error
    }
  }

  async function executeExchange(): Promise<void> {
    if (exchanging.value) throw new Error('兑换正在处理，请勿重复提交')
    if (!activeQuote.value || !executionKey.value || !activeQuote.value.expiresAt || Date.parse(activeQuote.value.expiresAt) <= Date.now()) {
      throw new Error('兑换报价不存在或已失效')
    }
    exchanging.value = true
    const version = accountVersion
    lastError.value = null
    try {
      const result = await tokenMarketService.executeExchange({ quoteId: activeQuote.value.quoteId, direction: activeQuote.value.direction, sourceAmount: activeQuote.value.sourceAmount, idempotencyKey: executionKey.value })
      if (version !== accountVersion) throw new Error('登录账户已变更，请核对兑换记录')
      walletSnapshot.value = result.wallet
      if (bootstrap.value) bootstrap.value.wallet = result.wallet
      activeQuote.value = null
      executionKey.value = null
    } catch (error) {
      if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '兑换失败'
      throw error
    } finally { if (version === accountVersion) exchanging.value = false }
  }

  async function search(query: string): Promise<void> {
    const version = accountVersion
    searchLoading.value = true
    lastError.value = null
    try {
      const result = await tokenMarketService.search(query)
      if (version !== accountVersion) return
      searchProducts.value = result.products
      searchMerchants.value = result.merchants
    } catch (error) {
      if (version === accountVersion) lastError.value = error instanceof Error ? error.message : '搜索失败'
      throw error
    } finally { if (version === accountVersion) searchLoading.value = false }
  }

  async function savePlatformUsername(username: string): Promise<void> {
    await updateProfile({ username })
    await auth.refreshUser()
  }

  function clearQuote(): void { quoteVersion++; activeQuote.value = null; executionKey.value = null }

  return {
    bootstrap, loading, exchanging, searchLoading, resourceLoading, activeQuote, activeProduct, orderItems, orderTotal, orderHasMore, exchangeHistory, walletSnapshot,
    searchProducts, searchMerchants, lastError, wallet, categories, products, merchants, activities, exchangeRate,
    initialize, loadProduct, loadProducts, loadOrders, loadWallet, loadExchangeHistory, quoteExchange, executeExchange, search, savePlatformUsername, clearQuote,
  }
})
