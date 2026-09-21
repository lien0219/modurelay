<template>
  <span class="relative flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-gray-100 font-semibold text-gray-500 dark:bg-dark-700 dark:text-gray-300">
    <span v-if="!resolvedIcon || failed">{{ fallback }}</span>
    <img
      v-else-if="isImageURL"
      :src="resolvedIcon"
      :alt="label"
      class="h-full w-full object-contain p-1"
      loading="lazy"
      decoding="async"
      @error="failed = true"
    />
    <IconifyIcon
      v-else
      :icon="resolvedIcon"
      class="h-[70%] w-[70%]"
      @vue:mounted="failed = false"
    />
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Icon as IconifyIcon } from '@iconify/vue'

const props = defineProps<{
  icon?: string
  label?: string
}>()

const failed = ref(false)
const resolvedIcon = computed(() => String(props.icon || '').trim())
const isImageURL = computed(() => {
  const value = resolvedIcon.value
  return value.startsWith('/') || value.startsWith('data:image/')
})
const fallback = computed(() => String(props.label || '?').trim().slice(0, 1).toUpperCase() || '?')

watch(resolvedIcon, () => {
  failed.value = false
})
</script>
