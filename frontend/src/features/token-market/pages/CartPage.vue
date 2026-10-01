<template>
  <TokenMarketSubpageLayout eyebrow="SHOPPING CART" title="购物车" description="统一承载商城、外卖、数字商品与服务商品的 Token 结算入口。">
    <template #actions><button class="tm-button secondary" type="button" @click="router.push('/token-market')">继续购物</button></template>

    <div class="cart-grid">
      <section class="items glass-card">
        <div class="head"><strong>已选商品</strong><span>{{ rows.length }} 件</span></div>
        <article v-for="row in rows" :key="row.product.id" class="row">
          <button class="thumb" type="button" @click="router.push(`/token-market/products/${row.product.id}`)">✦</button>
          <div class="info"><span>{{ merchantName(row.product.merchantId) }}</span><h3>{{ row.product.name }}</h3><p>{{ row.product.priceToken.toLocaleString('zh-CN') }} T / 件</p></div>
          <div class="qty"><button @click="change(row.product.id,-1)">−</button><b>{{ row.qty }}</b><button @click="change(row.product.id,1)">＋</button></div>
          <strong class="subtotal">{{ (row.product.priceToken * row.qty).toLocaleString('zh-CN') }} T</strong>
          <button class="remove" type="button" @click="remove(row.product.id)">×</button>
        </article>
        <div v-if="!rows.length" class="empty">购物车还是空的，去市场挑点东西吧。</div>
      </section>

      <aside class="summary glass-card">
        <span class="eyebrow">TOKEN CHECKOUT</span><h2>结算</h2>
        <dl><div><dt>商品小计</dt><dd>{{ subtotal.toLocaleString('zh-CN') }} T</dd></div><div><dt>平台优惠</dt><dd class="discount">-{{ discount.toLocaleString('zh-CN') }} T</dd></div><div><dt>配送/服务费</dt><dd>{{ fee.toLocaleString('zh-CN') }} T</dd></div></dl>
        <div class="total"><span>应付</span><strong>{{ total.toLocaleString('zh-CN') }} T</strong></div>
        <div class="balance"><span>Token 余额</span><b>{{ tokenBalance.toLocaleString('zh-CN') }} T</b></div>
        <button class="tm-button primary checkout" :disabled="!rows.length" type="button" @click="checkout">确认支付</button>
        <p>当前仅为结算骨架，后续接入订单创建、库存锁定、幂等支付和账本扣款。</p>
      </aside>
    </div>
  </TokenMarketSubpageLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'
import { useTokenMarketStore } from '../store'
import type { TokenMarketProduct } from '../types'

const route=useRoute(); const router=useRouter(); const market=useTokenMarketStore(); const quantities=ref<Record<string,number>>({}); const removedIds=ref<Set<string>>(new Set())
const selectedIds=computed(()=>{const queryId=String(route.query.add||route.query.buy||'');const base=market.products.slice(0,3).map(p=>p.id);const ids=queryId?[queryId,...base.filter(id=>id!==queryId)]:base;return ids.filter(id=>!removedIds.value.has(id))})
const rows=computed(()=>selectedIds.value.map(id=>market.products.find(p=>p.id===id)).filter((p):p is TokenMarketProduct=>Boolean(p)).map(product=>({product,qty:quantities.value[product.id]??(String(route.query.add||route.query.buy)===product.id?Math.max(1,Number(route.query.qty)||1):1)})))
const subtotal=computed(()=>rows.value.reduce((sum,row)=>sum+row.product.priceToken*row.qty,0)); const discount=computed(()=>Math.floor(subtotal.value*.03)); const fee=computed(()=>rows.value.length?60:0); const total=computed(()=>subtotal.value-discount.value+fee.value); const tokenBalance=computed(()=>market.wallet?.tokenBalance??0)
function merchantName(id:string){return market.merchants.find(m=>m.id===id)?.name??'Token Market 商户'}
function change(id:string,delta:number){const current=rows.value.find(r=>r.product.id===id)?.qty??1;quantities.value={...quantities.value,[id]:Math.max(1,current+delta)}}
function remove(id:string){const next=new Set(removedIds.value);next.add(id);removedIds.value=next}
function checkout(){void router.push({path:'/token-market/orders',query:{created:'1'}})}
onMounted(()=>market.initialize())
</script>

<style scoped>
.cart-grid{display:grid;grid-template-columns:minmax(0,1fr) 330px;gap:18px}.items{padding:18px}.head{display:flex;justify-content:space-between;padding:4px 4px 16px;border-bottom:1px solid rgba(106,136,199,.12)}.head span{color:#6f85a8;font-size:10px}.row{display:grid;grid-template-columns:72px minmax(0,1fr) 110px 120px 24px;gap:14px;align-items:center;padding:16px 2px;border-bottom:1px solid rgba(106,136,199,.1)}.thumb{height:64px;border:1px solid rgba(111,140,205,.15);border-radius:12px;color:#dce7ff;background:linear-gradient(135deg,#172555,#0b1833);font-size:24px;cursor:pointer}.info span{color:#657da6;font-size:8px}.info h3{margin:4px 0;font-size:12px}.info p{margin:0;color:#899bb9;font-size:9px}.qty{display:flex;border:1px solid rgba(112,141,204,.16);border-radius:8px;overflow:hidden}.qty>*{width:36px;height:32px;display:grid;place-items:center;border:0;color:#dce7f8;background:#091733}.qty button{cursor:pointer}.subtotal{text-align:right;font-size:12px}.remove{border:0;color:#6f84a7;background:transparent;cursor:pointer}.summary{align-self:start;padding:22px;position:sticky;top:84px}.eyebrow{color:#687fa7;font-size:9px;font-weight:800;letter-spacing:.13em}.summary h2{margin:6px 0 18px}.summary dl{margin:0}.summary dl div{display:flex;justify-content:space-between;padding:9px 0;color:#788cab;font-size:10px}.discount{color:#55dfb1}.total{display:flex;justify-content:space-between;align-items:end;margin-top:12px;padding-top:16px;border-top:1px solid rgba(110,140,204,.16)}.total span{color:#7f92ae;font-size:10px}.total strong{font-size:20px}.balance{display:flex;justify-content:space-between;margin:16px 0;padding:11px;border-radius:9px;background:rgba(70,91,150,.08);font-size:10px}.balance span{color:#7185a6}.checkout{width:100%}.summary p{color:#667da2;font-size:9px;line-height:1.6}.empty{padding:50px;text-align:center;color:#7186a8;font-size:11px}@media(max-width:950px){.cart-grid{grid-template-columns:1fr}.summary{position:static}.row{grid-template-columns:58px 1fr 90px}.subtotal,.remove{display:none}}
</style>
