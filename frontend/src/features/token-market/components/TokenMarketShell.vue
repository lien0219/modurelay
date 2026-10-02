<template>
  <div class="tm-page">
    <a class="tm-skip" href="#tm-content">跳转到内容</a>
    <header class="tm-topbar">
      <RouterLink class="tm-brand" to="/token-market" aria-label="Token Market 首页">
        <img src="/token-market/icons/ai.svg" alt="" />
        <span>Token Market</span>
      </RouterLink>
      <form class="tm-search" role="search" @submit.prevent="submitSearch">
        <img src="/token-market/icons/search.svg" alt="" />
        <input v-model="search" aria-label="搜索商品、商家或服务" placeholder="搜索商品、商家或服务" />
      </form>
      <div class="tm-top-actions">
        <button class="tm-mobile-search" type="button" aria-label="搜索" @click="searchOpen=!searchOpen"><img src="/token-market/icons/search.svg" alt="" /></button>
        <RouterLink class="tm-balance" to="/token-market/wallet">{{ market.wallet ? `${market.wallet.tokenBalance.toLocaleString('zh-CN')} T` : 'Token 钱包' }}</RouterLink>
        <RouterLink class="tm-cart-link" to="/token-market/cart" aria-label="购物车"><img src="/token-market/icons/cart.svg" alt="" /><span v-if="draft.cartCount" class="tm-count">{{ draft.cartCount }}</span></RouterLink>
        <RouterLink to="/token-market/messages" aria-label="消息中心"><img src="/token-market/icons/message.svg" alt="" /></RouterLink>
        <RouterLink class="tm-user" to="/token-market/account" aria-label="个人中心">{{ auth.user?.username?.slice(0, 1).toUpperCase() || 'U' }}</RouterLink>
      </div>
      <form v-if="searchOpen" class="tm-mobile-search-form" role="search" @submit.prevent="submitSearch"><input v-model="search" autofocus aria-label="搜索商品、商家或服务" placeholder="搜索商品、商家或服务" /><button type="submit">搜索</button></form>
    </header>
    <div class="tm-layout">
      <aside class="tm-sidebar" aria-label="市场导航">
        <nav>
          <RouterLink v-for="item in visibleNavigation" :key="item.to" :to="item.to" :class="{ 'is-active': active(item.to) }">
            <img :src="`/token-market/icons/${item.icon}.svg`" alt="" /><span>{{ item.label }}</span>
          </RouterLink>
        </nav>
        <div class="tm-sidebar-note"><strong>{{ route.path.startsWith('/token-market/merchant-') ? '安心经营，清晰管理' : '一个 Token' }}<br />{{ route.path.startsWith('/token-market/merchant-') ? '等待真实账本' : '连接更多可能' }}</strong><RouterLink :to="route.path.startsWith('/token-market/merchant-') ? '/token-market/merchant-products' : '/token-market/wallet'">{{ route.path.startsWith('/token-market/merchant-') ? '查看商品 →' : '了解 Token →' }}</RouterLink></div>
      </aside>
      <main id="tm-content" class="tm-content" tabindex="-1">
        <div v-if="isTokenMarketDemo" class="tm-demo-note" role="note">演示数据 · 余额、价格和交易记录仅供界面预览；支付、兑换和结算不会提交</div>
        <div v-if="market.loading && !market.bootstrap" class="tm-feedback" role="status">正在加载市场数据…</div>
        <div v-else-if="market.lastError && !market.bootstrap" class="tm-feedback" role="alert"><strong>市场暂时无法加载</strong><p>{{ market.lastError }}</p><button class="tm-button" type="button" @click="retry">重新加载</button></div>
        <slot v-else />
      </main>
    </div>
    <nav class="tm-bottom-nav" aria-label="移动端导航">
      <RouterLink v-for="item in mobileNavigation" :key="item.to" :to="item.to" :class="{ 'is-active': active(item.to) }">
        <img :src="`/token-market/icons/${item.icon}.svg`" alt="" /><span>{{ item.label }}</span>
      </RouterLink>
    </nav>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { isTokenMarketDemo } from '../service'
import { useMarketDraftStore } from '../experience'
import { useTokenMarketStore } from '../store'
import '../styles.css'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const market = useTokenMarketStore()
const draft = useMarketDraftStore()
const search = ref(String(route.query.q ?? ''))
const searchOpen = ref(false)
const navigation = [
  { label: '市场首页', icon: 'home', to: '/token-market' },
  { label: '品质商城', icon: 'shop', to: '/token-market/channel?category=mall' },
  { label: '外卖美食', icon: 'food', to: '/token-market/channel?category=delivery' },
  { label: '数字商品', icon: 'digital', to: '/token-market/channel?category=digital' },
  { label: 'AI 额度', icon: 'ai', to: '/token-market/channel?category=ai_credit' },
  { label: '服务市场', icon: 'service', to: '/token-market/channel?category=services' },
  { label: '我的订单', icon: 'orders', to: '/token-market/orders' },
  { label: '我的收藏', icon: 'heart', to: '/token-market/favorites' },
  { label: 'Token 钱包', icon: 'wallet', to: '/token-market/wallet' },
  { label: '消息中心', icon: 'message', to: '/token-market/messages' },
  { label: '个人中心', icon: 'user', to: '/token-market/account' },
  { label: '商家中心', icon: 'store', to: '/token-market/merchant-center' },
]
const merchantNavigation = [
  { label: '经营概览', icon: 'home', to: '/token-market/merchant-center' },
  { label: '商品管理', icon: 'shop', to: '/token-market/merchant-products' },
  { label: '商家订单', icon: 'orders', to: '/token-market/merchant-orders' },
  { label: '结算中心', icon: 'wallet', to: '/token-market/merchant-settlement' },
  { label: '客户消息', icon: 'message', to: '/token-market/messages' },
  { label: '入驻资料', icon: 'settings', to: '/token-market/merchant-apply' },
  { label: '返回市场', icon: 'back', to: '/token-market' },
]
const visibleNavigation = computed(() => route.path.startsWith('/token-market/merchant-') ? merchantNavigation : navigation)
const mobileNavigation = [
  { label: '首页', icon: 'home', to: '/token-market' },
  { label: '分类', icon: 'shop', to: '/token-market/channel?category=mall' },
  { label: '购物车', icon: 'cart', to: '/token-market/cart' },
  { label: '订单', icon: 'orders', to: '/token-market/orders' },
  { label: '我的', icon: 'user', to: '/token-market/account' },
]
function active(to: string): boolean {
  if (to === '/token-market') return route.path === to
  const [path, query] = to.split('?')
  if (path === '/token-market/channel' && route.path.startsWith('/token-market/merchants/')) {
    return market.merchants.find(item => item.id === String(route.params.id))?.categoryId === new URLSearchParams(query).get('category')
  }
  if (path === '/token-market/channel' && route.path.startsWith('/token-market/products/')) {
    return market.activeProduct?.categoryId === new URLSearchParams(query).get('category')
  }
  if (!route.path.startsWith(path)) return false
  return !query || new URLSearchParams(query).get('category') === route.query.category
}
function submitSearch(): void {
  const q = search.value.trim()
  if (q) { searchOpen.value = false; void router.push({ path: '/token-market/search', query: { q } }) }
}
async function retry(): Promise<void> {
  try { await market.initialize(true) } catch { /* State panel shows the service error. */ }
}
onMounted(() => { if (!market.bootstrap) void retry() })
watch(() => route.query.q, value => { search.value = String(value ?? '') })
watch(() => route.fullPath, () => window.scrollTo({ top: 0, behavior: 'instant' }))
</script>
