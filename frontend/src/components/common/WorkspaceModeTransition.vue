<template>
  <div
    v-show="active"
    ref="overlayRef"
    class="workspace-mode-transition"
    role="status"
    aria-live="polite"
    :aria-label="statusText"
  >
    <div ref="leftPanelRef" class="workspace-mode-transition__panel is-left" aria-hidden="true">
      <span></span>
      <i></i>
      <i></i>
    </div>
    <div ref="rightPanelRef" class="workspace-mode-transition__panel is-right" aria-hidden="true">
      <span></span>
      <i></i>
      <i></i>
    </div>
    <div ref="centerRef" class="workspace-mode-transition__center">
      <span class="workspace-mode-transition__mark" aria-hidden="true">
        <Icon :name="direction === 'to-canvas' ? 'sparkles' : 'key'" size="md" />
      </span>
      <strong>{{ statusText }}</strong>
      <span class="workspace-mode-transition__track" aria-hidden="true"><i></i></span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { gsap } from 'gsap'
import Icon from '@/components/icons/Icon.vue'
import {
  registerWorkspaceModeTransitionRunner,
  type WorkspaceModeTransitionDirection,
} from '@/utils/workspaceModeTransition'

const props = defineProps<{
  routeStage: HTMLElement | null
}>()

const { t } = useI18n()
const overlayRef = ref<HTMLElement | null>(null)
const leftPanelRef = ref<HTMLElement | null>(null)
const rightPanelRef = ref<HTMLElement | null>(null)
const centerRef = ref<HTMLElement | null>(null)
const active = ref(false)
const direction = ref<WorkspaceModeTransitionDirection>('to-canvas')
const statusText = computed(() => t(`canvas.transition.${direction.value}`))

let timeline: gsap.core.Timeline | null = null
let media: gsap.MatchMedia | null = null
let reduceMotion = false
let unregisterRunner: (() => void) | null = null

function runTimeline(build: (instance: gsap.core.Timeline) => void) {
  return new Promise<void>((resolve) => {
    timeline?.kill()
    timeline = gsap.timeline({
      onComplete: () => {
        timeline = null
        resolve()
      },
    })
    build(timeline)
  })
}

function focusDestination() {
  const target = document.querySelector<HTMLElement>('#canvas-workspace-content, .app-main')
  if (!target) return
  const hadTabindex = target.hasAttribute('tabindex')
  if (!hadTabindex) target.setAttribute('tabindex', '-1')
  target.focus({ preventScroll: true })
  if (!hadTabindex) {
    requestAnimationFrame(() => target.removeAttribute('tabindex'))
  }
}

function resetVisualState() {
  timeline?.kill()
  timeline = null
  if (props.routeStage) gsap.set(props.routeStage, { clearProps: 'opacity,visibility,transform' })
  if (overlayRef.value) gsap.set(overlayRef.value, { autoAlpha: 0 })
  if (leftPanelRef.value) gsap.set(leftPanelRef.value, { xPercent: -102 })
  if (rightPanelRef.value) gsap.set(rightPanelRef.value, { xPercent: 102 })
  if (centerRef.value) gsap.set(centerRef.value, { autoAlpha: 0, scale: 0.96 })
  document.body.classList.remove('workspace-mode-transitioning')
  active.value = false
}

async function playTransition(request: {
  direction: WorkspaceModeTransitionDirection
  navigate: () => Promise<unknown>
  keepOverlay?: boolean
}) {
  direction.value = request.direction

  if (reduceMotion || !props.routeStage || !overlayRef.value || !leftPanelRef.value || !rightPanelRef.value || !centerRef.value) {
    await request.navigate()
    if (!request.keepOverlay) {
      await nextTick()
      focusDestination()
    }
    return
  }

  active.value = true
  document.body.classList.add('workspace-mode-transitioning')
  await nextTick()

  const stage = props.routeStage
  const travel = request.direction === 'to-canvas' ? -18 : 18
  gsap.set(overlayRef.value, { autoAlpha: 1 })
  gsap.set(leftPanelRef.value, { xPercent: -102 })
  gsap.set(rightPanelRef.value, { xPercent: 102 })
  gsap.set(centerRef.value, { autoAlpha: 0, scale: 0.96 })

  let keepOverlay = false
  try {
    await runTimeline((tl) => {
      tl.addLabel('close', 0)
        .to(stage, { autoAlpha: 0.42, x: travel, scale: 0.988, duration: 0.18, ease: 'power1.in' }, 'close')
        .to(leftPanelRef.value, { xPercent: 0, duration: 0.24, ease: 'power2.inOut' }, 'close')
        .to(rightPanelRef.value, { xPercent: 0, duration: 0.24, ease: 'power2.inOut' }, 'close')
        .to(centerRef.value, { autoAlpha: 1, scale: 1, duration: 0.16, ease: 'power2.out' }, 'close+=0.08')
    })

    await request.navigate()
    keepOverlay = request.keepOverlay === true
    if (keepOverlay) return
    await nextTick()

    gsap.set(stage, { autoAlpha: 0, x: -travel, scale: 0.992 })
    await runTimeline((tl) => {
      tl.addLabel('open', 0)
        .to(centerRef.value, { autoAlpha: 0, scale: 1.03, duration: 0.14, ease: 'power1.in' }, 'open')
        .to(leftPanelRef.value, { xPercent: -102, duration: 0.3, ease: 'power2.inOut' }, 'open+=0.08')
        .to(rightPanelRef.value, { xPercent: 102, duration: 0.3, ease: 'power2.inOut' }, 'open+=0.08')
        .to(stage, { autoAlpha: 1, x: 0, scale: 1, duration: 0.28, ease: 'power2.out' }, 'open+=0.12')
    })
    focusDestination()
  } finally {
    if (!keepOverlay) resetVisualState()
  }
}

async function playArrival(nextDirection: WorkspaceModeTransitionDirection) {
  direction.value = nextDirection
  sessionStorage.removeItem('modurelay-workspace-door')

  if (reduceMotion || !props.routeStage || !overlayRef.value || !leftPanelRef.value || !rightPanelRef.value || !centerRef.value) {
    focusDestination()
    return
  }

  active.value = true
  document.body.classList.add('workspace-mode-transitioning')
  await nextTick()

  const stage = props.routeStage
  gsap.set(overlayRef.value, { autoAlpha: 1 })
  gsap.set(leftPanelRef.value, { xPercent: 0 })
  gsap.set(rightPanelRef.value, { xPercent: 0 })
  gsap.set(centerRef.value, { autoAlpha: 0, scale: 1.03 })
  gsap.set(stage, { autoAlpha: 0, x: nextDirection === 'to-relay' ? -18 : 18, scale: 0.992 })

  try {
    await runTimeline((tl) => {
      tl.addLabel('open', 0)
        .to(centerRef.value, { autoAlpha: 0, scale: 1.03, duration: 0.14, ease: 'power1.in' }, 'open')
        .to(leftPanelRef.value, { xPercent: -102, duration: 0.3, ease: 'power2.inOut' }, 'open+=0.08')
        .to(rightPanelRef.value, { xPercent: 102, duration: 0.3, ease: 'power2.inOut' }, 'open+=0.08')
        .to(stage, { autoAlpha: 1, x: 0, scale: 1, duration: 0.28, ease: 'power2.out' }, 'open+=0.12')
    })
    focusDestination()
  } finally {
    resetVisualState()
  }
}

onMounted(() => {
  media = gsap.matchMedia()
  media.add('(prefers-reduced-motion: reduce)', () => {
    reduceMotion = true
    return () => { reduceMotion = false }
  })
  resetVisualState()
  unregisterRunner = registerWorkspaceModeTransitionRunner(playTransition)
  const arrival = sessionStorage.getItem('modurelay-workspace-door')
  if (arrival === 'to-relay' || arrival === 'to-canvas') {
    void playArrival(arrival)
  }
})

onBeforeUnmount(() => {
  unregisterRunner?.()
  unregisterRunner = null
  media?.revert()
  media = null
  resetVisualState()
})
</script>

<style scoped>
.workspace-mode-transition {
  position: fixed;
  z-index: 300;
  inset: 0;
  overflow: hidden;
  background: color-mix(in srgb, var(--color-bg-deep) 34%, transparent);
  pointer-events: auto;
  visibility: hidden;
}

.workspace-mode-transition__panel {
  position: absolute;
  top: 0;
  bottom: 0;
  width: calc(50% + 1px);
  overflow: hidden;
  border-color: var(--glass-border);
  background: var(--glass-fallback-strong);
  box-shadow: var(--shadow-overlay);
  will-change: transform;
}

.workspace-mode-transition__panel.is-left {
  left: 0;
  border-right: 1px solid var(--glass-border);
}

.workspace-mode-transition__panel.is-right {
  right: 0;
  border-left: 1px solid var(--glass-highlight);
}

.workspace-mode-transition__panel span,
.workspace-mode-transition__panel i {
  position: absolute;
  display: block;
  border: 1px solid var(--color-border-subtle);
  border-radius: 8px;
  background: var(--color-surface-soft);
}

.workspace-mode-transition__panel span {
  top: 18%;
  width: min(340px, 62%);
  height: 42%;
}

.workspace-mode-transition__panel i {
  bottom: 18%;
  width: min(180px, 34%);
  height: 8px;
}

.workspace-mode-transition__panel.is-left span,
.workspace-mode-transition__panel.is-left i { right: 12%; }
.workspace-mode-transition__panel.is-right span,
.workspace-mode-transition__panel.is-right i { left: 12%; }
.workspace-mode-transition__panel i:last-child { bottom: calc(18% - 22px); width: min(120px, 24%); }

.workspace-mode-transition__center {
  position: absolute;
  z-index: 2;
  top: 50%;
  left: 50%;
  display: flex;
  width: min(280px, calc(100% - 48px));
  align-items: center;
  flex-direction: column;
  gap: 12px;
  color: var(--color-text-primary);
  text-align: center;
  transform: translate(-50%, -50%);
  will-change: transform, opacity;
}

.workspace-mode-transition__mark {
  display: grid;
  width: 52px;
  height: 52px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 12px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.workspace-mode-transition__center strong {
  font-size: 14px;
  font-weight: 700;
  letter-spacing: 0;
}

.workspace-mode-transition__track {
  position: relative;
  width: 112px;
  height: 2px;
  overflow: hidden;
  border-radius: 2px;
  background: var(--color-border);
}

.workspace-mode-transition__track i {
  position: absolute;
  inset: 0;
  background: var(--color-primary);
  transform-origin: left center;
  animation: workspace-mode-progress 0.58s var(--ease-standard) both;
}

@keyframes workspace-mode-progress {
  from { transform: scaleX(0); }
  to { transform: scaleX(1); }
}

:global(body.workspace-mode-transitioning) {
  overflow: hidden;
}

@media (prefers-reduced-motion: reduce) {
  .workspace-mode-transition__track i { animation: none; }
}
</style>
