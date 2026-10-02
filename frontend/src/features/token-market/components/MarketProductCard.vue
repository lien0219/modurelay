<template>
  <article class="tm-product-card tm-panel">
    <RouterLink :to="`/token-market/products/${product.id}`" class="tm-product-open" :aria-label="`查看${product.name}`">
      <div class="tm-product-art"><img :src="productArt(product)" :alt="product.name" loading="lazy" /></div>
      <div class="tm-product-copy"><h3>{{ product.name }}</h3><p>{{ merchantName }}</p><strong class="tm-price">{{ formatToken(product.priceToken) }}</strong></div>
    </RouterLink>
    <button class="tm-product-add tm-icon-button" type="button" :aria-label="`将${product.name}加入购物车`" title="加入购物车" @click="add">
      <img src="/token-market/icons/plus.svg" alt="" />
    </button>
  </article>
</template>
<script setup lang="ts">
import { useMarketDraftStore } from '../experience'
import { formatToken, productArt } from '../presentation'
import type { TokenMarketProduct } from '../types'
const props = defineProps<{ product: TokenMarketProduct; merchantName?: string }>()
const emit = defineEmits<{ added: [name: string] }>()
const draft = useMarketDraftStore()
function add(): void { draft.addToCart(props.product.id); emit('added', props.product.name) }
</script>
<style scoped>
.tm-product-card{position:relative;min-width:0;overflow:hidden;border-radius:8px}.tm-product-open{display:block}.tm-product-art{height:138px;background:radial-gradient(circle at 50% 35%,#223870,#0a1530 75%);overflow:hidden}.tm-product-art img{display:block;width:100%;height:100%;object-fit:contain}.tm-product-copy{padding:13px 16px 16px;min-height:112px}.tm-product-copy h3{margin:0 0 4px;overflow-wrap:anywhere}.tm-product-copy p{margin:0 0 4px;color:var(--tm-muted);font-size:12px}.tm-price{font-size:21px}.tm-product-add{position:absolute;right:7px;bottom:9px}.tm-product-add img{width:20px;height:20px}@media(max-width:700px){.tm-product-card{border-radius:14px}.tm-product-open{display:flex;align-items:center;gap:12px;padding:12px}.tm-product-art{flex:none;width:104px;height:104px;border-radius:10px}.tm-product-copy{min-width:0;min-height:0;padding:0 28px 0 0}.tm-product-copy h3{font-size:16px}.tm-product-copy p{font-size:12px}.tm-product-add{right:9px;bottom:8px}}
</style>
