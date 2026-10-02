<template>
  <div v-if="isResult" class="tm-checkout-result tm-panel"><img src="/token-market/icons/info.svg" alt="" /><h1>支付结果待核验</h1><p>没有收到服务端订单与支付结果。本页面不会根据本地操作显示支付成功。</p><RouterLink class="tm-button primary" to="/token-market/orders">查看我的订单</RouterLink><RouterLink class="tm-button" to="/token-market">继续逛逛</RouterLink></div>
  <div v-else class="tm-checkout"><div class="tm-heading"><h1>确认订单</h1><p>确认商品与收货信息，支付前需由服务端返回最终应付金额。</p></div>
    <div v-if="!items.length" class="tm-empty"><h2>没有待结算商品</h2><p>请先在购物车选择商品。</p><RouterLink class="tm-button primary" to="/token-market/cart">返回购物车</RouterLink></div>
    <div v-else class="tm-checkout-grid">
      <div class="tm-checkout-main">
        <section v-if="needsAddress" class="tm-panel tm-checkout-panel"><div class="tm-row"><h2>收货地址</h2><RouterLink class="tm-text-link" to="/token-market/addresses">管理地址</RouterLink></div><div v-if="draft.data.addresses.length" class="tm-address-options"><label v-for="address in draft.data.addresses" :key="address.id"><input v-model="addressId" type="radio" :value="address.id" name="checkout-address" /><span><strong>{{ address.label }} · {{ address.name }}</strong><small>{{ address.phone }} · {{ address.detail }}</small></span></label></div><p v-else class="tm-muted">尚未填写收货地址。</p><RouterLink v-if="!draft.data.addresses.length" class="tm-button" to="/token-market/addresses">新增地址</RouterLink></section>
        <section class="tm-panel tm-checkout-panel"><h2>商品信息</h2><div v-for="line in items" :key="line.productId" class="tm-checkout-item"><span>{{ productName(line.productId) }} × {{ line.quantity }}</span><strong class="tm-price">{{ formatToken((productPrice(line.productId) || 0) * line.quantity) }}</strong></div></section>
        <label class="tm-field"><span>订单备注</span><textarea v-model="note" class="tm-textarea" maxlength="300" placeholder="选填，告诉商家你的其他需求" /></label>
        <section class="tm-panel tm-checkout-panel"><h2>支付方式</h2><p>Token 钱包 · {{ market.wallet ? `可用 ${formatToken(market.wallet.tokenBalance)}` : '余额待加载' }}{{ isTokenMarketDemo ? '（演示余额）' : '' }}</p></section>
      </div>
      <aside class="tm-panel tm-checkout-summary"><h2>订单结算</h2><div class="tm-row"><span>商品小计</span><strong>{{ formatToken(subtotal) }}</strong></div><div class="tm-row"><span>运费、优惠及手续费</span><span>待服务端确认</span></div><hr /><span class="tm-muted">预估商品小计</span><strong class="tm-price">{{ formatToken(subtotal) }}</strong><button class="tm-button primary" type="button" @click="showUnavailable=true">核对支付条件</button><small>当前缺少服务端结算报价、订单创建与支付接口，不会扣款。</small></aside>
    </div>
    <div v-if="showUnavailable" class="tm-dialog-backdrop" @click.self="showUnavailable=false"><div class="tm-dialog" role="alertdialog" aria-modal="true" aria-labelledby="checkout-dialog-title"><h2 id="checkout-dialog-title">暂不能提交订单</h2><p>服务端结算报价、订单创建和 Token 扣款接口尚未接入。当前商品小计不是实际应付金额，购物车草稿会保留。</p><div class="tm-dialog-actions"><button class="tm-button primary" type="button" @click="showUnavailable=false">知道了</button></div></div></div>
  </div>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useMarketDraftStore } from '../experience'
import { formatToken } from '../presentation'
import { isTokenMarketDemo } from '../service'
import { useTokenMarketStore } from '../store'
const route = useRoute()
const draft = useMarketDraftStore()
const market = useTokenMarketStore()
const isResult = computed(() => route.path.endsWith('/payment-result'))
const items = computed(() => {
  const buyId = typeof route.query.buy === 'string' ? route.query.buy : ''
  const merchantId = typeof route.query.merchant === 'string' ? route.query.merchant : ''
  const lines = buyId ? draft.data.cart.filter(line => line.productId === buyId) : merchantId ? draft.selectedCart.filter(line => market.products.some(item => item.id === line.productId && item.merchantId === merchantId)) : draft.selectedCart
  return lines.filter(line => market.products.some(item => item.id === line.productId))
})
const needsAddress = computed(() => items.value.some(line => ['mall', 'delivery'].includes(market.products.find(item => item.id === line.productId)?.categoryId || '')))
const subtotal = computed(() => items.value.reduce((sum, line) => sum + (productPrice(line.productId) || 0) * line.quantity, 0))
const addressId = ref(draft.data.addresses.find(item => item.isDefault)?.id || draft.data.addresses[0]?.id || '')
const note = ref('')
const showUnavailable = ref(false)
function productName(id: string): string { return market.products.find(item => item.id === id)?.name || '商品已下架' }
function productPrice(id: string): number | null { return market.products.find(item => item.id === id)?.priceToken ?? null }
</script>
<style scoped>
.tm-checkout-grid{display:grid;grid-template-columns:minmax(0,1fr) 336px;gap:24px}.tm-checkout-main{display:grid;align-content:start;gap:20px}.tm-checkout-panel,.tm-checkout-summary{padding:24px}.tm-checkout-panel h2,.tm-checkout-summary h2{margin:0 0 17px}.tm-checkout-panel p{margin:0;color:var(--tm-muted)}.tm-address-options{display:grid;gap:10px}.tm-address-options label{display:flex;align-items:center;gap:12px;padding:14px;border:1px solid var(--tm-border);border-radius:10px}.tm-address-options input{accent-color:var(--tm-blue)}.tm-address-options small{display:block;color:var(--tm-muted)}.tm-checkout-item{display:flex;justify-content:space-between;align-items:center;gap:14px;padding:14px 0;border-bottom:1px solid var(--tm-border)}.tm-checkout-item .tm-price{font-size:19px}.tm-checkout-summary{align-self:start}.tm-checkout-summary .tm-row{margin:16px 0;color:var(--tm-muted)}.tm-checkout-summary hr{margin:20px 0;border:0;border-top:1px solid var(--tm-border)}.tm-checkout-summary>.tm-price{display:block;margin:8px 0 18px;font-size:31px}.tm-checkout-summary .tm-button{width:100%}.tm-checkout-summary small{display:block;margin-top:13px;color:var(--tm-muted)}.tm-checkout-result{min-height:440px;padding:65px 20px;text-align:center}.tm-checkout-result img{width:42px}.tm-checkout-result p{color:var(--tm-muted)}.tm-checkout-result .tm-button{margin:8px}
@media(max-width:950px){.tm-checkout-grid{grid-template-columns:1fr}.tm-checkout-summary{order:1}}
@media(max-width:700px){.tm-checkout-panel,.tm-checkout-summary{padding:18px}.tm-checkout-main{gap:14px}}
</style>
