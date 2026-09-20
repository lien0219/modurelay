<template>
  <div class="resource-editor" :class="{ 'is-disabled': disabled }">
    <div class="resource-editor__toolbar">
      <div class="resource-editor__tools">
        <button
          v-for="tool in tools"
          :key="tool.command"
          type="button"
          :title="tool.label"
          :aria-label="tool.label"
          :disabled="disabled"
          @mousedown.prevent="applyCommand(tool.command, tool.value)"
        >
          <span v-if="tool.symbol" :class="{ 'is-italic': tool.command === 'italic' }">{{ tool.symbol }}</span>
          <Icon v-else :name="tool.icon || 'link'" size="sm" />
        </button>
      </div>
      <span class="resource-editor__count">{{ textLength }}/{{ maxLength }}</span>
    </div>

    <div
      ref="editor"
      class="resource-editor__surface"
      role="textbox"
      aria-multiline="true"
      :aria-label="placeholder"
      :data-placeholder="placeholder"
      :contenteditable="!disabled"
      @input="onInput"
      @keydown="onKeydown"
    />
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

onMounted(() => {
  void render(props.modelValue)
})
</script>

<style scoped>
.resource-editor {
  overflow: hidden;
  border: 1px solid var(--color-border-strong);
  border-radius: 12px;
  background: var(--color-surface);
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

.resource-editor:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.resource-editor.is-disabled {
  opacity: .6;
}

.resource-editor__toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 40px;
  padding: 4px 6px;
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}

.resource-editor__tools {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}

.resource-editor__tools button {
  display: grid;
  min-width: 30px;
  height: 30px;
  place-items: center;
  padding: 0 7px;
  border: 0;
  border-radius: 7px;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .76rem;
  font-weight: 650;
}

.resource-editor__tools button:hover:not(:disabled) {
  background: var(--color-surface);
  color: var(--color-primary);
}

.resource-editor__tools button:disabled {
  cursor: not-allowed;
}

.is-italic {
  font-style: italic;
}

.resource-editor__count {
  flex: 0 0 auto;
  padding: 0 5px;
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .62rem;
}

.resource-editor__surface {
  min-height: 150px;
  max-height: 340px;
  overflow-y: auto;
  padding: 12px 13px;
  outline: none;
  color: var(--color-text-primary);
  font-size: .78rem;
  line-height: 1.65;
}

.resource-editor__surface:empty::before {
  color: var(--color-text-muted);
  content: attr(data-placeholder);
  pointer-events: none;
}

.resource-editor__tools button:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 1px;
}
</style>
