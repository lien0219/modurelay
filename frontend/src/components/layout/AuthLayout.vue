<template>
  <div class="auth-layout">
    <button
      type="button"
      class="auth-theme-toggle"
      :aria-label="isDark ? 'Switch to light mode' : 'Switch to dark mode'"
      :aria-pressed="isDark"
      :title="isDark ? 'Light mode' : 'Dark mode'"
      @click="toggleTheme"
    >
      <Icon :name="isDark ? 'sun' : 'moon'" size="md" aria-hidden="true" />
    </button>

    <main class="auth-workbench">
      <section class="auth-brand-panel">
        <div class="auth-brand-lockup">
          <div class="auth-brand-mark">
            <img :src="siteLogo || brand.logo" :alt="siteName" class="h-full w-full object-contain" />
          </div>
          <div class="min-w-0">
            <p class="auth-brand-kicker">MODURELAY / CONSOLE</p>
            <h1 class="auth-brand-name">{{ siteName }}</h1>
          </div>
        </div>

        <div class="auth-brand-story">
          <p class="auth-brand-subtitle">{{ siteSubtitle }}</p>
        </div>
      </section>

      <section class="auth-focus-panel">
        <div class="auth-card">
          <slot />
        </div>

        <div class="auth-footer text-sm">
          <slot name="footer" />
        </div>

        <div class="auth-copyright text-xs">
          &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { brand } from '@/config/brand'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { toggleThemeWithTransition } from '@/utils/themeTransition'
import { sanitizeUrl } from '@/utils/url'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || brand.slogan)
const isDark = ref(document.documentElement.classList.contains('dark'))

const currentYear = computed(() => new Date().getFullYear())

function toggleTheme(event: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-layout {
  position: relative;
  display: flex;
  min-height: 100vh;
  min-height: 100dvh;
  align-items: center;
  justify-content: center;
  overflow-x: hidden;
  padding: clamp(24px, 4vw, 56px);
  background: var(--color-bg);
}

.auth-theme-toggle {
  position: absolute;
  z-index: 2;
  top: 24px;
  right: 24px;
  display: inline-flex;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--glass-border);
  border-radius: 12px;
  cursor: pointer;
  color: var(--color-text-secondary);
  background-color: var(--glass-bg-strong);
  box-shadow: var(--shadow-sm), inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(var(--glass-blur));
  backdrop-filter: blur(var(--glass-blur));
  transition:
    color var(--motion-fast) var(--ease-standard),
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.auth-theme-toggle:hover {
  color: var(--color-primary);
  border-color: var(--color-primary-border);
  background-color: var(--color-surface-overlay);
  transform: translateY(-1px);
}

.auth-theme-toggle:active {
  transform: translateY(0);
}

.auth-workbench {
  position: relative;
  z-index: 1;
  display: grid;
  width: min(1140px, 100%);
  min-width: 0;
  min-height: min(700px, calc(100dvh - 64px));
  grid-template-columns: minmax(340px, 0.92fr) minmax(440px, 1.08fr);
  overflow: hidden;
  border: 1px solid var(--glass-border);
  border-radius: 20px;
  background-color: var(--color-surface);
  box-shadow: var(--shadow-overlay), inset 0 1px 0 var(--glass-highlight);
}

.auth-brand-panel {
  position: relative;
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: space-between;
  padding: clamp(36px, 5vw, 60px);
  border-right: 1px solid var(--color-primary-border);
  color: var(--color-text-primary);
  background-color: color-mix(in srgb, var(--color-primary-soft) 78%, var(--color-surface));
}

.auth-brand-lockup {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 16px;
}

.auth-brand-mark {
  display: flex;
  width: 64px;
  height: 64px;
  flex: 0 0 64px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid var(--color-primary-border);
  border-radius: 16px;
  background: var(--color-surface-raised);
  box-shadow: var(--shadow-md), inset 0 1px 0 var(--glass-highlight);
}

.auth-brand-kicker {
  margin-bottom: 7px;
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 11px;
  font-weight: 700;
  line-height: 1.2;
}

.auth-brand-name {
  overflow-wrap: anywhere;
  color: var(--color-text-primary);
  font-size: clamp(28px, 3vw, 38px);
  font-weight: 700;
  line-height: 1.15;
  text-wrap: balance;
}

.auth-brand-story {
  display: grid;
  min-width: 0;
  gap: 20px;
}

.auth-brand-subtitle {
  max-width: 390px;
  color: var(--color-text-secondary);
  font-size: clamp(18px, 2vw, 24px);
  font-weight: 600;
  line-height: 1.45;
  text-wrap: balance;
}

.auth-focus-panel {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  padding: clamp(36px, 5vw, 68px);
  background-color: color-mix(in srgb, var(--glass-bg-strong) 84%, var(--color-surface));
  -webkit-backdrop-filter: blur(var(--glass-blur-strong)) saturate(var(--glass-saturate));
  backdrop-filter: blur(var(--glass-blur-strong)) saturate(var(--glass-saturate));
}

.auth-card {
  width: 100%;
  min-width: 0;
  max-width: 440px;
  margin-inline: auto;
}

.auth-footer {
  max-width: 440px;
  margin: 28px auto 0;
  color: var(--color-text-muted);
  text-align: center;
}

.auth-copyright {
  max-width: 440px;
  margin: 22px auto 0;
  color: var(--color-text-disabled);
  text-align: center;
}

:deep(.auth-page) {
  min-width: 0;
}

:deep(.auth-page-heading) {
  position: relative;
  padding-left: 18px;
  text-align: left;
}

:deep(.auth-page-heading::before) {
  position: absolute;
  top: 3px;
  bottom: 3px;
  left: 0;
  width: 3px;
  border-radius: 3px;
  background-color: var(--color-primary);
  content: '';
}

:deep(.auth-page-title) {
  color: var(--color-text-primary);
  font-size: 28px;
  font-weight: 700;
  line-height: 1.25;
  text-wrap: balance;
}

:deep(.auth-page-subtitle) {
  margin-top: 7px;
  color: var(--color-text-muted);
  font-size: 14px;
  line-height: 1.55;
}

:deep(.auth-form) {
  min-width: 0;
}

:deep(.auth-input-wrap) {
  position: relative;
}

:deep(.auth-input-wrap .input) {
  min-height: 46px;
  border-radius: 10px;
  background-color: color-mix(in srgb, var(--color-surface-raised) 88%, var(--color-bg-subtle));
  box-shadow: inset 0 1px 0 var(--glass-highlight);
}

:deep(.auth-input-wrap .input:hover:not(:disabled):not(:focus-visible)) {
  border-color: var(--color-border-strong);
}

:deep(.auth-input-wrap:focus-within .auth-input-icon) {
  color: var(--color-primary);
}

:deep(.auth-input-icon) {
  color: var(--color-text-disabled);
  transition: color var(--motion-fast) var(--ease-standard);
}

:deep(.auth-password-toggle) {
  display: flex;
  min-width: 44px;
  align-items: center;
  justify-content: center;
  padding-inline: 12px;
  border-radius: 0 10px 10px 0;
  color: var(--color-text-muted);
  cursor: pointer;
  transition:
    color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

:deep(.auth-password-toggle:hover:not(:disabled)) {
  color: var(--color-primary);
  background-color: var(--color-primary-soft);
}

:deep(.auth-password-toggle:disabled) {
  cursor: not-allowed;
  opacity: 0.5;
}

:deep(.auth-submit) {
  min-height: 46px;
  border: 1px solid color-mix(in srgb, var(--color-primary) 82%, var(--color-border));
  border-radius: 10px;
  box-shadow: var(--shadow-sm), inset 0 1px 0 var(--glass-highlight);
}

:deep(.auth-submit:not(:disabled):hover) {
  box-shadow: var(--shadow-md), inset 0 1px 0 var(--glass-highlight);
}

:deep(.auth-link) {
  color: var(--color-primary);
  font-weight: 600;
  text-decoration: none;
  transition: color var(--motion-fast) var(--ease-standard);
}

:deep(.auth-link:hover) {
  color: var(--color-primary-hover);
}

:deep(.auth-divider-line) {
  background-color: var(--color-border);
}

:deep(.auth-divider-label) {
  color: var(--color-text-muted);
}

:global(.dark) .auth-brand-panel {
  background-color: color-mix(in srgb, var(--color-primary-soft) 44%, var(--color-surface));
}

:global(.dark) .auth-focus-panel {
  background-color: color-mix(in srgb, var(--glass-bg-strong) 74%, var(--color-surface));
}

@supports not ((-webkit-backdrop-filter: blur(1px)) or (backdrop-filter: blur(1px))) {
  .auth-theme-toggle,
  .auth-focus-panel {
    background-color: var(--glass-fallback-strong);
  }
}

@media (max-width: 900px) {
  .auth-layout {
    align-items: flex-start;
    padding: 76px 16px 24px;
  }

  .auth-workbench {
    min-height: 0;
    grid-template-columns: 1fr;
  }

  .auth-brand-panel {
    min-height: 196px;
    gap: 24px;
    padding: 28px 32px;
    border-right: 0;
    border-bottom: 1px solid var(--color-primary-border);
  }

  .auth-brand-story {
    display: none;
  }

  .auth-focus-panel {
    padding: 32px 24px;
  }
}

@media (max-width: 480px) {
  .auth-layout {
    padding: 72px 12px 16px;
  }

  .auth-theme-toggle {
    top: 16px;
    right: 16px;
  }

  .auth-workbench {
    border-radius: 14px;
  }

  .auth-brand-panel {
    min-height: 128px;
    padding: 22px;
  }

  .auth-brand-mark {
    width: 52px;
    height: 52px;
    flex-basis: 52px;
  }

  .auth-brand-name {
    font-size: 24px;
  }

  .auth-focus-panel {
    padding: 30px 20px 26px;
  }

  :deep(.auth-page-title) {
    font-size: 24px;
  }

  :deep(.auth-page-subtitle) {
    font-size: 13px;
  }
}

@media (min-width: 700px) and (max-height: 560px) {
  .auth-layout {
    align-items: flex-start;
    padding: 20px 72px 20px 20px;
  }

  .auth-workbench {
    min-height: 0;
    grid-template-columns: minmax(240px, 0.8fr) minmax(420px, 1.2fr);
  }

  .auth-brand-panel {
    min-height: 0;
    padding: 28px;
    border-right: 1px solid var(--color-primary-border);
    border-bottom: 0;
  }

  .auth-brand-story {
    display: grid;
    gap: 12px;
  }

  .auth-brand-subtitle {
    font-size: 16px;
  }

  .auth-focus-panel {
    padding: 28px 36px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-theme-toggle:hover,
  .auth-theme-toggle:active,
  :deep(.auth-submit:not(:disabled):hover) {
    transform: none;
  }
}
</style>
