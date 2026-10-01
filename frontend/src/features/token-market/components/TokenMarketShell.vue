<template>
  <main class="tm-page" @click="emit('background-click')">
    <div class="tm-shell">
      <slot />
    </div>
  </main>
</template>

<script setup lang="ts">
import '../styles.css'

const emit = defineEmits<{
  'background-click': []
}>()
</script>
