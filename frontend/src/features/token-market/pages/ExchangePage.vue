<template>
  <div class="tm-exchange">
    <div class="tm-heading"><h1>{{ recharge ? '充值中心' : '余额兑换' }}</h1><p>{{ recharge ? '先通过已有平台充值入口增加账户余额，再获取服务端兑换报价。' : '平台余额与 Token 双向兑换，到账金额以服务端报价为准。' }}</p></div>
    <div class="tm-exchange-grid">
      <section class="tm-panel tm-exchange-form" v-if="recharge"><h2>选择充值金额</h2><div class="tm-exchange-presets"><button v-for="preset in [50,100,200,500,1000]" :key="preset" class="tm-button" :class="{ primary: amount === preset }" type="button" @click="amount=preset">¥ {{ preset }}</button></div><label class="tm-field"><span>自定义金额（展示草稿）</span><input v-model.number="amount" class="tm-input" type="number" min="1" inputmode="decimal" /></label><p>此处金额仅为草稿。平台充值页会显示实际可用的支付方式、金额和结果。</p><RouterLink class="tm-button primary tm-exchange-full" to="/recharge">前往平台账户充值</RouterLink><RouterLink class="tm-text-link" to="/token-market/exchange">已有余额？获取兑换报价 →</RouterLink></section>
      <section class="tm-panel tm-exchange-form" v-else><h2>选择兑换方向</h2><div class="tm-tabs"><button class="tm-tab" :class="{ 'is-active': direction === 'balance_to_token' }" type="button" @click="setDirection('balance_to_token')">平台余额 → Token</button><button class="tm-tab" :class="{ 'is-active': direction === 'token_to_balance' }" type="button" @click="setDirection('token_to_balance')">Token → 平台余额</button></div><div class="tm-exchange-rate"><img src="/token-market/icons/shield.svg" alt="" /><span><strong>服务端兑换报价</strong><small>{{ market.bootstrap ? `参考比率：${market.exchangeRate} Token / 平台余额单位${isTokenMarketDemo ? '（演示）' : ''}` : '等待服务端配置' }}</small></span></div><label class="tm-field"><span>兑换数量（{{ direction === 'balance_to_token' ? '平台余额' : 'Token' }}）</span><input v-model.number="amount" class="tm-input" type="number" min="0.01" step="any" inputmode="decimal" :disabled="quoting || market.exchanging" /><small>当前可用：{{ sourceBalance }}</small></label><div class="tm-swap-icon"><img src="/token-market/icons/swap.svg" alt="" /></div><label class="tm-field"><span>预计到账</span><output class="tm-input tm-output">{{ quote ? destinationText : '获取报价后显示' }}</output></label><div v-if="quote" class="tm-quote"><dl><div><dt>报价依据</dt><dd>{{ quote.basis || '服务端未提供' }}</dd></div><div><dt>适用模型</dt><dd>{{ quote.applicableModels?.join('、') || '通用钱包兑换' }}</dd></div><div><dt>适用类型</dt><dd>{{ quote.tokenTypes?.join(' / ') || '通用 Token' }}</dd></div><div><dt>更新时间</dt><dd>{{ formatDate(quote.updatedAt || quote.quotedAt) }}</dd></div><div><dt>有效期</dt><dd>{{ quote.expiresAt ? formatDate(quote.expiresAt) : '服务端未提供，暂不能执行' }}</dd></div></dl></div><p v-if="error" class="tm-exchange-error" role="alert">{{ error }}</p><button class="tm-button primary tm-exchange-full" type="button" :disabled="quoting || !amount || amount <= 0" @click="quoteNow">{{ quoting ? '正在获取报价…' : '获取兑换报价' }}</button><button v-if="quote" class="tm-button tm-exchange-full" type="button" :disabled="market.exchanging || !canExecute" @click="confirmOpen=true">确认兑换</button><small class="tm-muted">{{ isTokenMarketDemo ? '演示模式不会修改余额，也不会产生交易。' : '无有效期或已过期的报价不能执行。' }}</small></section>
      <aside class="tm-panel tm-exchange-aside"><h2>账户与报价</h2><div class="tm-row"><span>平台余额</span><strong>{{ market.wallet ? `¥ ${market.wallet.platformBalance.toLocaleString('zh-CN')}` : '—' }}</strong></div><div class="tm-row"><span>Token 余额</span><strong>{{ market.wallet ? formatToken(market.wallet.tokenBalance) : '—' }}</strong></div><hr /><p>兑换的服务端规则、手续费、限额、报价有效期和实际到账数量应由服务端返回。页面示例数值不构成固定规则。</p><RouterLink class="tm-text-link" to="/token-market/exchange-history">查看兑换记录 →</RouterLink></aside>
    </div>
    <div v-if="confirmOpen" class="tm-dialog-backdrop" @click.self="confirmOpen=false"><div class="tm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="exchange-confirm-title"><h2 id="exchange-confirm-title">确认兑换</h2><p>支出 {{ sourceText }}，服务端报价到账 {{ destinationText }}。</p><p v-if="isTokenMarketDemo">演示模式不会提交兑换。</p><div class="tm-dialog-actions"><button class="tm-button" type="button" @click="confirmOpen=false">返回</button><button class="tm-button primary" type="button" :disabled="market.exchanging || isTokenMarketDemo" @click="execute">{{ market.exchanging ? '处理中…' : '确认兑换' }}</button></div></div></div>
    <div v-if="success" class="tm-dialog-backdrop"><div class="tm-dialog" role="status" aria-live="polite"><img src="/token-market/icons/check.svg" alt="" /><h2>兑换已提交</h2><p>请以服务端钱包余额与兑换记录为准。</p><div class="tm-dialog-actions"><RouterLink class="tm-button primary" to="/token-market/exchange-history">查看记录</RouterLink><button class="tm-button" @click="success=false">关闭</button></div></div></div>
  </div>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { formatToken } from '../presentation'
import { isTokenMarketDemo } from '../service'
import { useTokenMarketStore } from '../store'
import type { TokenExchangeDirection, TokenExchangeQuote } from '../types'
const route = useRoute()
const market = useTokenMarketStore()
const recharge = computed(() => route.path.endsWith('/recharge'))
const direction = ref<TokenExchangeDirection>('balance_to_token')
const amount = ref(100)
const quote = ref<TokenExchangeQuote | null>(null)
const quoting = ref(false)
const error = ref('')
const confirmOpen = ref(false)
const success = ref(false)
let quoteRequestId = 0
let expiryTimer: number | undefined
const canExecute = computed(() => !isTokenMarketDemo && !!quote.value?.expiresAt && Date.parse(quote.value.expiresAt) > Date.now())
const sourceBalance = computed(() => direction.value === 'balance_to_token' ? `¥ ${market.wallet?.platformBalance.toLocaleString('zh-CN') ?? '—'}` : market.wallet ? formatToken(market.wallet.tokenBalance) : '—')
const sourceText = computed(() => direction.value === 'balance_to_token' ? `¥ ${Number(amount.value).toLocaleString('zh-CN')}` : formatToken(Number(amount.value)))
const destinationText = computed(() => !quote.value ? '' : direction.value === 'balance_to_token' ? formatToken(quote.value.destinationAmount) : `¥ ${quote.value.destinationAmount.toLocaleString('zh-CN')}`)
function clearQuote(): void {
  quoteRequestId++
  window.clearTimeout(expiryTimer)
  quote.value = null
  quoting.value = false
  market.clearQuote()
}
function setDirection(next: TokenExchangeDirection): void { direction.value = next; amount.value = next === 'balance_to_token' ? 100 : 10000; clearQuote(); error.value = ''; confirmOpen.value = false }
async function quoteNow(): Promise<void> {
  const requestId = ++quoteRequestId
  window.clearTimeout(expiryTimer)
  quoting.value = true; error.value = ''; quote.value = null
  try {
    const result = await market.quoteExchange(direction.value, Number(amount.value))
    if (requestId !== quoteRequestId) return
    quote.value = result
    if (result.expiresAt) {
      const remaining = Date.parse(result.expiresAt) - Date.now()
      if (remaining <= 0) { clearQuote(); error.value = '报价已过期，请重新获取'; return }
      expiryTimer = window.setTimeout(() => { if (quote.value?.quoteId === result.quoteId) { clearQuote(); error.value = '报价已过期，请重新获取' } }, Math.min(remaining + 100, 2_147_483_647))
    }
  } catch (failure) { if (requestId === quoteRequestId) error.value = failure instanceof Error ? failure.message : '报价失败，请稍后重试' }
  finally { if (requestId === quoteRequestId) quoting.value = false }
}
async function execute(): Promise<void> {
  if (!canExecute.value) return
  error.value = ''
  try { await market.executeExchange(); confirmOpen.value = false; clearQuote(); success.value = true }
  catch (failure) { confirmOpen.value = false; error.value = failure instanceof Error ? failure.message : '兑换失败，请核对钱包记录' }
}
function formatDate(value: string): string { return new Date(value).toLocaleString('zh-CN', { dateStyle: 'medium', timeStyle: 'medium' }) }
watch(amount, clearQuote)
onBeforeUnmount(clearQuote)
</script>
<style scoped>
.tm-exchange-grid{display:grid;grid-template-columns:minmax(0,1.8fr) minmax(280px,1fr);gap:24px}.tm-exchange-form,.tm-exchange-aside{align-self:start;padding:24px}.tm-exchange-form h2,.tm-exchange-aside h2{margin:0 0 17px}.tm-exchange-form>.tm-tabs{margin:0 0 16px}.tm-exchange-form .tm-tab{min-width:175px}.tm-exchange-rate{display:flex;align-items:center;gap:12px;padding:15px;margin-bottom:20px;border-radius:10px;background:var(--tm-raised)}.tm-exchange-rate img{width:23px}.tm-exchange-rate small{display:block;color:var(--tm-muted)}.tm-swap-icon img{width:27px}.tm-output{display:flex;align-items:center;color:var(--tm-muted)}.tm-quote{margin-bottom:17px;padding:14px;border:1px solid var(--tm-border);border-radius:10px}.tm-quote dl{margin:0}.tm-quote dl div{display:flex;justify-content:space-between;gap:16px;padding:5px 0}.tm-quote dt{color:var(--tm-muted)}.tm-quote dd{margin:0;text-align:right;overflow-wrap:anywhere}.tm-exchange-full{width:100%;margin-bottom:10px}.tm-exchange-error{color:#ff8997}.tm-exchange-aside .tm-row{padding:13px 0}.tm-exchange-aside .tm-row span,.tm-exchange-aside p{color:var(--tm-muted)}.tm-exchange-aside hr{border:0;border-top:1px solid var(--tm-border);margin:18px 0}.tm-exchange-presets{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-bottom:20px}.tm-exchange-form>p{color:var(--tm-muted)}.tm-exchange-form>a.tm-text-link{display:block;text-align:center;margin:16px}
@media(max-width:900px){.tm-exchange-grid{grid-template-columns:1fr}}@media(max-width:700px){.tm-exchange-form,.tm-exchange-aside{padding:17px}.tm-exchange-form .tm-tabs{flex-wrap:nowrap}.tm-exchange-form .tm-tab{min-width:0;padding:0 8px;font-size:13px}.tm-exchange-presets{grid-template-columns:repeat(2,1fr)}}
</style>
