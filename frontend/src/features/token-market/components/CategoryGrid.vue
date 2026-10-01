<template>
  <section class="tm-section">
    <div class="tm-section-heading">
      <div><h2>探索分类</h2><p>选择你感兴趣的服务类型</p></div>
    </div>
    <div class="grid">
      <button v-for="category in categories" :key="category.id" type="button" @click="emit('select', category.label)">
        <span class="icon">{{ iconMap[category.id] }}</span>
        <span class="copy"><strong>{{ category.label }}</strong><small>{{ category.description }}</small></span>
        <span class="arrow">→</span>
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { TokenMarketCategory, TokenMarketCategoryId } from '../types'

defineProps<{ categories: TokenMarketCategory[] }>()
const emit = defineEmits<{ select: [label: string] }>()

const iconMap: Record<TokenMarketCategoryId, string> = {
  mall: '◫',
  delivery: '♨',
  digital: '⬢',
  ai_credit: '◇',
  services: '♙',
}
</script>

<style scoped>
.grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:14px}.grid button{display:grid;grid-template-columns:46px 1fr auto;align-items:center;gap:12px;min-height:86px;padding:16px;border:1px solid rgba(101,133,200,.16);border-radius:16px;color:#dfe9fa;background:linear-gradient(180deg,rgba(14,30,66,.72),rgba(7,17,40,.76));text-align:left;cursor:pointer;transition:.18s ease}.grid button:hover{transform:translateY(-2px);border-color:rgba(103,128,255,.38);box-shadow:0 14px 35px rgba(0,0,0,.24)}.icon{display:grid;place-items:center;width:46px;height:46px;border-radius:13px;color:#93a7ff;background:radial-gradient(circle at 30% 20%,rgba(134,114,255,.45),rgba(55,50,134,.48));font-size:21px}.copy strong,.copy small{display:block}.copy strong{font-size:13px}.copy small{margin-top:4px;color:#7285a5;font-size:9px}.arrow{color:#7186aa}@media(max-width:1100px){.grid{grid-template-columns:repeat(3,1fr)}}@media(max-width:700px){.grid{grid-template-columns:1fr 1fr}}@media(max-width:480px){.grid{grid-template-columns:1fr}}
</style>
