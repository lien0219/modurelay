<template>
  <TokenMarketShell>
    <header class="sub-header">
      <div class="tm-container bar">
        <button class="brand" type="button" @click="router.push('/token-market')">
          <span class="brand-mark">T</span>
          <span><strong>Token Market</strong><small>交易市场</small></span>
        </button>
        <nav class="nav" aria-label="Token Market secondary navigation">
          <RouterLink to="/token-market">首页</RouterLink>
          <RouterLink to="/token-market/cart">购物车</RouterLink>
          <RouterLink to="/token-market/orders">订单</RouterLink>
          <RouterLink to="/token-market/wallet">钱包</RouterLink>
          <RouterLink to="/token-market/merchant-center">商家中心</RouterLink>
        </nav>
        <div class="wallet-pill">
          <span>钱包</span>
          <strong>{{ tokenBalance.toLocaleString('zh-CN') }} T</strong>
        </div>
      </div>
    </header>

    <div class="tm-container subpage">
      <button class="back" type="button" @click="goBack">← 返回</button>
      <div class="heading-row">
        <div>
          <span class="eyebrow">{{ eyebrow }}</span>
          <h1>{{ title }}</h1>
          <p v-if="description">{{ description }}</p>
        </div>
        <div class="actions"><slot name="actions" /></div>
      </div>

      <TokenMarketAsyncState
        :loading="market.loading"
        :error="market.lastError"
        :empty="empty"
        :forbidden="forbidden"
        :skeleton="resolvedSkeleton"
        :skeleton-count="skeletonCount"
        :empty-title="emptyTitle"
        :empty-description="emptyDescription"
        :forbidden-title="forbiddenTitle"
        :forbidden-description="forbiddenDescription"
        @retry="retry"
      >
        <template v-if="$slots.emptyActions" #emptyActions><slot name="emptyActions" /></template>
        <template v-if="$slots.permissionActions" #permissionActions><slot name="permissionActions" /></template>
        <slot />
      </TokenMarketAsyncState>
    </div>
  </TokenMarketShell>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useTokenMarketStore } from '../store'
import type { TokenMarketSkeletonVariant } from '../pageState'
import TokenMarketAsyncState from './TokenMarketAsyncState.vue'
import TokenMarketShell from './TokenMarketShell.vue'

const props = withDefaults(defineProps<{
  title: string
  description?: string
  eyebrow?: string
  empty?: boolean
  forbidden?: boolean
  skeleton?: TokenMarketSkeletonVariant
  skeletonCount?: number
  emptyTitle?: string
  emptyDescription?: string
  forbiddenTitle?: string
  forbiddenDescription?: string
}>(), {
  description: '',
  eyebrow: 'TOKEN MARKET',
  empty: false,
  forbidden: false,
  skeleton: undefined,
  skeletonCount: 4,
  emptyTitle: '这里还没有内容',
  emptyDescription: '相关数据准备好后会显示在这里。',
  forbiddenTitle: '当前功能暂不可用',
  forbiddenDescription: '当前账户暂未获得此功能的访问权限。',
})

const route = useRoute()
const router = useRouter()
const market = useTokenMarketStore()
const tokenBalance = computed(() => market.wallet?.tokenBalance ?? 0)
const resolvedSkeleton = computed<TokenMarketSkeletonVariant>(() => {
  if (props.skeleton) return props.skeleton
  const name = String(route.name ?? '')
  if (name.includes('Product') || name.includes('MerchantStore')) return 'detail'
  if (name.includes('Wallet')) return 'wallet'
  if (name.includes('MerchantCenter')) return 'dashboard'
  return 'list'
})

function goBack(): void {
  if (window.history.length > 1) router.back()
  else void router.push('/token-market')
}

async function retry(): Promise<void> {
  await market.initialize(true)
}

onMounted(async () => {
  if (!market.bootstrap) await market.initialize()
})
</script>

<style scoped>
.sub-header{position:sticky;top:0;z-index:60;border-bottom:1px solid rgba(110,142,208,.13);background:rgba(2,8,23,.84);backdrop-filter:blur(20px)}.bar{height:66px;display:flex;align-items:center;gap:24px}.brand{display:flex;align-items:center;gap:9px;border:0;color:white;background:transparent;cursor:pointer}.brand-mark{display:grid;place-items:center;width:34px;height:34px;border-radius:10px;background:linear-gradient(135deg,#793eff,#2c9dff);box-shadow:0 8px 28px rgba(74,82,255,.3);font-weight:900}.brand strong,.brand small{display:block;text-align:left}.brand strong{font-size:13px}.brand small{margin-top:1px;color:#697fa7;font-size:9px}.nav{display:flex;align-items:center;gap:6px;flex:1}.nav a{padding:8px 10px;border-radius:8px;color:#7f92b4;font-size:11px;text-decoration:none;transition:.18s ease}.nav a:hover,.nav a.router-link-active{color:#fff;background:rgba(71,99,165,.14)}.wallet-pill{display:flex;align-items:center;gap:8px;padding:8px 12px;border:1px solid rgba(110,142,208,.15);border-radius:10px;background:rgba(7,19,43,.7)}.wallet-pill span{color:#657da7;font-size:9px}.wallet-pill strong{font-size:11px}.subpage{padding:26px 0 70px}.back{margin-bottom:16px;border:0;color:#7790b8;background:transparent;font-size:11px;cursor:pointer}.back:hover{color:#fff}.heading-row{display:flex;align-items:flex-end;justify-content:space-between;gap:20px;margin-bottom:24px}.eyebrow{color:#6b82ad;font-size:9px;font-weight:800;letter-spacing:.14em}.heading-row h1{margin:6px 0 7px;font-size:clamp(24px,3vw,40px);letter-spacing:-.035em}.heading-row p{max-width:700px;margin:0;color:#7c8fac;font-size:12px;line-height:1.7}.actions{display:flex;gap:8px}@media(max-width:900px){.nav{display:none}.wallet-pill{margin-left:auto}.heading-row{align-items:flex-start;flex-direction:column}.subpage{padding-inline:14px}}
</style>
