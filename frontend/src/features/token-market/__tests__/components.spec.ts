import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import CategoryGrid from '../components/CategoryGrid.vue'
import ExchangePanel from '../components/ExchangePanel.vue'
import FeaturedProducts from '../components/FeaturedProducts.vue'
import TokenMarketAsyncState from '../components/TokenMarketAsyncState.vue'
import TokenMarketSkeleton from '../components/TokenMarketSkeleton.vue'
import TokenMarketStatePanel from '../components/TokenMarketStatePanel.vue'
import WalletHeroCard from '../components/WalletHeroCard.vue'
import type { TokenMarketCategory, TokenMarketMerchant, TokenMarketProduct, TokenMarketWalletSnapshot } from '../types'

const categories: TokenMarketCategory[] = [
  { id: 'mall', label: '商城', description: '精选好物' },
]

const merchants: TokenMarketMerchant[] = [
  { id: 'merchant-1', name: '测试商家', categoryId: 'mall', rating: 4.9, soldCount: 100 },
]

const products: TokenMarketProduct[] = [
  { id: 'product-1', name: '测试商品', categoryId: 'mall', priceToken: 1880, merchantId: 'merchant-1' },
]

const wallet: TokenMarketWalletSnapshot = {
  platformBalance: 100,
  tokenBalance: 10000,
  frozenToken: 10,
  pendingToken: 20,
  todaySpentToken: 30,
  yesterdayIncomeToken: 40,
  updatedAt: new Date(0).toISOString(),
}

describe('token market components', () => {
  it('emits category selection without owning navigation state', async () => {
    const wrapper = mount(CategoryGrid, { props: { categories } })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('select')).toEqual([['商城']])
  })

  it('emits exchange changes instead of mutating wallet state locally', async () => {
    const wrapper = mount(ExchangePanel, {
      props: {
        mode: 'balance',
        sourceValue: 100,
        formattedOutput: '10,000',
        exchangeRate: 100,
        platformBalance: 100,
        tokenBalance: 10000,
        loading: false,
      },
    })

    const buttons = wrapper.findAll('button')
    await buttons[1].trigger('click')
    expect(wrapper.emitted('mode-change')).toEqual([['token']])
  })

  it('emits product selection with the domain product', async () => {
    const wrapper = mount(FeaturedProducts, { props: { products, merchants } })
    await wrapper.get('.card').trigger('click')
    expect(wrapper.emitted('select')?.[0]?.[0]).toEqual(products[0])
  })

  it('keeps wallet visibility controlled by its parent', async () => {
    const wrapper = mount(WalletHeroCard, { props: { wallet, exchangeRate: 100, visible: true } })
    const eye = wrapper.findAll('button')[0]
    await eye.trigger('click')
    expect(wrapper.emitted('toggle-visible')).toHaveLength(1)
    expect(wrapper.text()).toContain('10,000')
  })

  it('renders the shared skeleton for loading states', () => {
    const wrapper = mount(TokenMarketAsyncState, {
      props: { loading: true, skeleton: 'detail' },
      slots: { default: '<div>ready</div>' },
    })
    expect(wrapper.findComponent(TokenMarketSkeleton).exists()).toBe(true)
    expect(wrapper.text()).not.toContain('ready')
  })

  it('renders the shared empty state and hides page content', () => {
    const wrapper = mount(TokenMarketAsyncState, {
      props: {
        empty: true,
        emptyTitle: '暂无订单',
        emptyDescription: '当前没有订单记录',
      },
      slots: { default: '<div>order-list</div>' },
    })
    expect(wrapper.findComponent(TokenMarketStatePanel).exists()).toBe(true)
    expect(wrapper.text()).toContain('暂无订单')
    expect(wrapper.text()).not.toContain('order-list')
  })

  it('emits retry from the shared error state', async () => {
    const wrapper = mount(TokenMarketAsyncState, {
      props: { error: 'network error' },
    })
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
  })

  it('renders the permission placeholder consistently', () => {
    const wrapper = mount(TokenMarketAsyncState, {
      props: {
        forbidden: true,
        forbiddenTitle: '需要商家权限',
        forbiddenDescription: '当前账户尚未开通商家中心',
      },
    })
    expect(wrapper.text()).toContain('需要商家权限')
    expect(wrapper.text()).toContain('当前账户尚未开通商家中心')
  })
})
