<template>
  <section class="exchange-card tm-glass" @click.stop>
    <div class="heading">
      <div><span>快速兑换</span><p>平台余额和 Token 自由兑换</p></div>
      <small>汇率 · 1 ¥ = {{ exchangeRate }} T</small>
    </div>

    <div class="tabs">
      <button type="button" :class="{ active: mode === 'balance' }" @click="emit('mode-change', 'balance')">平台余额 ↔ Token</button>
      <button type="button" :class="{ active: mode === 'token' }" @click="emit('mode-change', 'token')">Token ↔ 平台余额</button>
    </div>

    <label class="amount-box">
      <span>{{ mode === 'balance' ? '¥' : 'T' }}</span>
      <input :value="sourceValue" type="number" min="0" @input="emit('source-change', Number(($event.target as HTMLInputElement).value))" />
      <small>{{ mode === 'balance' ? `余额 ¥ ${platformBalance.toLocaleString()}` : `余额 ${tokenBalance.toLocaleString()} T` }}</small>
    </label>

    <button class="swap" type="button" aria-label="切换兑换方向" @click="emit('swap')">⇅</button>

    <div class="amount-box output">
      <span>{{ mode === 'balance' ? 'T' : '¥' }}</span>
      <strong>{{ formattedOutput }}</strong>
      <small>{{ mode === 'balance' ? `≈ ¥ ${(sourceValue || 0).toLocaleString()}` : `≈ ${(sourceValue || 0).toLocaleString()} T` }}</small>
    </div>

    <div class="actions">
      <button class="tm-primary-button" type="button" :disabled="loading" @click="emit('submit')">{{ loading ? '处理中...' : mode === 'balance' ? '兑换 Token' : '兑换余额' }}</button>
      <button class="tm-secondary-button" type="button" @click="emit('swap')">{{ mode === 'balance' ? '兑换余额' : '兑换 Token' }}</button>
    </div>
  </section>
</template>

<script setup lang="ts">
import type { TokenMarketExchangeMode } from '../ui'

defineProps<{
  mode: TokenMarketExchangeMode
  sourceValue: number
  formattedOutput: string
  exchangeRate: number
  platformBalance: number
  tokenBalance: number
  loading: boolean
}>()

const emit = defineEmits<{
  'mode-change': [mode: TokenMarketExchangeMode]
  'source-change': [value: number]
  swap: []
  submit: []
}>()
</script>

<style scoped>
.exchange-card{position:relative;min-height:280px;padding:26px;border-radius:24px}.heading{display:flex;align-items:flex-start;justify-content:space-between;gap:12px}.heading>div>span{font-size:16px;font-weight:800}.heading p{margin:5px 0 0;color:#7085aa;font-size:10px}.heading small{color:#6a7da0;font-size:9px}.tabs{display:grid;grid-template-columns:1fr 1fr;gap:7px;margin:20px 0 12px}.tabs button{height:36px;border:0;border-radius:9px;color:#7890b5;background:rgba(13,30,67,.74);font:inherit;font-size:10px;font-weight:700;cursor:pointer}.tabs button.active{color:white;background:linear-gradient(90deg,#5265ff,#24c6ff);box-shadow:0 8px 26px rgba(52,94,255,.24)}.amount-box{display:grid;grid-template-columns:30px 1fr auto;align-items:center;min-height:50px;padding:0 14px;border:1px solid rgba(102,135,207,.16);border-radius:10px;background:rgba(6,17,41,.84)}.amount-box span{color:#dce8fd;font-weight:900}.amount-box input{min-width:0;border:0;outline:0;color:#fff;background:transparent;font:inherit;font-size:14px;font-weight:900}.amount-box small{color:#6e83aa;font-size:9px}.amount-box.output{margin-top:10px}.amount-box.output strong{font-size:14px}.swap{position:absolute;right:36px;top:176px;z-index:2;width:28px;height:28px;border:1px solid rgba(108,142,218,.28);border-radius:50%;color:#afc4ec;background:#10244d;cursor:pointer}.actions{display:grid;grid-template-columns:1fr 1fr;gap:9px;margin-top:14px}.actions button{min-height:38px;font-size:10px}@media(max-width:620px){.exchange-card{padding:22px}.heading{display:block}.heading small{display:block;margin-top:5px}.swap{right:30px}}
</style>
