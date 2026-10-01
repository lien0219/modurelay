<template>
  <TokenMarketSubpageLayout eyebrow="ORDER CENTER" title="我的订单" description="统一承载商城、外卖、数字商品和服务订单，数据由 Commerce Service 提供。" :empty="filteredOrders.length===0" empty-title="暂无符合条件的订单" empty-description="当前筛选条件下没有订单记录。">
    <template #actions><button class="tm-button secondary" @click="router.push('/token-market')">返回市场</button></template>
    <template #emptyActions><button class="tm-button primary" @click="activeTab='全部'">查看全部订单</button></template>
    <section class="summary-grid"><article v-for="item in summary" :key="item.label" class="glass-card metric"><span>{{item.label}}</span><strong>{{item.value}}</strong><small>{{item.hint}}</small></article></section>
    <section class="glass-card orders-card"><div class="tabs"><button v-for="tab in tabs" :key="tab" :class="{active:activeTab===tab}" @click="activeTab=tab">{{tab}}</button></div><article v-for="order in filteredOrders" :key="order.id" class="order-row"><div class="type-icon">{{iconFor(order.categoryId)}}</div><div class="order-main"><div><span>{{labelFor(order.categoryId)}}</span><small>{{order.id}}</small></div><h3>{{order.title}}</h3><p>{{formatTime(order.createdAt)}} · {{order.statusText}}</p></div><div class="order-amount"><strong>{{order.amountToken.toLocaleString('zh-CN')}} T</strong><span>{{statusLabel(order.status)}}</span></div><button class="tm-button secondary">查看详情</button></article></section>
  </TokenMarketSubpageLayout>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'
import { useTokenMarketStore } from '../store'
import type { TokenMarketCategoryId, TokenMarketOrderStatus } from '../types'
const router=useRouter(); const market=useTokenMarketStore(); const activeTab=ref('全部'); const tabs=['全部','待支付','履约中','已完成','退款/售后']
const tabStatus:Record<string,TokenMarketOrderStatus|undefined>={'待支付':'pending_payment','履约中':'fulfilling','已完成':'completed','退款/售后':'after_sale'}
const filteredOrders=computed(()=>activeTab.value==='全部'?market.orderItems:market.orderItems.filter(o=>o.status===tabStatus[activeTab.value]))
const summary=computed(()=>[{label:'当前订单',value:String(market.orderItems.length),hint:'当前已加载'},{label:'订单金额',value:`${market.orderItems.reduce((s,o)=>s+o.amountToken,0).toLocaleString('zh-CN')} T`,hint:'当前列表合计'},{label:'履约中',value:String(market.orderItems.filter(o=>o.status==='fulfilling').length),hint:'等待交付'},{label:'售后中',value:String(market.orderItems.filter(o=>o.status==='after_sale').length),hint:'需关注'}])
const labelFor=(id:TokenMarketCategoryId)=>market.categories.find(c=>c.id===id)?.label??'订单'; const iconFor=(id:TokenMarketCategoryId)=>({mall:'◇',delivery:'◉',digital:'✦',ai_credit:'✦',services:'◆'}[id]); const statusLabel=(s:TokenMarketOrderStatus)=>({pending_payment:'待支付',fulfilling:'履约中',completed:'已完成',after_sale:'退款/售后',cancelled:'已取消'}[s]); const formatTime=(v:string)=>new Date(v).toLocaleString('zh-CN',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'})
onMounted(async()=>{await market.initialize();await market.loadOrders()})
</script>
<style scoped>
.summary-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin-bottom:18px}.metric{padding:18px}.metric span,.metric small{display:block;color:#7085a8;font-size:9px}.metric strong{display:block;margin:7px 0 5px;font-size:20px}.orders-card{padding:14px 18px}.tabs{display:flex;gap:5px;padding-bottom:12px;border-bottom:1px solid rgba(109,139,203,.12)}.tabs button{padding:8px 11px;border:0;border-radius:8px;color:#7388aa;background:transparent;font-size:10px;cursor:pointer}.tabs button.active{color:#fff;background:rgba(79,91,255,.16)}.order-row{display:grid;grid-template-columns:48px minmax(0,1fr) 150px 100px;gap:14px;align-items:center;padding:16px 2px;border-bottom:1px solid rgba(109,139,203,.1)}.type-icon{display:grid;place-items:center;width:44px;height:44px;border-radius:12px;background:linear-gradient(135deg,#172653,#0a1732);color:#dce7ff}.order-main>div{display:flex;gap:8px}.order-main span,.order-main p{color:#7690b9;font-size:9px}.order-main small{color:#526888;font-size:8px}.order-main h3{margin:5px 0 4px;font-size:12px}.order-main p{margin:0}.order-amount{text-align:right}.order-amount strong,.order-amount span{display:block}.order-amount span{margin-top:4px;color:#67d8ae;font-size:9px}@media(max-width:900px){.summary-grid{grid-template-columns:repeat(2,1fr)}.order-row{grid-template-columns:44px 1fr 110px}.order-row>.tm-button{display:none}}
</style>