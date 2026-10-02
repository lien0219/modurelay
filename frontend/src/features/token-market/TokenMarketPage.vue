<template>
  <div class="tm-home">
    <div class="tm-home-top">
      <section class="tm-home-hero">
        <img src="/token-market/art/hero-space.png" alt="发光的 Token 标识置于星空场景中" fetchpriority="high" />
        <div class="tm-home-hero-copy">
          <span>TOKEN MARKET · 探索你的数字生活</span>
          <h1>用 Token<br />解锁更多精彩</h1>
          <p>好物、数字商品与专业服务，一站直达。</p>
          <RouterLink class="tm-button primary" to="/token-market/channel?category=ai_credit">开始探索 <span aria-hidden="true">→</span></RouterLink>
        </div>
      </section>
      <section class="tm-home-wallet tm-panel">
        <h2>我的 Token 钱包</h2>
        <strong>{{ market.wallet ? formatToken(market.wallet.tokenBalance) : '—' }}</strong>
        <span>可用余额{{ isTokenMarketDemo ? '（演示）' : '' }}</span>
        <div class="tm-home-wallet-actions">
          <RouterLink class="tm-button primary" to="/token-market/recharge">充值</RouterLink>
          <RouterLink class="tm-button" to="/token-market/exchange">兑换</RouterLink>
        </div>
        <p v-if="market.wallet">今日消费 {{ formatToken(market.wallet.todaySpentToken) }}</p>
      </section>
    </div>
    <div class="tm-category-grid">
      <RouterLink v-for="category in market.categories" :key="category.id" class="tm-category tm-panel" :to="`/token-market/channel?category=${category.id}`">
        <img :src="`/token-market/icons/${categoryIcon[category.id]}.svg`" alt="" />
        <span><strong>{{ category.label }}</strong><small>{{ category.description }}</small></span>
      </RouterLink>
    </div>
    <section class="tm-section">
      <div class="tm-section-head"><h2>为你精选</h2><RouterLink class="tm-text-link" to="/token-market/channel?category=mall">查看全部 →</RouterLink></div>
      <div v-if="featured.length" class="tm-home-products">
        <MarketProductCard v-for="product in featured" :key="product.id" :product="product" :merchant-name="merchantName(product.merchantId)" @added="showToast" />
      </div>
      <div v-else class="tm-empty"><h2>暂无精选商品</h2><p>商品上架后会在这里展示。</p></div>
    </section>
    <section class="tm-section">
      <div class="tm-section-head"><h2>值得关注的商家</h2><RouterLink class="tm-text-link" to="/token-market/channel?category=services">查看全部 →</RouterLink></div>
      <div class="tm-merchant-grid">
        <RouterLink v-for="merchant in market.merchants.slice(0, 3)" :key="merchant.id" class="tm-merchant-card tm-panel" :to="`/token-market/merchants/${merchant.id}`">
          <img :src="`/token-market/icons/${categoryIcon[merchant.categoryId]}.svg`" alt="" />
          <span><strong>{{ merchant.name }}</strong><small>{{ merchant.rating.toFixed(1) }} · {{ merchant.soldCount.toLocaleString('zh-CN') }} 笔成交</small></span>
          <span aria-hidden="true">→</span>
        </RouterLink>
      </div>
    </section>
    <div v-if="toast" class="tm-toast" role="status">{{ toast }}已加入本地购物车草稿</div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { RouterLink } from 'vue-router'
import MarketProductCard from './components/MarketProductCard.vue'
import { categoryLabels, formatToken } from './presentation'
import { isTokenMarketDemo } from './service'
import { useTokenMarketStore } from './store'
const market = useTokenMarketStore()
const categoryIcon: Record<string, string> = { mall: 'shop', delivery: 'food', digital: 'digital', ai_credit: 'ai', services: 'service' }
const featured = computed(() => ['p-headset', 'p-keyboard', 'p-ai-art', 'p-light'].map(id => market.products.find(item => item.id === id)).filter((item): item is NonNullable<typeof item> => !!item).concat(market.products.filter(item => !['p-headset', 'p-keyboard', 'p-ai-art', 'p-light'].includes(item.id))).slice(0, 4))
const toast = ref('')
let timer: number | undefined
function merchantName(id: string): string { return market.merchants.find(item => item.id === id)?.name || categoryLabels.mall }
function showToast(name: string): void { toast.value = name; window.clearTimeout(timer); timer = window.setTimeout(() => { toast.value = '' }, 3000) }
onBeforeUnmount(() => window.clearTimeout(timer))
</script>
<style scoped>
.tm-home-top{display:grid;grid-template-columns:minmax(0,2.4fr) minmax(260px,1fr);gap:24px}.tm-home-hero{position:relative;min-height:300px;overflow:hidden;border-radius:20px;background:#101934}.tm-home-hero>img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;object-position:center}.tm-home-hero::after{content:'';position:absolute;inset:0;background:linear-gradient(90deg,#050b1cdd 0%,#050b1c66 46%,transparent 78%)}.tm-home-hero-copy{position:relative;z-index:1;padding:32px;max-width:460px}.tm-home-hero-copy>span{display:inline-block;margin-bottom:16px;padding:5px 10px;border-radius:5px;background:#182743c9;color:var(--tm-cyan);font-size:12px}.tm-home-hero-copy h1{font-size:48px;line-height:1.22;margin:0 0 12px}.tm-home-hero-copy p{color:var(--tm-muted)}.tm-home-hero-copy .tm-button{min-width:164px}.tm-home-wallet{padding:24px}.tm-home-wallet h2{margin:0 0 18px;font-size:17px}.tm-home-wallet>strong{display:block;font-size:36px;line-height:1.3}.tm-home-wallet>span,.tm-home-wallet p{color:var(--tm-muted);font-size:12px}.tm-home-wallet-actions{display:flex;gap:10px;margin-top:16px;padding-top:16px;border-top:1px solid var(--tm-border)}.tm-home-wallet-actions a{flex:1}.tm-home-wallet p{margin:18px 0 0}.tm-category-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:16px;margin-top:24px}.tm-category{display:flex;align-items:center;gap:14px;min-width:0;min-height:77px;padding:14px 17px}.tm-category:hover,.tm-merchant-card:hover{border-color:var(--tm-cyan)}.tm-category img{width:29px;height:29px}.tm-category span{min-width:0}.tm-category strong,.tm-category small,.tm-merchant-card strong,.tm-merchant-card small{display:block}.tm-category small,.tm-merchant-card small{color:var(--tm-muted);font-size:12px}.tm-home-products{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:16px}.tm-merchant-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.tm-merchant-card{display:flex;align-items:center;gap:16px;min-height:90px;padding:18px}.tm-merchant-card img{width:32px;height:32px}.tm-merchant-card span:nth-child(2){min-width:0;flex:1}.tm-merchant-card span:last-child{color:var(--tm-muted)}
@media(max-width:1100px){.tm-home-top{grid-template-columns:1fr}.tm-home-wallet{display:grid;grid-template-columns:1fr auto;align-items:center}.tm-home-wallet h2,.tm-home-wallet>span,.tm-home-wallet p{grid-column:1}.tm-home-wallet-actions{grid-column:2;grid-row:1/4;border:0;margin:0;padding:0}.tm-category-grid{grid-template-columns:repeat(3,1fr)}.tm-home-products{grid-template-columns:repeat(2,1fr)}}
@media(max-width:700px){.tm-home-top{gap:16px}.tm-home-hero{min-height:250px;border-radius:14px}.tm-home-hero-copy{padding:25px}.tm-home-hero-copy>span{display:none}.tm-home-hero-copy h1{font-size:32px}.tm-home-hero-copy p{max-width:225px;font-size:13px}.tm-home-wallet{display:block;padding:20px}.tm-home-wallet h2{font-size:14px;color:var(--tm-muted)}.tm-home-wallet>strong{font-size:30px}.tm-home-wallet-actions{display:none}.tm-home-wallet p{display:none}.tm-category-grid{grid-template-columns:repeat(5,1fr);gap:4px;margin-top:19px}.tm-category{display:flex;flex-direction:column;min-height:78px;gap:5px;padding:7px 2px;border:0;background:none;text-align:center}.tm-category img{width:27px;height:27px}.tm-category strong{font-size:12px;white-space:nowrap}.tm-category small{display:none}.tm-home-products{grid-template-columns:1fr;gap:12px}.tm-merchant-grid{grid-template-columns:1fr;gap:10px}.tm-section{margin-top:27px}.tm-section-head h2{font-size:21px}}
.tm-home-hero-copy h1{font-weight:700}
</style>
