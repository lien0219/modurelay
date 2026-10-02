<template>
  <div v-if="merchant" class="tm-store">
    <section class="tm-store-hero tm-panel">
      <div class="tm-store-art" :class="`type-${merchant.categoryId}`"><img :src="merchant.categoryId === 'delivery' ? '/token-market/icons/3d-coin.svg' : merchant.categoryId === 'digital' ? '/token-market/art/chat-token.png' : '/token-market/art/model-core.png'" alt="" /></div>
      <div class="tm-store-info">
        <span class="tm-open-label">{{ merchant.categoryId === 'delivery' ? '外卖店铺' : 'Token Market 商家' }}</span>
        <h1>{{ merchant.name }}<span v-if="merchant.categoryId === 'delivery'"> · Token 外卖</span></h1>
        <p class="tm-store-stats">★ {{ merchant.rating.toFixed(1) }} · {{ merchant.soldCount.toLocaleString('zh-CN') }} 笔成交</p>
        <p>{{ categoryLabels[merchant.categoryId] }} · 查看当前上架商品</p>
      </div>
      <button class="tm-button tm-store-follow" type="button" :aria-pressed="favorite" @click="draft.toggleMerchantFavorite(merchant.id)">{{ favorite ? '已关注店铺' : '♡ 关注店铺' }}</button>
    </section>
    <div class="tm-tabs" role="tablist" aria-label="店铺商品分类">
      <button v-for="tab in tabs" :key="tab" class="tm-tab" :class="{ 'is-active': selectedTab === tab }" type="button" role="tab" :aria-selected="selectedTab === tab" @click="selectedTab=tab">{{ tab }}</button>
    </div>
    <div class="tm-store-main">
      <div>
        <div class="tm-section-head"><h2>{{ selectedTab === '全部商品' ? '店铺商品' : selectedTab }}</h2><select v-model="sort" class="tm-select tm-store-sort" aria-label="商品排序"><option value="default">综合排序</option><option value="low">价格从低到高</option><option value="high">价格从高到低</option></select></div>
        <div v-if="loading" class="tm-feedback" role="status">正在加载店铺商品…</div>
        <div v-else-if="error" class="tm-empty" role="alert"><h2>商品加载失败</h2><p>{{ error }}</p><button class="tm-button" type="button" @click="loadProducts">重新加载</button></div>
        <div v-else-if="visibleProducts.length" class="tm-store-products"><MarketProductCard v-for="product in visibleProducts" :key="product.id" :product="product" :merchant-name="merchant.name" @added="showToast" /></div>
        <div v-else class="tm-empty"><img src="/token-market/icons/shop.svg" alt="" /><h2>暂无商品</h2><p>该分类目前没有上架商品。</p><button class="tm-button" type="button" @click="selectedTab='全部商品'">查看全部</button></div>
      </div>
      <aside class="tm-store-cart tm-panel">
        <h2>购物车（{{ storeCartCount }}）</h2>
        <div v-if="storeCart.length">
          <div v-for="line in storeCart" :key="line.productId" class="tm-store-cart-line"><span>{{ productName(line.productId) }}</span><strong class="tm-price">{{ formatToken(productPrice(line.productId) * line.quantity) }}</strong><div><button type="button" :disabled="line.quantity <= 1" :aria-label="`减少${productName(line.productId)}数量`" @click="draft.setQuantity(line.productId,line.quantity-1)">−</button><span>{{ line.quantity }}</span><button type="button" :disabled="line.quantity >= 99" :aria-label="`增加${productName(line.productId)}数量`" @click="draft.setQuantity(line.productId,line.quantity+1)">+</button></div></div>
          <p class="tm-muted">商品小计，运费及优惠由服务端结算时确认</p>
          <strong class="tm-store-cart-total tm-price">{{ formatToken(storeCartTotal) }}</strong>
          <RouterLink class="tm-button primary" to="/token-market/cart">查看购物车 →</RouterLink>
        </div>
        <div v-else class="tm-muted">从商品卡片添加商品后，可在这里查看本地购物车草稿。</div>
      </aside>
    </div>
    <div v-if="storeCartCount" class="tm-store-mobile-checkout"><div><small>商品小计 · 待服务端结算</small><strong class="tm-price">{{ formatToken(storeCartTotal) }}</strong></div><RouterLink class="tm-button primary" :to="`/token-market/checkout?merchant=${merchant.id}`">去结算（{{ storeCartCount }}）</RouterLink></div>
    <div v-if="toast" class="tm-toast" role="status">{{ toast }}已加入本地购物车草稿</div>
  </div>
  <div v-else class="tm-empty"><h1>商家不存在</h1><p>该链接可能已失效，或商家暂未开放。</p><RouterLink class="tm-button" to="/token-market">返回市场首页</RouterLink></div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import MarketProductCard from '../components/MarketProductCard.vue'
import { useMarketDraftStore } from '../experience'
import { categoryLabels, formatToken } from '../presentation'
import { useTokenMarketStore } from '../store'
import type { TokenMarketProduct } from '../types'
const route = useRoute()
const market = useTokenMarketStore()
const draft = useMarketDraftStore()
const merchant = computed(() => market.merchants.find(item => item.id === String(route.params.id)))
const products = ref<TokenMarketProduct[]>([])
const loading = ref(false)
const error = ref('')
let requestId = 0
const favorite = computed(() => !!merchant.value && draft.data.favoriteMerchantIds.includes(merchant.value.id))
const tabs = ['全部商品', '热销', '新品']
const selectedTab = ref('全部商品')
const sort = ref('default')
const visibleProducts = computed(() => {
  let items = [...products.value]
  if (selectedTab.value === '热销') items = items.filter(item => item.badge === '热销')
  if (selectedTab.value === '新品') items = items.filter(item => item.badge === '新品')
  if (sort.value === 'low') items.sort((a, b) => a.priceToken - b.priceToken)
  if (sort.value === 'high') items.sort((a, b) => b.priceToken - a.priceToken)
  return items
})
const storeCart = computed(() => draft.data.cart.filter(line => products.value.some(item => item.id === line.productId)))
const storeCartCount = computed(() => storeCart.value.reduce((sum, line) => sum + line.quantity, 0))
const storeCartTotal = computed(() => storeCart.value.reduce((sum, line) => sum + productPrice(line.productId) * line.quantity, 0))
function productName(id: string): string { return products.value.find(item => item.id === id)?.name || '商品已下架' }
function productPrice(id: string): number { return products.value.find(item => item.id === id)?.priceToken || 0 }
async function loadProducts(): Promise<void> {
  const id = ++requestId
  products.value = []
  error.value = ''
  if (!merchant.value) return
  loading.value = true
  try {
    const loaded = await market.loadProducts({ merchantId: merchant.value.id, pageSize: 100 })
    if (id === requestId) products.value = loaded
  } catch (failure) {
    if (id === requestId) error.value = failure instanceof Error ? failure.message : '请稍后重试'
  } finally {
    if (id === requestId) loading.value = false
  }
}
const toast = ref('')
let timer: number | undefined
function showToast(name: string): void { toast.value = name; window.clearTimeout(timer); timer = window.setTimeout(() => { toast.value = '' }, 3000) }
watch(() => route.params.id, () => { selectedTab.value = '全部商品'; sort.value = 'default'; void loadProducts() }, { immediate: true })
onBeforeUnmount(() => { requestId++; window.clearTimeout(timer) })
</script>
<style scoped>
.tm-store-hero{display:flex;align-items:center;gap:24px;min-height:220px;padding:24px}.tm-store-art{flex:none;display:grid;place-items:center;width:310px;height:170px;border-radius:12px;background:radial-gradient(circle at 50% 45%,#2c3d77,#0a1530 70%);overflow:hidden}.tm-store-art img{width:100%;height:100%;object-fit:contain}.tm-store-art.type-delivery{background:linear-gradient(135deg,#354161,#16233f 55%,#432f47)}.tm-store-art.type-delivery img{width:72px;height:72px;filter:drop-shadow(0 10px 15px #0007)}.tm-store-info{min-width:0;flex:1}.tm-store-info h1{margin:12px 0}.tm-store-info p{margin:8px 0;color:var(--tm-muted)}.tm-store-info .tm-store-stats{color:#ffc77d}.tm-open-label{color:var(--tm-success);font-size:12px}.tm-store-follow{align-self:flex-start;min-width:150px}.tm-store-main{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:24px}.tm-store-sort{width:178px;min-height:42px}.tm-store-products{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.tm-store-cart{align-self:start;padding:24px}.tm-store-cart h2{margin-bottom:18px}.tm-store-cart-line{display:grid;grid-template-columns:1fr auto;gap:7px;padding:10px 0;border-bottom:1px solid var(--tm-border)}.tm-store-cart-line>.tm-price{font-size:16px}.tm-store-cart-line>div{grid-column:2;display:flex;align-items:center;gap:10px}.tm-store-cart-line button{border:0;background:none;color:var(--tm-cyan);font-size:20px}.tm-store-cart p{font-size:12px;margin:22px 0 10px}.tm-store-cart-total{display:block;margin-bottom:14px;font-size:29px}.tm-store-cart .tm-button{width:100%}
@media(max-width:1100px){.tm-store-main{grid-template-columns:1fr}.tm-store-cart{order:-1}.tm-store-cart>div{display:flex;align-items:center;gap:16px;flex-wrap:wrap}.tm-store-cart-line{min-width:190px}.tm-store-cart p{margin:0}.tm-store-cart-total{margin:0}.tm-store-cart .tm-button{width:auto}.tm-store-products{grid-template-columns:repeat(3,1fr)}}
@media(max-width:700px){.tm-store-hero{display:block;padding:0;border:0;background:none}.tm-store-art{width:100%;height:170px}.tm-store-info h1{font-size:25px;margin:16px 0 6px}.tm-store-info p{font-size:13px}.tm-store-follow{margin-top:9px}.tm-tabs .tm-tab{font-size:14px;padding:0 8px}.tm-store-products{grid-template-columns:1fr;gap:12px}.tm-store-cart{order:1;padding:16px}.tm-store-cart>div{display:block}.tm-store-cart-line{display:none}.tm-store-cart p{margin:0 0 8px}.tm-store-cart-total{display:inline-block;margin-right:15px}.tm-store-sort{width:140px}}
.tm-store-art.type-delivery img{width:132px;height:132px}
.tm-store-mobile-checkout{display:none}
@media(max-width:700px){.tm-store{padding-bottom:90px}.tm-store-mobile-checkout{position:fixed;left:0;right:0;bottom:calc(76px + env(safe-area-inset-bottom));z-index:45;display:flex;align-items:center;justify-content:space-between;gap:12px;min-height:70px;padding:10px 16px;border-top:1px solid var(--tm-border);background:var(--tm-surface)}.tm-store-mobile-checkout small{display:block;color:var(--tm-muted);font-size:11px}.tm-store-mobile-checkout .tm-price{font-size:22px}.tm-store-mobile-checkout .tm-button{flex:none}}
</style>
