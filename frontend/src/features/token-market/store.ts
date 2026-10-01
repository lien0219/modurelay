import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { tokenMarketService } from './service'
import type {
  TokenExchangeDirection,
  TokenExchangeQuote,
  TokenMarketBootstrap,
  TokenMarketMerchant,
  TokenMarketProduct,
} from './types'

export const useTokenMarketStore = defineStore('token-market', () => {
  const bootstrap = ref<TokenMarketBootstrap | null>(null)
  const loading = ref(false)
  const exchanging = ref(false)
  const searchLoading = ref(false)
  const activeQuote = ref<TokenExchangeQuote | null>(null)
  const searchProducts = ref<TokenMarketProduct[]>([])
  const searchMerchants = ref<TokenMarketMerchant[]>([])
  const lastError = ref<string | null>(null)

  const wallet = computed(() => bootstrap.value?.wallet ?? null)
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
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : 'Token 市场加载失败'
      throw error
    } finally {
      loading.value = false
    }
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
      const result = await tokenMarketService.executeExchange({
        quoteId: activeQuote.value.quoteId,
        direction: activeQuote.value.direction,
        sourceAmount: activeQuote.value.sourceAmount,
      })
      if (bootstrap.value) bootstrap.value.wallet = result.wallet
      activeQuote.value = null
    } catch (error) {
      lastError.value = error instanceof Error ? error.message : '兑换失败'
      throw error
    } finally {
      exchanging.value = false
    }
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
    } finally {
      searchLoading.value = false
    }
  }

  function clearQuote(): void {
    activeQuote.value = null
  }

  return {
    bootstrap,
    loading,
    exchanging,
    searchLoading,
    activeQuote,
    searchProducts,
    searchMerchants,
    lastError,
    wallet,
    categories,
    products,
    merchants,
    activities,
    exchangeRate,
    initialize,
    quoteExchange,
    executeExchange,
    search,
    clearQuote,
  }
})
