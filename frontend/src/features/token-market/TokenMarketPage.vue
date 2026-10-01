<template>
  <TokenMarketShell @background-click="closeMenus">
    <TokenMarketHeader
      :items="navItems"
      :search="search"
      @back="goBack"
      @navigate="navigateSection"
      @search="submitSearch"
      @update:search="search = $event"
      @toggle-notifications="notificationsOpen = !notificationsOpen"
      @toggle-profile="profileOpen = !profileOpen"
    />

    <div class="tm-container page-content">
      <div class="hero-grid">
        <WalletHeroCard
          :wallet="market.wallet"
          :exchange-rate="market.exchangeRate"
          :visible="walletVisible"
          @toggle-visible="walletVisible = !walletVisible"
          @details="openPanel('wallet')"
        />
        <ExchangePanel
          :mode="mode"
          :source-value="sourceValue"
          :formatted-output="formattedOutput"
          :exchange-rate="market.exchangeRate"
          :platform-balance="platformBalance"
          :token-balance="tokenBalance"
          :loading="market.exchanging"
          @mode-change="setMode"
          @source-change="sourceValue = $event"
          @swap="swapMode"
          @submit="openExchange"
        />
      </div>

      <CategoryGrid :categories="market.categories" @select="selectCategory" />

      <div class="content-grid">
        <FeaturedProducts
          class="products"
          :products="market.products"
          :merchants="market.merchants"
          @select="openProduct"
          @more="selectCategory('商城')"
        />
        <ActivityFeed class="activity" :activities="market.activities" @more="openPanel('activity')" />
      </div>

      <MerchantGrid
        :merchants="market.merchants"
        @select="openMerchant($event.name)"
        @more="selectCategory('服务市场')"
      />
    </div>

    <Transition name="tm-fade">
      <div v-if="notificationsOpen" class="floating-menu notifications" @click.stop>
        <strong>通知</strong>
        <span>兑换功能当前使用 Mock Service</span>
        <span>3 个商品价格发生变化</span>
      </div>
    </Transition>

    <Transition name="tm-fade">
      <div v-if="profileOpen" class="floating-menu profile" @click.stop>
        <button type="button" @click="goBack">返回 ModuRelay</button>
        <button type="button" @click="openPanel('wallet')">钱包中心</button>
        <button type="button" @click="openPanel('orders')">我的订单</button>
      </div>
    </Transition>

    <MarketDialog
      :dialog="dialog"
      :source-text="exchangeSourceText"
      :destination-text="exchangeDestinationText"
      :loading="market.exchanging"
      @close="closeDialog"
      @confirm-exchange="confirmExchange"
      @add-cart="addCart"
    />

    <Transition name="tm-fade">
      <div v-if="toast" class="toast">{{ toast }}</div>
    </Transition>

    <button v-if="cartCount" class="cart-fab" type="button" @click="openPanel('cart')">🛒 <span>{{ cartCount }}</span></button>
  </TokenMarketShell>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import ActivityFeed from './components/ActivityFeed.vue'
import CategoryGrid from './components/CategoryGrid.vue'
import ExchangePanel from './components/ExchangePanel.vue'
import FeaturedProducts from './components/FeaturedProducts.vue'
import MarketDialog from './components/MarketDialog.vue'
import MerchantGrid from './components/MerchantGrid.vue'
import TokenMarketHeader from './components/TokenMarketHeader.vue'
import TokenMarketShell from './components/TokenMarketShell.vue'
import WalletHeroCard from './components/WalletHeroCard.vue'
import { calculateExchangeDestination } from './domain'
import { useTokenMarketStore } from './store'
import type { TokenExchangeDirection, TokenMarketProduct } from './types'
import type { TokenMarketDialogState, TokenMarketExchangeMode, TokenMarketNavItem, TokenMarketPanelKind } from './ui'

const router = useRouter()
const market = useTokenMarketStore()
const search = ref('')
const mode = ref<TokenMarketExchangeMode>('balance')
const sourceValue = ref(100)
const walletVisible = ref(true)
const notificationsOpen = ref(false)
const profileOpen = ref(false)
const dialog = ref<TokenMarketDialogState | null>(null)
const toast = ref('')
const cartCount = ref(0)
let toastTimer: number | undefined

const navItems: TokenMarketNavItem[] = [
  { label: '首页', section: 'home' },
  { label: '商城', section: '商城' },
  { label: '外卖', section: '外卖' },
  { label: '数字商品', section: '数字商品' },
  { label: 'AI额度', section: 'AI额度' },
  { label: '服务市场', section: '服务市场' },
]

const tokenBalance = computed(() => market.wallet?.tokenBalance ?? 0)
const platformBalance = computed(() => market.wallet?.platformBalance ?? 0)
const direction = computed<TokenExchangeDirection>(() => mode.value === 'balance' ? 'balance_to_token' : 'token_to_balance')
const outputValue = computed(() => calculateExchangeDestination(direction.value, Number(sourceValue.value) || 0, market.exchangeRate))
const quotedDestination = computed(() => market.activeQuote?.destinationAmount ?? outputValue.value)
const formattedOutput = computed(() => mode.value === 'balance'
  ? outputValue.value.toLocaleString('zh-CN')
  : outputValue.value.toLocaleString('zh-CN', { maximumFractionDigits: 2 }))
const exchangeSourceText = computed(() => mode.value === 'balance'
  ? `¥ ${(sourceValue.value || 0).toLocaleString('zh-CN')}`
  : `${(sourceValue.value || 0).toLocaleString('zh-CN')} T`)
const exchangeDestinationText = computed(() => mode.value === 'balance'
  ? `${quotedDestination.value.toLocaleString('zh-CN')} T`
  : `¥ ${quotedDestination.value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })}`)

function goBack(): void {
  if (window.history.length > 1) router.back()
  else void router.push('/dashboard')
}

function closeMenus(): void {
  notificationsOpen.value = false
  profileOpen.value = false
}

function closeDialog(): void {
  dialog.value = null
  market.clearQuote()
}

function setMode(next: TokenMarketExchangeMode): void {
  mode.value = next
  sourceValue.value = next === 'balance' ? 100 : 10000
  market.clearQuote()
}

function swapMode(): void {
  setMode(mode.value === 'balance' ? 'token' : 'balance')
}

async function openExchange(): Promise<void> {
  try {
    await market.quoteExchange(direction.value, Number(sourceValue.value) || 0)
    dialog.value = { type: 'exchange' }
  } catch (error) {
    showToast(error instanceof Error ? error.message : '兑换报价失败')
  }
}

async function confirmExchange(): Promise<void> {
  try {
    await market.executeExchange()
    dialog.value = null
    showToast('页面演示兑换成功 · 当前使用 Mock Service')
  } catch (error) {
    showToast(error instanceof Error ? error.message : '兑换失败')
  }
}

function navigateSection(item: TokenMarketNavItem): void {
  if (item.section === 'home') return showToast('已经在 Token 交易市场首页')
  selectCategory(item.label)
}

function selectCategory(category: string): void {
  dialog.value = {
    type: 'info',
    title: category,
    message: `${category}子页面入口已经预留。当前组件与数据层已解耦，可直接继续挂接对应 Commerce Service 与独立路由。`,
  }
}

function openProduct(product: TokenMarketProduct): void {
  dialog.value = { type: 'product', title: product.name, price: product.priceToken.toLocaleString('zh-CN') }
}

function openMerchant(merchant: string): void {
  dialog.value = {
    type: 'info',
    title: merchant,
    message: '商家主页、商品列表、评价、配送和 Token 结算能力将在商户系统阶段通过独立服务契约接入。',
  }
}

function openPanel(type: TokenMarketPanelKind): void {
  const copy: Record<TokenMarketPanelKind, [string, string]> = {
    wallet: ['Token 钱包', '钱包明细、充值、兑换、冻结与账本流水的入口已经预留。'],
    orders: ['我的订单', '商城、外卖、数字商品和服务订单后续统一进入订单中心。'],
    activity: ['交易动态', '完整交易流水将在账本与订单接口完成后接入。'],
    cart: ['购物车', `当前购物车共有 ${cartCount.value} 件商品。结算能力将在 Token 支付系统接入后启用。`],
  }
  const [title, message] = copy[type]
  dialog.value = { type: 'info', title, message }
}

function addCart(title: string): void {
  cartCount.value += 1
  dialog.value = null
  showToast(`${title} 已加入购物车`)
}

async function submitSearch(): Promise<void> {
  const q = search.value.trim()
  if (!q) return
  try {
    await market.search(q)
    const product = market.searchProducts[0]
    const merchant = market.searchMerchants[0]
    if (product) openProduct(product)
    else if (merchant) openMerchant(merchant.name)
    else showToast(`未找到“${q}”相关结果`)
  } catch (error) {
    showToast(error instanceof Error ? error.message : '搜索失败')
  }
}

function showToast(message: string): void {
  toast.value = message
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => { toast.value = '' }, 2500)
}

function handleKeydown(event: KeyboardEvent): void {
  if (event.key === 'Escape') {
    closeDialog()
    closeMenus()
  }
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    document.querySelector<HTMLInputElement>('.tm-header-search input')?.focus()
  }
}

onMounted(async () => {
  document.body.classList.add('token-market-route')
  document.title = 'Token 交易市场'
  window.addEventListener('keydown', handleKeydown)
  try {
    await market.initialize()
  } catch (error) {
    showToast(error instanceof Error ? error.message : 'Token 市场加载失败')
  }
})

onBeforeUnmount(() => {
  document.body.classList.remove('token-market-route')
  window.removeEventListener('keydown', handleKeydown)
  if (toastTimer) window.clearTimeout(toastTimer)
})
</script>

<style scoped>
.page-content{padding:18px 0 54px}.hero-grid{display:grid;grid-template-columns:minmax(0,1.75fr) minmax(330px,.85fr);gap:18px}.content-grid{display:grid;grid-template-columns:minmax(0,1fr) 300px;gap:18px;align-items:stretch}.activity{margin-top:28px}.floating-menu{position:fixed;z-index:70;top:70px;width:220px;padding:12px;border:1px solid rgba(99,139,226,.28);border-radius:12px;background:rgba(6,18,42,.96);box-shadow:0 22px 60px rgba(0,0,0,.4);backdrop-filter:blur(18px)}.floating-menu.notifications{right:70px}.floating-menu.profile{right:18px;width:170px}.floating-menu strong,.floating-menu span{display:block}.floating-menu strong{margin-bottom:7px}.floating-menu span{padding:7px 0;border-top:1px solid rgba(102,130,190,.1);color:#9aacca;font-size:10px}.floating-menu button{width:100%;padding:8px;border:0;border-radius:7px;color:#cbd8ef;background:transparent;text-align:left;cursor:pointer}.floating-menu button:hover{background:rgba(77,108,178,.14)}.toast{position:fixed;z-index:130;left:50%;bottom:28px;transform:translateX(-50%);max-width:calc(100% - 32px);padding:11px 16px;border:1px solid rgba(109,143,218,.28);border-radius:10px;color:#eaf2ff;background:rgba(7,21,48,.97);box-shadow:0 15px 44px rgba(0,0,0,.42);font-size:11px}.cart-fab{position:fixed;z-index:50;right:24px;bottom:24px;width:52px;height:52px;border:1px solid rgba(121,141,255,.5);border-radius:50%;color:#fff;background:linear-gradient(135deg,#4f61ff,#843eff);box-shadow:0 15px 44px rgba(71,54,255,.35);cursor:pointer}.cart-fab span{position:absolute;top:-5px;right:-4px;display:grid;place-items:center;min-width:19px;height:19px;border:2px solid #061022;border-radius:999px;background:#ff547e;font-size:9px;font-weight:800}@media(max-width:1100px){.hero-grid{grid-template-columns:1fr}.content-grid{grid-template-columns:1fr}.activity{margin-top:0}}@media(max-width:720px){.page-content{padding-top:8px}.floating-menu.notifications{right:12px}}
</style>
