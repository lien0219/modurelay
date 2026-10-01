<template>
  <section class="category-row">
    <button v-for="(item,index) in categories" :key="item.id" class="category-card" type="button" @click="emit('select',item.label)">
      <span class="icon-wrap" :class="`tone-${index}`"><MarketIcon :name="icon(item.id)" /></span>
      <span class="copy"><strong>{{ item.label }}</strong><small>{{ item.description }}</small></span>
      <MarketIcon class="arrow" name="arrow" />
    </button>
  </section>
</template>
<script setup lang="ts">
import MarketIcon from './MarketIcon.vue'
import type { TokenMarketCategory,TokenMarketCategoryId } from '../types'
defineProps<{categories:TokenMarketCategory[]}>()
const emit=defineEmits<{select:[label:string]}>()
function icon(id:TokenMarketCategoryId){return ({mall:'bag',delivery:'cup',digital:'game',ai_credit:'ai',services:'store'} as const)[id]}
</script>
<style scoped>
.category-row{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:12px;margin-top:14px}.category-card{display:grid;grid-template-columns:56px 1fr 20px;align-items:center;gap:12px;min-height:84px;padding:12px 14px;border:1px solid rgba(104,134,200,.18);border-radius:14px;color:#edf3ff;background:linear-gradient(120deg,rgba(16,31,65,.95),rgba(8,18,40,.96));text-align:left;cursor:pointer;transition:.18s}.category-card:hover{transform:translateY(-2px);border-color:rgba(100,131,255,.42);box-shadow:0 12px 30px rgba(28,47,104,.28)}.icon-wrap{display:grid;place-items:center;width:56px;height:56px;border-radius:14px;font-size:30px;box-shadow:inset 0 1px rgba(255,255,255,.08),0 8px 24px rgba(0,0,0,.2)}.tone-0{color:#ff7d93;background:linear-gradient(135deg,#7d274f,#ef6b85)}.tone-1{color:#ffd18e;background:linear-gradient(135deg,#704b1f,#d48945)}.tone-2{color:#63b5ff;background:linear-gradient(135deg,#183d87,#3556d6)}.tone-3{color:#6fd7ff;background:linear-gradient(135deg,#2b33ad,#5c51ff)}.tone-4{color:#d79cff;background:linear-gradient(135deg,#4f278f,#8b45d1)}.copy strong,.copy small{display:block}.copy strong{font-size:13px}.copy small{margin-top:5px;color:#8699b8;font-size:8px}.arrow{justify-self:end;color:#7690bd;font-size:16px}@media(max-width:1000px){.category-row{grid-template-columns:repeat(2,1fr)}}@media(max-width:560px){.category-row{grid-template-columns:1fr}}
</style>
