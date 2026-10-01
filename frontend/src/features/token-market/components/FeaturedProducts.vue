<template>
  <section class="tm-section">
    <div class="tm-section-heading">
      <div><h2>精选推荐</h2><p>为你推荐的热门商品与服务</p></div>
      <button class="tm-link-button" type="button" @click="emit('more')">查看全部 →</button>
    </div>
    <div class="grid">
      <button v-for="(product,index) in products" :key="product.id" type="button" class="card" @click="emit('select', product)">
        <div class="visual" :class="`visual-${index % 6}`"><span>{{ visualGlyphs[index % visualGlyphs.length] }}</span></div>
        <div class="body">
          <span v-if="product.badge" class="badge">{{ product.badge }}</span>
          <h3>{{ product.name }}</h3>
          <p>{{ merchantName(product.merchantId) }}</p>
          <strong>{{ product.priceToken.toLocaleString('zh-CN') }} T</strong>
        </div>
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { TokenMarketMerchant, TokenMarketProduct } from '../types'

const props = defineProps<{
  products: TokenMarketProduct[]
  merchants: TokenMarketMerchant[]
}>()

const emit = defineEmits<{
  select: [product: TokenMarketProduct]
  more: []
}>()

const visualGlyphs = ['◉','⌨','AI','GPT','✦','◆']
function merchantName(id: string): string {
  return props.merchants.find((merchant) => merchant.id === id)?.name ?? 'Token Market 商户'
}
</script>

<style scoped>
.grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:13px}.card{padding:0;border:1px solid rgba(103,133,197,.15);border-radius:15px;overflow:hidden;color:#e4edfb;background:linear-gradient(180deg,rgba(12,28,62,.78),rgba(5,15,36,.86));text-align:left;cursor:pointer;transition:.2s ease}.card:hover{transform:translateY(-3px);border-color:rgba(111,130,255,.38);box-shadow:0 16px 38px rgba(0,0,0,.28)}.visual{display:grid;place-items:center;height:125px;background:radial-gradient(circle at 55% 28%,rgba(89,116,255,.46),transparent 30%),linear-gradient(145deg,#101b3c,#081229)}.visual span{display:grid;place-items:center;width:70px;height:70px;border-radius:22px;color:white;background:linear-gradient(145deg,rgba(130,135,255,.88),rgba(62,74,174,.48));box-shadow:0 20px 45px rgba(49,49,159,.35),inset 0 1px rgba(255,255,255,.22);font-size:20px;font-weight:900}.visual-1 span{color:#bca5ff}.visual-2 span{border-radius:50%;background:linear-gradient(145deg,#504aff,#7f55ff)}.visual-3 span{background:linear-gradient(145deg,#2edc9c,#178f72)}.visual-4 span{color:#ffe07b;background:linear-gradient(145deg,#ffd861,#8b6520)}.visual-5 span{background:linear-gradient(145deg,#5867ff,#2648bf)}.body{position:relative;padding:13px}.badge{position:absolute;top:12px;right:10px;padding:2px 6px;border-radius:999px;color:#ff7aa8;background:rgba(255,74,136,.12);font-size:7px}.body h3{margin:0;padding-right:30px;font-size:11px}.body p{margin:6px 0 11px;color:#7183a3;font-size:8px}.body strong{color:#ff7eae;font-size:14px}.body strong::first-letter{color:#ff7eae}@media(max-width:1200px){.grid{grid-template-columns:repeat(3,1fr)}}@media(max-width:650px){.grid{grid-template-columns:1fr 1fr}}@media(max-width:420px){.grid{grid-template-columns:1fr}}
</style>
