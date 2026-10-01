<template>
  <div class="skeleton" :class="`variant-${variant}`" aria-hidden="true">
    <div class="block hero"></div>
    <div class="grid">
      <div v-for="index in count" :key="index" class="block card">
        <span></span><span></span><span></span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { TokenMarketSkeletonVariant } from '../pageState'

withDefaults(defineProps<{ variant?: TokenMarketSkeletonVariant; count?: number }>(), {
  variant: 'list',
  count: 4,
})
</script>

<style scoped>
.skeleton{display:grid;gap:16px}.block{position:relative;overflow:hidden;border:1px solid rgba(113,142,205,.12);border-radius:16px;background:rgba(8,19,42,.78)}.block::after{content:"";position:absolute;inset:0;transform:translateX(-100%);background:linear-gradient(90deg,transparent,rgba(255,255,255,.045),transparent);animation:shimmer 1.35s infinite}.hero{height:150px}.grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px}.card{min-height:118px;padding:18px}.card span{display:block;height:10px;margin-bottom:12px;border-radius:999px;background:rgba(133,158,211,.10)}.card span:nth-child(1){width:52%}.card span:nth-child(2){width:78%}.card span:nth-child(3){width:34%}.variant-detail .hero{height:300px}.variant-detail .grid{grid-template-columns:1fr 1fr}.variant-dashboard .grid{grid-template-columns:repeat(3,minmax(0,1fr))}.variant-wallet .hero{height:190px;background:linear-gradient(135deg,rgba(82,88,255,.12),rgba(38,185,255,.05))}@keyframes shimmer{100%{transform:translateX(100%)}}@media(max-width:850px){.grid,.variant-detail .grid,.variant-dashboard .grid{grid-template-columns:1fr}}
</style>
