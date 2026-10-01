<template>
  <TokenMarketSubpageLayout
    eyebrow="PRODUCT DETAIL"
    :title="product?.name ?? '商品详情'"
    description="商品详情、SKU、库存、配送和 Token 支付均通过独立 Commerce API 接入；当前页面为可交互骨架。"
    :empty="!market.loading && !!market.bootstrap && !product"
    empty-title="商品不存在"
    empty-description="该商品可能已下架、不可用，或当前链接已失效。"
  >
    <template #actions>
      <button class="tm-button secondary" type="button" @click="router.push('/token-market/cart')">购物车</button>
    </template>
    <template #emptyActions>
      <button class="tm-button primary" type="button" @click="router.push('/token-market')">返回市场首页</button>
    </template>

    <template v-if="product">
      <div class="detail-grid">
        <section class="visual glass-card">
          <div class="visual-orb"><span>{{ categoryIcon }}</span></div>
          <div class="visual-meta"><span>{{ product.badge || '精选商品' }}</span><strong>{{ categoryLabel }}</strong></div>
        </section>

        <section class="info glass-card">
          <span class="merchant" @click="openMerchant">{{ merchant?.name || 'Token Market 商户' }} ›</span>
          <h2>{{ product.name }}</h2>
          <p>企业级商品详情骨架已预留规格、库存、保障、配送方式、售后规则与商家信息区域。</p>
          <div class="price"><b>{{ product.priceToken.toLocaleString('zh-CN') }}</b><span>T</span></div>
          <div class="sku-block">
            <label>规格</label>
            <div class="chips"><button class="active" type="button">标准版</button><button type="button">高级版</button><button type="button">企业版</button></div>
          </div>
          <div class="buy-row">
            <div class="qty"><button type="button" @click="quantity=Math.max(1,quantity-1)">−</button><strong>{{ quantity }}</strong><button type="button" @click="quantity+=1">＋</button></div>
            <button class="tm-button secondary grow" type="button" @click="addToCart">加入购物车</button>
            <button class="tm-button primary grow" type="button" @click="buyNow">立即购买 · {{ total.toLocaleString('zh-CN') }} T</button>
          </div>
        </section>
      </div>

      <section class="below-grid">
        <article class="glass-card section"><span class="eyebrow">DETAILS</span><h3>商品详情</h3><p>这里将接入富文本详情、规格参数、服务说明、数字商品交付规则或实体商品物流信息。</p></article>
        <article class="glass-card section"><span class="eyebrow">PROTECTION</span><h3>Token Market 交易保障</h3><p>预留担保交易、退款、争议处理、商家保证金与履约状态展示。</p></article>
      </section>
    </template>
  </TokenMarketSubpageLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'
import { useTokenMarketStore } from '../store'

const route = useRoute()
const router = useRouter()
const market = useTokenMarketStore()
const quantity = ref(1)
const product = computed(() => market.products.find(item => item.id === String(route.params.id)))
const merchant = computed(() => market.merchants.find(item => item.id === product.value?.merchantId))
const total = computed(() => (product.value?.priceToken ?? 0) * quantity.value)
const categoryLabel = computed(() => market.categories.find(item => item.id === product.value?.categoryId)?.label ?? '精选')
const categoryIcon = computed(() => ({ mall:'⌁', delivery:'◉', digital:'◇', ai_credit:'✦', services:'◆' }[product.value?.categoryId ?? 'mall']))

function openMerchant(): void { if (merchant.value) void router.push(`/token-market/merchants/${merchant.value.id}`) }
function addToCart(): void { void router.push({ path:'/token-market/cart', query:{ add: product.value?.id, qty:String(quantity.value) } }) }
function buyNow(): void { void router.push({ path:'/token-market/cart', query:{ buy: product.value?.id, qty:String(quantity.value) } }) }
onMounted(() => market.initialize())
</script>

<style scoped>
.detail-grid{display:grid;grid-template-columns:minmax(0,1.05fr) minmax(360px,.95fr);gap:18px}.visual{position:relative;min-height:500px;display:grid;place-items:center;overflow:hidden}.visual:before{content:"";position:absolute;width:420px;height:420px;border-radius:50%;background:radial-gradient(circle,rgba(85,112,255,.22),transparent 68%);filter:blur(4px)}.visual-orb{position:relative;display:grid;place-items:center;width:260px;height:260px;border:1px solid rgba(129,150,255,.24);border-radius:38%;background:linear-gradient(145deg,rgba(108,61,255,.32),rgba(19,164,255,.14));box-shadow:inset 0 0 70px rgba(116,78,255,.25),0 30px 80px rgba(0,0,0,.38);transform:rotate(-8deg)}.visual-orb span{font-size:86px;transform:rotate(8deg);text-shadow:0 0 32px rgba(145,168,255,.65)}.visual-meta{position:absolute;left:24px;bottom:22px}.visual-meta span,.visual-meta strong{display:block}.visual-meta span{color:#7186ab;font-size:9px}.visual-meta strong{margin-top:5px;font-size:15px}.info{padding:28px}.merchant{color:#7293c9;font-size:11px;cursor:pointer}.info h2{margin:12px 0 10px;font-size:30px}.info p{color:#7e91b0;font-size:12px;line-height:1.75}.price{display:flex;align-items:baseline;gap:8px;margin:28px 0;color:#fff}.price b{font-size:38px}.price span{color:#a9baff;font-weight:800}.sku-block label{display:block;margin-bottom:10px;color:#788dad;font-size:10px}.chips{display:flex;gap:8px}.chips button{padding:9px 12px;border:1px solid rgba(113,139,203,.16);border-radius:9px;color:#7f92b2;background:#091936;cursor:pointer}.chips button.active{border-color:#6278ff;color:#fff;background:rgba(82,91,255,.16)}.buy-row{display:flex;gap:9px;margin-top:28px}.qty{display:flex;align-items:center;border:1px solid rgba(107,135,199,.18);border-radius:9px;overflow:hidden}.qty button,.qty strong{width:36px;height:40px;display:grid;place-items:center;border:0;color:#dbe6fa;background:#091833}.qty button{cursor:pointer}.grow{flex:1}.below-grid{display:grid;grid-template-columns:1fr 1fr;gap:18px;margin-top:18px}.section{padding:22px}.section h3{margin:7px 0 8px}.section p{margin:0;color:#788cad;font-size:11px;line-height:1.7}.eyebrow{color:#6a80a8;font-size:9px;font-weight:800;letter-spacing:.12em}@media(max-width:900px){.detail-grid,.below-grid{grid-template-columns:1fr}.buy-row{flex-wrap:wrap}.visual{min-height:360px}}
</style>
