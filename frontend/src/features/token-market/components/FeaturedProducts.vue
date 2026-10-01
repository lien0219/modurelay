<template>
  <section class="tm-section featured">
    <div class="tm-section-heading">
      <div class="title-row"><span class="fire">◆</span><div><h2>精选商品</h2><p>优质好物，Token 即刻兑换</p></div></div>
      <button class="tm-link-button" type="button" @click="emit('more')">查看更多 →</button>
    </div>
    <div class="product-grid">
      <article v-for="(product,index) in products.slice(0,6)" :key="product.id" class="product-card" @click="emit('select',product)">
        <div class="product-image"><DesignSprite v-bind="sprite(index)" :radius="12" /></div>
        <div class="product-body">
          <strong>{{ product.name }}</strong>
          <div class="tags"><span>{{ product.badge || tag(index) }}</span><span>{{ category(product.categoryId) }}</span></div>
          <div class="price-row"><b>{{ product.priceToken.toLocaleString('zh-CN') }} T</b><button type="button" aria-label="加入购物车" @click.stop="emit('select',product)"><MarketIcon name="bag" /></button></div>
        </div>
      </article>
    </div>
  </section>
</template>
<script setup lang="ts">
import DesignSprite from './DesignSprite.vue'
import MarketIcon from './MarketIcon.vue'
import type { TokenMarketMerchant,TokenMarketProduct,TokenMarketCategoryId } from '../types'
const props=defineProps<{products:TokenMarketProduct[];merchants:TokenMarketMerchant[]}>()
const emit=defineEmits<{select:[product:TokenMarketProduct];more:[]}>()
const sprites=[{x:40,y:525,width:205,height:105},{x:255,y:525,width:200,height:105},{x:470,y:525,width:195,height:105},{x:680,y:525,width:195,height:105},{x:890,y:525,width:195,height:105},{x:1100,y:525,width:205,height:105}]
function sprite(i:number){return sprites[i]||sprites[0]}
function tag(i:number){return ['官方正品','限时特惠','热门','官方充值','品质好物','热门'][i]||'精选'}
function category(id:TokenMarketCategoryId){return ({mall:'商城',delivery:'外卖',digital:'数字商品',ai_credit:'AI额度',services:'服务'} as const)[id]}
</script>
<style scoped>
.title-row{display:flex;align-items:center;gap:10px}.fire{color:#ff8a68;font-size:16px}.product-grid{display:grid;grid-template-columns:repeat(6,minmax(0,1fr));gap:10px}.product-card{overflow:hidden;border:1px solid rgba(111,141,205,.18);border-radius:14px;background:linear-gradient(180deg,rgba(15,31,67,.95),rgba(8,20,45,.96));cursor:pointer;transition:.18s}.product-card:hover{transform:translateY(-2px);border-color:rgba(92,126,255,.42);box-shadow:0 12px 30px rgba(28,49,104,.28)}.product-image{height:104px;background:#07142d}.product-body{padding:10px 11px 11px}.product-body>strong{display:block;overflow:hidden;color:#eef4ff;font-size:11px;text-overflow:ellipsis;white-space:nowrap}.tags{display:flex;gap:5px;margin-top:7px}.tags span{padding:3px 5px;border-radius:5px;color:#95a9cf;background:rgba(69,91,151,.25);font-size:7px}.tags span:first-child{color:#b7c9ff;background:rgba(77,93,195,.28)}.price-row{display:flex;align-items:center;justify-content:space-between;margin-top:8px}.price-row b{color:#ff86a9;font-size:14px}.price-row button{display:grid;place-items:center;width:28px;height:28px;border:1px solid rgba(105,132,227,.3);border-radius:8px;color:#cbd8ff;background:linear-gradient(135deg,#34459f,#22376e);cursor:pointer}.price-row button :deep(svg){font-size:14px}@media(max-width:1200px){.product-grid{grid-template-columns:repeat(3,1fr)}}@media(max-width:650px){.product-grid{grid-template-columns:repeat(2,1fr)}}
</style>
