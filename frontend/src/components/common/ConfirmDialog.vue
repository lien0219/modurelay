<template>
  <BaseDialog
    :show="show"
    :title="title"
    width="narrow"
    :close-on-escape="!confirming"
    :close-on-click-outside="false"
    :show-close-button="!confirming"
    @close="handleCancel"
  >
    <div class="space-y-4">
      <div
        v-if="danger"
        class="inline-flex items-center gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs font-medium text-red-700 dark:border-red-900/50 dark:bg-red-950/40 dark:text-red-300"
      >
        <Icon name="exclamationTriangle" size="sm" aria-hidden="true" />
        <span>{{ t('common.destructiveAction') }}</span>
      </div>
      <p class="text-sm text-gray-600 dark:text-gray-400">{{ message }}</p>
      <slot></slot>
    </div>

    <template #footer>
      <div class="flex justify-end space-x-3">
        <button
          ref="cancelButtonRef"
          type="button"
          class="btn btn-secondary"
          :disabled="confirming"
          @click="handleCancel"
        >
          {{ cancelText }}
        </button>
        <button
          type="button"
          class="btn"
          :class="[
            danger ? 'btn-danger' : 'btn-primary',
            (confirming || localLocked) && 'btn-loading'
          ]"
          :disabled="confirming || localLocked"
          :aria-busy="confirming || localLocked || undefined"
          @click="handleConfirm"
        >
          {{ confirming || localLocked ? t('common.processing') : confirmText }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch, onBeforeUnmount } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

interface Props {
  show: boolean
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  /** When true, blocks Escape/close and disables confirm to prevent double submit */
  confirming?: boolean
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

const props = withDefaults(defineProps<Props>(), {
  danger: false,
  confirming: false
})

const { t } = useI18n()

const confirmText = computed(() => props.confirmText || t('common.confirm'))
const cancelText = computed(() => props.cancelText || t('common.cancel'))

const emit = defineEmits<Emits>()
const cancelButtonRef = ref<HTMLButtonElement | null>(null)
const localLocked = ref(false)
let unlockTimer: ReturnType<typeof setTimeout> | null = null

const clearUnlockTimer = () => {
  if (unlockTimer != null) {
    clearTimeout(unlockTimer)
    unlockTimer = null
  }
}

const handleConfirm = () => {
  if (props.confirming || localLocked.value) return
  localLocked.value = true
  // If parent drives `confirming`, unlock when that flag clears.
  // Otherwise briefly block double-clicks, then allow retry (e.g. failed ops).
  if (!props.confirming) {
    clearUnlockTimer()
    unlockTimer = setTimeout(() => {
      localLocked.value = false
      unlockTimer = null
    }, 450)
  }
  emit('confirm')
}

const handleCancel = () => {
  if (props.confirming) return
  clearUnlockTimer()
  localLocked.value = false
  emit('cancel')
}

watch(
  () => props.show,
  async (open) => {
    if (!open) {
      clearUnlockTimer()
      localLocked.value = false
      return
    }
    // Safety: land on Cancel (esp. danger), after BaseDialog's default autofocus.
    await nextTick()
    requestAnimationFrame(() => {
      cancelButtonRef.value?.focus()
    })
  }
)

watch(
  () => props.confirming,
  (value) => {
    if (!value) {
      clearUnlockTimer()
      localLocked.value = false
    }
  }
)

onBeforeUnmount(() => {
  clearUnlockTimer()
})
</script>
