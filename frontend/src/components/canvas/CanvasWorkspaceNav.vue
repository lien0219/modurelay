<template>
  <div class="canvas-workspace-shell" :class="{ 'is-workspace': workspace, 'is-menu-open': menuOpen }">
    <a class="canvas-workspace-shell__skip" href="#canvas-workspace-content">
      {{ t('canvas.workspaceNav.skipToContent') }}
    </a>

    <header class="canvas-workspace-nav">
      <div class="canvas-workspace-nav__inner">
        <RouterLink
          class="canvas-workspace-nav__brand"
          :to="{ name: 'CanvasHome' }"
          :aria-label="t('canvas.workspaceNav.homeAria')"
          @click="closeMenu"
        >
          <span class="canvas-workspace-nav__brand-mark" aria-hidden="true">
            <img v-if="siteLogo" :src="siteLogo" :alt="siteName" />
            <Icon v-else name="sparkles" size="sm" />
          </span>
          <span class="canvas-workspace-nav__brand-copy">
            <strong>{{ siteName }}</strong>
            <small>{{ t('canvas.workspaceNav.brand') }}</small>
          </span>
        </RouterLink>

        <button
          type="button"
          class="canvas-workspace-nav__menu-button"
          :aria-expanded="menuOpen"
          aria-controls="canvas-workspace-navigation"
          :aria-label="menuOpen ? t('canvas.workspaceNav.closeMenu') : t('canvas.workspaceNav.openMenu')"
          @click="menuOpen = !menuOpen"
        >
          <Icon :name="menuOpen ? 'x' : 'menu'" size="sm" aria-hidden="true" />
        </button>

        <nav
          id="canvas-workspace-navigation"
          class="canvas-workspace-nav__links"
          :class="{ 'is-open': menuOpen }"
          :aria-label="t('canvas.workspaceNav.aria')"
        >
          <RouterLink
            v-for="item in navigationItems"
            :key="item.key"
            :to="item.to"
            :class="{ 'is-active': isActive(item.key) }"
            :aria-current="isActive(item.key) ? 'page' : undefined"
            @click="handleNavigationClick($event, item)"
          >
            <Icon :name="item.icon" size="sm" aria-hidden="true" />
            <span>{{ item.label }}</span>
          </RouterLink>
        </nav>

        <div class="canvas-workspace-nav__actions">
          <div class="canvas-workspace-nav__mode" role="tablist" :aria-label="t('canvas.workspaceNav.modeSwitcherAria')">
            <button type="button" role="tab" aria-selected="true" class="is-active">
              <Icon name="sparkles" size="xs" aria-hidden="true" />
              <span>{{ t('canvas.workspaceNav.canvasMode') }}</span>
            </button>
            <button
              type="button"
              role="tab"
              aria-selected="false"
              data-testid="workspace-mode-relay"
              :disabled="workspaceModeTransitioning"
              @click="switchToRelay"
            >
              <Icon name="key" size="xs" aria-hidden="true" />
              <span>{{ t('canvas.workspaceNav.relayMode') }}</span>
            </button>
          </div>
          <button
            type="button"
            class="canvas-workspace-nav__icon-button"
            :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
            :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
            @click="toggleTheme"
          >
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" aria-hidden="true" />
          </button>
        </div>
      </div>
    </header>

    <div class="canvas-workspace-shell__content">
      <slot />
    </div>

    <button
      v-if="menuOpen"
      type="button"
      class="canvas-workspace-nav__scrim"
      :aria-label="t('canvas.workspaceNav.closeMenu')"
      @click="closeMenu"
    ></button>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter, type RouteLocationRaw } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { brand } from '@/config/brand'
import { useThemeMode } from '@/composables/useThemeMode'
import { sanitizeUrl } from '@/utils/url'
import { toggleThemeWithTransition } from '@/utils/themeTransition'
import {
  navigateWithWorkspaceModeTransition,
  workspaceModeTransitioning,
} from '@/utils/workspaceModeTransition'

type NavigationKey = 'projects' | 'images' | 'video' | 'prompts' | 'assets' | 'settings'
type WorkspaceIcon = 'grid' | 'sparkles' | 'play' | 'document' | 'inbox' | 'cog'

interface NavigationItem {
  key: NavigationKey
  label: string
  icon: WorkspaceIcon
  to: RouteLocationRaw
}

withDefaults(defineProps<{
  workspace?: boolean
}>(), {
  workspace: false,
})

const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const { t } = useI18n()
const isDark = useThemeMode()
const menuOpen = ref(false)
const siteName = computed(() => appStore.siteName || brand.name)
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))

const navigationItems = computed<NavigationItem[]>(() => [
  { key: 'projects', label: t('canvas.workspaceNav.myCanvases'), icon: 'grid', to: { name: 'CanvasHome', hash: '#canvas-library' } },
  { key: 'images', label: t('canvas.workspaceNav.imageWorkbench'), icon: 'sparkles', to: { name: 'BatchImageGuide' } },
  { key: 'video', label: t('canvas.workspaceNav.videoWorkbench'), icon: 'play', to: { name: 'CanvasVideo' } },
  { key: 'prompts', label: t('canvas.workspaceNav.promptLibrary'), icon: 'document', to: { name: 'CanvasHome', hash: '#canvas-prompts' } },
  { key: 'assets', label: t('canvas.workspaceNav.myAssets'), icon: 'inbox', to: { name: 'CanvasHome', hash: '#canvas-assets' } },
  { key: 'settings', label: t('canvas.workspaceNav.settings'), icon: 'cog', to: { name: 'Keys' } },
])

function isActive(key: NavigationKey) {
  if (key === 'video') return route.name === 'CanvasVideo'
  if (key === 'images') return route.name === 'BatchImageGuide'
  if (key === 'projects' && route.name === 'CanvasEditor') return true
  if (key === 'settings') return false
  if (route.name !== 'CanvasHome') return false
  if (key === 'projects') return !route.hash || route.hash === '#canvas-library'
  if (key === 'prompts') return route.hash === '#canvas-prompts'
  if (key === 'assets') return route.hash === '#canvas-assets'
  return false
}

function closeMenu() {
  menuOpen.value = false
}

function toggleTheme(event: MouseEvent) {
  toggleThemeWithTransition(isDark.value, event)
}

function switchToRelay() {
  closeMenu()
  void navigateWithWorkspaceModeTransition(router, { name: 'Keys' }, 'to-relay')
}

function handleNavigationClick(event: MouseEvent, item: NavigationItem) {
  closeMenu()
  if (item.key !== 'settings' || event.button !== 0 || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return
  event.preventDefault()
  switchToRelay()
}

watch(() => route.fullPath, closeMenu)
</script>

<style scoped>
.canvas-workspace-shell {
  display: flex;
  width: 100%;
  height: 100dvh;
  min-height: 36rem;
  flex-direction: column;
  overflow: hidden;
  background: var(--color-bg);
  color: var(--color-text-primary);
}

.canvas-workspace-shell__skip {
  position: fixed;
  z-index: 340;
  top: 8px;
  left: 12px;
  padding: 8px 12px;
  border: 1px solid var(--color-primary-border);
  border-radius: 8px;
  background: var(--color-surface-overlay);
  color: var(--color-primary);
  font-size: 13px;
  font-weight: 700;
  opacity: 0;
  pointer-events: none;
  transform: translateY(-8px);
}

.canvas-workspace-shell__skip:focus {
  opacity: 1;
  pointer-events: auto;
  transform: translateY(0);
}

.canvas-workspace-nav {
  position: relative;
  z-index: 40;
  flex: 0 0 60px;
  border-bottom: 1px solid var(--glass-border);
  background: var(--glass-fallback-strong);
  box-shadow: inset 0 -1px 0 var(--glass-highlight);
}

@supports (backdrop-filter: blur(16px)) {
  .canvas-workspace-nav {
    background: var(--glass-bg-strong);
    backdrop-filter: blur(18px) saturate(115%);
  }
}

.canvas-workspace-nav__inner {
  display: flex;
  width: min(1200px, 100%);
  min-height: 60px;
  align-items: center;
  gap: 14px;
  margin: 0 auto;
  padding: 0 18px;
}

.canvas-workspace-nav__brand {
  display: inline-flex;
  min-width: 0;
  flex: 0 1 auto;
  align-items: center;
  gap: 9px;
  color: var(--color-text-primary);
  text-decoration: none;
}

.canvas-workspace-nav__brand-mark {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  overflow: hidden;
  border: 1px solid var(--color-primary-border);
  border-radius: 8px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-workspace-nav__brand-mark img {
  display: block;
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.canvas-workspace-nav__brand-copy {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 1px;
}

.canvas-workspace-nav__brand-copy strong,
.canvas-workspace-nav__brand-copy small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.canvas-workspace-nav__brand-copy strong { font-size: 13px; font-weight: 750; }
.canvas-workspace-nav__brand-copy small { color: var(--color-text-muted); font-size: 10px; font-weight: 600; }

.canvas-workspace-nav__links {
  display: flex;
  min-width: 0;
  flex: 1;
  align-items: center;
  gap: 2px;
}

.canvas-workspace-nav__links a {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  gap: 6px;
  padding: 0 9px;
  border: 1px solid transparent;
  border-radius: 7px;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 650;
  text-decoration: none;
  white-space: nowrap;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-workspace-nav__links a:hover {
  border-color: var(--color-border);
  background: var(--color-surface-soft);
  color: var(--color-text-primary);
}

.canvas-workspace-nav__links a.is-active {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.canvas-workspace-nav__actions {
  display: flex;
  flex: 0 0 auto;
  align-items: center;
  gap: 6px;
}

.canvas-workspace-nav__mode {
  display: grid;
  grid-template-columns: repeat(2, minmax(70px, 1fr));
  gap: 3px;
  padding: 3px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface-soft);
}

.canvas-workspace-nav__mode button {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  justify-content: center;
  gap: 5px;
  padding: 0 9px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--color-text-secondary);
  font: inherit;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

.canvas-workspace-nav__mode button:hover { color: var(--color-text-primary); }
.canvas-workspace-nav__mode button.is-active {
  border-color: var(--color-primary-border);
  background: var(--color-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-xs);
}

.canvas-workspace-nav__mode button:disabled {
  cursor: wait;
  opacity: 0.62;
}

.canvas-workspace-nav__icon-button,
.canvas-workspace-nav__menu-button {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    color var(--motion-fast) var(--ease-standard);
}

.canvas-workspace-nav__icon-button:hover,
.canvas-workspace-nav__menu-button:hover {
  border-color: var(--color-border);
  background: var(--color-surface-soft);
  color: var(--color-text-primary);
}

.canvas-workspace-nav__menu-button { display: none; }

.canvas-workspace-shell__content {
  min-width: 0;
  min-height: 0;
  flex: 1;
  overflow: auto;
  overscroll-behavior: contain;
}

.canvas-workspace-shell.is-workspace .canvas-workspace-shell__content { overflow: hidden; }

.canvas-workspace-nav__scrim {
  position: fixed;
  z-index: 38;
  inset: 60px 0 0;
  border: 0;
  background: color-mix(in srgb, var(--color-bg-deep) 62%, transparent);
  cursor: pointer;
}

.canvas-workspace-nav__brand:focus-visible,
.canvas-workspace-nav__links a:focus-visible,
.canvas-workspace-nav__mode button:focus-visible,
.canvas-workspace-nav__icon-button:focus-visible,
.canvas-workspace-nav__menu-button:focus-visible,
.canvas-workspace-nav__scrim:focus-visible {
  outline: 2px solid var(--color-primary);
  outline-offset: 2px;
}

@media (max-width: 1120px) {
  .canvas-workspace-nav__brand-copy strong { display: none; }
  .canvas-workspace-nav__brand-copy small { color: var(--color-text-primary); font-size: 12px; }
  .canvas-workspace-nav__links a { padding: 0 7px; }
  .canvas-workspace-nav__links a svg { display: none; }
}

@media (max-width: 900px) {
  .canvas-workspace-nav__inner { gap: 8px; padding: 0 12px; }
  .canvas-workspace-nav__brand { margin-right: auto; }
  .canvas-workspace-nav__menu-button { display: grid; }
  .canvas-workspace-nav__links {
    position: absolute;
    z-index: 42;
    top: calc(100% + 8px);
    right: 12px;
    display: grid;
    width: min(300px, calc(100vw - 24px));
    gap: 4px;
    padding: 8px;
    border: 1px solid var(--glass-border);
    border-radius: 10px;
    background: var(--glass-fallback-strong);
    box-shadow: var(--shadow-overlay);
    opacity: 0;
    pointer-events: none;
    transform: translateY(-8px);
    transition:
      opacity var(--motion-fast) var(--ease-standard),
      transform var(--motion-fast) var(--ease-standard);
  }
  @supports (backdrop-filter: blur(16px)) {
    .canvas-workspace-nav__links {
      background: var(--glass-bg-strong);
      backdrop-filter: blur(18px) saturate(115%);
    }
  }
  .canvas-workspace-nav__links.is-open {
    opacity: 1;
    pointer-events: auto;
    transform: translateY(0);
  }
  .canvas-workspace-nav__links a { min-height: 44px; padding: 0 11px; }
  .canvas-workspace-nav__links a svg { display: block; }
}

@media (max-width: 560px) {
  .canvas-workspace-shell { min-height: 30rem; }
  .canvas-workspace-nav,
  .canvas-workspace-nav__inner { min-height: 58px; }
  .canvas-workspace-nav { flex-basis: 58px; }
  .canvas-workspace-nav__brand-copy { display: none; }
  .canvas-workspace-nav__mode { grid-template-columns: minmax(52px, 1fr) minmax(66px, 1fr); }
  .canvas-workspace-nav__mode button { min-height: 36px; padding: 0 6px; }
  .canvas-workspace-nav__icon-button { display: none; }
  .canvas-workspace-nav__scrim { inset: 58px 0 0; }
}

@media (prefers-reduced-motion: reduce) {
  .canvas-workspace-nav__links,
  .canvas-workspace-nav__links a,
  .canvas-workspace-nav__mode button,
  .canvas-workspace-nav__icon-button,
  .canvas-workspace-nav__menu-button {
    transition-duration: 0.01ms;
  }
}
</style>
