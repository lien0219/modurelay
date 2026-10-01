<template>
  <span
    class="copyable-identifier"
    :style="{ maxWidth }"
    :title="displayValue"
  >
    <code class="copyable-identifier__value">{{ displayValue }}</code>
    <CopyButton
      v-if="copyValue"
      class="copyable-identifier__copy"
      :text="copyValue"
    />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import CopyButton from '@/components/common/CopyButton.vue'

interface Props {
  value?: string | number | null
  prefix?: string
  maxWidth?: string
  emptyText?: string
}

const props = withDefaults(defineProps<Props>(), {
  prefix: '',
  maxWidth: '18rem',
  emptyText: '-',
})

const copyValue = computed(() => {
  if (props.value == null) return ''
  return String(props.value).trim()
})

const displayValue = computed(() => copyValue.value ? `${props.prefix}${copyValue.value}` : props.emptyText)
</script>

<style scoped>
.copyable-identifier {
  display: inline-flex;
  min-width: 0;
  max-width: 100%;
  align-items: center;
  gap: 0.25rem;
  vertical-align: middle;
}

.copyable-identifier__value {
  display: block;
  min-width: 0;
  flex: 1 1 auto;
  overflow: hidden;
  color: inherit;
  font: inherit;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.copyable-identifier__copy {
  flex: 0 0 auto;
}
</style>
