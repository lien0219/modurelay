<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed right-4 top-20 z-[9999] flex flex-col gap-3"
      aria-live="polite"
      aria-atomic="true"
    >
      <AnimatePresence>
        <motion.div
          v-for="toast in toasts"
          :key="toast.id"
          class="toast-item pointer-events-auto"
          :class="getToneClass(toast.type)"
          :initial="toastInitial"
          :animate="toastAnimate"
          :exit="toastExit"
          :transition="toastTransition"
          @mouseenter="pauseToast(toast.id)"
          @mouseleave="resumeToast(toast.id)"
        >
          <div class="toast-status-line" aria-hidden="true"></div>
          <div class="p-4">
            <div class="flex items-start gap-3">
              <!-- Icon -->
              <div class="toast-icon mt-0.5 flex-shrink-0">
                <Icon
                  :name="getToastIconName(toast.type)"
                  size="md"
                  aria-hidden="true"
                />
              </div>

              <!-- Content -->
              <div class="min-w-0 flex-1">
                <p v-if="toast.title" class="toast-title text-sm font-semibold">
                  {{ toast.title }}
                </p>
                <p
                  :class="[
                    'toast-message text-sm leading-relaxed',
                    toast.title ? 'mt-1' : 'toast-message-standalone'
                  ]"
                >
                  {{ toast.message }}
                </p>
              </div>

              <!-- Close button -->
              <button
                @click="removeToast(toast.id)"
                class="toast-close -m-1 flex-shrink-0 rounded-lg p-1"
                aria-label="Close notification"
              >
                <Icon name="x" size="sm" />
              </button>
            </div>
          </div>

          <!-- Progress bar -->
          <div v-if="toast.duration" class="toast-progress-track">
            <div
              :class="[
                'h-full toast-progress',
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

const getToneClass = (type: string): string => {
  const tones: Record<string, string> = {
    success: 'toast-tone-success',
    error: 'toast-tone-error',
    warning: 'toast-tone-warning',
    info: 'toast-tone-info'
  }
  return tones[type] || tones.info
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
.toast-item {
  --toast-tone: var(--color-info);
  position: relative;
  min-width: 320px;
  max-width: min(28rem, calc(100vw - 2rem));
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  color: var(--color-text-primary);
  background-color: var(--color-surface-overlay);
  box-shadow: var(--shadow-overlay), inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(var(--glass-blur-strong)) saturate(var(--glass-saturate));
  backdrop-filter: blur(var(--glass-blur-strong)) saturate(var(--glass-saturate));
}

.toast-tone-success {
  --toast-tone: var(--color-success);
}

.toast-tone-error {
  --toast-tone: var(--color-danger);
}

.toast-tone-warning {
  --toast-tone: var(--color-warning);
}

.toast-status-line {
  height: 3px;
  background-color: var(--toast-tone);
}

.toast-icon {
  display: flex;
  width: 2rem;
  height: 2rem;
  align-items: center;
  justify-content: center;
  border: 1px solid color-mix(in srgb, var(--toast-tone) 34%, transparent);
  border-radius: 9px;
  color: var(--toast-tone);
  background-color: color-mix(in srgb, var(--toast-tone) 12%, transparent);
}

.toast-title {
  color: var(--color-text-primary);
}

.toast-message {
  overflow-wrap: anywhere;
  color: var(--color-text-secondary);
}

.toast-message.toast-message-standalone {
  color: var(--color-text-primary);
}

.toast-close {
  color: var(--color-text-muted);
  transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard);
}

.toast-close:hover {
  color: var(--color-text-primary);
  background-color: var(--color-surface-soft);
}

.toast-progress-track {
  height: 2px;
  background-color: var(--color-border-subtle);
}

.toast-progress {
  width: 100%;
  background-color: var(--toast-tone);
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

@media (max-width: 420px) {
  .toast-item {
    min-width: 0;
    width: calc(100vw - 2rem);
  }
}
</style>
