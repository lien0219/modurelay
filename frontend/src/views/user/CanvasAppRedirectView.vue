<template>
  <main class="canvas-app-handoff" role="status" aria-live="polite">
    <span class="sr-only">{{ t('canvas.launchingOriginal') }}</span>
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { navigateToWorkspaceUrlWithTransition } from '@/utils/workspaceModeTransition'

const { t } = useI18n()
const infiniteCanvasUrl = '/infinite-canvas/canvas'

onMounted(() => {
  void navigateToWorkspaceUrlWithTransition(infiniteCanvasUrl, 'to-canvas', { replace: true })
})
</script>

<style scoped>
.canvas-app-handoff {
  min-height: 100dvh;
  background: var(--color-bg);
}
</style>
