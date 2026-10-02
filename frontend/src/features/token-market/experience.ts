import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useAuthStore } from '@/stores/auth'

export interface MarketCartLine { productId: string; quantity: number; selected: boolean; variant: string }
export interface MarketAddress { id: string; label: string; name: string; phone: string; detail: string; isDefault: boolean }
export interface MarketProductDraft {
  name: string
  categoryId: string
  modelId: string
  tokenTypes: Array<'input' | 'output' | 'cache'>
  modelTokenQuantity: string
  packageCombination: string
  salePriceToken: string
  description: string
}
interface LocalDrafts {
  cart: MarketCartLine[]
  favoriteProductIds: string[]
  favoriteMerchantIds: string[]
  addresses: MarketAddress[]
  nickname: string
  bio: string
  merchantApplication: Record<string, string>
  productDrafts: MarketProductDraft[]
}
const storageKey = 'modurelay.token-market.ui-drafts.v3'
const sessionKey = 'modurelay.token-market.session-drafts.v3'
function readDrafts(accountId: number | null): LocalDrafts {
  const fallback: LocalDrafts = { cart: [], favoriteProductIds: [], favoriteMerchantIds: [], addresses: [], nickname: '', bio: '', merchantApplication: {}, productDrafts: [] }
  if (accountId == null) return fallback
  try {
    const saved = JSON.parse(localStorage.getItem(`${storageKey}:${accountId}`) || '{}') as Partial<LocalDrafts>
    const session = JSON.parse(sessionStorage.getItem(`${sessionKey}:${accountId}`) || '{}') as Partial<LocalDrafts>
    return { ...fallback, ...saved, addresses: session.addresses ?? [], merchantApplication: session.merchantApplication ?? {} }
  } catch { return fallback }
}

export const useMarketDraftStore = defineStore('token-market-ui-drafts', () => {
  const auth = useAuthStore()
  const accountId = computed(() => auth.user?.id ?? null)
  const data = ref<LocalDrafts>(readDrafts(accountId.value))
  const cartCount = computed(() => data.value.cart.reduce((total, item) => total + item.quantity, 0))
  const selectedCart = computed(() => data.value.cart.filter(item => item.selected))
  watch(accountId, id => { data.value = readDrafts(id) }, { flush: 'sync' })
  watch(data, value => {
    if (accountId.value == null) return
    const { addresses, merchantApplication, ...persistent } = value
    localStorage.setItem(`${storageKey}:${accountId.value}`, JSON.stringify(persistent))
    sessionStorage.setItem(`${sessionKey}:${accountId.value}`, JSON.stringify({ addresses, merchantApplication }))
  }, { deep: true })

  function addToCart(productId: string, variant = '标准版'): void {
    const line = data.value.cart.find(item => item.productId === productId && item.variant === variant)
    if (line) { line.quantity = Math.min(99, line.quantity + 1); line.selected = true }
    else data.value.cart.push({ productId, quantity: 1, selected: true, variant })
  }
  function setQuantity(productId: string, quantity: number): void {
    const line = data.value.cart.find(item => item.productId === productId)
    if (line) line.quantity = Math.min(99, Math.max(1, Math.trunc(quantity || 1)))
  }
  function removeFromCart(productId: string): void { data.value.cart = data.value.cart.filter(item => item.productId !== productId) }
  function toggleProductFavorite(id: string): void {
    const ids = data.value.favoriteProductIds
    data.value.favoriteProductIds = ids.includes(id) ? ids.filter(item => item !== id) : [...ids, id]
  }
  function toggleMerchantFavorite(id: string): void {
    const ids = data.value.favoriteMerchantIds
    data.value.favoriteMerchantIds = ids.includes(id) ? ids.filter(item => item !== id) : [...ids, id]
  }
  function saveAddress(address: MarketAddress): void {
    if (address.isDefault) data.value.addresses.forEach(item => { item.isDefault = false })
    const index = data.value.addresses.findIndex(item => item.id === address.id)
    if (index < 0) data.value.addresses.push(address)
    else data.value.addresses[index] = address
  }
  function removeAddress(id: string): void { data.value.addresses = data.value.addresses.filter(item => item.id !== id) }
  function saveProductDraft(product: MarketProductDraft): void { data.value.productDrafts.push(product) }
  function updateProductDraft(index: number, product: MarketProductDraft): void {
    if (index >= 0 && index < data.value.productDrafts.length) data.value.productDrafts[index] = product
  }
  return { data, cartCount, selectedCart, addToCart, setQuantity, removeFromCart, toggleProductFavorite, toggleMerchantFavorite, saveAddress, removeAddress, saveProductDraft, updateProductDraft }
})
