<template>
  <section class="tm-section merchants">
    <div class="tm-section-heading">
      <div class="title-row"><span class="crown">◆</span><div><h2>热门商家</h2><p>高评分 · 高成交 · 快速履约</p></div></div>
      <button class="tm-link-button" type="button" @click="emit('more')">全部商家 →</button>
    </div>
    <div class="merchant-grid">
      <article v-for="(merchant,index) in merchants.slice(0,5)" :key="merchant.id" class="merchant-card" @click="emit('select',merchant)">
        <div class="logo"><DesignSprite v-bind="sprite(index)" :radius="12" /></div>
        <div class="merchant-copy"><strong>{{ merchant.name }}</strong><small>★ {{ merchant.rating.toFixed(1) }} · {{ formatSold(merchant.soldCount) }} 成交</small></div>
        <button type="button">进店 →</button>
      </article>
    </div>
  </section>
</template>
<script setup lang="ts">
import DesignSprite from './DesignSprite.vue'
import type { TokenMarketMerchant } from '../types'
defineProps<{merchants:TokenMarketMerchant[]}>()
const emit=defineEmits<{select:[merchant:TokenMarketMerchant];more:[]}>()
const sprites=[{x:55,y:805,width:60,height:60},{x:320,y:805,width:60,height:60},{x:585,y:805,width:60,height:60},{x:845,y:805,width:60,height:60},{x:1110,y:805,width:60,height:60}]
function sprite(i:number){return sprites[i]||sprites[0]}
function formatSold(value:number){if(value>=10000)return `${(value/10000).toFixed(1)}万`;return value.toLocaleString('zh-CN')}
</script>
<style scoped>
.title-row{display:flex;align-items:center;gap:10px}.crown{color:#ffd56a;font-size:16px}.merchant-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:10px}.merchant-card{display:grid;grid-template-columns:54px 1fr auto;align-items:center;gap:10px;padding:12px;border:1px solid rgba(105,136,204,.17);border-radius:13px;background:linear-gradient(180deg,rgba(13,28,61,.95),rgba(7,18,40,.96));cursor:pointer;transition:.18s}.merchant-card:hover{transform:translateY(-2px);border-color:rgba(94,126,255,.38)}.logo{width:54px;height:54px}.merchant-copy strong,.merchant-copy small{display:block}.merchant-copy strong{font-size:11px}.merchant-copy small{margin-top:5px;color:#7f93b6;font-size:8px}.merchant-card>button{padding:6px 8px;border:1px solid rgba(100,132,206,.2);border-radius:8px;color:#a9bce0;background:#0b1b3a;font-size:8px;cursor:pointer}@media(max-width:1100px){.merchant-grid{grid-template-columns:repeat(2,1fr)}}@media(max-width:600px){.merchant-grid{grid-template-columns:1fr}}
</style>
