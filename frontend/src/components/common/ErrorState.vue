<template>
  <div class="error-state" role="alert">
    <div class="error-state__icon" aria-hidden="true">
      <slot name="icon">
        <Icon name="exclamationTriangle" size="lg" />
      </slot>
    </div>
    <h3 class="error-state__title">{{ title || t('common.error') }}</h3>
    <p v-if="description" class="error-state__description">{{ description }}</p>
    <div v-if="retryable || $slots.action" class="mt-5">
      <slot name="action">
        <button
          v-if="retryable"
          type="button"
          class="btn btn-secondary btn-sm"
          :disabled="retrying"
          :class="{ 'btn-loading': retrying }"
          @click="$emit('retry')"
        >
          <Icon name="refresh" size="sm" class="mr-1.5" />
          {{ retryText || t('common.retry') }}
        </button>
      </slot>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

interface Props {
  title?: string
  description?: string
  retryable?: boolean
  retrying?: boolean
  retryText?: string
}

withDefaults(defineProps<Props>(), {
  retryable: true,
  retrying: false
})

defineEmits<{
  (e: 'retry'): void
}>()

const { t } = useI18n()
</script>

<style scoped>
.error-state {
  @apply flex flex-col items-center justify-center px-4 py-10 text-center;
}

.error-state__icon {
  @apply mb-4 flex h-14 w-14 items-center justify-center rounded-2xl;
  @apply bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400;
  @apply border border-amber-200/80 dark:border-amber-800/60;
}

.error-state__title {
  @apply text-base font-semibold text-gray-900 dark:text-white;
}

.error-state__description {
  @apply mt-1.5 max-w-sm text-sm text-gray-600 dark:text-dark-300;
}
</style>
