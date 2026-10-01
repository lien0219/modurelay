<template>
  <TokenMarketSubpageLayout eyebrow="ORDER CENTER" title="我的订单" description="统一承载商城、外卖、数字商品和服务订单，后续按订单类型挂接不同履约状态机。">
    <template #actions><button class="tm-button secondary" type="button" @click="router.push('/token-market')">返回市场</button></template>

    <section class="summary-grid">
      <article v-for="item in summary" :key="item.label" class="glass-card metric"><span>{{ item.label }}</span><strong>{{ item.value }}</strong><small>{{ item.hint }}</small></article>
    </section>

    <section class="glass-card orders-card">
      <div class="tabs"><button v-for="tab in tabs" :key="tab" :class="{active:activeTab===tab}" @click="activeTab=tab">{{ tab }}</button></div>
      <article v-for="order in filteredOrders" :key="order.id" class="order-row">
        <div class="type-icon">{{ order.icon }}</div>
        <div class="order-main"><div><span>{{ order.type }}</span><small>{{ order.id }}</small></div><h3>{{ order.title }}</h3><p>{{ order.time }} · {{ order.statusText }}</p></div>
        <div class="order-amount"><strong>{{ order.amount.toLocaleString('zh-CN') }} T</strong><span>{{ order.status }}</span></div>
        <button class="tm-button secondary" type="button">查看详情</button>
      </article>
    </section>
  </TokenMarketSubpageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'

const router=useRouter(); const activeTab=ref('全部')
const tabs=['全部','待支付','履约中','已完成','退款/售后']
const orders=[
{id:'TM202610010001',type:'商城',icon:'◇',title:'星芒无线蓝牙耳机',time:'今天 14:32',status:'已完成',statusText:'已确认收货',amount:2880},
{id:'TM202610010002',type:'外卖',icon:'◉',title:'深夜食堂 · 双人套餐',time:'今天 12:08',status:'履约中',statusText:'骑手配送中',amount:1280},
{id:'TM202609300087',type:'数字商品',icon:'✦',title:'AI 绘画专业版',time:'昨天 22:16',status:'已完成',statusText:'自动交付完成',amount:980},
{id:'TM202609290041',type:'服务',icon:'◆',title:'AI 工作流部署服务',time:'09-29 16:20',status:'退款/售后',statusText:'售后处理中',amount:8600},
]
const filteredOrders=computed(()=>activeTab.value==='全部'?orders:orders.filter(o=>o.status===activeTab.value))
const summary=[{label:'本月订单',value:'18',hint:'较上月 +22%'},{label:'本月消费',value:'28,460 T',hint:'已结算订单'},{label:'履约中',value:'2',hint:'等待交付'},{label:'售后中',value:'1',hint:'需关注'}]
</script>

<style scoped>
.summary-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin-bottom:18px}.metric{padding:18px}.metric span,.metric small{display:block;color:#7085a8;font-size:9px}.metric strong{display:block;margin:7px 0 5px;font-size:20px}.orders-card{padding:14px 18px}.tabs{display:flex;gap:5px;padding-bottom:12px;border-bottom:1px solid rgba(109,139,203,.12)}.tabs button{padding:8px 11px;border:0;border-radius:8px;color:#7388aa;background:transparent;font-size:10px;cursor:pointer}.tabs button.active{color:#fff;background:rgba(79,91,255,.16)}.order-row{display:grid;grid-template-columns:48px minmax(0,1fr) 150px 100px;gap:14px;align-items:center;padding:16px 2px;border-bottom:1px solid rgba(109,139,203,.1)}.type-icon{display:grid;place-items:center;width:44px;height:44px;border-radius:12px;background:linear-gradient(135deg,#172653,#0a1732);color:#dce7ff}.order-main>div{display:flex;gap:8px;align-items:center}.order-main span{color:#7690b9;font-size:9px}.order-main small{color:#526888;font-size:8px}.order-main h3{margin:5px 0 4px;font-size:12px}.order-main p{margin:0;color:#6f83a3;font-size:9px}.order-amount{text-align:right}.order-amount strong,.order-amount span{display:block}.order-amount strong{font-size:12px}.order-amount span{margin-top:4px;color:#67d8ae;font-size:9px}@media(max-width:900px){.summary-grid{grid-template-columns:repeat(2,1fr)}.order-row{grid-template-columns:44px 1fr 110px}.order-row>.tm-button{display:none}}@media(max-width:560px){.summary-grid{grid-template-columns:1fr 1fr}.tabs{overflow:auto}.order-amount{display:none}}
</style>
