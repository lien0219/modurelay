<template>
  <TokenMarketSkeleton v-if="loading" :variant="skeleton" :count="skeletonCount" />
  <TokenMarketStatePanel v-else-if="forbidden" kind="forbidden" :title="forbiddenTitle" :description="forbiddenDescription">
    <template v-if="$slots.permissionActions" #actions><slot name="permissionActions" /></template>
  </TokenMarketStatePanel>
  <TokenMarketStatePanel v-else-if="error" kind="error" :title="errorTitle" :description="error">
    <template #actions><button class="tm-button primary" type="button" @click="emit('retry')">重新加载</button></template>
  </TokenMarketStatePanel>
  <TokenMarketStatePanel v-else-if="empty" kind="empty" :title="emptyTitle" :description="emptyDescription">
    <template v-if="$slots.emptyActions" #actions><slot name="emptyActions" /></template>
  </TokenMarketStatePanel>
  <slot v-else />
</template>

<script setup lang="ts">
import TokenMarketSkeleton from './TokenMarketSkeleton.vue'
import TokenMarketStatePanel from './TokenMarketStatePanel.vue'
import type { TokenMarketSkeletonVariant } from '../pageState'

withDefaults(defineProps<{
  loading?: boolean
  error?: string | null
  empty?: boolean
  forbidden?: boolean
  skeleton?: TokenMarketSkeletonVariant
  skeletonCount?: number
  errorTitle?: string
  emptyTitle?: string
  emptyDescription?: string
  forbiddenTitle?: string
  forbiddenDescription?: string
}>(), {
  loading: false,
  error: null,
  empty: false,
  forbidden: false,
  skeleton: 'list',
  skeletonCount: 4,
  errorTitle: '加载失败',
  emptyTitle: '这里还没有内容',
  emptyDescription: '相关数据准备好后会显示在这里。',
  forbiddenTitle: '当前功能暂不可用',
  forbiddenDescription: '当前账户暂未获得此功能的访问权限。',
})

const emit = defineEmits<{ retry: [] }>()
</script>
