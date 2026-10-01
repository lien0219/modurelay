<template>
  <section class="activity-card tm-glass">
    <div class="header">
      <div><h2>交易动态</h2><p>实时成交与兑换记录</p></div>
      <button class="tm-link-button" type="button" @click="emit('more')">查看更多 →</button>
    </div>
    <div class="list">
      <article v-for="activity in activities" :key="activity.id">
        <span class="dot"></span>
        <div class="copy"><strong>{{ activity.title }}</strong><small>{{ activity.detail }}</small></div>
        <div class="meta"><strong v-if="activity.tokenAmount" :class="{ income: activity.tokenAmount > 0 }">{{ activity.tokenAmount > 0 ? '+' : '' }}{{ activity.tokenAmount.toLocaleString('zh-CN') }} T</strong><small>{{ formatTime(activity.occurredAt) }}</small></div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { TokenMarketActivity } from '../types'

defineProps<{ activities: TokenMarketActivity[] }>()
const emit = defineEmits<{ more: [] }>()

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}
</script>

<style scoped>
.activity-card{height:100%;padding:22px;border-radius:20px}.header{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.header h2{margin:0;font-size:16px}.header p{margin:5px 0 0;color:#7285a5;font-size:9px}.list{margin-top:14px}.list article{display:grid;grid-template-columns:10px 1fr auto;align-items:center;gap:10px;padding:12px 0;border-top:1px solid rgba(102,132,193,.09)}.list article:first-child{border-top:0}.dot{width:6px;height:6px;border-radius:50%;background:#596eff;box-shadow:0 0 12px #596eff}.copy strong,.copy small,.meta strong,.meta small{display:block}.copy strong{font-size:10px}.copy small{margin-top:3px;color:#6e83a7;font-size:8px}.meta{text-align:right}.meta strong{color:#ff87ac;font-size:10px}.meta strong.income{color:#55e7b4}.meta small{margin-top:3px;color:#596d91;font-size:7px}
</style>
