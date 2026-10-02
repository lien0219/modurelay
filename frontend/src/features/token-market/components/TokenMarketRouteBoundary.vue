<template>
  <div v-if="permissionState === 'pending'" class="tm-feedback" role="status">正在校验访问权限…</div>
  <div v-else-if="permissionState === 'denied'" class="tm-empty" role="alert">
    <img src="/token-market/icons/shield.svg" alt="" /><h2>需要额外权限</h2>
    <p>当前账户暂未获得此功能的访问权限。</p>
    <RouterLink class="tm-button" to="/token-market">返回市场首页</RouterLink>
  </div>
  <div v-else-if="error" class="tm-empty" role="alert">
    <h2>页面暂时无法加载</h2><p>{{ errorMessage }}</p>
    <button class="tm-button" type="button" @click="reset">重新加载</button>
  </div>
  <slot v-else />
</template>
<script setup lang="ts">
import { computed, onErrorCaptured, ref, watchEffect } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import type { TokenMarketPermissionResult } from '../pageState'
const route = useRoute()
const router = useRouter()
const error = ref<unknown>(null)
const permissionState = ref<TokenMarketPermissionResult>('allowed')
const errorMessage = computed(() => error.value instanceof Error ? error.value.message : '发生了未预期的页面错误。')
watchEffect(() => {
  const preview = String(route.query.marketPermission ?? '')
  permissionState.value = preview === 'denied' || preview === 'pending' ? preview : 'allowed'
})
onErrorCaptured(captured => { error.value = captured; return false })
function reset(): void {
  error.value = null
  void router.replace({ path: route.path, query: { ...route.query, retry: String(Date.now()) }, hash: route.hash })
}
</script>
