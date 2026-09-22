<template>
  <span class="relative flex shrink-0 items-center justify-center overflow-hidden rounded-lg bg-gray-100 font-semibold text-gray-500 dark:bg-dark-700 dark:text-gray-300">
    <span v-if="showFallback">{{ fallback }}</span>
    <img
      v-else-if="isImageURL && imageObjectURL"
      :src="imageObjectURL"
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
    />
  </span>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { Icon as IconifyIcon } from '@iconify/vue'

const props = defineProps<{
  icon?: string
  label?: string
}>()

const failed = ref(false)
const imageObjectURL = ref('')
let requestController: AbortController | null = null

const resolvedIcon = computed(() => String(props.icon || '').trim())
const isImageURL = computed(() => {
  const value = resolvedIcon.value
  return value.startsWith('/') || value.startsWith('data:image/')
})
const fallback = computed(() => String(props.label || '?').trim().slice(0, 1).toUpperCase() || '?')
const showFallback = computed(() => !resolvedIcon.value || failed.value || (isImageURL.value && !imageObjectURL.value))

function clearObjectURL() {
  if (imageObjectURL.value.startsWith('blob:')) URL.revokeObjectURL(imageObjectURL.value)
  imageObjectURL.value = ''
}

async function loadImageURL(value: string) {
  requestController?.abort()
  requestController = null
  clearObjectURL()
  failed.value = false

  if (!value || !isImageURL.value) return
  if (value.startsWith('data:image/')) {
    imageObjectURL.value = value
    return
  }

  const controller = new AbortController()
  requestController = controller
  try {
    // Fetch first instead of binding the provider-proxy URL directly to <img>.
    // A missing optional icon can then fail quietly and fall back to the
    // service monogram instead of flooding DevTools with broken-image 404s.
    const response = await fetch(value, {
      credentials: 'same-origin',
      cache: 'force-cache',
      signal: controller.signal,
    })
    if (!response.ok || response.status === 204) {
      failed.value = true
      return
    }
    const contentType = String(response.headers.get('content-type') || '').toLowerCase()
    if (!contentType.startsWith('image/')) {
      failed.value = true
      return
    }
    const blob = await response.blob()
    if (!blob.size) {
      failed.value = true
      return
    }
    imageObjectURL.value = URL.createObjectURL(blob)
  } catch (error) {
    if ((error as { name?: string })?.name !== 'AbortError') failed.value = true
  }
}

watch(resolvedIcon, value => {
  void loadImageURL(value)
}, { immediate: true })

onBeforeUnmount(() => {
  requestController?.abort()
  clearObjectURL()
})
</script>
