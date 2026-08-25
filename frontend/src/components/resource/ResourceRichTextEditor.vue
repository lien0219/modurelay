<template>
  <div class="resource-editor overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-600 dark:bg-dark-800" :class="disabled ? 'opacity-60' : ''">
    <div class="flex flex-wrap items-center gap-1 border-b border-gray-200 bg-gray-50 px-2 py-1.5 dark:border-dark-700 dark:bg-dark-900/60">
      <button v-for="tool in tools" :key="tool.command" type="button" class="inline-flex h-8 min-w-8 items-center justify-center rounded-lg px-2 text-sm font-semibold text-gray-600 transition hover:bg-white hover:text-primary-700 dark:text-dark-300 dark:hover:bg-dark-700 dark:hover:text-primary-300" :title="tool.label" :aria-label="tool.label" :disabled="disabled" @mousedown.prevent="applyCommand(tool.command, tool.value)">
        <span v-if="tool.symbol" :class="tool.command === 'italic' ? 'italic' : ''">{{ tool.symbol }}</span>
        <Icon v-else :name="tool.icon || 'link'" size="sm" />
      </button>
      <span class="ml-auto px-2 text-xs tabular-nums text-gray-400 dark:text-dark-500">{{ textLength }}/{{ maxLength }}</span>
    </div>
    <div ref="editor" class="resource-editor__surface min-h-[140px] max-h-[320px] overflow-y-auto px-3 py-3 text-sm leading-6 text-gray-800 outline-none empty:before:text-gray-400 empty:before:content-[attr(data-placeholder)] dark:text-dark-100 dark:empty:before:text-dark-500" role="textbox" aria-multiline="true" :aria-label="placeholder" :data-placeholder="placeholder" :contenteditable="!disabled" @input="onInput" @keydown="onKeydown" />
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, ref, watch } from 'vue'
import DOMPurify from 'dompurify'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  maxLength?: number
  disabled?: boolean
}>(), {
  placeholder: '',
  maxLength: 5000,
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
  'update:textLength': [value: number]
}>()

const editor = ref<HTMLElement | null>(null)
const textLength = ref(0)
let lastValidHTML = ''

const tools = [
  { command: 'bold', symbol: 'B', label: 'Bold' },
  { command: 'italic', symbol: 'I', label: 'Italic' },
  { command: 'underline', symbol: 'U', label: 'Underline' },
  { command: 'insertUnorderedList', symbol: '•', label: 'Bulleted list' },
  { command: 'formatBlock', value: 'blockquote', symbol: '“', label: 'Quote' },
  { command: 'createLink', icon: 'link' as const, label: 'Insert link' },
]

function sanitize(value: string): string {
  return DOMPurify.sanitize(value, {
    ALLOWED_TAGS: ['p', 'br', 'strong', 'b', 'em', 'i', 'u', 's', 'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'a'],
    ALLOWED_ATTR: ['href', 'target', 'rel'],
    FORBID_ATTR: ['style', 'class', 'id'],
    ALLOW_DATA_ATTR: false,
  })
}

function plainText(value: string): string {
  const node = document.createElement('div')
  node.innerHTML = value
  return (node.textContent || '').replace(/\u00a0/g, ' ').trim()
}

function sync(value: string): void {
  const clean = sanitize(value)
  const length = plainText(clean).length
  textLength.value = length
  emit('update:textLength', length)
  if (length > props.maxLength) {
    if (editor.value) editor.value.innerHTML = lastValidHTML
    textLength.value = plainText(lastValidHTML).length
    emit('update:textLength', textLength.value)
    return
  }
  lastValidHTML = clean
  emit('update:modelValue', clean)
}

function onInput(): void {
  if (editor.value) sync(editor.value.innerHTML)
}

function applyCommand(command: string, value?: string): void {
  if (props.disabled || !editor.value) return
  editor.value.focus()
  if (command === 'createLink') {
    const url = window.prompt('URL')?.trim()
    if (!url) return
    document.execCommand('createLink', false, url)
  } else {
    document.execCommand(command, false, value)
  }
  onInput()
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    editor.value?.blur()
  }
}

async function render(value: string): Promise<void> {
  lastValidHTML = sanitize(value)
  textLength.value = plainText(lastValidHTML).length
  emit('update:textLength', textLength.value)
  await nextTick()
  if (editor.value && editor.value.innerHTML !== lastValidHTML) editor.value.innerHTML = lastValidHTML
}

watch(() => props.modelValue, value => {
  if (editor.value && sanitize(editor.value.innerHTML) !== sanitize(value)) void render(value)
})

onMounted(() => { void render(props.modelValue) })
</script>
