<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed right-4 top-4 z-[9999] flex flex-col gap-3"
      aria-live="polite"
      aria-atomic="true"
    >
      <AnimatePresence>
        <motion.div
          v-for="toast in toasts"
          :key="toast.id"
          class="pointer-events-auto min-w-[320px] max-w-md overflow-hidden rounded-xl border-l-4 glass-strong"
          :class="getBorderColor(toast.type)"
          :initial="toastInitial"
          :animate="toastAnimate"
          :exit="toastExit"
          :transition="toastTransition"
          @mouseenter="pauseToast(toast.id)"
          @mouseleave="resumeToast(toast.id)"
        >
          <div class="p-4">
            <div class="flex items-start gap-3">
              <!-- Icon -->
              <div class="mt-0.5 flex-shrink-0">
                <Icon
                  :name="getToastIconName(toast.type)"
                  size="md"
                  :class="getIconColor(toast.type)"
                  aria-hidden="true"
                />
              </div>

              <!-- Content -->
              <div class="min-w-0 flex-1">
                <p v-if="toast.title" class="text-sm font-semibold text-gray-900 dark:text-white">
                  {{ toast.title }}
                </p>
                <p
                  :class="[
                    'text-sm leading-relaxed',
                    toast.title
                      ? 'mt-1 text-gray-600 dark:text-gray-300'
                      : 'text-gray-900 dark:text-white'
                  ]"
                >
                  {{ toast.message }}
                </p>
              </div>

              <!-- Close button -->
              <button
                @click="removeToast(toast.id)"
                class="-m-1 flex-shrink-0 rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:text-gray-500 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                aria-label="Close notification"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>

          <!-- Progress bar -->
          <div v-if="toast.duration" class="h-1 bg-gray-100/80 dark:bg-dark-700/80">
            <div
              :class="[
                'h-full toast-progress',
                getProgressBarColor(toast.type),
                pausedIds.has(toast.id) && 'toast-progress-paused'
              ]"
              :style="progressStyle(toast)"
            ></div>
          </div>
        </motion.div>
      </AnimatePresence>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, reactive } from 'vue'
import { AnimatePresence, motion } from 'motion-v'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'
import type { Toast } from '@/types'

const appStore = useAppStore()
const prefersReducedMotion = usePrefersReducedMotion()
const pausedIds = reactive(new Set<string>())

const toasts = computed(() => appStore.toasts)

const toastInitial = computed(() =>
  prefersReducedMotion.value
    ? { opacity: 0 }
    : { opacity: 0, x: 24 }
)
const toastAnimate = computed(() =>
  prefersReducedMotion.value
    ? { opacity: 1 }
    : { opacity: 1, x: 0 }
)
const toastExit = computed(() =>
  prefersReducedMotion.value
    ? { opacity: 0 }
    : { opacity: 0, x: 16 }
)
const toastTransition = computed(() => ({
  duration: prefersReducedMotion.value ? 0.01 : 0.2,
  ease: 'easeOut'
}))

const getToastIconName = (type: string): 'checkCircle' | 'xCircle' | 'exclamationTriangle' | 'infoCircle' => {
  switch (type) {
    case 'success':
      return 'checkCircle'
    case 'error':
      return 'xCircle'
    case 'warning':
      return 'exclamationTriangle'
    case 'info':
    default:
      return 'infoCircle'
  }
}

const getIconColor = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'text-emerald-500',
    error: 'text-red-500',
    warning: 'text-amber-500',
    info: 'text-primary-500'
  }
  return colors[type] || colors.info
}

const getBorderColor = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'border-emerald-500',
    error: 'border-red-500',
    warning: 'border-amber-500',
    info: 'border-primary-500'
  }
  return colors[type] || colors.info
}

const getProgressBarColor = (type: string): string => {
  const colors: Record<string, string> = {
    success: 'bg-emerald-500',
    error: 'bg-red-500',
    warning: 'bg-amber-500',
    info: 'bg-primary-500'
  }
  return colors[type] || colors.info
}

const progressStyle = (toast: Toast) => {
  if (!toast.duration) return undefined
  return {
    animationDuration: `${toast.duration}ms`,
    animationPlayState: pausedIds.has(toast.id) ? 'paused' : 'running'
  }
}

const pauseToast = (id: string) => {
  pausedIds.add(id)
  appStore.pauseToast(id)
}

const resumeToast = (id: string) => {
  pausedIds.delete(id)
  appStore.resumeToast(id)
}

const removeToast = (id: string) => {
  pausedIds.delete(id)
  appStore.hideToast(id)
}
</script>

<style scoped>
.toast-progress {
  width: 100%;
  animation-name: toast-progress-shrink;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

.toast-progress-paused {
  animation-play-state: paused;
}

@keyframes toast-progress-shrink {
  from {
    width: 100%;
  }
  to {
    width: 0%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .toast-progress {
    animation: none;
    width: 0%;
  }
}
</style>
