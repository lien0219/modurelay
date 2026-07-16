<template>
  <div class="w-full">
    <label v-if="label" :for="resolvedId" class="input-label mb-1.5 block" :class="labelStateClass">
      {{ label }}
      <span v-if="required" class="text-red-500" aria-hidden="true">*</span>
    </label>
    <div class="relative">
      <div
        v-if="$slots.prefix"
        class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-gray-400 dark:text-dark-400"
      >
        <slot name="prefix"></slot>
      </div>

      <input
        :id="resolvedId"
        ref="inputRef"
        :type="resolvedType"
        :value="modelValue ?? ''"
        :disabled="disabled"
        :required="required"
        :placeholder="placeholderText"
        :autocomplete="autocomplete"
        :readonly="readonly"
        :aria-invalid="!!error || undefined"
        :aria-required="required || undefined"
        :aria-describedby="describedBy"
        :aria-busy="validating || undefined"
        :class="[
          'input w-full transition-all duration-200',
          $slots.prefix ? 'pl-11' : '',
          hasSuffix ? 'pr-11' : '',
          error ? 'input-error' : '',
          showSuccess && !error ? 'input-success' : '',
          disabled ? 'cursor-not-allowed bg-gray-100 opacity-60 dark:bg-dark-900' : ''
        ]"
        @input="onInput"
        @change="$emit('change', ($event.target as HTMLInputElement).value)"
        @blur="$emit('blur', $event)"
        @focus="$emit('focus', $event)"
        @keyup.enter="$emit('enter', $event)"
      />

      <div
        v-if="hasSuffix"
        class="absolute inset-y-0 right-0 flex items-center pr-2.5 text-gray-400 dark:text-dark-400"
      >
        <slot name="suffix">
          <span
            v-if="validating"
            class="mr-1.5 h-4 w-4 rounded-full border-2 border-primary-500 border-r-transparent"
            :class="prefersReducedMotion ? 'opacity-70' : 'animate-spin'"
            aria-hidden="true"
          />
          <button
            v-if="isPassword"
            type="button"
            class="password-toggle"
            :class="{ 'password-toggle--pressed': passwordPressed }"
            :aria-label="passwordVisible ? t('common.hidePassword') : t('common.showPassword')"
            :aria-pressed="passwordVisible"
            :disabled="disabled"
            @click="togglePassword"
            @mousedown="passwordPressed = true"
            @mouseup="passwordPressed = false"
            @mouseleave="passwordPressed = false"
            @blur="passwordPressed = false"
          >
            <Icon :name="passwordVisible ? 'eyeOff' : 'eye'" size="md" />
          </button>
        </slot>
      </div>
    </div>

    <ValidationMessage
      v-if="error"
      :id="errorId"
      :message="error"
      tone="error"
    />
    <ValidationMessage
      v-else-if="validating"
      :id="statusId"
      :message="validatingMessage || t('common.validating')"
      tone="validating"
    />
    <ValidationMessage
      v-else-if="showSuccess"
      :id="statusId"
      :message="successMessage || t('common.fieldValid')"
      tone="success"
    />
    <ValidationMessage
      v-else-if="hint"
      :id="hintId"
      :message="hint"
      tone="hint"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useId, useSlots } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ValidationMessage from './ValidationMessage.vue'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'

interface Props {
  modelValue: string | number | null | undefined
  type?: string
  label?: string
  placeholder?: string
  disabled?: boolean
  required?: boolean
  readonly?: boolean
  error?: string
  hint?: string
  /** Show success state only when validation already succeeded */
  success?: boolean | string
  validating?: boolean
  validatingMessage?: string
  id?: string
  autocomplete?: string
  /** Enable built-in password visibility toggle when type is password */
  passwordToggle?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  type: 'text',
  disabled: false,
  required: false,
  readonly: false,
  validating: false,
  success: false,
  passwordToggle: true
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'change', value: string): void
  (e: 'blur', event: FocusEvent): void
  (e: 'focus', event: FocusEvent): void
  (e: 'enter', event: KeyboardEvent): void
}>()

const { t } = useI18n()
const slots = useSlots()
const prefersReducedMotion = usePrefersReducedMotion()
const inputRef = ref<HTMLInputElement | null>(null)
const generatedId = useId()
const passwordVisible = ref(false)
const passwordPressed = ref(false)

const resolvedId = computed(() => props.id || generatedId)
const errorId = computed(() => `${resolvedId.value}-error`)
const hintId = computed(() => `${resolvedId.value}-hint`)
const statusId = computed(() => `${resolvedId.value}-status`)
const placeholderText = computed(() => props.placeholder || '')

const isPassword = computed(() => props.type === 'password' && props.passwordToggle)
const resolvedType = computed(() => {
  if (!isPassword.value) return props.type
  return passwordVisible.value ? 'text' : 'password'
})

const hasError = computed(() => !!props.error)
const showSuccess = computed(() => {
  if (hasError.value || props.validating) return false
  if (typeof props.success === 'string') return props.success.length > 0
  return !!props.success
})
const successMessage = computed(() =>
  typeof props.success === 'string' ? props.success : undefined
)

const hasSuffix = computed(
  () => !!slots.suffix || isPassword.value || props.validating
)

const describedBy = computed(() => {
  if (hasError.value) return errorId.value
  if (props.validating || showSuccess.value) return statusId.value
  if (props.hint) return hintId.value
  return undefined
})

const labelStateClass = computed(() => {
  if (hasError.value) return 'text-red-600 dark:text-red-400'
  if (showSuccess.value) return 'text-emerald-700 dark:text-emerald-400'
  return ''
})

const onInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', value)
}

const togglePassword = () => {
  if (props.disabled) return
  passwordVisible.value = !passwordVisible.value
}

defineExpose({
  focus: () => inputRef.value?.focus(),
  select: () => inputRef.value?.select()
})
</script>

<style scoped>
.password-toggle {
  @apply inline-flex h-8 w-8 items-center justify-center rounded-lg;
  @apply text-gray-400 transition-transform duration-150;
  @apply hover:text-gray-600 dark:hover:text-dark-300;
  @apply focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40;
  @apply disabled:cursor-not-allowed disabled:opacity-50;
}

.password-toggle--pressed:not(:disabled) {
  transform: scale(0.92);
}

@media (prefers-reduced-motion: reduce) {
  .password-toggle,
  .password-toggle--pressed:not(:disabled) {
    transition: none;
    transform: none;
  }
}
</style>
