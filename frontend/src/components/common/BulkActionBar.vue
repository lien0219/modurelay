<template>
  <AnimatePresence>
    <motion.div
      v-if="show"
      :key="barKey"
      class="bulk-action-bar glass-panel"
      role="region"
      :aria-label="ariaLabel || t('common.bulkActions')"
      :initial="motionInitial"
      :animate="motionAnimate"
      :exit="motionExit"
      :transition="motionTransition"
    >
      <div class="bulk-action-bar__meta">
        <slot name="meta">
          <span v-if="selectedCount > 0" class="bulk-action-bar__count">
            {{ t('common.selectedCount', { count: selectedCount }) }}
          </span>
        </slot>
      </div>
      <div class="bulk-action-bar__actions">
        <slot />
      </div>
    </motion.div>
  </AnimatePresence>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { AnimatePresence, motion } from 'motion-v'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'

interface Props {
  show?: boolean
  selectedCount?: number
  ariaLabel?: string
}

withDefaults(defineProps<Props>(), {
  show: true,
  selectedCount: 0
})

const { t } = useI18n()
const prefersReducedMotion = usePrefersReducedMotion()

const barKey = 'bulk-action-bar'

const motionInitial = computed(() =>
  prefersReducedMotion.value ? { opacity: 0 } : { opacity: 0, y: 6 }
)
const motionAnimate = computed(() =>
  prefersReducedMotion.value ? { opacity: 1 } : { opacity: 1, y: 0 }
)
const motionExit = computed(() =>
  prefersReducedMotion.value ? { opacity: 0 } : { opacity: 0, y: 6 }
)
const motionTransition = computed(() => ({
  duration: prefersReducedMotion.value ? 0.01 : 0.19,
  ease: 'easeOut'
}))
</script>

<style scoped>
.bulk-action-bar {
  @apply mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl px-3 py-2.5;
  border-color: rgba(20, 184, 166, 0.28);
}

.bulk-action-bar__meta {
  @apply flex min-w-0 flex-wrap items-center gap-2;
}

.bulk-action-bar__count {
  @apply text-sm font-medium text-primary-900 dark:text-primary-100;
}

.bulk-action-bar__actions {
  @apply flex flex-wrap items-center gap-2;
}
</style>
