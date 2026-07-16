<template>
  <div class="form-field w-full" :class="rootClass">
    <div class="mb-1.5 flex items-center justify-between gap-2">
      <label
        v-if="label"
        :for="fieldId"
        class="input-label mb-0"
        :class="{
          'text-red-600 dark:text-red-400': hasError,
          'text-emerald-700 dark:text-emerald-400': showSuccess && !hasError
        }"
      >
        {{ label }}
        <span v-if="required" class="text-red-500" aria-hidden="true">*</span>
      </label>
      <span v-if="optional && !required" class="text-xs text-gray-400 dark:text-dark-500">
        {{ t('common.optional') }}
      </span>
    </div>

    <slot
      :id="fieldId"
      :described-by="describedBy"
      :invalid="hasError"
      :success="showSuccess"
    />

    <ValidationMessage
      v-if="hasError"
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
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import ValidationMessage from './ValidationMessage.vue'

interface Props {
  label?: string
  error?: string
  hint?: string
  success?: boolean | string
  validating?: boolean
  validatingMessage?: string
  required?: boolean
  optional?: boolean
  id?: string
  rootClass?: string
}

const props = withDefaults(defineProps<Props>(), {
  required: false,
  optional: false,
  validating: false,
  success: false
})

const { t } = useI18n()
const generatedId = useId()

const fieldId = computed(() => props.id || generatedId)
const errorId = computed(() => `${fieldId.value}-error`)
const hintId = computed(() => `${fieldId.value}-hint`)
const statusId = computed(() => `${fieldId.value}-status`)

const hasError = computed(() => !!props.error)
const showSuccess = computed(() => {
  if (hasError.value || props.validating) return false
  if (typeof props.success === 'string') return props.success.length > 0
  return !!props.success
})
const successMessage = computed(() =>
  typeof props.success === 'string' ? props.success : undefined
)

const describedBy = computed(() => {
  if (hasError.value) return errorId.value
  if (props.validating || showSuccess.value) return statusId.value
  if (props.hint) return hintId.value
  return undefined
})
</script>
