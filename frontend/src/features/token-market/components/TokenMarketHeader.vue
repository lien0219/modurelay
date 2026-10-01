<template>
  <header class="tm-header tm-container" @click.stop>
    <button class="tm-brand" type="button" @click="emit('back')">
      <span class="tm-brand-mark">MR</span>
      <span>
        <strong>Token 交易市场</strong>
        <small>ModuRelay Ecosystem</small>
      </span>
    </button>

    <nav class="tm-nav" aria-label="Token Market">
      <button
        v-for="item in items"
        :key="item.label"
        type="button"
        :class="{ active: item.section === 'home' }"
        @click="emit('navigate', item)"
      >
        {{ item.label }}
      </button>
    </nav>

    <label class="tm-header-search">
      <span aria-hidden="true">⌕</span>
      <input
        :value="search"
        type="search"
        placeholder="搜索商品、商家或关键词..."
        @input="emit('update:search', ($event.target as HTMLInputElement).value)"
        @keyup.enter="emit('search')"
      />
      <kbd>⌘ K</kbd>
    </label>

    <div class="tm-header-actions">
      <button type="button" aria-label="通知" @click="emit('toggle-notifications')">◌</button>
      <button class="avatar" type="button" aria-label="个人菜单" @click="emit('toggle-profile')">T</button>
    </div>
  </header>
</template>

<script setup lang="ts">
import type { TokenMarketNavItem } from '../ui'

defineProps<{
  items: TokenMarketNavItem[]
  search: string
}>()

const emit = defineEmits<{
  back: []
  navigate: [item: TokenMarketNavItem]
  search: []
  'update:search': [value: string]
  'toggle-notifications': []
  'toggle-profile': []
}>()
</script>

<style scoped>
.tm-header{position:sticky;top:0;z-index:40;display:grid;grid-template-columns:auto 1fr minmax(260px,360px) auto;align-items:center;gap:20px;min-height:72px;padding:10px 0;background:linear-gradient(180deg,rgba(2,8,23,.94),rgba(2,8,23,.76),transparent);backdrop-filter:blur(12px)}.tm-brand{display:flex;align-items:center;gap:10px;border:0;color:#fff;background:transparent;text-align:left;cursor:pointer}.tm-brand-mark{display:grid;place-items:center;width:36px;height:36px;border-radius:10px;background:linear-gradient(135deg,#8759ff,#4f6dff 48%,#27c4ff);box-shadow:0 10px 30px rgba(86,75,255,.35);font-size:10px;font-weight:900}.tm-brand strong,.tm-brand small{display:block}.tm-brand strong{font-size:14px}.tm-brand small{margin-top:2px;color:#687da5;font-size:8px;letter-spacing:.1em}.tm-nav{display:flex;justify-content:center;gap:4px}.tm-nav button{padding:8px 10px;border:0;border-radius:8px;color:#7184a8;background:transparent;font:inherit;font-size:11px;cursor:pointer}.tm-nav button:hover,.tm-nav button.active{color:#dce8ff;background:rgba(91,116,179,.1)}.tm-header-search{display:grid;grid-template-columns:26px 1fr auto;align-items:center;height:38px;padding:0 10px;border:1px solid rgba(95,125,188,.18);border-radius:10px;background:rgba(7,18,42,.84);color:#667b9f}.tm-header-search input{min-width:0;border:0;outline:0;color:#d6e2f6;background:transparent;font:inherit;font-size:11px}.tm-header-search kbd{padding:3px 6px;border:1px solid rgba(95,126,189,.2);border-radius:5px;color:#64789c;background:rgba(23,43,78,.8);font-size:8px}.tm-header-actions{display:flex;gap:8px}.tm-header-actions button{display:grid;place-items:center;width:34px;height:34px;border:1px solid rgba(100,132,194,.16);border-radius:10px;color:#9db1d3;background:rgba(8,22,49,.7);cursor:pointer}.tm-header-actions .avatar{border-radius:50%;color:white;background:linear-gradient(135deg,#725cff,#2aa9f8);font-size:10px;font-weight:900}@media(max-width:1100px){.tm-header{grid-template-columns:auto 1fr auto}.tm-nav{display:none}.tm-header-search{grid-column:2}}@media(max-width:720px){.tm-header{grid-template-columns:1fr auto;gap:10px}.tm-header-search{grid-column:1/-1;grid-row:2}.tm-brand small{display:none}}
</style>
