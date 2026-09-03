<template>
  <div
    ref="stageRef"
    class="sylva-scene"
    :class="{ 'sylva-scene-ready': sourceReady }"
    :style="scenePresentation"
    role="img"
    aria-label="ModuRelay living relay landscape"
  >
    <iframe
      v-if="!hasFallback"
      ref="frameRef"
      class="sylva-scene-frame"
      :src="SOURCE_URL"
      title="ThreeUI Sylva Living Green renderer"
      sandbox="allow-same-origin allow-scripts"
      loading="eager"
      tabindex="-1"
      aria-hidden="true"
      @load="handleFrameLoad"
      @error="handleFrameError"
    ></iframe>

    <div v-else class="sylva-scene-fallback" aria-hidden="true">
      <div class="sylva-fallback-landscape">
        <i class="sylva-fallback-root sylva-fallback-root-one"></i>
        <i class="sylva-fallback-root sylva-fallback-root-two"></i>
        <i class="sylva-fallback-leaf sylva-fallback-leaf-one"></i>
        <i class="sylva-fallback-leaf sylva-fallback-leaf-two"></i>
        <i class="sylva-fallback-leaf sylva-fallback-leaf-three"></i>
      </div>
      <div class="sylva-fallback-core">
        <span>M</span>
        <i></i>
      </div>
    </div>

    <div class="sylva-scene-tone" aria-hidden="true"></div>
    <div class="sylva-scene-depth" aria-hidden="true"></div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, type CSSProperties } from 'vue'

const SOURCE_URL = '/threeui/sylva/inner-green-3d.html'
const PRESENTATION_STYLE_ID = 'modurelay-sylva-scene-presentation'
const READY_TIMEOUT_MS = 16_000

withDefaults(defineProps<{ progress?: number }>(), { progress: 0 })
const emit = defineEmits<{ ready: [] }>()

const stageRef = ref<HTMLElement | null>(null)
const frameRef = ref<HTMLIFrameElement | null>(null)
const hasFallback = ref(false)
const sourceReady = ref(false)

let readyEmitted = false
let readyPollTimer: number | null = null
let readyDeadline = 0
let sourceCanvas: HTMLCanvasElement | null = null
let pointerFrame = 0
let pendingPointer: {
  clientX: number
  clientY: number
  pointerId: number
  pointerType: string
  isPrimary: boolean
} | null = null

const scenePresentation = computed(() => {
  return {
    '--sylva-opacity': '1',
    '--sylva-scale': '1',
    '--sylva-shift': '0vh',
  } as CSSProperties
})

function supportsWebGL(): boolean {
  try {
    const canvas = document.createElement('canvas')
    const context = canvas.getContext('webgl2') || canvas.getContext('webgl')
    context?.getExtension?.('WEBGL_lose_context')?.loseContext()
    return Boolean(context)
  } catch {
    return false
  }
}

function finishReady() {
  if (readyEmitted) return
  readyEmitted = true
  emit('ready')
}

function clearReadyPoll() {
  if (readyPollTimer !== null) {
    window.clearTimeout(readyPollTimer)
    readyPollTimer = null
  }
}

function releaseFrameDocument() {
  clearReadyPoll()
  cancelAnimationFrame(pointerFrame)
  pointerFrame = 0
  pendingPointer = null
  sourceCanvas?.removeEventListener('webglcontextlost', handleContextLoss)
  sourceCanvas = null

  const frame = frameRef.value
  if (frame) frame.src = 'about:blank'
}

function useFallback(error?: unknown) {
  if (hasFallback.value) return
  if (error) console.warn('[HomeHeroScene] Sylva source unavailable; using the static fallback.', error)
  releaseFrameDocument()
  sourceReady.value = false
  hasFallback.value = true
  finishReady()
}

function installScenePresentation(frameDocument: Document) {
  frameDocument.documentElement.setAttribute('data-modurelay-presentation', 'scene')

  let style = frameDocument.getElementById(PRESENTATION_STYLE_ID) as HTMLStyleElement | null
  if (!style) {
    style = frameDocument.createElement('style')
    style.id = PRESENTATION_STYLE_ID
    frameDocument.head.appendChild(style)
  }

  style.textContent = `
    html[data-modurelay-presentation="scene"],
    html[data-modurelay-presentation="scene"] body {
      width: 100% !important;
      height: 100% !important;
      min-height: 100% !important;
      overflow: hidden !important;
      background: #4a4d44 !important;
    }
    html[data-modurelay-presentation="scene"] body * {
      visibility: hidden !important;
      pointer-events: none !important;
    }
    html[data-modurelay-presentation="scene"] #hero {
      position: fixed !important;
      inset: 0 !important;
      width: 100vw !important;
      height: 100vh !important;
      min-height: 100vh !important;
      overflow: hidden !important;
      visibility: visible !important;
      background: #4a4d44 !important;
    }
    html[data-modurelay-presentation="scene"] #hero::before,
    html[data-modurelay-presentation="scene"] #hero::after {
      display: none !important;
    }
    html[data-modurelay-presentation="scene"] #scene {
      position: absolute !important;
      inset: 0 !important;
      width: 100% !important;
      height: 100% !important;
      visibility: visible !important;
      opacity: 1 !important;
      pointer-events: none !important;
    }
  `
}

function pollForSourceReady() {
  clearReadyPoll()
  const frame = frameRef.value
  const view = frame?.contentWindow as (Window & { __ready?: boolean }) | null

  if (view?.__ready) {
    sourceReady.value = true
    finishReady()
    return
  }

  if (performance.now() >= readyDeadline) {
    useFallback(new Error('Timed out while waiting for the verified Sylva renderer.'))
    return
  }

  readyPollTimer = window.setTimeout(pollForSourceReady, 100)
}

function handleContextLoss(event: Event) {
  event.preventDefault()
  useFallback(new Error('The Sylva WebGL context was lost.'))
}

function handleFrameLoad() {
  const frame = frameRef.value
  const frameDocument = frame?.contentDocument
  if (!frame || !frameDocument || frame.src === 'about:blank') return

  try {
    installScenePresentation(frameDocument)
    sourceCanvas = frameDocument.querySelector<HTMLCanvasElement>('#scene')
    if (!sourceCanvas) throw new Error('The verified Sylva canvas was not found.')

    sourceCanvas.addEventListener('webglcontextlost', handleContextLoss, { once: true })
    frame.contentWindow?.dispatchEvent(new Event('resize'))
    readyDeadline = performance.now() + READY_TIMEOUT_MS
    pollForSourceReady()
  } catch (error) {
    useFallback(error)
  }
}

function handleFrameError() {
  useFallback(new Error('The verified Sylva document failed to load.'))
}

function dispatchFramePointer(type: 'pointermove' | 'pointerleave', pointer?: typeof pendingPointer) {
  const view = frameRef.value?.contentWindow as any
  if (!view) return

  try {
    if (type === 'pointermove' && pointer && view.PointerEvent) {
      view.dispatchEvent(new view.PointerEvent(type, {
        clientX: pointer.clientX,
        clientY: pointer.clientY,
        pointerId: pointer.pointerId,
        pointerType: pointer.pointerType,
        isPrimary: pointer.isPrimary,
      }))
    } else {
      view.dispatchEvent(new view.Event(type))
    }
  } catch {
    // The scene remains usable without pointer parallax when a browser blocks cross-frame events.
  }
}

function forwardPointer(event: PointerEvent) {
  pendingPointer = {
    clientX: event.clientX,
    clientY: event.clientY,
    pointerId: event.pointerId,
    pointerType: event.pointerType,
    isPrimary: event.isPrimary,
  }
  if (pointerFrame) return
  pointerFrame = requestAnimationFrame(() => {
    pointerFrame = 0
    dispatchFramePointer('pointermove', pendingPointer)
  })
}

function forwardPointerLeave() {
  cancelAnimationFrame(pointerFrame)
  pointerFrame = 0
  pendingPointer = null
  dispatchFramePointer('pointerleave')
}

onMounted(() => {
  if (!stageRef.value || !supportsWebGL()) {
    useFallback()
    return
  }

  window.addEventListener('pointermove', forwardPointer, { passive: true })
  window.addEventListener('pointerleave', forwardPointerLeave)
})

onBeforeUnmount(() => {
  window.removeEventListener('pointermove', forwardPointer)
  window.removeEventListener('pointerleave', forwardPointerLeave)
  releaseFrameDocument()
})
</script>

<style scoped>
.sylva-scene {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background: #4a4d44;
  color-scheme: dark;
  contain: layout paint;
  isolation: isolate;
}

.sylva-scene-frame,
.sylva-scene-fallback,
.sylva-scene-tone,
.sylva-scene-depth {
  position: absolute;
  inset: 0;
}

.sylva-scene-frame {
  display: block;
  width: 100%;
  height: 100%;
  border: 0;
  opacity: 0;
  background: #4a4d44;
  backface-visibility: hidden;
  contain: layout paint size;
  filter: saturate(1.28) contrast(1.08) brightness(0.98);
  pointer-events: none;
  transform: translateY(var(--sylva-shift)) scale(var(--sylva-scale));
  transform-origin: center 58%;
}

.sylva-scene-ready .sylva-scene-frame {
  opacity: var(--sylva-opacity);
}

.sylva-scene-tone,
.sylva-scene-depth {
  z-index: 2;
  pointer-events: none;
}

.sylva-scene-tone {
  background: rgba(7, 12, 10, 0.04);
  box-shadow: inset 0 0 16vw rgba(6, 11, 13, 0.26);
}

.sylva-scene-depth {
  box-shadow:
    inset 30vw 0 24vw -25vw rgba(5, 10, 12, 0.34),
    inset 0 -20vh 20vh -18vh rgba(5, 10, 12, 0.2);
}

.sylva-scene-fallback {
  display: grid;
  place-items: center;
  opacity: var(--sylva-opacity);
  background: #102a1f;
}

.sylva-fallback-landscape {
  position: absolute;
  inset: 0;
  overflow: hidden;
  background:
    radial-gradient(ellipse at 72% 64%, rgba(110, 175, 93, 0.44), transparent 24%),
    radial-gradient(ellipse at 26% 74%, rgba(49, 114, 73, 0.72), transparent 30%),
    #102a1f;
}

.sylva-fallback-root {
  position: absolute;
  display: block;
  width: 74%;
  height: 25%;
  border: 34px solid rgba(52, 82, 50, 0.88);
  border-top-color: transparent;
  border-radius: 50%;
  box-shadow: inset 0 -8px rgba(116, 150, 90, 0.28), 0 22px 34px rgba(2, 10, 7, 0.34);
}
.sylva-fallback-root-one { right: -18%; bottom: 13%; transform: rotate(-17deg); }
.sylva-fallback-root-two { bottom: 7%; left: -24%; transform: rotate(12deg) scale(0.82); }

.sylva-fallback-leaf {
  position: absolute;
  display: block;
  width: 28px;
  height: 120px;
  border: 1px solid rgba(181, 225, 155, 0.42);
  border-radius: 80% 10% 80% 10%;
  background: rgba(81, 150, 83, 0.52);
  transform-origin: bottom;
}
.sylva-fallback-leaf-one { right: 22%; bottom: 18%; transform: rotate(24deg); }
.sylva-fallback-leaf-two { right: 29%; bottom: 16%; transform: rotate(-12deg) scale(0.78); }
.sylva-fallback-leaf-three { bottom: 15%; left: 25%; transform: rotate(-28deg) scale(0.9); }

.sylva-fallback-core {
  position: relative;
  z-index: 1;
  display: grid;
  width: 176px;
  aspect-ratio: 1;
  place-items: center;
  border: 1px solid rgba(129, 140, 248, 0.58);
  border-radius: 50%;
  color: #f5f7fb;
  background: rgba(19, 23, 32, 0.82);
  box-shadow: 0 20px 48px rgba(0, 0, 0, 0.34), inset 0 0 24px rgba(34, 211, 238, 0.08);
  font: 600 54px/1 ui-monospace, SFMono-Regular, Menlo, monospace;
}

.sylva-fallback-core i {
  position: absolute;
  inset: 22px;
  border: 1px solid rgba(34, 211, 238, 0.34);
  border-radius: 50%;
}

@media (max-width: 700px) {
  .sylva-scene-frame {
    transform: translateY(calc(var(--sylva-shift) + 15vh)) scale(1.02);
    transform-origin: center 66%;
  }

  .sylva-scene-depth {
    box-shadow:
      inset 0 32vh 18vh -16vh rgba(5, 10, 12, 0.58),
      inset 0 -16vh 18vh -14vh rgba(5, 10, 12, 0.42);
  }

  .sylva-fallback-core {
    width: 132px;
    margin-top: 24vh;
    font-size: 42px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .sylva-scene-frame {
    transform: none;
  }
}

@media (max-width: 700px) and (prefers-reduced-motion: reduce) {
  .sylva-scene-frame {
    transform: translateY(15vh) scale(1.02);
  }
}
</style>
