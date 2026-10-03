<template>
  <span class="block min-w-0 [overflow-wrap:anywhere]">
    <span v-if="hasDisplayName" class="block font-medium text-[var(--color-text-primary)]">{{ name?.trim() }}</span>
    <span
      class="block font-mono"
      :class="hasDisplayName ? 'mt-0.5 text-xs text-[var(--color-text-secondary)]' : 'text-[var(--color-text-primary)]'"
    >{{ id }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  id: string
  name?: string
}>()

const hasDisplayName = computed(() => Boolean(props.name?.trim() && props.name.trim() !== props.id))
</script>
