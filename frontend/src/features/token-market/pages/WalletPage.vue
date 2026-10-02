<template>
  <div class="tm-wallet" :class="{ 'is-history': historyOnly }">
    <div class="tm-heading"><h1>{{ historyOnly ? '兑换记录' : 'Token 钱包' }}</h1><p>余额和兑换记录来自当前数据源{{ isTokenMarketDemo ? '（演示）' : '' }}。</p></div>
    <div v-if="loading" class="tm-feedback" role="status">正在加载钱包…</div>
    <div v-else-if="error" class="tm-empty" role="alert"><h2>钱包加载失败</h2><p>{{ error }}</p><button class="tm-button" type="button" @click="load">重新加载</button></div>
    <template v-else>
      <div v-if="!historyOnly" class="tm-wallet-metrics">
        <div class="tm-panel tm-wallet-primary">
          <span>可用 Token</span><strong>{{ wallet ? formatToken(wallet.tokenBalance) : '—' }}</strong>
          <div class="tm-wallet-inline-actions"><RouterLink class="tm-button primary" to="/token-market/recharge">充值</RouterLink><RouterLink class="tm-button" to="/token-market/exchange">兑换</RouterLink></div>
          <small>今日消费 {{ wallet ? formatToken(wallet.todaySpentToken) : '—' }}</small>
          <small class="tm-wallet-extra">待结算 {{ wallet ? formatToken(wallet.pendingToken) : '—' }} · 冻结 {{ wallet ? formatToken(wallet.frozenToken) : '—' }}</small>
        </div>
        <div class="tm-panel tm-wallet-secondary"><span>待结算</span><strong>{{ wallet ? formatToken(wallet.pendingToken) : '—' }}</strong><small>具体可用时间由服务端确定</small></div>
        <div class="tm-panel tm-wallet-secondary"><span>冻结 Token</span><strong>{{ wallet ? formatToken(wallet.frozenToken) : '—' }}</strong><small>按服务端账本状态展示</small></div>
      </div>
      <div v-if="!historyOnly" class="tm-wallet-actions"><RouterLink class="tm-button primary tm-wallet-desktop-action" to="/token-market/recharge">充值 Token</RouterLink><RouterLink class="tm-button tm-wallet-desktop-action" to="/token-market/exchange">余额兑换</RouterLink><RouterLink class="tm-button" to="/token-market/exchange-history">查看兑换记录</RouterLink></div>
      <div v-if="!historyOnly" class="tm-wallet-recent-heading"><h2>最近兑换</h2><span>仅展示可核实的兑换记录</span></div>
      <div class="tm-tabs" role="tablist" aria-label="兑换方向"><button v-for="tab in tabs" :key="tab.value" class="tm-tab" :class="{ 'is-active': direction === tab.value }" role="tab" type="button" :aria-selected="direction === tab.value" @click="setDirection(tab.value)">{{ tab.label }}</button></div>
      <template v-if="market.exchangeHistory.length">
        <div class="tm-panel tm-table-scroll tm-wallet-table"><table class="tm-data-table"><thead><tr><th>兑换记录</th><th>支出</th><th>到账</th><th>时间</th><th>状态</th></tr></thead><tbody><tr v-for="record in market.exchangeHistory" :key="record.id"><td>{{ record.id }}<small>{{ record.direction === 'balance_to_token' ? '平台余额 → Token' : 'Token → 平台余额' }}</small></td><td>{{ formatUnit(record.sourceAmount,record.sourceUnit) }}</td><td>{{ formatUnit(record.destinationAmount,record.destinationUnit) }}</td><td>{{ new Date(record.createdAt).toLocaleString('zh-CN') }}</td><td>{{ record.status === 'success' ? '已完成' : record.status === 'failed' ? '失败' : '处理中' }}</td></tr></tbody></table></div>
        <div class="tm-wallet-mobile-records"><article v-for="record in market.exchangeHistory" :key="record.id" class="tm-panel"><div class="tm-row"><strong>{{ record.direction === 'balance_to_token' ? '余额换 Token' : 'Token 换余额' }}</strong><strong class="tm-price">{{ record.direction === 'balance_to_token' ? '+' : '−' }}{{ formatToken(record.direction === 'balance_to_token' ? record.destinationAmount : record.sourceAmount) }}</strong></div><small>{{ new Date(record.createdAt).toLocaleString('zh-CN') }} · {{ record.status === 'success' ? '已完成' : record.status === 'failed' ? '失败' : '处理中' }}</small><p>{{ formatUnit(record.sourceAmount,record.sourceUnit) }} → {{ formatUnit(record.destinationAmount,record.destinationUnit) }}</p></article></div>
      </template>
      <div v-else class="tm-empty"><img src="/token-market/icons/wallet.svg" alt="" /><h2>暂无兑换记录</h2><p>完成兑换后，服务端记录会显示在这里。</p></div>
      <p v-if="!historyOnly" class="tm-wallet-note">完整钱包流水、充值与退款明细需要独立账本接口，目前仅展示现有兑换记录。</p>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { formatToken } from '../presentation'
import { isTokenMarketDemo } from '../service'
import { useTokenMarketStore } from '../store'
import type { TokenExchangeDirection, TokenMarketMoneyUnit } from '../types'
const route = useRoute()
const router = useRouter()
const market = useTokenMarketStore()
const wallet = computed(() => market.wallet)
const historyOnly = computed(() => route.path.endsWith('/exchange-history'))
const tabs: Array<{ label: string; value: TokenExchangeDirection | 'all' }> = [{ label: '全部记录', value: 'all' }, { label: '余额换 Token', value: 'balance_to_token' }, { label: 'Token 换余额', value: 'token_to_balance' }]
const direction = computed<TokenExchangeDirection | 'all'>(() => tabs.some(tab => tab.value === route.query.direction) ? route.query.direction as TokenExchangeDirection : 'all')
const loading = ref(false)
const error = ref('')
async function load(): Promise<void> {
  loading.value = true; error.value = ''
  try { await Promise.all([market.loadWallet(), market.loadExchangeHistory({ direction: direction.value === 'all' ? undefined : direction.value })]) }
  catch (failure) { error.value = failure instanceof Error ? failure.message : '请稍后重试' }
  finally { loading.value = false }
}
function formatUnit(amount: number, unit: TokenMarketMoneyUnit): string { return unit === 'token' ? formatToken(amount) : `¥ ${amount.toLocaleString('zh-CN', { maximumFractionDigits: 2 })}` }
function setDirection(value: TokenExchangeDirection | 'all'): void { void router.push({ path: route.path, query: { ...route.query, direction: value === 'all' ? undefined : value } }) }
watch(() => route.fullPath, load, { immediate: true })
</script>
<style scoped>
.tm-wallet-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}.tm-wallet-metrics>div{min-height:130px;padding:24px}.tm-wallet-metrics span,.tm-wallet-metrics small{display:block;color:var(--tm-muted)}.tm-wallet-metrics span{font-size:12px}.tm-wallet-metrics strong{display:block;margin:14px 0;font-size:34px;line-height:1.2}.tm-wallet-metrics small{font-size:12px;color:var(--tm-success)}.tm-wallet-actions{display:flex;gap:12px;margin:24px 0}.tm-wallet-actions .tm-button{min-width:178px}.tm-wallet td small{display:block;color:var(--tm-muted)}.tm-wallet-note{margin-top:18px;color:var(--tm-muted);font-size:12px}
@media(max-width:850px){.tm-wallet-metrics{grid-template-columns:1fr 1fr}.tm-wallet-metrics>div:first-child{grid-column:1/-1}}@media(max-width:700px){.tm-wallet-metrics{grid-template-columns:1fr 1fr}.tm-wallet-metrics>div{min-height:112px;padding:16px}.tm-wallet-metrics>div:first-child{grid-column:1/-1}.tm-wallet-metrics strong{font-size:25px}.tm-wallet-actions{flex-wrap:wrap}.tm-wallet-actions .tm-button{flex:1;min-width:130px}.tm-wallet .tm-tabs{flex-wrap:nowrap;overflow-x:auto}.tm-wallet .tm-tab{white-space:nowrap}}
.tm-wallet-inline-actions,.tm-wallet-extra,.tm-wallet-mobile-records,.tm-wallet-recent-heading{display:none}
@media(max-width:700px){
  .tm-wallet-metrics{display:block}
  .tm-wallet-metrics .tm-wallet-secondary{display:none}
  .tm-wallet-metrics .tm-wallet-primary{min-height:0;padding:23px}
  .tm-wallet-primary strong{font-size:34px}
  .tm-wallet-inline-actions{display:flex;gap:10px;margin:22px 0 18px}
  .tm-wallet-inline-actions .tm-button{flex:1}
  .tm-wallet-extra{display:block;margin-top:6px;color:var(--tm-muted)!important}
  .tm-wallet-actions{margin:12px 0 26px}
  .tm-wallet-actions .tm-wallet-desktop-action{display:none}
  .tm-wallet-actions .tm-button{width:100%}
  .tm-wallet-recent-heading{display:flex;align-items:baseline;justify-content:space-between;gap:12px;margin-bottom:12px}
  .tm-wallet-recent-heading h2{margin:0;font-size:22px}
  .tm-wallet-recent-heading span{color:var(--tm-muted);font-size:12px;text-align:right}
  .tm-wallet:not(.is-history)>.tm-tabs{display:none}
  .tm-wallet-table{display:none}
  .tm-wallet-mobile-records{display:grid;gap:12px}
  .tm-wallet-mobile-records article{padding:17px}
  .tm-wallet-mobile-records .tm-row{align-items:flex-start}
  .tm-wallet-mobile-records .tm-price{font-size:18px}
  .tm-wallet-mobile-records small{display:block;margin-top:10px;color:var(--tm-muted)}
  .tm-wallet-mobile-records p{margin:8px 0 0;color:var(--tm-muted);font-size:12px}
}
</style>
