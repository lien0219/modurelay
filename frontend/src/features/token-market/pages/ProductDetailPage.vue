<template>
  <div class="tm-detail">
    <div v-if="loading" class="tm-feedback" role="status">正在加载商品详情…</div>
    <div v-else-if="error" class="tm-empty" role="alert"><h2>商品加载失败</h2><p>{{ error }}</p><button class="tm-button" @click="load">重新加载</button></div>
    <div v-else-if="!product" class="tm-empty"><h2>商品不存在</h2><p>商品可能已下架或链接已失效。</p><RouterLink class="tm-button" to="/token-market">返回市场</RouterLink></div>
    <template v-else>
      <div class="tm-breadcrumb"><RouterLink to="/token-market">市场首页</RouterLink> / <RouterLink :to="`/token-market/channel?category=${product.categoryId}`">{{ categoryLabels[product.categoryId] }}</RouterLink> / 商品详情</div>
      <div class="tm-detail-grid">
        <div class="tm-detail-art tm-panel"><img :src="productArt(product)" :alt="product.name" /></div>
        <section class="tm-detail-info">
          <span class="tm-detail-badge">{{ product.badge || categoryLabels[product.categoryId] }}</span>
          <h1>{{ product.name }}</h1>
          <p>{{ merchant?.name || 'Token Market 商家' }} · {{ categoryLabels[product.categoryId] }}</p>
          <strong class="tm-price">{{ formatToken(product.priceToken) }}</strong>
          <div class="tm-detail-divider"></div>
          <div v-if="product.categoryId === 'ai_credit'" class="tm-detail-model">
            <h2>模型 Token 配置</h2>
            <dl><div><dt>适用模型</dt><dd>{{ product.modelId || '商家尚未提供' }}</dd></div><div><dt>Token 类型</dt><dd>{{ product.tokenTypes?.join(' / ') || '商家尚未提供' }}</dd></div><div><dt>模型 Token 数量</dt><dd>{{ product.modelTokenQuantity || '商家尚未提供' }}</dd></div><div><dt>组合</dt><dd>{{ product.packageCombination || '单项商品' }}</dd></div><div><dt>钱包支付金额</dt><dd>{{ formatToken(product.priceToken) }}</dd></div></dl>
            <p class="tm-muted">模型 Token 数量与钱包支付金额为不同字段。官方参考价和兑换报价以服务端实时数据为准。</p>
          </div>
          <label class="tm-detail-quantity">购买数量 <span><button type="button" aria-label="减少数量" :disabled="quantity <= 1" @click="quantity--">−</button><output>{{ quantity }}</output><button type="button" aria-label="增加数量" :disabled="quantity >= 99" @click="quantity++">+</button></span></label>
          <p class="tm-detail-hint">{{ product.categoryId === 'digital' || product.categoryId === 'ai_credit' ? '数字商品的交付内容与时效以商家说明为准。' : '运费、优惠与最终应付金额由结算接口确认。' }}</p>
          <div class="tm-detail-actions"><button class="tm-button" type="button" @click="addCart(false)">加入购物车</button><button class="tm-button primary" type="button" @click="addCart(true)">去结算</button><button class="tm-icon-button" type="button" :aria-label="favorite ? '取消收藏' : '收藏商品'" :aria-pressed="favorite" @click="draft.toggleProductFavorite(product.id)"><img src="/token-market/icons/heart.svg" alt="" /></button></div>
        </section>
      </div>
      <div class="tm-tabs" role="tablist" aria-label="商品信息"><button v-for="tab in ['商品详情','购买须知','用户评价']" :key="tab" class="tm-tab" :class="{ 'is-active': activeTab === tab }" role="tab" type="button" :aria-selected="activeTab === tab" @click="activeTab=tab">{{ tab }}</button></div>
      <section class="tm-detail-section tm-panel"><template v-if="activeTab === '商品详情'"><h2>{{ product.name }}</h2><p>价格、适用范围、交付方式与售后条款请以商家发布的商品信息和服务端结算结果为准。</p></template><template v-else-if="activeTab === '购买须知'"><h2>购买须知</h2><p>下单前请核对商品规格、模型、Token 类型和数量。退款与交付遵循平台已有规则，相关接口接入前无法在线完成交易。</p></template><template v-else><h2>用户评价</h2><p>评价接口尚未接入，暂无可核实的评价。</p></template></section>
      <section v-if="merchant" class="tm-detail-merchant tm-panel"><img src="/token-market/icons/store.svg" alt="" /><div><h2>{{ merchant.name }}</h2><p>{{ categoryLabels[merchant.categoryId] }}</p></div><RouterLink class="tm-button" :to="`/token-market/merchants/${merchant.id}`">进入店铺</RouterLink></section>
      <div v-if="toast" class="tm-toast" role="status">{{ toast }}</div>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useMarketDraftStore } from '../experience'
import { categoryLabels, formatToken, productArt } from '../presentation'
import { useTokenMarketStore } from '../store'
import type { TokenMarketProduct } from '../types'
const route = useRoute()
const router = useRouter()
const market = useTokenMarketStore()
const draft = useMarketDraftStore()
const product = ref<TokenMarketProduct | null>(null)
const loading = ref(false)
const error = ref('')
const quantity = ref(1)
const activeTab = ref('商品详情')
const toast = ref('')
let timer: number | undefined
const merchant = computed(() => market.merchants.find(item => item.id === product.value?.merchantId))
const favorite = computed(() => !!product.value && draft.data.favoriteProductIds.includes(product.value.id))
async function load(): Promise<void> {
  loading.value = true; error.value = ''; product.value = null
  try { product.value = await market.loadProduct(String(route.params.id)) }
  catch (failure) { error.value = failure instanceof Error ? failure.message : '请稍后重试' }
  finally { loading.value = false }
}
function addCart(checkout: boolean): void {
  if (!product.value) return
  for (let index = 0; index < quantity.value; index++) draft.addToCart(product.value.id)
  if (checkout) void router.push({ path: '/token-market/checkout', query: { buy: product.value.id } })
  else { toast.value = '已加入本地购物车草稿'; window.clearTimeout(timer); timer = window.setTimeout(() => { toast.value = '' }, 2800) }
}
watch(() => route.params.id, load, { immediate: true })
onBeforeUnmount(() => window.clearTimeout(timer))
</script>
<style scoped>
.tm-breadcrumb{margin-bottom:22px;color:var(--tm-muted);font-size:12px}.tm-breadcrumb a:hover{color:var(--tm-cyan)}.tm-detail-grid{display:grid;grid-template-columns:minmax(0,1fr) minmax(330px,1fr);gap:32px}.tm-detail-art{display:grid;place-items:center;aspect-ratio:1.4;background:radial-gradient(circle at 50% 38%,#223670,#0b1633 72%);overflow:hidden}.tm-detail-art img{width:100%;height:100%;object-fit:contain}.tm-detail-info{min-width:0}.tm-detail-badge{display:inline-block;padding:5px 10px;border-radius:5px;background:var(--tm-raised);color:var(--tm-cyan);font-size:12px}.tm-detail-info h1{margin:18px 0 10px}.tm-detail-info>p{color:var(--tm-muted)}.tm-detail-info>.tm-price{display:block;margin:18px 0;font-size:36px}.tm-detail-divider{height:1px;background:var(--tm-border);margin:18px 0}.tm-detail-model{margin-bottom:18px}.tm-detail-model h2{font-size:16px}.tm-detail-model dl{margin:0}.tm-detail-model dl div{display:flex;gap:12px;padding:6px 0}.tm-detail-model dt{min-width:105px;color:var(--tm-muted)}.tm-detail-model dd{margin:0;overflow-wrap:anywhere}.tm-detail-model p{font-size:12px}.tm-detail-quantity{display:flex;align-items:center;gap:20px}.tm-detail-quantity span{display:flex;align-items:center;gap:12px}.tm-detail-quantity button{width:36px;height:36px;border:1px solid var(--tm-border);border-radius:8px;background:var(--tm-surface);color:var(--tm-cyan)}.tm-detail-quantity output{min-width:20px;text-align:center}.tm-detail-hint{margin:14px 0;color:var(--tm-muted);font-size:12px}.tm-detail-actions{display:flex;align-items:center;gap:10px}.tm-detail-actions>.tm-button{flex:1}.tm-detail-section{padding:24px;min-height:120px}.tm-detail-section p,.tm-detail-merchant p{color:var(--tm-muted)}.tm-detail-merchant{display:flex;align-items:center;gap:18px;margin-top:22px;padding:20px}.tm-detail-merchant img{width:32px}.tm-detail-merchant div{flex:1}.tm-detail-merchant h2{margin:0}.tm-detail-merchant p{margin:3px 0 0}
@media(max-width:850px){.tm-detail-grid{grid-template-columns:1fr}.tm-detail-art{aspect-ratio:1.5}.tm-detail-merchant{flex-wrap:wrap}}
@media(max-width:700px){.tm-detail-grid{gap:20px}.tm-detail-art{aspect-ratio:1.35}.tm-detail-info>.tm-price{font-size:29px}.tm-detail-actions{position:sticky;bottom:76px;z-index:10;padding:10px 0;background:var(--tm-bg)}.tm-detail-section{padding:17px}.tm-detail .tm-tabs{flex-wrap:nowrap;overflow-x:auto}.tm-detail .tm-tab{white-space:nowrap}}
</style>
