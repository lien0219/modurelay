<template>
  <TokenMarketSubpageLayout
    eyebrow="MERCHANT STORE"
    :title="merchant?.name ?? '商家主页'"
    description="统一承载商家介绍、商品、评分、配送与售后能力；后续由 Merchant/Commerce API 驱动。"
  >
    <template #actions>
      <button class="tm-button secondary" type="button">♡ 收藏商家</button>
      <button class="tm-button primary" type="button">联系商家</button>
    </template>

    <section v-if="merchant" class="store-hero glass-card">
      <div class="avatar">{{ merchant.name.slice(0,1) }}</div>
      <div class="meta"><span class="eyebrow">VERIFIED MERCHANT</span><h2>{{ merchant.name }}</h2><p>官方认证商家 · Token Market 担保交易</p></div>
      <div class="stats"><div><b>{{ merchant.rating.toFixed(1) }}</b><span>评分</span></div><div><b>{{ merchant.soldCount.toLocaleString('zh-CN') }}</b><span>成交</span></div><div><b>99.8%</b><span>履约率</span></div></div>
    </section>

    <section class="toolbar glass-card"><button class="active">全部商品</button><button>热销</button><button>新品</button><button>服务</button><span></span><select><option>综合排序</option><option>价格从低到高</option></select></section>

    <div class="products">
      <button v-for="product in products" :key="product.id" class="product glass-card" type="button" @click="router.push(`/token-market/products/${product.id}`)">
        <div class="art">✦</div><span>{{ product.badge || '精选' }}</span><h3>{{ product.name }}</h3><p>{{ product.priceToken.toLocaleString('zh-CN') }} <small>T</small></p>
      </button>
      <div v-if="!products.length" class="empty glass-card">该商家暂时没有上架商品。</div>
    </div>
  </TokenMarketSubpageLayout>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'
import { useTokenMarketStore } from '../store'

const route = useRoute(); const router = useRouter(); const market = useTokenMarketStore()
const merchant = computed(() => market.merchants.find(item => item.id === String(route.params.id)))
const products = computed(() => market.products.filter(item => item.merchantId === merchant.value?.id))
onMounted(() => market.initialize())
</script>

<style scoped>
.store-hero{display:flex;align-items:center;gap:18px;padding:24px}.avatar{display:grid;place-items:center;width:76px;height:76px;border-radius:22px;background:linear-gradient(135deg,#6f42ff,#258fff);font-size:28px;font-weight:900;box-shadow:0 18px 46px rgba(63,68,255,.28)}.meta{flex:1}.eyebrow{color:#6d83aa;font-size:9px;font-weight:800;letter-spacing:.13em}.meta h2{margin:5px 0 4px}.meta p{margin:0;color:#7890b4;font-size:11px}.stats{display:flex;gap:24px}.stats div{text-align:center}.stats b,.stats span{display:block}.stats b{font-size:18px}.stats span{margin-top:4px;color:#6c82a8;font-size:9px}.toolbar{display:flex;align-items:center;gap:6px;margin:18px 0;padding:10px}.toolbar button,.toolbar select{padding:8px 11px;border:0;border-radius:8px;color:#7287aa;background:transparent;font-size:10px}.toolbar button.active{color:#fff;background:rgba(82,91,255,.15)}.toolbar span{flex:1}.toolbar select{border:1px solid rgba(111,140,205,.15);background:#081733}.products{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:14px}.product{padding:14px;text-align:left;cursor:pointer}.art{height:150px;display:grid;place-items:center;margin-bottom:12px;border-radius:12px;background:radial-gradient(circle at 60% 30%,rgba(105,80,255,.38),transparent 35%),linear-gradient(135deg,#101d45,#07132c);font-size:42px}.product>span{color:#6d83aa;font-size:8px}.product h3{min-height:42px;margin:6px 0;color:#dfe8f8;font-size:12px}.product p{margin:0;color:#ff7aa7;font-size:17px;font-weight:900}.product small{color:#a9b8d3}.empty{grid-column:1/-1;padding:40px;text-align:center;color:#7187aa}@media(max-width:1000px){.products{grid-template-columns:repeat(2,1fr)}.stats{display:none}}@media(max-width:600px){.products{grid-template-columns:1fr}.store-hero{align-items:flex-start}.avatar{width:58px;height:58px}}
</style>
