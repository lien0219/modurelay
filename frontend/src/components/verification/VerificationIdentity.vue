<template>
  <span class="inline-flex min-w-0 items-center gap-2">
    <span class="flex h-6 w-6 shrink-0 items-center justify-center overflow-hidden rounded-md border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-700">
      <IconifyIcon
        v-if="platformIcon"
        :icon="platformIcon"
        class="h-4 w-4"
        aria-hidden="true"
      />
      <span
        v-else-if="countryFlagClass"
        class="fi text-sm"
        :class="countryFlagClass"
        aria-hidden="true"
      />
      <img
        v-else-if="imageUrl && !imageFailed"
        :src="imageUrl"
        alt=""
        aria-hidden="true"
        class="h-4 w-4 object-contain"
        loading="lazy"
        @error="imageFailed = true"
      />
      <Icon v-else :name="kind === 'country' ? 'globe' : 'grid'" size="xs" class="text-gray-500 dark:text-gray-300" aria-hidden="true" />
    </span>
    <span class="min-w-0">
      <span class="block truncate font-medium text-gray-900 dark:text-white">{{ displayLabel }}</span>
      <span v-if="showCode && normalizedCode" class="block font-mono text-[11px] uppercase text-gray-500 dark:text-gray-400">{{ normalizedCode }}</span>
    </span>
  </span>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Icon as IconifyIcon } from '@iconify/vue'
import amazonIcon from '@iconify-icons/logos/aws'
import discordIcon from '@iconify-icons/logos/discord-icon'
import githubIcon from '@iconify-icons/logos/github-icon'
import googleIcon from '@iconify-icons/logos/google-icon'
import microsoftIcon from '@iconify-icons/logos/microsoft-icon'
import openaiIcon from '@iconify-icons/logos/openai-icon'
import telegramIcon from '@iconify-icons/logos/telegram'
import whatsappIcon from '@iconify-icons/logos/whatsapp-icon'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  kind: 'platform' | 'country'
  code?: string
  label?: string
  icon?: string
  showCode?: boolean
}>(), {
  code: '',
  label: '',
  icon: '',
  showCode: true,
})

const imageFailed = ref(false)
const normalizedCode = computed(() => props.code.trim())
const displayLabel = computed(() => props.label.trim() || normalizedCode.value || '-')

const platformIcons: Record<string, typeof googleIcon> = {
  amazon: amazonIcon,
  discord: discordIcon,
  github: githubIcon,
  google: googleIcon,
  microsoft: microsoftIcon,
  openai: openaiIcon,
  telegram: telegramIcon,
  whatsapp: whatsappIcon,
}

const platformIcon = computed(() => (
  props.kind === 'platform' ? platformIcons[normalizedCode.value.toLowerCase()] : undefined
))
const countryFlagClass = computed(() => (
  props.kind === 'country' && /^[a-z]{2}$/i.test(normalizedCode.value)
    ? `fi-${normalizedCode.value.toLowerCase()}`
    : ''
))

const imageUrl = computed(() => {
  if (platformIcon.value || countryFlagClass.value) return ''
  const customIcon = props.icon.trim()
  if (/^https?:\/\/cdn\.simpleicons\.org(?:\/|$)/i.test(customIcon)) return ''
  return /^(https?:\/\/|data:image\/|\/(?!\/))/i.test(customIcon) ? customIcon : ''
})

watch(imageUrl, () => { imageFailed.value = false })
</script>
