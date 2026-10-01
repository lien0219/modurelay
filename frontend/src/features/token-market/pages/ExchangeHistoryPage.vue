<template>
  <TokenMarketSubpageLayout
    eyebrow="EXCHANGE HISTORY"
    title="兑换记录"
    description="记录平台余额与 Token 双向兑换的报价快照、汇率、状态和交易号；未来直接对接兑换账本。"
    :empty="filtered.length === 0"
    empty-title="暂无兑换记录"
    empty-description="完成余额与 Token 的兑换后，交易记录会显示在这里。"
  >
    <template #actions><button class="tm-button primary" type="button" @click="router.push('/token-market')">发起兑换</button></template>
    <template #emptyActions><button class="tm-button primary" type="button" @click="router.push('/token-market')">去兑换</button></template>

    <section class="glass-card history-card">
      <div class="filters"><button v-for="item in filters" :key="item" :class="{active:filter===item}" @click="filter=item">{{ item }}</button><span></span><button class="export">导出记录</button></div>
      <div class="table-head"><span>时间</span><span>方向</span><span>来源金额</span><span>汇率</span><span>到账金额</span><span>状态</span><span>交易号</span></div>
      <article v-for="row in filtered" :key="row.id" class="row"><span>{{ row.time }}</span><strong>{{ row.direction }}</strong><span>{{ row.source }}</span><span>{{ row.rate }}</span><b>{{ row.destination }}</b><span class="status">{{ row.status }}</span><code>{{ row.id }}</code></article>
    </section>
  </TokenMarketSubpageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import TokenMarketSubpageLayout from '../components/TokenMarketSubpageLayout.vue'
const router=useRouter(); const filter=ref('全部'); const filters=['全部','余额 → Token','Token → 余额']
const rows=[{id:'EX202610010001',time:'10-01 15:20',direction:'余额 → Token',source:'¥ 100.00',rate:'1 : 100',destination:'10,000 T',status:'成功'},{id:'EX202609300088',time:'09-30 20:18',direction:'Token → 余额',source:'25,000 T',rate:'100 : 1',destination:'¥ 250.00',status:'成功'},{id:'EX202609290042',time:'09-29 11:06',direction:'余额 → Token',source:'¥ 500.00',rate:'1 : 100',destination:'50,000 T',status:'成功'},{id:'EX202609280019',time:'09-28 09:43',direction:'Token → 余额',source:'10,000 T',rate:'100 : 1',destination:'¥ 100.00',status:'成功'}]
const filtered=computed(()=>filter.value==='全部'?rows:rows.filter(row=>row.direction===filter.value))
</script>

<style scoped>
.history-card{padding:14px 18px}.filters{display:flex;gap:5px;align-items:center;padding-bottom:12px;border-bottom:1px solid rgba(109,139,203,.12)}.filters button{padding:8px 11px;border:0;border-radius:8px;color:#7388aa;background:transparent;font-size:10px;cursor:pointer}.filters button.active{color:#fff;background:rgba(79,91,255,.16)}.filters span{flex:1}.filters .export{border:1px solid rgba(111,140,205,.14);background:#091833}.table-head,.row{display:grid;grid-template-columns:120px 125px 130px 100px 130px 80px 1fr;gap:12px;align-items:center}.table-head{padding:14px 2px 10px;color:#62789c;font-size:9px}.row{padding:15px 2px;border-top:1px solid rgba(109,139,203,.09);color:#899bb7;font-size:10px}.row strong,.row b{color:#e1e9f8}.status{display:inline-flex;width:max-content;padding:4px 7px;border-radius:999px;color:#60d8ad;background:rgba(52,185,139,.1)}code{overflow:hidden;color:#64799c;font-family:ui-monospace,monospace;font-size:8px;text-overflow:ellipsis;white-space:nowrap}@media(max-width:900px){.history-card{overflow:auto}.table-head,.row{min-width:850px}}
</style>
