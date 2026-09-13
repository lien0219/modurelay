<template>
  <main class="canvas-app-redirect" aria-labelledby="canvas-app-redirect-title">
    <LoadingSpinner size="lg" />
    <div class="canvas-app-redirect__copy" aria-live="polite">
      <h1 id="canvas-app-redirect-title">{{ t('canvas.launchingOriginal') }}</h1>
      <p>{{ t('canvas.launchingOriginalDescription') }}</p>
    </div>
    <a class="canvas-app-redirect__link" :href="infiniteCanvasUrl">
      {{ t('canvas.openOriginal') }}
    </a>
  </main>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'

const { t } = useI18n()
const infiniteCanvasUrl = '/infinite-canvas/canvas'

onMounted(() => {
  window.location.replace(infiniteCanvasUrl)
})
</script>

<style scoped>
.canvas-app-redirect {
  display: grid;
  min-height: 100dvh;
  place-content: center;
  justify-items: center;
  gap: 20px;
  padding: 24px;
  color: var(--color-text-primary);
  background: var(--color-bg);
  text-align: center;
}

.canvas-app-redirect__copy {
  display: grid;
  max-width: 520px;
  gap: 8px;
}

.canvas-app-redirect h1 {
  margin: 0;
  font-size: 24px;
  font-weight: 650;
  line-height: 1.25;
}

.canvas-app-redirect p {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: 14px;
  line-height: 1.6;
}

.canvas-app-redirect__link {
  min-height: 40px;
  padding: 9px 16px;
  border-radius: 8px;
  color: #fff;
  background: var(--color-primary);
  font-size: 14px;
  font-weight: 600;
  text-decoration: none;
  transition: background-color var(--motion-fast) var(--ease-standard), box-shadow var(--motion-fast) var(--ease-standard);
}

.canvas-app-redirect__link:hover {
  background: var(--color-primary-hover);
}

.canvas-app-redirect__link:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 3px;
}

@media (prefers-reduced-motion: reduce) {
  .canvas-app-redirect__link {
    transition-duration: 0.01ms;
  }
}
</style>
