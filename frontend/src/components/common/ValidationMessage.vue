<template>
  <AnimatePresence>
    <motion.p
      v-if="visible && message"
      :key="`${tone}-${message}`"
      :id="id"
      role="alert"
      :aria-live="tone === 'error' ? 'assertive' : 'polite'"
      :class="[
        'validation-message',
        tone === 'error' && 'validation-message--error',
        tone === 'success' && 'validation-message--success',
        tone === 'hint' && 'validation-message--hint',
        tone === 'validating' && 'validation-message--validating'
      ]"
      :initial="motionInitial"
      :animate="motionAnimate"
      :exit="motionExit"
      :transition="motionTransition"
    >
      <span
        v-if="tone === 'validating'"
        class="validation-message__spinner"
        aria-hidden="true"
      />
      <Icon
        v-else-if="tone === 'success'"
        name="checkCircle"
        size="xs"
        class="flex-shrink-0"
        aria-hidden="true"
      />
      <Icon
        v-else-if="tone === 'error'"
        name="exclamationCircle"
        size="xs"
        class="flex-shrink-0"
        aria-hidden="true"
      />
      <span>{{ message }}</span>
    </motion.p>
  </AnimatePresence>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { AnimatePresence, motion } from 'motion-v'
import Icon from '@/components/icons/Icon.vue'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'

type ValidationTone = 'error' | 'success' | 'hint' | 'validating'

interface Props {
  message?: string
  tone?: ValidationTone
  id?: string
  show?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  tone: 'error',
  show: true
})

const prefersReducedMotion = usePrefersReducedMotion()

const visible = computed(() => props.show !== false && !!props.message)

const motionInitial = computed(() =>
  prefersReducedMotion.value ? { opacity: 0 } : { opacity: 0, y: -2 }
)
const motionAnimate = computed(() =>
  prefersReducedMotion.value ? { opacity: 1 } : { opacity: 1, y: 0 }
)
const motionExit = computed(() =>
  prefersReducedMotion.value ? { opacity: 0 } : { opacity: 0, y: -2 }
)
const motionTransition = computed(() => ({
  duration: prefersReducedMotion.value ? 0.01 : 0.15,
  ease: 'easeOut'
}))
</script>

<style scoped>
.validation-message {
  @apply mt-1.5 flex items-center gap-1.5 text-xs;
}

.validation-message--error {
  @apply text-red-600 dark:text-red-400;
}

.validation-message--success {
  @apply text-emerald-600 dark:text-emerald-400;
}

.validation-message--hint {
  @apply text-gray-500 dark:text-dark-400;
}

.validation-message--validating {
  @apply text-primary-600 dark:text-primary-400;
}

.validation-message__spinner {
  @apply h-3 w-3 flex-shrink-0 rounded-full border-2 border-current border-r-transparent;
  animation: validation-spin 0.7s linear infinite;
}

@keyframes validation-spin {
  to {
    transform: rotate(360deg);
  }
}

@media (prefers-reduced-motion: reduce) {
  .validation-message__spinner {
    animation: none;
    border-right-color: currentColor;
    opacity: 0.7;
  }
}
</style>
