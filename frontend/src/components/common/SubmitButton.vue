<template>
  <button
    :type="type"
    class="btn"
    :class="[
      variantClass,
      sizeClass,
      status === 'loading' && 'btn-loading',
      status === 'success' && 'btn-success-flash',
      status === 'error' && 'btn-error-flash',
      fullWidth && 'w-full'
    ]"
    :disabled="disabled || status === 'loading' || status === 'success'"
    :aria-busy="status === 'loading' || undefined"
    @click="onClick"
  >
    <slot>
      <Icon v-if="status === 'success'" name="check" size="sm" class="mr-1.5" />
      <Icon v-else-if="status === 'error'" name="exclamationCircle" size="sm" class="mr-1.5" />
      {{ labelText }}
    </slot>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

type SubmitStatus = 'idle' | 'loading' | 'success' | 'error'
type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'success'
type ButtonSize = 'sm' | 'md' | 'lg'

interface Props {
  status?: SubmitStatus
  label?: string
  loadingLabel?: string
  successLabel?: string
  errorLabel?: string
  variant?: ButtonVariant
  size?: ButtonSize
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  fullWidth?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  status: 'idle',
  variant: 'primary',
  size: 'md',
  type: 'submit',
  disabled: false,
  fullWidth: false
})

const emit = defineEmits<{
  (e: 'click', event: MouseEvent): void
}>()

const { t } = useI18n()

const variantClass = computed(() => {
  const map: Record<ButtonVariant, string> = {
    primary: 'btn-primary',
    secondary: 'btn-secondary',
    danger: 'btn-danger',
    success: 'btn-success'
  }
  return map[props.variant]
})

const sizeClass = computed(() => {
  if (props.size === 'sm') return 'btn-sm'
  if (props.size === 'lg') return 'btn-lg'
  return 'btn-md'
})

const labelText = computed(() => {
  if (props.status === 'loading') return props.loadingLabel || t('common.saving')
  if (props.status === 'success') return props.successLabel || t('common.saved')
  if (props.status === 'error') return props.errorLabel || t('common.error')
  return props.label || t('common.save')
})

const onClick = (event: MouseEvent) => {
  if (props.disabled || props.status === 'loading' || props.status === 'success') {
    event.preventDefault()
    return
  }
  emit('click', event)
}
</script>
