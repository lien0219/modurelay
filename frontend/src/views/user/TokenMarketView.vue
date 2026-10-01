<template>
  <main class="tm-page" @click="closeMenus">
    <section class="tm-stage" aria-label="Token 交易市场">
      <img class="tm-design" src="/token-market/market-dashboard.webp" alt="" draggable="false" aria-hidden="true" />

      <button class="hotspot back" type="button" aria-label="返回 ModuRelay" @click.stop="goBack"></button>

      <nav class="tm-top-hotspots" aria-label="Token 市场导航">
        <button
          v-for="item in navItems"
          :key="item.label"
          class="hotspot"
          :style="item.style"
          type="button"
          :aria-label="item.label"
          @click.stop="navigateSection(item)"
        ></button>
      </nav>

      <label class="tm-search">
        <span class="sr-only">搜索商品、商家或关键词</span>
        <input v-model="search" type="search" placeholder="搜索商品、商家或关键词..." @keyup.enter="submitSearch" />
      </label>

      <button class="hotspot notify" type="button" aria-label="通知" @click.stop="notificationsOpen = !notificationsOpen"></button>
      <button class="hotspot profile" type="button" aria-label="个人菜单" @click.stop="profileOpen = !profileOpen"></button>
      <button class="hotspot wallet-detail" type="button" aria-label="钱包明细" @click.stop="openPanel('wallet')"></button>
      <button class="hotspot wallet-eye" type="button" :aria-label="walletVisible ? '隐藏余额' : '显示余额'" @click.stop="walletVisible = !walletVisible"></button>
      <div v-if="!walletVisible" class="wallet-mask">••••••</div>

      <section class="tm-exchange-ui" @click.stop>
        <button class="exchange-tab left" :class="{ active: mode === 'balance' }" type="button" @click="setMode('balance')">平台余额 ↔ Token</button>
        <button class="exchange-tab right" :class="{ active: mode === 'token' }" type="button" @click="setMode('token')">Token ↔ 平台余额</button>

        <label class="exchange-input source">
          <span>{{ mode === 'balance' ? '¥' : 'T' }}</span>
          <input v-model.number="sourceValue" type="number" min="0" />
          <small>{{ mode === 'balance' ? `余额 ¥${platformBalance}` : `余额 ${tokenBalance.toLocaleString()} T` }}</small>
        </label>

        <button class="swap" type="button" aria-label="切换兑换方向" @click="swapMode">⇅</button>

        <div class="exchange-input output">
          <span>{{ mode === 'balance' ? 'T' : '¥' }}</span>
          <strong>{{ formattedOutput }}</strong>
          <small>{{ mode === 'balance' ? `≈ ¥ ${sourceValue || 0}` : `≈ ${(sourceValue || 0).toLocaleString()} T` }}</small>
        </div>

        <button class="exchange-action primary" type="button" :disabled="market.exchanging" @click="openExchange">{{ market.exchanging ? '处理中...' : mode === 'balance' ? '兑换 Token' : '兑换余额' }}</button>
        <button class="exchange-action secondary" type="button" @click="swapMode">{{ mode === 'balance' ? '兑换余额' : '兑换 Token' }}</button>
      </section>

      <button
        v-for="(category, index) in categories"
        :key="category.id"
        class="hotspot category"
        :style="{ left: `${2.35 + index * 16.35}%` }"
        type="button"
        :aria-label="category.label"
        @click.stop="selectCategory(category.label)"
      ></button>

      <button
        v-for="(product, index) in products"
        :key="product.id"
        class="hotspot product"
        :style="{ left: `${2.4 + index * 12.68}%` }"
        type="button"
        :aria-label="product.name"
        @click.stop="openProduct(product)"
      ></button>

      <button
        v-for="(merchant, index) in merchants"
        :key="merchant.id"
        class="hotspot merchant"
        :style="{ left: `${2.45 + index * 15.7}%` }"
        type="button"
        :aria-label="merchant.name"
        @click.stop="openMerchant(merchant.name)"
      ></button>

      <button class="hotspot activity-more" type="button" aria-label="查看更多交易动态" @click.stop="openPanel('activity')"></button>

      <Transition name="fade">
        <div v-if="notificationsOpen" class="mini-menu notifications" @click.stop>
          <strong>通知</strong>
          <span>兑换功能当前为交互预览</span>
          <span>3 个商品价格发生变化</span>
        </div>
      </Transition>

      <Transition name="fade">
        <div v-if="profileOpen" class="mini-menu profile-menu" @click.stop>
          <button type="button" @click="goBack">返回 ModuRelay</button>
          <button type="button" @click="openPanel('wallet')">钱包中心</button>
          <button type="button" @click="openPanel('orders')">我的订单</button>
        </div>
      </Transition>
    </section>

    <Transition name="fade">
      <div v-if="dialog" class="tm-overlay" @click.self="closeDialog">
        <section class="tm-dialog">
          <button class="close" type="button" aria-label="关闭" @click="closeDialog">×</button>

          <template v-if="dialog.type === 'exchange'">
            <span class="eyebrow">QUICK EXCHANGE</span>
            <h2>确认兑换</h2>
            <div class="exchange-summary">
              <strong>{{ mode === 'balance' ? `¥ ${sourceValue || 0}` : `${(sourceValue || 0).toLocaleString()} T` }}</strong>
              <span>→</span>
              <strong>{{ mode === 'balance' ? `${quotedDestination.toLocaleString()} T` : `¥ ${quotedDestination.toFixed(2)}` }}</strong>
            </div>
            <p>当前阶段通过 TokenMarketService 的 Mock 实现完成报价与执行。后续切换真实 API 时页面无需重写。</p>
            <div class="actions">
              <button type="button" class="secondary-btn" @click="closeDialog">取消</button>
              <button type="button" class="primary-btn" :disabled="market.exchanging" @click="confirmExchange">{{ market.exchanging ? '处理中...' : '确认兑换' }}</button>
            </div>
          </template>

          <template v-else-if="dialog.type === 'product'">
            <span class="eyebrow">FEATURED PRODUCT</span>
            <h2>{{ dialog.title }}</h2>
            <p>商品详情页、SKU、库存、购物车和 Token 支付接口将在交易系统后端完成后接入。</p>
            <div class="price">{{ dialog.price }} T</div>
            <div class="actions">
              <button type="button" class="secondary-btn" @click="closeDialog">继续浏览</button>
              <button type="button" class="primary-btn" @click="addCart(dialog.title)">加入购物车</button>
            </div>
          </template>

          <template v-else>
            <span class="eyebrow">TOKEN MARKET</span>
            <h2>{{ dialog.title }}</h2>
            <p>{{ dialog.message }}</p>
            <button type="button" class="primary-btn full" @click="closeDialog">知道了</button>
          </template>
        </section>
      </div>
    </Transition>

    <Transition name="toast">
      <div v-if="toast" class="tm-toast">{{ toast }}</div>
    </Transition>

    <button v-if="cartCount" class="cart-fab" type="button" @click="openPanel('cart')">🛒 <span>{{ cartCount }}</span></button>
  </main>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { calculateExchangeDestination, useTokenMarketStore } from '@/features/token-market'
import type { TokenExchangeDirection, TokenMarketProduct } from '@/features/token-market'

type Mode = 'balance' | 'token'
type Dialog =
  | { type: 'exchange' }
  | { type: 'product'; title: string; price: string }
  | { type: 'info'; title: string; message: string }

const router = useRouter()
const market = useTokenMarketStore()
const search = ref('')
const mode = ref<Mode>('balance')
const sourceValue = ref(100)
const walletVisible = ref(true)
const notificationsOpen = ref(false)
const profileOpen = ref(false)
const dialog = ref<Dialog | null>(null)
const toast = ref('')
const cartCount = ref(0)
let toastTimer: number | undefined

const navItems = [
  { label: '首页', section: 'home', style: { left: '17.2%', width: '5.8%' } },
  { label: '商城', section: '商城', style: { left: '23.2%', width: '5.8%' } },
  { label: '外卖', section: '外卖', style: { left: '29.1%', width: '5.8%' } },
  { label: '数字商品', section: '数字商品', style: { left: '35.2%', width: '7.5%' } },
  { label: 'AI额度', section: 'AI额度', style: { left: '43.1%', width: '6.2%' } },
  { label: '服务市场', section: '服务市场', style: { left: '49.6%', width: '7.4%' } },
]

const categories = computed(() => market.categories)
const products = computed(() => market.products)
const merchants = computed(() => market.merchants)
const tokenBalance = computed(() => market.wallet?.tokenBalance ?? 0)
const platformBalance = computed(() => market.wallet?.platformBalance ?? 0)
const direction = computed<TokenExchangeDirection>(() => mode.value === 'balance' ? 'balance_to_token' : 'token_to_balance')
const quotedDestination = computed(() => market.activeQuote?.destinationAmount ?? outputValue.value)

const outputValue = computed(() =>
  calculateExchangeDestination(direction.value, Number(sourceValue.value) || 0, market.exchangeRate)
)
const formattedOutput = computed(() =>
  mode.value === 'balance'
    ? outputValue.value.toLocaleString('zh-CN')
    : outputValue.value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
)

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

function setMode(next: Mode): void {
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

function navigateSection(item: (typeof navItems)[number]): void {
  if (item.section === 'home') return showToast('已经在 Token 交易市场首页')
  selectCategory(item.section)
}

function selectCategory(category: string): void {
  dialog.value = {
    type: 'info',
    title: category,
    message: `${category}子页面入口已经预留。当前基础层已按独立 feature 领域拆分，后续可直接挂接对应 service 与路由。`,
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

function openPanel(type: string): void {
  const copy: Record<string, [string, string]> = {
    wallet: ['Token 钱包', '钱包明细、充值、兑换、冻结与账本流水的页面入口已经预留。'],
    orders: ['我的订单', '商城、外卖、数字商品和服务订单后续统一进入订单中心。'],
    activity: ['交易动态', '完整交易流水将在账本与订单接口完成后接入。'],
    cart: ['购物车', `当前购物车共有 ${cartCount.value} 件商品。结算能力将在 Token 支付系统接入后启用。`],
  }
  const [title, message] = copy[type] ?? ['Token Market', '功能入口已预留。']
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
    document.querySelector<HTMLInputElement>('.tm-search input')?.focus()
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
:global(body.token-market-route){margin:0;overflow-x:hidden;background:#020817}.tm-page{min-height:100dvh;background:#020817;color:#eef4ff;font-family:"Noto Sans SC Variable","Noto Sans SC",system-ui,sans-serif}.tm-stage{position:relative;width:min(100vw,1672px);aspect-ratio:1672/941;margin:0 auto;overflow:hidden;background:#020817;box-shadow:0 0 100px #000}.tm-design{position:absolute;inset:0;width:100%;height:100%;object-fit:contain;user-select:none;pointer-events:none}.hotspot{position:absolute;z-index:5;border:0;background:transparent;cursor:pointer}.hotspot:hover{outline:1px solid rgba(89,151,255,.18);background:rgba(70,112,255,.035)}.back{left:1.7%;top:1.1%;width:14.3%;height:4.2%}.notify{right:5.05%;top:1.25%;width:2.1%;height:3.3%;border-radius:50%}.profile{right:1.35%;top:1.1%;width:2.2%;height:3.7%;border-radius:50%}.wallet-detail{left:66.45%;top:17.8%;width:6.4%;height:4.5%;border-radius:10px}.wallet-eye{left:57.15%;top:9.7%;width:2.4%;height:3%;border-radius:50%}.wallet-mask{position:absolute;z-index:8;left:48.3%;top:17.2%;width:16%;padding:.15% .5%;color:#edf4ff;background:#132d62;font-size:clamp(18px,2.4vw,42px);font-weight:800;letter-spacing:.12em}.tm-top-hotspots .hotspot{top:0;height:5.8%}.tm-search{position:absolute;z-index:8;right:8.9%;top:1.24%;width:19.2%;height:3.25%}.tm-search input{width:100%;height:100%;padding:0 12% 0 11%;border:0;outline:0;border-radius:10px;color:#cbd7ef;background:rgba(5,15,36,.85);font:inherit;font-size:clamp(8px,.75vw,12px);box-sizing:border-box}.tm-search input::placeholder{color:#6d7e9f}.sr-only{position:absolute;width:1px;height:1px;padding:0;margin:-1px;overflow:hidden;clip:rect(0,0,0,0);white-space:nowrap;border:0}.tm-exchange-ui{position:absolute;z-index:12;left:74.9%;top:8.55%;width:22.55%;height:29.6%;pointer-events:none}.tm-exchange-ui>*{pointer-events:auto}.exchange-tab{position:absolute;top:11.5%;height:11.6%;border:0;border-radius:9px;color:#8195bb;background:transparent;font-size:clamp(7px,.66vw,11px);font-weight:700;cursor:pointer}.exchange-tab.left{left:4.2%;width:47%}.exchange-tab.right{right:4%;width:43.5%}.exchange-tab.active{color:#fff;background:linear-gradient(90deg,#4c66ff,#2fbaff);box-shadow:0 4px 18px rgba(59,97,255,.28)}.exchange-input{position:absolute;left:4.2%;width:91.7%;height:16.2%;display:grid;grid-template-columns:8% 1fr auto;align-items:center;padding:0 3%;border:1px solid rgba(102,135,207,.17);border-radius:7px;background:rgba(8,20,45,.88);box-sizing:border-box}.exchange-input.source{top:35.3%}.exchange-input.output{top:57.7%}.exchange-input>span{display:grid;place-items:center;color:#dfe9ff;font-weight:800}.exchange-input input{min-width:0;border:0;outline:0;color:#fff;background:transparent;font-size:clamp(10px,.9vw,15px);font-weight:800}.exchange-input strong{font-size:clamp(10px,.9vw,15px)}.exchange-input small{color:#8295b9;font-size:clamp(6px,.6vw,10px)}.swap{position:absolute;z-index:2;left:47.2%;top:49.6%;width:6.5%;aspect-ratio:1;border:1px solid rgba(108,142,218,.3);border-radius:50%;color:#b4c8ed;background:#10234c;font-size:clamp(7px,.7vw,12px);cursor:pointer}.exchange-action{position:absolute;bottom:3.5%;height:13.7%;border-radius:8px;font-size:clamp(7px,.7vw,11px);font-weight:800;cursor:pointer}.exchange-action:disabled,.primary-btn:disabled{opacity:.55;cursor:wait}.exchange-action.primary{left:4.2%;width:47%;border:0;color:#fff;background:linear-gradient(90deg,#743cff,#4c66ff,#24c6ff)}.exchange-action.secondary{right:4%;width:43.5%;border:1px solid rgba(103,135,205,.24);color:#d3def2;background:rgba(5,16,38,.82)}.category{top:39.2%;width:15.55%;height:9.5%;border-radius:12px}.product{top:55.6%;width:12.15%;height:22%;border-radius:12px}.merchant{top:83.6%;width:14.9%;height:12%;border-radius:12px}.activity-more{right:2.2%;top:51.4%;width:5.2%;height:2.6%}.mini-menu{position:absolute;z-index:30;width:230px;padding:12px;border:1px solid rgba(99,139,226,.28);border-radius:12px;background:rgba(6,18,42,.96);box-shadow:0 22px 60px rgba(0,0,0,.4);backdrop-filter:blur(18px)}.notifications{top:5.2%;right:4%}.profile-menu{top:5.2%;right:1%;width:165px}.mini-menu strong,.mini-menu span{display:block}.mini-menu strong{margin-bottom:7px}.mini-menu span{padding:7px 0;border-top:1px solid rgba(102,130,190,.1);color:#9aacca;font-size:11px}.mini-menu button{width:100%;padding:8px;border:0;border-radius:7px;color:#cbd8ef;background:transparent;text-align:left;cursor:pointer}.mini-menu button:hover{background:rgba(77,108,178,.14)}.tm-overlay{position:fixed;z-index:100;inset:0;display:grid;place-items:center;padding:20px;background:rgba(0,6,18,.72);backdrop-filter:blur(8px)}.tm-dialog{position:relative;width:min(470px,100%);padding:26px;border:1px solid rgba(105,139,219,.32);border-radius:18px;background:linear-gradient(180deg,#10224d 0%,#07152f 55%,#040d1f 100%);box-shadow:0 30px 100px rgba(0,0,0,.52),0 0 70px rgba(72,82,255,.12)}.tm-dialog .close{position:absolute;top:14px;right:14px;width:32px;height:32px;border:1px solid rgba(106,136,202,.25);border-radius:50%;color:#9db0d4;background:#13244a;font-size:19px;cursor:pointer}.eyebrow{color:#7289b6;font-size:10px;font-weight:800;letter-spacing:.14em}.tm-dialog h2{margin:5px 0 14px;font-size:24px}.tm-dialog p{color:#8b9dbd;font-size:12px;line-height:1.75}.exchange-summary{display:grid;grid-template-columns:1fr 36px 1fr;align-items:center;gap:8px;margin:20px 0}.exchange-summary strong{display:grid;place-items:center;min-height:68px;border:1px solid rgba(105,136,205,.18);border-radius:11px;background:rgba(6,19,46,.74)}.exchange-summary span{color:#7087b5;text-align:center}.price{margin:18px 0;color:#ff82aa;font-size:26px;font-weight:900}.actions{display:flex;gap:10px;margin-top:20px}.actions>*{flex:1}.primary-btn,.secondary-btn{min-height:40px;border-radius:9px;font-weight:800;cursor:pointer}.primary-btn{border:0;color:#fff;background:linear-gradient(90deg,#743cff,#4d67ff,#25c5ff)}.secondary-btn{border:1px solid rgba(100,132,206,.24);color:#d3def1;background:#0a1834}.full{width:100%;margin-top:14px}.tm-toast{position:fixed;z-index:130;left:50%;bottom:28px;transform:translateX(-50%);max-width:calc(100% - 32px);padding:11px 16px;border:1px solid rgba(109,143,218,.28);border-radius:10px;color:#eaf2ff;background:rgba(7,21,48,.97);box-shadow:0 15px 44px rgba(0,0,0,.42);font-size:11px}.cart-fab{position:fixed;z-index:50;right:24px;bottom:24px;width:52px;height:52px;border:1px solid rgba(121,141,255,.5);border-radius:50%;color:#fff;background:linear-gradient(135deg,#4f61ff,#843eff);box-shadow:0 15px 44px rgba(71,54,255,.35);cursor:pointer}.cart-fab span{position:absolute;top:-5px;right:-4px;display:grid;place-items:center;min-width:19px;height:19px;border:2px solid #061022;border-radius:999px;background:#ff547e;font-size:9px;font-weight:800}.fade-enter-active,.fade-leave-active,.toast-enter-active,.toast-leave-active{transition:opacity .18s ease,transform .18s ease}.fade-enter-from,.fade-leave-to{opacity:0}.toast-enter-from,.toast-leave-to{opacity:0;transform:translate(-50%,8px)}@media(max-width:900px){.tm-stage{width:100vw;min-width:980px;transform-origin:top left}.tm-page{overflow-x:auto}}
</style>
