<template>
  <div
    v-show="active"
    ref="overlayRef"
    class="workspace-mode-transition"
    aria-hidden="true"
  >
    <div ref="leftPanelRef" class="workspace-mode-transition__panel is-left"></div>
    <div ref="rightPanelRef" class="workspace-mode-transition__panel is-right"></div>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { gsap } from 'gsap'
import {
  registerWorkspaceModeTransitionRunner,
  type WorkspaceModeTransitionDirection,
} from '@/utils/workspaceModeTransition'

defineProps<{
  routeStage: HTMLElement | null
}>()

const overlayRef = ref<HTMLElement | null>(null)
const leftPanelRef = ref<HTMLElement | null>(null)
const rightPanelRef = ref<HTMLElement | null>(null)
const active = ref(false)

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
  if (overlayRef.value) gsap.set(overlayRef.value, { autoAlpha: 0 })
  if (leftPanelRef.value) gsap.set(leftPanelRef.value, { xPercent: -102, '--workspace-door-blur': '0px' })
  if (rightPanelRef.value) gsap.set(rightPanelRef.value, { xPercent: 102, '--workspace-door-blur': '0px' })
  document.body.classList.remove('workspace-mode-transitioning')
  active.value = false
}

async function playTransition(request: {
  direction: WorkspaceModeTransitionDirection
  navigate: () => Promise<unknown>
  keepOverlay?: boolean
}) {
  if (reduceMotion || !overlayRef.value || !leftPanelRef.value || !rightPanelRef.value) {
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

  gsap.set(overlayRef.value, { autoAlpha: 1 })
  gsap.set(leftPanelRef.value, { xPercent: -102, '--workspace-door-blur': '0px' })
  gsap.set(rightPanelRef.value, { xPercent: 102, '--workspace-door-blur': '0px' })

  let keepOverlay = false
  try {
    await runTimeline((tl) => {
      tl.addLabel('close', 0)
        .to(leftPanelRef.value, { xPercent: 0, duration: 0.36, ease: 'power2.inOut' }, 'close')
        .to(rightPanelRef.value, { xPercent: 0, duration: 0.36, ease: 'power2.inOut' }, 'close')
        .to([leftPanelRef.value, rightPanelRef.value], { '--workspace-door-blur': '20px', duration: 0.34, ease: 'power1.inOut' }, 'close')
    })

    await request.navigate()
    keepOverlay = request.keepOverlay === true
    if (keepOverlay) return
    await nextTick()

    await runTimeline((tl) => {
      tl.addLabel('open', 0)
        .to([leftPanelRef.value, rightPanelRef.value], { '--workspace-door-blur': '0px', duration: 0.3, ease: 'power1.inOut' }, 'open')
        .to(leftPanelRef.value, { xPercent: -102, duration: 0.42, ease: 'power2.inOut' }, 'open')
        .to(rightPanelRef.value, { xPercent: 102, duration: 0.42, ease: 'power2.inOut' }, 'open')
    })
    focusDestination()
  } finally {
    if (!keepOverlay) resetVisualState()
  }
}

async function playArrival(nextDirection: WorkspaceModeTransitionDirection) {
  void nextDirection
  sessionStorage.removeItem('modurelay-workspace-door')

  if (reduceMotion || !overlayRef.value || !leftPanelRef.value || !rightPanelRef.value) {
    focusDestination()
    return
  }

  active.value = true
  document.body.classList.add('workspace-mode-transitioning')
  await nextTick()

  gsap.set(overlayRef.value, { autoAlpha: 1 })
  gsap.set(leftPanelRef.value, { xPercent: 0, '--workspace-door-blur': '20px' })
  gsap.set(rightPanelRef.value, { xPercent: 0, '--workspace-door-blur': '20px' })

  try {
    await runTimeline((tl) => {
      tl.addLabel('open', 0)
        .to([leftPanelRef.value, rightPanelRef.value], { '--workspace-door-blur': '0px', duration: 0.3, ease: 'power1.inOut' }, 'open')
        .to(leftPanelRef.value, { xPercent: -102, duration: 0.42, ease: 'power2.inOut' }, 'open')
        .to(rightPanelRef.value, { xPercent: 102, duration: 0.42, ease: 'power2.inOut' }, 'open')
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
  background: transparent;
  pointer-events: auto;
  visibility: hidden;
}

.workspace-mode-transition__panel {
  --workspace-door-blur: 0px;
  position: absolute;
  top: 0;
  bottom: 0;
  width: calc(50% + 1px);
  background: color-mix(in srgb, var(--glass-bg-strong) 62%, transparent);
  -webkit-backdrop-filter: blur(var(--workspace-door-blur)) saturate(1.06);
  backdrop-filter: blur(var(--workspace-door-blur)) saturate(1.06);
  box-shadow:
    inset 0 0 0 1px var(--glass-highlight),
    var(--shadow-overlay);
  will-change: transform, backdrop-filter;
}

.workspace-mode-transition__panel.is-left {
  left: 0;
  border-right: 1px solid var(--glass-border);
  transform-origin: left center;
}

.workspace-mode-transition__panel.is-right {
  right: 0;
  border-left: 1px solid var(--glass-highlight);
  transform-origin: right center;
}

:global(body.workspace-mode-transitioning) {
  overflow: hidden;
}

</style>
