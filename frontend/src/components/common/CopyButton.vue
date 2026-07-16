<template>
  <button
    type="button"
    class="copy-btn inline-flex items-center gap-1 rounded-lg p-1 transition-colors"
    :class="[
      copied
        ? 'text-emerald-500'
        : 'text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300',
      pressed && !prefersReducedMotion && 'copy-btn-pressed'
    ]"
    :title="copied ? copiedLabel : copyLabel"
    :aria-label="copied ? copiedLabel : copyLabel"
    @click="handleCopy"
    @mousedown="pressed = true"
    @mouseup="pressed = false"
    @mouseleave="pressed = false"
  >
    <AnimatePresence mode="wait">
      <motion.span
        :key="copied ? 'copied' : 'copy'"
        class="inline-flex items-center gap-1"
        :initial="iconInitial"
        :animate="iconAnimate"
        :exit="iconExit"
        :transition="iconTransition"
      >
        <Icon :name="copied ? 'check' : 'clipboard'" size="sm" :stroke-width="copied ? 2 : undefined" />
        <span v-if="showLabel || copied" class="text-xs font-medium whitespace-nowrap">
          {{ copied ? copiedLabel : copyLabel }}
        </span>
      </motion.span>
    </AnimatePresence>
  </button>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { AnimatePresence, motion } from 'motion-v'
import Icon from '@/components/icons/Icon.vue'
import { useClipboard } from '@/composables/useClipboard'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'

interface Props {
  text: string
  showLabel?: boolean
  successMessage?: string
  resetMs?: number
}

const props = withDefaults(defineProps<Props>(), {
  showLabel: false,
  resetMs: 1500
})

const emit = defineEmits<{
  (e: 'copied', success: boolean): void
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const prefersReducedMotion = usePrefersReducedMotion()

const copied = ref(false)
const pressed = ref(false)
let resetTimer: ReturnType<typeof setTimeout> | null = null

const copyLabel = computed(() => t('keys.copyToClipboard'))
const copiedLabel = computed(() => t('common.copied'))

const iconInitial = computed(() =>
  prefersReducedMotion.value ? { opacity: 0 } : { opacity: 0, scale: 0.9 }
)
const iconAnimate = computed(() =>
  prefersReducedMotion.value ? { opacity: 1 } : { opacity: 1, scale: 1 }
)
const iconExit = computed(() =>
  prefersReducedMotion.value ? { opacity: 0 } : { opacity: 0, scale: 0.9 }
)
const iconTransition = computed(() => ({
  duration: prefersReducedMotion.value ? 0.01 : 0.15,
  ease: 'easeOut'
}))

function clearResetTimer() {
  if (resetTimer != null) {
    clearTimeout(resetTimer)
    resetTimer = null
  }
}

async function handleCopy() {
  const success = await copyToClipboard(props.text, props.successMessage ?? t('common.copied'))
  emit('copied', success)
  if (!success) return

  clearResetTimer()
  copied.value = true
  resetTimer = setTimeout(() => {
    copied.value = false
    resetTimer = null
  }, props.resetMs)
}

onBeforeUnmount(() => {
  clearResetTimer()
})
</script>

<style scoped>
.copy-btn {
  transition: transform 140ms ease-out, color 140ms ease-out, background-color 140ms ease-out;
}

.copy-btn:active,
.copy-btn-pressed {
  transform: scale(0.96);
}

@media (prefers-reduced-motion: reduce) {
  .copy-btn:active,
  .copy-btn-pressed {
    transform: none;
  }
}
</style>
