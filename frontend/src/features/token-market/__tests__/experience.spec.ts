import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { beforeEach, describe, expect, it } from 'vitest'
import { useAuthStore } from '@/stores/auth'
import type { User } from '@/types'
import { useMarketDraftStore } from '../experience'
import { useTokenMarketStore } from '../store'

describe('token market browser drafts', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
    setActivePinia(createPinia())
  })

  it('keeps cart and favorites with the authenticated account', async () => {
    const auth = useAuthStore()
    auth.user = { id: 101 } as User
    const drafts = useMarketDraftStore()
    drafts.addToCart('product-1')
    drafts.toggleProductFavorite('product-1')
    await nextTick()

    expect(localStorage.getItem('modurelay.token-market.ui-drafts.v3:101')).toContain('product-1')
    auth.user = { id: 202 } as User
    await nextTick()
    expect(drafts.cartCount).toBe(0)
    expect(drafts.data.favoriteProductIds).toEqual([])

    drafts.addToCart('product-2')
    await nextTick()
    expect(localStorage.getItem('modurelay.token-market.ui-drafts.v3:202')).toContain('product-2')
    auth.user = { id: 101 } as User
    await nextTick()
    expect(drafts.data.cart[0]?.productId).toBe('product-1')
    expect(drafts.data.favoriteProductIds).toEqual(['product-1'])
  })

  it('discards market data returned after an account change', async () => {
    const auth = useAuthStore()
    auth.user = { id: 101 } as User
    const market = useTokenMarketStore()
    const pending = market.initialize()
    auth.user = { id: 202 } as User
    await pending
    expect(market.bootstrap).toBeNull()

    await market.initialize()
    expect(market.wallet).not.toBeNull()
    auth.user = null
    expect(market.wallet).toBeNull()
    expect(market.orderItems).toEqual([])
  })

  it('discards an exchange quote when its input is changed', async () => {
    const auth = useAuthStore()
    auth.user = { id: 101 } as User
    const market = useTokenMarketStore()
    const pending = market.quoteExchange('balance_to_token', 10)
    market.clearQuote()
    await expect(pending).rejects.toThrow('报价条件已变更')
    expect(market.activeQuote).toBeNull()
  })
})
