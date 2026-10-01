<template>
  <section class="wallet-card tm-glass" @click.stop>
    <div class="wallet-copy">
      <span class="eyebrow">我的 Token 钱包</span>
      <div class="balance-line">
        <strong>{{ visible ? formatToken(wallet?.tokenBalance ?? 0) : '••••••' }}</strong>
        <span>T</span>
        <button type="button" :aria-label="visible ? '隐藏余额' : '显示余额'" @click="emit('toggle-visible')">◉</button>
      </div>
      <p>≈ ¥ {{ visible ? formatMoney((wallet?.tokenBalance ?? 0) / exchangeRate) : '••••' }}</p>
      <div class="stats">
        <div><span>今日消费</span><strong>{{ formatToken(wallet?.todaySpentToken ?? 0) }} T</strong></div>
        <div><span>待结算</span><strong class="cyan">{{ formatToken(wallet?.pendingToken ?? 0) }} T</strong></div>
        <div><span>冻结中</span><strong>{{ formatToken(wallet?.frozenToken ?? 0) }} T</strong></div>
        <div><span>昨日收入</span><strong class="green">+{{ formatToken(wallet?.yesterdayIncomeToken ?? 0) }} T</strong></div>
      </div>
    </div>
    <div class="wallet-orb" aria-hidden="true">
      <span>T</span>
    </div>
    <button class="details" type="button" @click="emit('details')">查看明细 <span>→</span></button>
  </section>
</template>

<script setup lang="ts">
import type { TokenMarketWalletSnapshot } from '../types'

defineProps<{
  wallet: TokenMarketWalletSnapshot | null
  exchangeRate: number
  visible: boolean
}>()

const emit = defineEmits<{
  'toggle-visible': []
  details: []
}>()

function formatToken(value: number): string {
  return value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })
}

function formatMoney(value: number): string {
  return value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}
</script>

<style scoped>
.wallet-card{position:relative;min-height:280px;padding:30px 34px;border-radius:24px;overflow:hidden;background:radial-gradient(circle at 68% 42%,rgba(65,120,255,.27),transparent 26%),linear-gradient(130deg,rgba(23,67,146,.95),rgba(7,27,72,.95) 48%,rgba(5,20,51,.97));}.wallet-copy{position:relative;z-index:2;max-width:57%}.eyebrow{color:#80a6e9;font-size:12px;font-weight:700;letter-spacing:.08em}.balance-line{display:flex;align-items:center;gap:10px;margin-top:14px}.balance-line strong{font-size:clamp(32px,4.2vw,58px);line-height:1;font-weight:900;letter-spacing:-.04em}.balance-line>span{margin-top:12px;color:#8eb3f2;font-weight:800}.balance-line button{margin-left:6px;border:0;color:#81a5dc;background:transparent;cursor:pointer}.wallet-copy>p{margin:10px 0 26px;color:#7e9bc9;font-size:12px}.stats{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px 28px}.stats div{display:flex;flex-direction:column;gap:4px}.stats span{color:#7690ba;font-size:10px}.stats strong{font-size:13px}.stats .cyan{color:#41d3ff}.stats .green{color:#55e7b4}.wallet-orb{position:absolute;right:13%;top:20%;display:grid;place-items:center;width:150px;aspect-ratio:1;border-radius:50%;color:#f8fbff;background:radial-gradient(circle at 35% 25%,#9fd0ff 0,#7398ff 24%,#7061ff 53%,#34479b 73%,#162453 100%);box-shadow:0 0 65px rgba(82,100,255,.5),inset -18px -22px 34px rgba(14,19,71,.45),inset 12px 14px 24px rgba(255,255,255,.22);font-size:52px;font-weight:300;text-shadow:0 8px 18px rgba(0,0,0,.3)}.wallet-orb::after{position:absolute;inset:-24px;content:"";border:1px solid rgba(105,128,255,.13);border-radius:50%;box-shadow:0 0 0 18px rgba(84,81,255,.035)}.details{position:absolute;right:22px;bottom:20px;padding:8px 12px;border:1px solid rgba(88,154,255,.4);border-radius:8px;color:#81b9ff;background:rgba(9,26,59,.56);font:inherit;font-size:10px;cursor:pointer}.details span{margin-left:7px}@media(max-width:900px){.wallet-copy{max-width:100%}.wallet-orb{right:8%;top:24%;width:110px;opacity:.55}.stats{padding-right:120px}}@media(max-width:620px){.wallet-card{padding:24px}.wallet-orb{display:none}.stats{padding-right:0}.wallet-copy{max-width:100%}}
</style>
