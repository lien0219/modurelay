<template>
  <div class="tm-orders">
    <div class="tm-heading"><h1>我的订单</h1><p>查看从服务端取得的订单与状态。</p></div>
    <div class="tm-tabs" role="tablist" aria-label="订单状态"><button v-for="tab in tabs" :key="tab.value" class="tm-tab" :class="{ 'is-active': status === tab.value }" type="button" role="tab" :aria-selected="status === tab.value" @click="setStatus(tab.value)">{{ tab.label }}</button></div>
    <form class="tm-orders-filters" @submit.prevent="searchOrders"><label><span>搜索订单</span><input v-model="searchInput" class="tm-input" placeholder="订单号或商品名称" /></label><button class="tm-button" type="submit">搜索</button></form>
    <div v-if="loading" class="tm-feedback" role="status">正在加载订单…</div>
    <div v-else-if="error" class="tm-empty" role="alert"><h2>订单加载失败</h2><p>{{ error }}</p><button class="tm-button" type="button" @click="load">重新加载</button></div>
    <div v-else-if="!market.orderItems.length" class="tm-empty"><img src="/token-market/icons/orders.svg" alt="" /><h2>暂无订单</h2><p>还没有符合条件的订单。</p><RouterLink class="tm-button primary" to="/token-market">去逛逛</RouterLink></div>
    <template v-else><div class="tm-panel tm-table-scroll"><table class="tm-data-table"><thead><tr><th>商品 / 订单号</th><th>金额</th><th>状态</th><th>下单时间</th><th>操作</th></tr></thead><tbody><tr v-for="order in market.orderItems" :key="order.id"><td><strong>{{ order.title }}</strong><small>{{ order.id }}</small></td><td>{{ formatToken(order.amountToken) }}</td><td>{{ order.statusText }}</td><td>{{ formatDate(order.createdAt) }}</td><td><RouterLink class="tm-text-link" :to="`/token-market/orders/${order.id}`">查看详情</RouterLink></td></tr></tbody></table></div><div class="tm-orders-pagination"><span>共 {{ market.orderTotal }} 笔订单</span><div><button class="tm-button" type="button" :disabled="page <= 1" @click="changePage(page-1)">上一页</button><span>{{ page }}</span><button class="tm-button" type="button" :disabled="!market.orderHasMore" @click="changePage(page+1)">下一页</button></div></div></template>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { formatToken } from '../presentation'
import { useTokenMarketStore } from '../store'
import type { TokenMarketOrderStatus } from '../types'
const route = useRoute()
const router = useRouter()
const market = useTokenMarketStore()
const tabs: Array<{ label: string; value: TokenMarketOrderStatus | 'all' }> = [
  { label: '全部订单', value: 'all' }, { label: '待付款', value: 'pending_payment' }, { label: '履约中', value: 'fulfilling' }, { label: '已完成', value: 'completed' }, { label: '退款 / 售后', value: 'after_sale' }, { label: '已取消', value: 'cancelled' },
]
const status = computed(() => tabs.some(item => item.value === route.query.status) ? String(route.query.status) as TokenMarketOrderStatus : 'all')
const page = computed(() => Math.max(1, Number(route.query.page) || 1))
const searchInput = ref(String(route.query.q ?? ''))
const loading = ref(false)
const error = ref('')
async function load(): Promise<void> {
  loading.value = true; error.value = ''
  try { await market.loadOrders({ status: status.value === 'all' ? undefined : status.value, query: String(route.query.q || ''), page: page.value, pageSize: 10 }) }
  catch (failure) { error.value = failure instanceof Error ? failure.message : '请稍后重试' }
  finally { loading.value = false }
}
function setStatus(value: string): void { void router.push({ path: route.path, query: { ...route.query, status: value === 'all' ? undefined : value, page: undefined } }) }
function searchOrders(): void { void router.push({ path: route.path, query: { ...route.query, q: searchInput.value.trim() || undefined, page: undefined } }) }
function changePage(next: number): void { void router.push({ path: route.path, query: { ...route.query, page: String(next) } }) }
function formatDate(value: string): string { return new Date(value).toLocaleString('zh-CN', { dateStyle: 'short', timeStyle: 'short' }) }
watch(() => route.fullPath, load, { immediate: true })
</script>
<style scoped>
.tm-orders .tm-tabs{overflow-x:auto;flex-wrap:nowrap}.tm-orders .tm-tab{white-space:nowrap;min-width:116px}.tm-orders-filters{display:flex;align-items:end;gap:14px;margin-bottom:20px}.tm-orders-filters label{display:grid;gap:6px;width:min(520px,100%)}.tm-orders-filters label span{color:var(--tm-muted);font-size:12px}.tm-orders td small{display:block;color:var(--tm-muted);font-size:12px}.tm-orders-pagination{display:flex;align-items:center;justify-content:space-between;gap:12px;margin-top:22px;color:var(--tm-muted)}.tm-orders-pagination>div{display:flex;align-items:center;gap:16px}
@media(max-width:700px){.tm-orders-filters{align-items:end}.tm-orders-filters .tm-button{padding:0 13px}.tm-orders-pagination{display:block}.tm-orders-pagination>div{justify-content:space-between;margin-top:15px}}
</style>
