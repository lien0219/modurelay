<template>
  <button
    type="button"
    @click="toggle"
    class="relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60 dark:focus-visible:ring-offset-dark-800"
    :class="[modelValue ? 'bg-primary-600' : 'bg-gray-200 dark:bg-dark-600']"
    :disabled="disabled"
    role="switch"
    :aria-checked="modelValue"
    :aria-label="label"
  >
    <span
      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition-transform duration-200 ease-in-out"
      :class="[modelValue ? 'translate-x-5' : 'translate-x-0']"
    />
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  disabled?: boolean
  label?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>
