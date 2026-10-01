<template>
  <section class="tm-section">
    <div class="tm-section-heading">
      <div><h2>热门商家</h2><p>高评分 · 高成交 · 快速履约</p></div>
      <button class="tm-link-button" type="button" @click="emit('more')">全部商家 →</button>
    </div>
    <div class="grid">
      <button v-for="merchant in merchants" :key="merchant.id" type="button" @click="emit('select', merchant)">
        <span class="logo">{{ merchant.name.slice(0,1) }}</span>
        <span class="copy">
          <strong>{{ merchant.name }}</strong>
          <small>★ {{ merchant.rating.toFixed(1) }} · 已售 {{ merchant.soldCount.toLocaleString('zh-CN') }}</small>
        </span>
        <span class="enter">→</span>
      </button>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { TokenMarketMerchant } from '../types'

defineProps<{ merchants: TokenMarketMerchant[] }>()
const emit = defineEmits<{
  select: [merchant: TokenMarketMerchant]
  more: []
}>()
</script>

<style scoped>
.grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:13px}.grid button{display:grid;grid-template-columns:44px 1fr auto;align-items:center;gap:11px;min-height:76px;padding:14px;border:1px solid rgba(104,135,198,.15);border-radius:14px;color:#dce8fa;background:linear-gradient(180deg,rgba(12,29,63,.7),rgba(6,16,38,.82));text-align:left;cursor:pointer;transition:.18s ease}.grid button:hover{transform:translateY(-2px);border-color:rgba(104,128,255,.36)}.logo{display:grid;place-items:center;width:44px;height:44px;border-radius:12px;color:#fff;background:linear-gradient(145deg,#8559ff,#4167ff 55%,#24bdf7);box-shadow:0 10px 25px rgba(66,73,255,.25);font-size:17px;font-weight:900}.copy strong,.copy small{display:block}.copy strong{font-size:11px}.copy small{margin-top:5px;color:#7185a7;font-size:8px}.enter{color:#6f84aa}@media(max-width:1050px){.grid{grid-template-columns:repeat(3,1fr)}}@media(max-width:650px){.grid{grid-template-columns:1fr 1fr}}@media(max-width:450px){.grid{grid-template-columns:1fr}}
</style>
