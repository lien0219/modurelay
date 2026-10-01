<template>
  <header class="tm-header" @click.stop>
    <div class="tm-container tm-header-inner">
      <button class="tm-brand" type="button" @click="emit('back')">
        <span class="tm-brand-mark"><span class="cube">◆</span></span>
        <span class="tm-brand-copy"><strong>Token 交易市场</strong></span>
      </button>

      <nav class="tm-nav" aria-label="Token Market">
        <button v-for="item in items" :key="item.label" type="button" :class="{active:item.section==='home'}" @click="emit('navigate',item)">{{ item.label }}</button>
      </nav>

      <label class="tm-header-search">
        <MarketIcon name="search" />
        <input :value="search" type="search" placeholder="搜索商品、商家或关键词..." @input="handleInput" @keyup.enter="emit('search')" />
        <kbd>⌘K</kbd>
      </label>

      <div class="tm-header-actions">
        <button type="button" aria-label="通知" @click="emit('toggle-notifications')"><MarketIcon name="bell" /></button>
        <button class="avatar" type="button" aria-label="个人菜单" @click="emit('toggle-profile')"><span>CA</span></button>
      </div>
    </div>
  </header>
</template>

<script setup lang="ts">
import MarketIcon from './MarketIcon.vue'
import type { TokenMarketNavItem } from '../ui'

defineProps<{items:TokenMarketNavItem[];search:string}>()
const emit=defineEmits<{back:[];navigate:[item:TokenMarketNavItem];search:[];'update:search':[value:string];'toggle-notifications':[];'toggle-profile':[]}>()
function handleInput(event:Event){emit('update:search',(event.target as HTMLInputElement).value)}
</script>

<style scoped>
.tm-header{position:sticky;top:0;z-index:50;border-bottom:1px solid rgba(87,121,194,.14);background:rgba(2,8,23,.9);backdrop-filter:blur(22px)}.tm-header-inner{height:58px;display:grid;grid-template-columns:auto 1fr minmax(270px,330px) auto;align-items:center;gap:18px}.tm-brand{display:flex;align-items:center;gap:10px;border:0;color:#fff;background:transparent;cursor:pointer}.tm-brand-mark{display:grid;place-items:center;width:32px;height:32px;border-radius:9px;background:linear-gradient(135deg,#6f46ff,#3a8dff);box-shadow:0 8px 25px rgba(77,83,255,.35)}.cube{font-size:14px;transform:rotate(45deg)}.tm-brand-copy strong{font-size:16px;letter-spacing:-.02em;white-space:nowrap}.tm-nav{display:flex;justify-content:center;gap:3px}.tm-nav button{position:relative;padding:18px 13px;border:0;color:#90a0bf;background:transparent;font:inherit;font-size:11px;cursor:pointer}.tm-nav button.active{color:#8db7ff}.tm-nav button.active:after{position:absolute;left:24%;right:24%;bottom:5px;height:2px;border-radius:999px;background:#4ea3ff;box-shadow:0 0 12px #4ea3ff;content:""}.tm-header-search{display:grid;grid-template-columns:20px 1fr auto;align-items:center;height:34px;padding:0 10px;border:1px solid rgba(98,127,189,.18);border-radius:10px;background:rgba(7,18,42,.9);color:#6e85ac}.tm-header-search input{min-width:0;border:0;outline:0;color:#d6e2f6;background:transparent;font:inherit;font-size:10px}.tm-header-search kbd{padding:2px 6px;border:1px solid rgba(95,126,189,.18);border-radius:5px;color:#64789c;background:rgba(23,43,78,.8);font-size:8px}.tm-header-actions{display:flex;gap:8px}.tm-header-actions button{display:grid;place-items:center;width:32px;height:32px;border:1px solid rgba(100,132,194,.16);border-radius:9px;color:#9db1d3;background:rgba(8,22,49,.7);cursor:pointer}.tm-header-actions .avatar{border-radius:50%;color:white;background:linear-gradient(135deg,#725cff,#2aa9f8);font-size:8px;font-weight:800}@media(max-width:1100px){.tm-header-inner{grid-template-columns:auto 1fr auto}.tm-nav{display:none}.tm-header-search{grid-column:2}}@media(max-width:720px){.tm-header-inner{height:auto;grid-template-columns:1fr auto;padding:10px 0}.tm-header-search{grid-column:1/-1;grid-row:2}.tm-brand-copy strong{font-size:14px}}
</style>
