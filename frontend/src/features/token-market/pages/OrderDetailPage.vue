<template>
  <div class="tm-order-detail">
    <div v-if="loading" class="tm-feedback" role="status">正在加载订单…</div>
    <div v-else-if="error" class="tm-empty" role="alert"><h2>订单加载失败</h2><p>{{ error }}</p><button class="tm-button" @click="load">重新加载</button></div>
    <div v-else-if="!order" class="tm-empty"><h2>无法取得订单详情</h2><p>现有接口只提供订单列表；该订单不在当前可取得的列表中，需要订单详情接口。</p><RouterLink class="tm-button" to="/token-market/orders">返回订单列表</RouterLink></div>
    <template v-else-if="afterSale">
      <div class="tm-heading"><h1>申请售后</h1><p>针对 {{ order.title }} 填写问题说明。</p></div>
      <div class="tm-order-columns"><form class="tm-panel tm-order-box" @submit.prevent="showUnavailable=true"><h2>{{ order.title }}</h2><div class="tm-tabs"><button v-for="type in ['仅退款','退货退款']" :key="type" class="tm-tab" :class="{ 'is-active': refundType === type }" type="button" @click="refundType=type">{{ type }}</button></div><label class="tm-field"><span>售后原因</span><select v-model="reason" class="tm-select" required><option value="">请选择原因</option><option>商品与描述不符</option><option>交付问题</option><option>其他问题</option></select></label><label class="tm-field"><span>申请金额（T）</span><input v-model.number="amount" class="tm-input" type="number" min="0.01" :max="order.amountToken" step="0.01" required /><small>最多可填写 {{ formatToken(order.amountToken) }}；实际可退金额由服务端规则决定。</small></label><label class="tm-field"><span>问题描述</span><textarea v-model="description" class="tm-textarea" required minlength="5" placeholder="请描述商品或服务遇到的问题" /></label><label class="tm-field"><span>凭证图片（本地预览，最多 6 张）</span><input class="tm-input" type="file" accept="image/*" multiple @change="selectFiles" /><small>{{ evidenceCount }} 张已选择；当前不会上传到服务器。</small></label><button class="tm-button primary" type="submit">检查申请</button></form><aside class="tm-panel tm-order-box"><h2>售后说明</h2><p>退款、退货、手续费和争议处理遵循平台已有业务规则。当前缺少售后提交与进度接口，表单不会创建申请。</p></aside></div>
    </template>
    <template v-else>
      <div class="tm-heading"><h1>订单详情</h1><p>订单号 {{ order.id }} <button class="tm-text-link" type="button" @click="copyId">复制</button></p></div>
      <section class="tm-panel tm-order-status"><h2>{{ order.statusText }}</h2><p>状态来自订单列表接口，履约时间、配送轨迹和交付内容需服务端详情接口提供。</p><span>{{ formatDate(order.createdAt) }}</span></section>
      <div class="tm-order-columns"><section class="tm-panel tm-order-box"><h2>商品与履约</h2><div class="tm-row"><strong>{{ order.title }}</strong><strong class="tm-price">{{ formatToken(order.amountToken) }}</strong></div><p>{{ categoryLabels[order.categoryId] }}</p><RouterLink v-if="order.productId" class="tm-button" :to="`/token-market/products/${order.productId}`">查看商品</RouterLink></section><aside class="tm-panel tm-order-box"><h2>订单信息</h2><dl><div><dt>订单编号</dt><dd>{{ order.id }}</dd></div><div><dt>创建时间</dt><dd>{{ formatDate(order.createdAt) }}</dd></div><div><dt>金额</dt><dd>{{ formatToken(order.amountToken) }}</dd></div><div><dt>状态</dt><dd>{{ order.statusText }}</dd></div></dl><div class="tm-order-actions"><RouterLink class="tm-button" :to="`/token-market/orders/${order.id}/after-sale`">申请售后</RouterLink><button v-if="order.status === 'pending_payment'" class="tm-button" type="button" @click="showCancel=true">取消订单</button></div></aside></div>
    </template>
    <div v-if="showUnavailable || showCancel" class="tm-dialog-backdrop" @click.self="closeDialog"><div class="tm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="order-dialog-title"><h2 id="order-dialog-title">{{ showCancel ? '确认取消订单' : '暂不能提交售后申请' }}</h2><p>{{ showCancel ? '订单取消与退款需要服务端接口和既有业务规则支持，目前不会改变订单状态。' : '售后申请接口尚未接入。你的输入只停留在当前页面，未提交给商家。' }}</p><div class="tm-dialog-actions"><button class="tm-button primary" type="button" @click="closeDialog">知道了</button></div></div></div>
    <div v-if="toast" class="tm-toast" role="status">{{ toast }}</div>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { categoryLabels, formatToken } from '../presentation'
import { useTokenMarketStore } from '../store'
import type { TokenMarketOrder } from '../types'
const route = useRoute()
const market = useTokenMarketStore()
const afterSale = computed(() => route.path.endsWith('/after-sale'))
const order = ref<TokenMarketOrder | null>(null)
const loading = ref(false)
const error = ref('')
const refundType = ref('仅退款')
const reason = ref('')
const amount = ref(0)
const description = ref('')
const evidenceCount = ref(0)
const showUnavailable = ref(false)
const showCancel = ref(false)
const toast = ref('')
async function load(): Promise<void> {
  loading.value = true; error.value = ''
  try {
    const items = await market.loadOrders({ pageSize: 100 })
    order.value = items.find(item => item.id === String(route.params.id)) ?? null
    amount.value = order.value?.amountToken ?? 0
  } catch (failure) { error.value = failure instanceof Error ? failure.message : '请稍后重试' }
  finally { loading.value = false }
}
function selectFiles(event: Event): void { evidenceCount.value = Math.min(6, (event.target as HTMLInputElement).files?.length ?? 0) }
function formatDate(value: string): string { return new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'short' }) }
function closeDialog(): void { showUnavailable.value = false; showCancel.value = false }
async function copyId(): Promise<void> { if (order.value) { await navigator.clipboard.writeText(order.value.id); toast.value = '订单号已复制'; window.setTimeout(() => { toast.value = '' }, 2200) } }
watch(() => route.params.id, load, { immediate: true })
</script>
<style scoped>
.tm-order-status{padding:24px;margin-bottom:24px}.tm-order-status h2{margin:0 0 8px}.tm-order-status p,.tm-order-status span,.tm-order-box p{color:var(--tm-muted)}.tm-order-columns{display:grid;grid-template-columns:minmax(0,1.6fr) minmax(290px,1fr);gap:24px}.tm-order-box{padding:24px;align-self:start}.tm-order-box h2{margin:0 0 17px}.tm-order-box>.tm-row{margin-bottom:16px}.tm-order-box dl{margin:0 0 18px}.tm-order-box dl div{display:flex;justify-content:space-between;gap:15px;padding:10px 0;border-bottom:1px solid var(--tm-border)}.tm-order-box dt{color:var(--tm-muted)}.tm-order-box dd{margin:0;text-align:right;overflow-wrap:anywhere}.tm-order-actions{display:flex;gap:10px}.tm-order-box form{display:block}
@media(max-width:850px){.tm-order-columns{grid-template-columns:1fr}}@media(max-width:700px){.tm-order-box,.tm-order-status{padding:18px}.tm-order-actions{flex-wrap:wrap}}
</style>
