<template>
  <TokenMarketSubpageLayout eyebrow="MERCHANT CENTER" title="商家中心" description="为入驻商家提供商品、订单、履约、收入、结算和经营数据入口；后续对接 Merchant/Settlement API。">
    <template #actions><button class="tm-button secondary" type="button">店铺设置</button><button class="tm-button primary" type="button">发布商品</button></template>

    <section class="metrics">
      <article v-for="item in metrics" :key="item.label" class="glass-card metric"><span>{{ item.label }}</span><strong>{{ item.value }}</strong><small>{{ item.hint }}</small></article>
    </section>

    <div class="merchant-grid">
      <section class="glass-card panel">
        <div class="section-head"><div><span class="eyebrow">TODAY ORDERS</span><h2>今日订单</h2></div><button @click="router.push('/token-market/orders')">查看全部</button></div>
        <article v-for="order in orders" :key="order.id" class="order"><div class="icon">{{ order.icon }}</div><div><strong>{{ order.title }}</strong><span>{{ order.id }} · {{ order.status }}</span></div><b>{{ order.amount.toLocaleString('zh-CN') }} T</b></article>
      </section>
      <aside class="glass-card panel settlement"><span class="eyebrow">SETTLEMENT</span><h2>结算概览</h2><div class="big"><span>待结算</span><strong>78,451 T</strong></div><dl><div><dt>今日收入</dt><dd>12,860 T</dd></div><div><dt>平台佣金</dt><dd>643 T</dd></div><div><dt>可提现余额</dt><dd>65,220 T</dd></div></dl><button class="tm-button primary">申请结算</button></aside>
    </div>

    <section class="glass-card panel tools"><div class="section-head"><div><span class="eyebrow">OPERATIONS</span><h2>经营工具</h2></div></div><div class="tool-grid"><button v-for="tool in tools" :key="tool.title"><b>{{ tool.icon }}</b><span>{{ tool.title }}<small>{{ tool.desc }}</small></span></button></div></section>
  </TokenMarketSubpageLayout>
</template>

<script setup lang="ts">
import { useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'
const router=useRouter()
const metrics=[{label:'今日营业额',value:'82,580 T',hint:'+18.4%'},{label:'今日订单',value:'143',hint:'完成 126'},{label:'待结算',value:'78,451 T',hint:'T+1 结算'},{label:'商品数',value:'38',hint:'在售 31'}]
const orders=[{id:'TM202610010118',icon:'◇',title:'AI 绘画专业版',status:'待交付',amount:980},{id:'TM202610010116',icon:'◆',title:'企业 AI 工作流服务',status:'履约中',amount:8600},{id:'TM202610010109',icon:'◉',title:'Token 外卖套餐',status:'配送中',amount:1280}]
const tools=[{icon:'▦',title:'商品管理',desc:'商品、SKU 与库存'},{icon:'▣',title:'订单履约',desc:'发货、配送与交付'},{icon:'◎',title:'营销中心',desc:'优惠券与活动'},{icon:'◇',title:'评价管理',desc:'评分与售后反馈'},{icon:'⇄',title:'结算记录',desc:'佣金与账单'},{icon:'⌁',title:'数据中心',desc:'经营分析与报表'}]
</script>

<style scoped>
.metrics{display:grid;grid-template-columns:repeat(4,1fr);gap:14px;margin-bottom:18px}.metric{padding:18px}.metric span,.metric small{display:block;color:#7085a8;font-size:9px}.metric strong{display:block;margin:7px 0 5px;font-size:20px}.merchant-grid{display:grid;grid-template-columns:minmax(0,1fr) 320px;gap:18px}.panel{padding:20px}.eyebrow{color:#6c82a8;font-size:9px;font-weight:800;letter-spacing:.13em}.section-head{display:flex;justify-content:space-between;align-items:center;padding-bottom:12px;border-bottom:1px solid rgba(109,139,203,.12)}.section-head h2,.settlement h2{margin:4px 0}.section-head button{border:0;color:#7188ad;background:transparent;font-size:9px;cursor:pointer}.order{display:grid;grid-template-columns:40px 1fr 110px;gap:12px;align-items:center;padding:14px 0;border-bottom:1px solid rgba(109,139,203,.1)}.icon{display:grid;place-items:center;width:38px;height:38px;border-radius:11px;background:#0b1a38}.order strong,.order span{display:block}.order strong{font-size:11px}.order span{margin-top:3px;color:#6f84a5;font-size:8px}.order b{text-align:right;font-size:11px}.settlement .big{margin:18px 0;padding:18px;border-radius:13px;background:linear-gradient(135deg,rgba(74,64,255,.18),rgba(30,143,255,.08))}.big span,.big strong{display:block}.big span{color:#6f84a6;font-size:9px}.big strong{margin-top:5px;font-size:24px}.settlement dl{margin:0 0 16px}.settlement dl div{display:flex;justify-content:space-between;padding:8px 0;color:#7388aa;font-size:9px}.settlement dd{margin:0;color:#dbe5f7}.settlement .tm-button{width:100%}.tools{margin-top:18px}.tool-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:10px;margin-top:14px}.tool-grid button{display:flex;gap:11px;align-items:center;padding:13px;border:1px solid rgba(110,139,203,.12);border-radius:11px;color:#dce6f8;background:rgba(8,20,45,.6);text-align:left;cursor:pointer}.tool-grid b{display:grid;place-items:center;width:34px;height:34px;border-radius:9px;background:rgba(82,91,255,.14)}.tool-grid span,.tool-grid small{display:block}.tool-grid span{font-size:10px}.tool-grid small{margin-top:3px;color:#6e82a5;font-size:8px}@media(max-width:950px){.metrics{grid-template-columns:repeat(2,1fr)}.merchant-grid{grid-template-columns:1fr}.tool-grid{grid-template-columns:repeat(2,1fr)}}@media(max-width:560px){.tool-grid{grid-template-columns:1fr}}
</style>
