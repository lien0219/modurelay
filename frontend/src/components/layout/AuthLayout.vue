<template>
  <div class="auth-layout">
    <div class="auth-background" aria-hidden="true"></div>

    <div class="auth-content">
      <div class="mb-8 text-center">
        <template v-if="settingsLoaded">
          <div class="auth-brand-mark mb-4">
            <img :src="siteLogo || brand.logo" :alt="brand.name" class="h-full w-full object-contain" />
          </div>
          <h1 class="mb-2 text-3xl font-bold text-[color:var(--color-text-primary)]">
            {{ siteName }}
          </h1>
          <p class="text-sm text-[color:var(--color-text-muted)]">
            {{ siteSubtitle }}
          </p>
        </template>
      </div>

      <div class="auth-card card-glass p-8">
        <slot />
      </div>

      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <div class="mt-8 text-center text-xs text-[color:var(--color-text-disabled)]">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { brand } from '@/config/brand'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || brand.slogan)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-layout {
  position: relative;
  display: flex;
  min-height: 100vh;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  padding: 16px;
  background: var(--color-bg);
}

.auth-background {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background: var(--color-bg);
}

.auth-content {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 448px;
}

.auth-brand-mark {
  display: inline-flex;
  width: 64px;
  height: 64px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--color-primary-border);
  border-radius: 14px;
  background: var(--color-primary);
  box-shadow: var(--shadow-md);
}

.auth-card {
  border-radius: 14px;
  box-shadow: var(--shadow-lg);
}
</style>
