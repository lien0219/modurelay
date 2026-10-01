<template>
  <TokenMarketSubpageLayout
    v-if="error || permissionState !== 'allowed'"
    :eyebrow="permissionState === 'denied' ? 'ACCESS CONTROL' : 'ROUTE BOUNDARY'"
    :title="permissionState === 'denied' ? '当前页面暂不可访问' : permissionState === 'pending' ? '正在校验访问权限' : '页面加载失败'"
    :description="permissionState === 'denied' ? '当前账户暂未获得此 Token Market 功能的访问权限。' : permissionState === 'pending' ? '正在完成权限校验，请稍候。' : '页面发生异常，您可以重新加载或返回 Token Market 首页。'"
  >
    <TokenMarketSkeleton v-if="permissionState === 'pending'" variant="detail" :count="3" />
    <TokenMarketStatePanel
      v-else
      :kind="permissionState === 'denied' ? 'forbidden' : 'error'"
      :title="permissionState === 'denied' ? '需要额外权限' : '页面暂时不可用'"
      :description="permissionState === 'denied' ? `权限占位：${permission || 'token-market.access'}。后续接入真实权限服务后由统一 resolver 控制。` : errorMessage"
    >
      <template #actions>
        <button v-if="permissionState !== 'denied'" class="tm-button secondary" type="button" @click="reset">重新加载</button>
        <button class="tm-button primary" type="button" @click="router.push('/token-market')">返回市场首页</button>
      </template>
    </TokenMarketStatePanel>
  </TokenMarketSubpageLayout>

  <slot v-else />
</template>

<script setup lang="ts">
import { computed, onErrorCaptured, ref, watchEffect } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import TokenMarketSkeleton from './TokenMarketSkeleton.vue'
import TokenMarketStatePanel from './TokenMarketStatePanel.vue'
import TokenMarketSubpageLayout from './TokenMarketSubpageLayout.vue'
import type { TokenMarketPermissionResult } from '../pageState'

const route = useRoute()
const router = useRouter()
const error = ref<unknown>(null)
const permissionState = ref<TokenMarketPermissionResult>('allowed')
const permission = computed(() => typeof route.meta.marketPermission === 'string' ? route.meta.marketPermission : '')
const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '发生了未预期的页面错误。')

watchEffect(() => {
  // Enterprise placeholder contract: current production behavior stays allow-by-default.
  // QA can preview the states with ?marketPermission=denied|pending. Replace this
  // resolver with the real permission/merchant-eligibility service later.
  const preview = String(route.query.marketPermission ?? '')
  if (preview === 'denied') permissionState.value = 'denied'
  else if (preview === 'pending') permissionState.value = 'pending'
  else permissionState.value = 'allowed'
})

onErrorCaptured((captured) => {
  error.value = captured
  return false
})

function reset(): void {
  error.value = null
  void router.replace({ path: route.path, query: { ...route.query, retry: String(Date.now()) }, hash: route.hash })
}
</script>
