<template>
  <div
    ref="layoutRef"
    class="auth-layout"
    @pointermove="handlePointerMove"
    @pointerleave="resetPointer"
  >
    <div class="auth-ambient auth-ambient-primary" aria-hidden="true"></div>
    <div class="auth-ambient auth-ambient-secondary" aria-hidden="true"></div>

    <div class="auth-console-pill">
      <span class="auth-console-dot" aria-hidden="true"></span>
      <span>{{ t('auth.gatewayConsole') }}</span>
    </div>

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
            <p class="auth-brand-name">{{ siteName }}</p>
            <p class="auth-brand-caption">{{ siteSubtitle }}</p>
          </div>
        </div>

        <div class="auth-hero">
          <div class="auth-hero-kicker">
            <span class="auth-hero-kicker-dot" aria-hidden="true"></span>
            {{ t('auth.heroEyebrow') }}
          </div>

          <h1 class="auth-hero-title">
            <span>{{ t('auth.heroTitleLead') }}</span>
            <strong>{{ t('auth.heroTitleAccent') }}</strong>
          </h1>

          <p class="auth-hero-description">
            {{ t('auth.heroDescription') }}
          </p>

          <div class="auth-capabilities" aria-label="Gateway capabilities">
            <span>{{ t('auth.capabilityRouting') }}</span>
            <span>{{ t('auth.capabilityBalancing') }}</span>
            <span>{{ t('auth.capabilityMetering') }}</span>
            <span>{{ t('auth.capabilityAvailability') }}</span>
          </div>
        </div>

        <div class="auth-network" aria-hidden="true">
          <svg class="auth-network-lines" viewBox="0 0 680 330" preserveAspectRatio="none">
            <path class="auth-route auth-route-a" d="M340 165 C270 165 250 90 155 78" />
            <path class="auth-route auth-route-b" d="M340 165 C420 155 438 78 535 68" />
            <path class="auth-route auth-route-c" d="M340 165 C266 176 246 250 160 258" />
            <path class="auth-route auth-route-d" d="M340 165 C418 178 440 245 528 254" />
          </svg>

          <div class="auth-network-ring auth-network-ring-one"></div>
          <div class="auth-network-ring auth-network-ring-two"></div>

          <div class="auth-provider auth-provider-openai">
            <span class="auth-provider-badge">GPT</span>
            <span class="auth-provider-copy">
              <strong>OpenAI</strong>
              <small>UPSTREAM</small>
            </span>
          </div>

          <div class="auth-provider auth-provider-anthropic">
            <span class="auth-provider-badge">CL</span>
            <span class="auth-provider-copy">
              <strong>Anthropic</strong>
              <small>UPSTREAM</small>
            </span>
          </div>

          <div class="auth-provider auth-provider-gemini">
            <span class="auth-provider-badge">GE</span>
            <span class="auth-provider-copy">
              <strong>Gemini</strong>
              <small>UPSTREAM</small>
            </span>
          </div>

          <div class="auth-provider auth-provider-deepseek">
            <span class="auth-provider-badge">DS</span>
            <span class="auth-provider-copy">
              <strong>DeepSeek</strong>
              <small>UPSTREAM</small>
            </span>
          </div>

          <div class="auth-router-hub">
            <div class="auth-router-mark">
              <img :src="siteLogo || brand.logo" alt="" class="h-full w-full object-contain" />
            </div>
            <div class="auth-router-copy">
              <strong>{{ siteName }}</strong>
              <span>{{ t('auth.routerCore') }}</span>
            </div>
          </div>
        </div>
      </section>

      <section class="auth-focus-panel">
        <div class="auth-card-shell">
          <div class="auth-card-badge" aria-hidden="true">
            <img :src="siteLogo || brand.logo" alt="" class="h-full w-full object-contain" />
          </div>

          <div class="auth-card">
            <slot />
          </div>

          <div class="auth-footer text-sm">
            <slot name="footer" />
          </div>

          <div class="auth-copyright text-xs">
            &copy; {{ currentYear }} {{ siteName }} · Secure AI Gateway
          </div>
        </div>
      </section>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { brand } from '@/config/brand'
import { useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import { toggleThemeWithTransition } from '@/utils/themeTransition'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()
const layoutRef = ref<HTMLElement | null>(null)

const siteName = computed(() => appStore.siteName || brand.name)
const siteLogo = computed(() =>
  sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true })
)
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || brand.slogan)
const isDark = ref(document.documentElement.classList.contains('dark'))

const currentYear = computed(() => new Date().getFullYear())

function toggleTheme(event: MouseEvent) {
  isDark.value = toggleThemeWithTransition(isDark.value, event)
}

function handlePointerMove(event: PointerEvent): void {
  if (event.pointerType !== 'mouse' || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
    return
  }

  const el = layoutRef.value
  if (!el) return

  const x = event.clientX / window.innerWidth
  const y = event.clientY / window.innerHeight
  const rotateY = (x - 0.5) * 3.2
  const rotateX = (0.5 - y) * 2.4

  el.style.setProperty('--auth-pointer-x', String(Math.round(x * 100)) + '%')
  el.style.setProperty('--auth-pointer-y', String(Math.round(y * 100)) + '%')
  el.style.setProperty('--auth-scene-x', rotateX.toFixed(2) + 'deg')
  el.style.setProperty('--auth-scene-y', rotateY.toFixed(2) + 'deg')
}

function resetPointer(): void {
  const el = layoutRef.value
  if (!el) return

  el.style.setProperty('--auth-pointer-x', '32%')
  el.style.setProperty('--auth-pointer-y', '36%')
  el.style.setProperty('--auth-scene-x', '0deg')
  el.style.setProperty('--auth-scene-y', '0deg')
}

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-layout {
  --auth-pointer-x: 32%;
  --auth-pointer-y: 36%;
  --auth-scene-x: 0deg;
  --auth-scene-y: 0deg;
  --auth-grid: color-mix(in srgb, var(--color-text-primary) 7%, transparent);
  --auth-card-bg: color-mix(in srgb, var(--color-surface) 92%, transparent);
  --auth-card-border: color-mix(in srgb, var(--color-border) 84%, transparent);
  --auth-node-bg: color-mix(in srgb, var(--color-surface) 91%, transparent);
  --auth-node-border: color-mix(in srgb, var(--color-border-strong) 72%, transparent);
  position: relative;
  min-height: 100vh;
  min-height: 100dvh;
  overflow-x: hidden;
  padding: clamp(24px, 3.6vw, 56px);
  color: var(--color-text-primary);
  background:
    radial-gradient(circle at 17% 18%, color-mix(in srgb, #5b5cf0 13%, transparent), transparent 30%),
    radial-gradient(circle at 25% 84%, color-mix(in srgb, #0d9488 10%, transparent), transparent 30%),
    var(--color-bg);
  isolation: isolate;
}

.auth-layout::before {
  position: fixed;
  z-index: -3;
  inset: 0;
  background-image:
    linear-gradient(var(--auth-grid) 1px, transparent 1px),
    linear-gradient(90deg, var(--auth-grid) 1px, transparent 1px);
  background-size: 42px 42px;
  content: '';
  -webkit-mask-image: linear-gradient(90deg, #000 0%, #000 58%, transparent 68%);
  mask-image: linear-gradient(90deg, #000 0%, #000 58%, transparent 68%);
  pointer-events: none;
}

.auth-layout::after {
  position: fixed;
  z-index: -2;
  inset: 0;
  background:
    radial-gradient(
      circle at var(--auth-pointer-x) var(--auth-pointer-y),
      color-mix(in srgb, var(--color-primary) 8%, transparent),
      transparent 24%
    );
  content: '';
  pointer-events: none;
  transition: background-position 160ms linear;
}

.auth-ambient {
  position: fixed;
  z-index: -1;
  width: 420px;
  height: 420px;
  border-radius: 999px;
  filter: blur(96px);
  opacity: 0.2;
  pointer-events: none;
}

.auth-ambient-primary {
  top: -220px;
  left: 5%;
  background: #6366f1;
}

.auth-ambient-secondary {
  bottom: -260px;
  left: 22%;
  background: #14b8a6;
}

.auth-console-pill,
.auth-theme-toggle {
  position: fixed;
  z-index: 10;
  top: 30px;
  border: 1px solid var(--auth-card-border);
  background: color-mix(in srgb, var(--color-surface) 76%, transparent);
  box-shadow: var(--shadow-sm), inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(18px) saturate(135%);
  backdrop-filter: blur(18px) saturate(135%);
}

.auth-console-pill {
  right: 88px;
  display: inline-flex;
  min-height: 42px;
  align-items: center;
  gap: 9px;
  padding: 0 16px;
  border-radius: 999px;
  color: var(--color-text-muted);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.01em;
}

.auth-console-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #22c55e;
  box-shadow: 0 0 0 5px color-mix(in srgb, #22c55e 12%, transparent);
}

.auth-theme-toggle {
  right: 30px;
  display: inline-flex;
  width: 42px;
  height: 42px;
  align-items: center;
  justify-content: center;
  border-radius: 13px;
  cursor: pointer;
  color: var(--color-text-secondary);
  transition:
    color var(--motion-fast) var(--ease-standard),
    border-color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard),
    transform var(--motion-fast) var(--ease-standard);
}

.auth-theme-toggle:hover {
  color: var(--color-primary);
  border-color: var(--color-primary-border);
  background: color-mix(in srgb, var(--color-primary-soft) 45%, var(--color-surface));
  transform: translateY(-1px);
}

.auth-theme-toggle:active {
  transform: translateY(0);
}

.auth-workbench {
  position: relative;
  z-index: 1;
  display: grid;
  width: min(1540px, 100%);
  min-height: calc(100dvh - clamp(48px, 7.2vw, 112px));
  margin-inline: auto;
  grid-template-columns: minmax(0, 1.28fr) minmax(460px, 0.72fr);
  align-items: center;
  gap: clamp(48px, 6vw, 108px);
}

.auth-brand-panel {
  position: relative;
  display: flex;
  min-width: 0;
  min-height: min(880px, calc(100dvh - 76px));
  flex-direction: column;
  justify-content: space-between;
  padding: clamp(10px, 1vw, 18px) 0;
}

.auth-brand-lockup {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 14px;
}

.auth-brand-mark {
  display: flex;
  width: 48px;
  height: 48px;
  flex: 0 0 48px;
  align-items: center;
  justify-content: center;
  overflow: hidden;
  border: 1px solid color-mix(in srgb, var(--color-primary-border) 72%, transparent);
  border-radius: 14px;
  background: color-mix(in srgb, var(--color-surface) 80%, transparent);
  box-shadow:
    0 12px 32px color-mix(in srgb, #0f766e 18%, transparent),
    inset 0 1px 0 var(--glass-highlight);
}

.auth-brand-name {
  color: var(--color-text-primary);
  font-size: 18px;
  font-weight: 800;
  line-height: 1.15;
  letter-spacing: -0.025em;
}

.auth-brand-caption {
  max-width: 300px;
  margin-top: 3px;
  overflow: hidden;
  color: var(--color-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 9px;
  font-weight: 600;
  line-height: 1.25;
  letter-spacing: 0.04em;
  text-overflow: ellipsis;
  text-transform: uppercase;
  white-space: nowrap;
}

.auth-hero {
  width: min(720px, 100%);
  margin-top: clamp(56px, 8vh, 110px);
}

.auth-hero-kicker {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 10px;
  padding: 0 14px;
  border: 1px solid color-mix(in srgb, var(--color-primary) 28%, var(--color-border));
  border-radius: 999px;
  color: var(--color-primary);
  background: color-mix(in srgb, var(--color-primary-soft) 50%, transparent);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.02em;
  box-shadow: inset 0 1px 0 var(--glass-highlight);
}

.auth-hero-kicker-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #14b8a6;
  box-shadow: 0 0 0 5px color-mix(in srgb, #14b8a6 12%, transparent);
}

.auth-hero-title {
  display: grid;
  margin-top: 34px;
  color: var(--color-text-primary);
  font-size: clamp(48px, 5.1vw, 76px);
  font-weight: 820;
  line-height: 0.98;
  letter-spacing: -0.06em;
  text-wrap: balance;
}

.auth-hero-title strong {
  margin-top: 6px;
  color: transparent;
  background: linear-gradient(100deg, #5b5ce8 3%, #3b82f6 50%, #0ea5a4 102%);
  -webkit-background-clip: text;
  background-clip: text;
  font-weight: inherit;
}

.auth-hero-description {
  max-width: 620px;
  margin-top: 24px;
  color: var(--color-text-secondary);
  font-size: clamp(15px, 1.2vw, 18px);
  line-height: 1.72;
}

.auth-capabilities {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 26px;
}

.auth-capabilities span {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  padding: 0 14px;
  border: 1px solid var(--auth-node-border);
  border-radius: 11px;
  color: var(--color-text-secondary);
  background: color-mix(in srgb, var(--auth-node-bg) 90%, transparent);
  box-shadow: var(--shadow-xs), inset 0 1px 0 var(--glass-highlight);
  font-size: 12px;
  font-weight: 650;
  -webkit-backdrop-filter: blur(12px);
  backdrop-filter: blur(12px);
}

.auth-network {
  position: relative;
  width: min(700px, 94%);
  height: 330px;
  margin-top: clamp(38px, 7vh, 80px);
  transform: perspective(1100px) rotateX(var(--auth-scene-x)) rotateY(var(--auth-scene-y));
  transform-style: preserve-3d;
  transition: transform 180ms ease-out;
}

.auth-network-lines {
  position: absolute;
  z-index: 1;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.auth-route {
  fill: none;
  stroke: color-mix(in srgb, var(--color-primary) 58%, transparent);
  stroke-width: 1.25;
  stroke-dasharray: 8 8;
  vector-effect: non-scaling-stroke;
  animation: auth-route-flow 7s linear infinite;
}

.auth-route-b,
.auth-route-c {
  animation-direction: reverse;
}

.auth-network-ring {
  position: absolute;
  z-index: 0;
  top: 50%;
  left: 50%;
  border: 1px solid color-mix(in srgb, var(--color-primary) 30%, transparent);
  border-radius: 999px;
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.auth-network-ring-one {
  width: 205px;
  height: 205px;
}

.auth-network-ring-two {
  width: 292px;
  height: 292px;
  border-color: color-mix(in srgb, #14b8a6 28%, transparent);
}

.auth-provider,
.auth-router-hub {
  position: absolute;
  z-index: 3;
  display: flex;
  align-items: center;
  border: 1px solid var(--auth-node-border);
  background: var(--auth-node-bg);
  box-shadow:
    0 18px 40px color-mix(in srgb, #0f172a 12%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(18px) saturate(125%);
  backdrop-filter: blur(18px) saturate(125%);
}

.auth-provider {
  min-width: 168px;
  min-height: 62px;
  gap: 11px;
  padding: 9px 13px;
  border-radius: 14px;
  animation: auth-node-float 5.8s ease-in-out infinite;
}

.auth-provider-openai {
  top: 13px;
  left: 6px;
}

.auth-provider-anthropic {
  bottom: 8px;
  left: 12px;
  animation-delay: -1.8s;
}

.auth-provider-gemini {
  top: 4px;
  right: 4px;
  animation-delay: -3.2s;
}

.auth-provider-deepseek {
  right: 12px;
  bottom: 8px;
  animation-delay: -4.1s;
}

.auth-provider-badge {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border-radius: 10px;
  color: white;
  background: linear-gradient(135deg, #4f46e5, #2585e8);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: -0.03em;
  box-shadow: 0 8px 18px color-mix(in srgb, #4f46e5 24%, transparent);
}

.auth-provider-copy {
  display: grid;
  min-width: 0;
  gap: 3px;
}

.auth-provider-copy strong {
  color: var(--color-text-primary);
  font-size: 12px;
  font-weight: 750;
}

.auth-provider-copy small {
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 8px;
  letter-spacing: 0.08em;
}

.auth-router-hub {
  top: 50%;
  left: 50%;
  flex-direction: column;
  gap: 10px;
  padding: 14px;
  border: 1px solid color-mix(in srgb, var(--color-primary) 34%, var(--auth-node-border));
  border-radius: 23px;
  transform: translate(-50%, -50%) translateZ(34px);
  background: color-mix(in srgb, var(--auth-node-bg) 92%, var(--color-primary-soft));
  box-shadow:
    0 26px 70px color-mix(in srgb, #4f46e5 22%, transparent),
    0 0 0 8px color-mix(in srgb, var(--color-primary) 4%, transparent),
    inset 0 1px 0 var(--glass-highlight);
}

.auth-router-mark {
  width: 76px;
  height: 76px;
  overflow: hidden;
  border-radius: 21px;
  box-shadow: 0 16px 36px color-mix(in srgb, #0f766e 26%, transparent);
}

.auth-router-copy {
  display: grid;
  gap: 2px;
  text-align: center;
}

.auth-router-copy strong {
  max-width: 126px;
  overflow: hidden;
  color: var(--color-text-primary);
  font-size: 11px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.auth-router-copy span {
  color: var(--color-text-disabled);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 7px;
  font-weight: 700;
  letter-spacing: 0.09em;
}

.auth-focus-panel {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: center;
  padding-block: 68px 28px;
}

.auth-card-shell {
  position: relative;
  display: flex;
  width: min(500px, 100%);
  min-height: 650px;
  flex-direction: column;
  padding: clamp(32px, 3vw, 44px);
  border: 1px solid var(--auth-card-border);
  border-radius: 28px;
  background: var(--auth-card-bg);
  box-shadow:
    0 34px 80px color-mix(in srgb, #0f172a 16%, transparent),
    0 10px 28px color-mix(in srgb, #0f172a 8%, transparent),
    inset 0 1px 0 var(--glass-highlight);
  -webkit-backdrop-filter: blur(24px) saturate(140%);
  backdrop-filter: blur(24px) saturate(140%);
}

.auth-card-shell::before {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background:
    linear-gradient(135deg, color-mix(in srgb, var(--color-primary) 5%, transparent), transparent 34%);
  content: '';
  pointer-events: none;
}

.auth-card-badge {
  position: absolute;
  z-index: 2;
  top: 34px;
  right: 34px;
  width: 36px;
  height: 36px;
  overflow: hidden;
  border: 1px solid var(--auth-card-border);
  border-radius: 11px;
  background: color-mix(in srgb, var(--color-surface-raised) 88%, transparent);
  box-shadow: var(--shadow-xs), inset 0 1px 0 var(--glass-highlight);
}

.auth-card {
  position: relative;
  z-index: 1;
  width: 100%;
  min-width: 0;
  flex: 1 0 auto;
  padding-top: 6px;
}

.auth-footer {
  position: relative;
  z-index: 1;
  margin-top: 26px;
  color: var(--color-text-muted);
  text-align: center;
}

.auth-copyright {
  position: relative;
  z-index: 1;
  margin-top: 34px;
  color: var(--color-text-disabled);
  text-align: center;
}

:deep(.auth-page) {
  min-width: 0;
}

:deep(.auth-page-heading) {
  max-width: calc(100% - 54px);
  padding: 0;
  text-align: left;
}

:deep(.auth-page-heading::before) {
  display: none;
}

:deep(.auth-page-title) {
  color: var(--color-text-primary);
  font-size: clamp(27px, 2.4vw, 32px);
  font-weight: 800;
  line-height: 1.18;
  letter-spacing: -0.035em;
  text-wrap: balance;
}

:deep(.auth-page-subtitle) {
  margin-top: 7px;
  color: var(--color-text-muted);
  font-size: 13px;
  line-height: 1.55;
}

:deep(.auth-form) {
  min-width: 0;
}

:deep(.input-label) {
  margin-bottom: 8px;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 700;
}

:deep(.auth-input-wrap) {
  position: relative;
}

:deep(.auth-input-wrap .input) {
  min-height: 50px;
  border-color: color-mix(in srgb, var(--color-border-strong) 74%, transparent);
  border-radius: 13px;
  background: color-mix(in srgb, var(--color-surface-raised) 90%, transparent);
  box-shadow:
    inset 0 1px 0 var(--glass-highlight),
    0 1px 2px color-mix(in srgb, #0f172a 4%, transparent);
  transition:
    border-color var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

:deep(.auth-input-wrap .input:hover:not(:disabled):not(:focus)) {
  border-color: color-mix(in srgb, var(--color-primary) 32%, var(--color-border-strong));
}

:deep(.auth-input-wrap .input:focus) {
  border-color: color-mix(in srgb, var(--color-primary) 76%, #64748b);
  box-shadow:
    0 0 0 3px color-mix(in srgb, var(--color-primary) 14%, transparent),
    inset 0 1px 0 var(--glass-highlight);
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
  border-radius: 0 13px 13px 0;
  color: var(--color-text-muted);
  cursor: pointer;
  transition:
    color var(--motion-fast) var(--ease-standard),
    background-color var(--motion-fast) var(--ease-standard);
}

:deep(.auth-password-toggle:hover:not(:disabled)) {
  color: var(--color-primary);
  background-color: color-mix(in srgb, var(--color-primary-soft) 64%, transparent);
}

:deep(.auth-password-toggle:disabled) {
  cursor: not-allowed;
  opacity: 0.5;
}

:deep(.auth-submit) {
  min-height: 50px;
  border: 0;
  border-radius: 13px;
  background-image: linear-gradient(102deg, #5a4de6 0%, #4f64e9 46%, #3d7bec 100%);
  box-shadow:
    0 14px 28px color-mix(in srgb, #4f46e5 24%, transparent),
    inset 0 1px 0 rgba(255, 255, 255, 0.24);
  font-weight: 750;
  letter-spacing: 0.01em;
  transition:
    transform var(--motion-fast) var(--ease-standard),
    filter var(--motion-fast) var(--ease-standard),
    box-shadow var(--motion-fast) var(--ease-standard);
}

:deep(.auth-submit:not(:disabled):hover) {
  filter: saturate(1.08) brightness(1.03);
  transform: translateY(-1px);
  box-shadow:
    0 18px 34px color-mix(in srgb, #4f46e5 30%, transparent),
    inset 0 1px 0 rgba(255, 255, 255, 0.26);
}

:deep(.auth-card .btn-secondary) {
  min-height: 46px;
  border-color: var(--auth-card-border);
  border-radius: 12px;
  background: color-mix(in srgb, var(--color-surface-raised) 70%, transparent);
  box-shadow: inset 0 1px 0 var(--glass-highlight);
}

:deep(.auth-card .btn-secondary:hover:not(:disabled)) {
  border-color: color-mix(in srgb, var(--color-primary) 32%, var(--color-border));
  background: color-mix(in srgb, var(--color-primary-soft) 42%, var(--color-surface-raised));
}

:deep(.auth-link) {
  color: var(--color-primary);
  font-weight: 650;
  text-decoration: none;
  transition: color var(--motion-fast) var(--ease-standard);
}

:deep(.auth-link:hover) {
  color: var(--color-primary-hover);
}

:deep(.auth-divider-line) {
  background: color-mix(in srgb, var(--color-border) 86%, transparent);
}

:deep(.auth-divider-label) {
  color: var(--color-text-muted);
}

:global(.dark) .auth-layout {
  --auth-grid: rgba(126, 146, 178, 0.11);
  --auth-card-bg: rgba(15, 22, 34, 0.82);
  --auth-card-border: rgba(118, 139, 172, 0.22);
  --auth-node-bg: rgba(18, 27, 41, 0.88);
  --auth-node-border: rgba(116, 138, 172, 0.25);
  background:
    radial-gradient(circle at 17% 18%, rgba(79, 70, 229, 0.16), transparent 30%),
    radial-gradient(circle at 25% 84%, rgba(13, 148, 136, 0.13), transparent 31%),
    #070b12;
}

:global(.dark) .auth-console-pill,
:global(.dark) .auth-theme-toggle {
  background: rgba(16, 24, 37, 0.8);
}

:global(.dark) .auth-card-shell {
  box-shadow:
    0 36px 86px rgba(0, 0, 0, 0.42),
    0 10px 30px rgba(0, 0, 0, 0.24),
    inset 0 1px 0 rgba(255, 255, 255, 0.035);
}

:global(.dark) :deep(.auth-input-wrap .input) {
  background: rgba(17, 26, 40, 0.88);
}

@keyframes auth-route-flow {
  to {
    stroke-dashoffset: -64;
  }
}

@keyframes auth-node-float {
  0%,
  100% {
    transform: translateY(0);
  }
  50% {
    transform: translateY(-5px);
  }
}

@supports not ((-webkit-backdrop-filter: blur(1px)) or (backdrop-filter: blur(1px))) {
  .auth-console-pill,
  .auth-theme-toggle,
  .auth-provider,
  .auth-router-hub,
  .auth-card-shell {
    background: var(--color-surface);
  }
}

@media (max-width: 1180px) {
  .auth-workbench {
    grid-template-columns: minmax(0, 1fr) minmax(430px, 0.8fr);
    gap: 44px;
  }

  .auth-hero-title {
    font-size: clamp(44px, 5vw, 62px);
  }

  .auth-network {
    width: 100%;
    transform: scale(0.92) perspective(1100px) rotateX(var(--auth-scene-x)) rotateY(var(--auth-scene-y));
    transform-origin: left center;
  }
}

@media (max-width: 940px) {
  .auth-layout {
    padding: 82px 18px 28px;
  }

  .auth-layout::before {
    -webkit-mask-image: linear-gradient(#000, transparent 74%);
    mask-image: linear-gradient(#000, transparent 74%);
  }

  .auth-console-pill {
    display: none;
  }

  .auth-theme-toggle {
    top: 20px;
    right: 20px;
  }

  .auth-workbench {
    width: min(560px, 100%);
    min-height: auto;
    grid-template-columns: 1fr;
    gap: 26px;
  }

  .auth-brand-panel {
    min-height: 0;
    padding: 0 6px;
  }

  .auth-hero,
  .auth-network {
    display: none;
  }

  .auth-brand-lockup {
    justify-content: center;
  }

  .auth-brand-caption {
    max-width: min(320px, 70vw);
  }

  .auth-focus-panel {
    padding: 0;
  }

  .auth-card-shell {
    width: 100%;
    min-height: 0;
    border-radius: 24px;
  }
}

@media (max-width: 520px) {
  .auth-layout {
    padding: 74px 12px 18px;
  }

  .auth-theme-toggle {
    top: 16px;
    right: 16px;
  }

  .auth-brand-mark {
    width: 44px;
    height: 44px;
    flex-basis: 44px;
    border-radius: 12px;
  }

  .auth-brand-name {
    font-size: 17px;
  }

  .auth-card-shell {
    padding: 28px 20px 24px;
    border-radius: 20px;
  }

  .auth-card-badge {
    top: 26px;
    right: 20px;
    width: 34px;
    height: 34px;
  }

  :deep(.auth-page-title) {
    font-size: 26px;
  }

  :deep(.auth-page-subtitle) {
    font-size: 12px;
  }
}

@media (min-width: 941px) and (max-height: 760px) {
  .auth-layout {
    padding-block: 22px;
  }

  .auth-brand-panel {
    min-height: 680px;
  }

  .auth-hero {
    margin-top: 42px;
  }

  .auth-network {
    height: 290px;
    margin-top: 22px;
    transform: scale(0.86) perspective(1100px) rotateX(var(--auth-scene-x)) rotateY(var(--auth-scene-y));
    transform-origin: left center;
  }

  .auth-card-shell {
    min-height: 610px;
  }

  .auth-focus-panel {
    padding-block: 58px 18px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-layout *,
  .auth-layout *::before,
  .auth-layout *::after {
    scroll-behavior: auto !important;
    animation-duration: 0.001ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.001ms !important;
  }

  .auth-network,
  .auth-theme-toggle:hover,
  :deep(.auth-submit:not(:disabled):hover) {
    transform: none;
  }
}
</style>
