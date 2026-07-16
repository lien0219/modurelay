<template>
  <div
    class="loading-state"
    role="status"
    :aria-label="ariaLabel || t('common.loading')"
    :aria-busy="true"
  >
    <span class="sr-only">{{ ariaLabel || t('common.loading') }}</span>
    <slot>
      <div v-if="variant === 'spinner'" class="flex flex-col items-center justify-center gap-3 py-10">
        <LoadingSpinner :size="spinnerSize" />
        <p v-if="message" class="text-sm text-gray-500 dark:text-dark-400">{{ message }}</p>
      </div>
      <div v-else class="loading-state__skeleton space-y-3" :aria-hidden="true">
        <Skeleton
          v-for="i in rows"
          :key="i"
          variant="rect"
          :height="rowHeight"
          class="loading-state__row"
        />
      </div>
    </slot>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from './LoadingSpinner.vue'
import Skeleton from './Skeleton.vue'

interface Props {
  variant?: 'skeleton' | 'spinner'
  rows?: number
  rowHeight?: number
  spinnerSize?: 'sm' | 'md' | 'lg' | 'xl'
  message?: string
  ariaLabel?: string
}

withDefaults(defineProps<Props>(), {
  variant: 'skeleton',
  rows: 5,
  rowHeight: 40,
  spinnerSize: 'md'
})

const { t } = useI18n()
</script>

<style scoped>
.loading-state {
  @apply w-full;
}

.loading-state__row {
  /* Prefer pulse over continuous shimmer to reduce paint cost */
  animation-duration: 1.4s;
}

@media (prefers-reduced-motion: reduce) {
  .loading-state__row {
    animation: none !important;
    opacity: 0.7;
  }
}
</style>
