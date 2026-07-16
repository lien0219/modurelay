<template>
  <label
    class="mr-checkbox"
    :class="{
      'mr-checkbox--disabled': disabled,
      'mr-checkbox--pressed': pressed && !disabled,
      'mr-checkbox--checked': modelValue
    }"
  >
    <input
      :id="id"
      ref="inputRef"
      type="checkbox"
      class="mr-checkbox__input"
      :checked="modelValue"
      :disabled="disabled"
      :aria-label="ariaLabel"
      :aria-describedby="ariaDescribedby"
      :aria-invalid="ariaInvalid || undefined"
      @change="onChange"
      @mousedown="pressed = true"
      @mouseup="pressed = false"
      @mouseleave="pressed = false"
      @keydown.space="pressed = true"
      @keyup.space="pressed = false"
      @blur="pressed = false"
    />
    <span class="mr-checkbox__box" aria-hidden="true">
      <Icon v-if="modelValue" name="check" size="xs" :stroke-width="2.5" class="mr-checkbox__icon" />
    </span>
    <span v-if="label || $slots.default" class="mr-checkbox__label">
      <slot>{{ label }}</slot>
    </span>
  </label>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from '@/components/icons/Icon.vue'

interface Props {
  modelValue?: boolean
  label?: string
  id?: string
  disabled?: boolean
  ariaLabel?: string
  ariaDescribedby?: string
  ariaInvalid?: boolean
}

withDefaults(defineProps<Props>(), {
  modelValue: false,
  disabled: false
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
  (e: 'change', value: boolean): void
}>()

const inputRef = ref<HTMLInputElement | null>(null)
const pressed = ref(false)

const onChange = (event: Event) => {
  const checked = (event.target as HTMLInputElement).checked
  emit('update:modelValue', checked)
  emit('change', checked)
}

defineExpose({
  focus: () => inputRef.value?.focus(),
  el: inputRef
})
</script>

<style scoped>
.mr-checkbox {
  @apply inline-flex cursor-pointer items-center gap-2 select-none;
}

.mr-checkbox--disabled {
  @apply cursor-not-allowed opacity-50;
}

.mr-checkbox__input {
  @apply sr-only;
}

.mr-checkbox__box {
  @apply relative flex h-4 w-4 flex-shrink-0 items-center justify-center rounded;
  @apply border border-gray-300 bg-white dark:border-dark-500 dark:bg-dark-800;
  transition:
    transform 120ms ease-out,
    background-color 150ms ease-out,
    border-color 150ms ease-out,
    box-shadow 150ms ease-out;
}

.mr-checkbox--checked .mr-checkbox__box {
  @apply border-primary-600 bg-primary-600 text-white dark:border-primary-500 dark:bg-primary-500;
}

.mr-checkbox__input:focus-visible + .mr-checkbox__box {
  @apply ring-2 ring-primary-500/50 ring-offset-2 dark:ring-offset-dark-900;
}

.mr-checkbox--pressed:not(.mr-checkbox--disabled) .mr-checkbox__box {
  transform: scale(0.92);
}

.mr-checkbox__icon {
  @apply text-white;
}

.mr-checkbox__label {
  @apply text-sm text-gray-700 dark:text-gray-300;
}

@media (prefers-reduced-motion: reduce) {
  .mr-checkbox__box {
    transition: none;
  }

  .mr-checkbox--pressed:not(.mr-checkbox--disabled) .mr-checkbox__box {
    transform: none;
  }
}
</style>
