<template>
  <BaseDialog
    :show="show"
    :title="t('canvas.editors.maskTitle')"
    width="extra-wide"
    :close-on-escape="!processing"
    :show-close-button="!processing"
    @close="emit('close')"
  >
    <div class="canvas-mask-editor" data-canvas-no-zoom>
      <div class="canvas-mask-editor__viewport">
        <div
          v-if="naturalWidth && naturalHeight"
          class="canvas-mask-editor__stage"
          :style="stageStyle"
        >
          <img :src="source" alt="" draggable="false" />
          <canvas
            ref="previewCanvasRef"
            :width="naturalWidth"
            :height="naturalHeight"
            class="canvas-mask-editor__canvas"
            :aria-label="t('canvas.editors.maskCanvas')"
            @pointerdown="startStroke"
            @pointermove="moveStroke"
            @pointerup="finishStroke"
            @pointercancel="finishStroke"
            @pointerleave="handlePointerLeave"
            @contextmenu.prevent
          ></canvas>
          <span v-if="brushPreview" class="canvas-mask-editor__brush-preview" :style="brushPreviewStyle" aria-hidden="true"></span>
        </div>
        <div v-else class="canvas-mask-editor__loading" role="status">{{ t('canvas.editors.loading') }}</div>
        <canvas ref="selectionCanvasRef" :width="naturalWidth" :height="naturalHeight" hidden></canvas>
        <img class="canvas-mask-editor__loader" :src="source" alt="" @load="readImageSize" @error="loadFailed = true" />
      </div>

      <aside class="canvas-mask-editor__controls">
        <div>
          <strong>{{ naturalWidth ? `${naturalWidth} x ${naturalHeight} px` : t('canvas.editors.loading') }}</strong>
          <p>{{ t('canvas.editors.maskHint') }}</p>
        </div>

        <div class="canvas-mask-editor__segments" role="group" :aria-label="t('canvas.editors.maskTool')">
          <button type="button" :class="{ 'is-active': mode === 'paint' }" :aria-pressed="mode === 'paint'" @click="mode = 'paint'">
            <Icon name="edit" size="sm" aria-hidden="true" />{{ t('canvas.editors.brush') }}
          </button>
          <button type="button" :class="{ 'is-active': mode === 'erase' }" :aria-pressed="mode === 'erase'" @click="mode = 'erase'">
            <Icon name="eraser" size="sm" aria-hidden="true" />{{ t('canvas.editors.erase') }}
          </button>
        </div>

        <div class="canvas-mask-editor__toolbar" role="toolbar" :aria-label="t('canvas.editors.maskHistory')">
          <button type="button" :disabled="!history.length" :title="t('canvas.editors.undoMask')" :aria-label="t('canvas.editors.undoMask')" @click="undoMask"><Icon name="undo" size="sm" /></button>
          <button type="button" :disabled="!redo.length" :title="t('canvas.editors.redoMask')" :aria-label="t('canvas.editors.redoMask')" @click="redoMask"><Icon name="redo" size="sm" /></button>
          <span aria-hidden="true"></span>
          <button type="button" :disabled="zoom <= 0.5" :title="t('canvas.editors.zoomOut')" :aria-label="t('canvas.editors.zoomOut')" @click="setZoom(zoom - 0.25)"><Icon name="minus" size="sm" /></button>
          <button type="button" class="canvas-mask-editor__zoom" :title="t('canvas.editors.resetZoom')" @click="setZoom(1)">{{ Math.round(zoom * 100) }}%</button>
          <button type="button" :disabled="zoom >= 2" :title="t('canvas.editors.zoomIn')" :aria-label="t('canvas.editors.zoomIn')" @click="setZoom(zoom + 0.25)"><Icon name="plus" size="sm" /></button>
        </div>

        <label class="canvas-mask-editor__range">
          <span>{{ t('canvas.editors.brushSize') }} <output>{{ brushSize }} px</output></span>
          <input v-model.number="brushSize" type="range" min="8" max="160" step="2" />
        </label>

        <label class="canvas-mask-editor__prompt">
          <span>{{ t('canvas.editors.editInstructions') }}</span>
          <textarea v-model="prompt" rows="6" maxlength="32000" :placeholder="t('canvas.editors.maskPlaceholder')" @input="error = ''"></textarea>
        </label>
        <p v-if="loadFailed || error" class="canvas-mask-editor__error" role="alert">{{ loadFailed ? t('canvas.editors.maskLoadFailed') : error }}</p>

        <button type="button" class="canvas-mask-editor__reset" :disabled="processing || !history.length" @click="resetMask">
          <Icon name="refresh" size="sm" aria-hidden="true" />{{ t('canvas.editors.reset') }}
        </button>
      </aside>
    </div>

    <template #footer>
      <div class="canvas-mask-editor__footer">
        <button type="button" class="canvas-mask-editor__cancel" :disabled="processing" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="canvas-mask-editor__secondary" :disabled="processing || exporting || !history.length" @click="submit(false)">
          <Icon name="download" size="sm" aria-hidden="true" />{{ t('canvas.editors.maskExport') }}
        </button>
        <button type="button" class="canvas-mask-editor__primary" :disabled="processing || exporting || !history.length" :aria-busy="processing || exporting || undefined" @click="submit(true)">
          <Icon name="sparkles" size="sm" aria-hidden="true" />{{ processing ? t('common.processing') : t('canvas.editors.maskGenerate') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { buildCanvasEditMask, normalizeCanvasMaskBrushSize, type CanvasImageMaskPayload } from '@/utils/canvasImageMask'

type DrawMode = 'paint' | 'erase'
type Point = { x: number; y: number }
type Stroke = { mode: DrawMode; size: number; points: Point[] }

const props = defineProps<{ show: boolean; source: string; processing?: boolean }>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'confirm', payload: CanvasImageMaskPayload): void
}>()
const { t } = useI18n()
const selectionCanvasRef = ref<HTMLCanvasElement | null>(null)
const previewCanvasRef = ref<HTMLCanvasElement | null>(null)
const naturalWidth = ref(0)
const naturalHeight = ref(0)
const mode = ref<DrawMode>('paint')
const brushSize = ref(100)
const zoom = ref(1)
const prompt = ref('')
const history = ref<Stroke[]>([])
const redo = ref<Stroke[]>([])
const drawing = ref<Stroke | null>(null)
const brushPreview = ref<{ x: number; y: number; size: number } | null>(null)
const error = ref('')
const loadFailed = ref(false)
const exporting = ref(false)

const stageStyle = computed(() => ({
  aspectRatio: `${naturalWidth.value} / ${naturalHeight.value}`,
  width: `${zoom.value * 100}%`,
}))
const brushPreviewStyle = computed(() => {
  const preview = brushPreview.value
  if (!preview) return undefined
  return {
    left: `${preview.x}px`,
    top: `${preview.y}px`,
    width: `${preview.size}px`,
    height: `${preview.size}px`,
  }
})

watch(() => [props.show, props.source] as const, ([show]) => {
  if (!show) return
  naturalWidth.value = 0
  naturalHeight.value = 0
  mode.value = 'paint'
  brushSize.value = 100
  zoom.value = 1
  prompt.value = ''
  error.value = ''
  loadFailed.value = false
  history.value = []
  redo.value = []
  drawing.value = null
  brushPreview.value = null
})

function readImageSize(event: Event) {
  const image = event.target as HTMLImageElement
  naturalWidth.value = image.naturalWidth
  naturalHeight.value = image.naturalHeight
}

function setZoom(value: number) {
  zoom.value = Math.min(2, Math.max(0.5, value))
}

function canvasPoint(canvas: HTMLCanvasElement, event: PointerEvent): Point {
  const rect = canvas.getBoundingClientRect()
  return {
    x: (event.clientX - rect.left) / Math.max(1, rect.width) * canvas.width,
    y: (event.clientY - rect.top) / Math.max(1, rect.height) * canvas.height,
  }
}

function updateBrushPreview(canvas: HTMLCanvasElement, event: PointerEvent) {
  const rect = canvas.getBoundingClientRect()
  brushPreview.value = {
    x: event.clientX - rect.left,
    y: event.clientY - rect.top,
    size: Math.max(4, normalizeCanvasMaskBrushSize(brushSize.value) * rect.width / Math.max(1, canvas.width)),
  }
}

function configureContext(context: CanvasRenderingContext2D, stroke: Stroke, preview: boolean) {
  context.lineCap = 'round'
  context.lineJoin = 'round'
  context.lineWidth = stroke.size
  context.globalCompositeOperation = stroke.mode === 'paint' ? 'source-over' : 'destination-out'
  context.strokeStyle = preview ? '#2563eb' : '#000'
  context.fillStyle = preview ? '#2563eb' : '#000'
}

function drawSegment(context: CanvasRenderingContext2D, from: Point, to: Point, size: number) {
  if (from.x === to.x && from.y === to.y) {
    context.beginPath()
    context.arc(to.x, to.y, size / 2, 0, Math.PI * 2)
    context.fill()
    return
  }
  context.beginPath()
  context.moveTo(from.x, from.y)
  context.lineTo(to.x, to.y)
  context.stroke()
}

function drawStrokeSegment(stroke: Stroke, point: Point) {
  const selectionContext = selectionCanvasRef.value?.getContext('2d', { willReadFrequently: true })
  const previewContext = previewCanvasRef.value?.getContext('2d')
  if (!selectionContext || !previewContext) return
  configureContext(selectionContext, stroke, false)
  configureContext(previewContext, stroke, true)
  const previous = stroke.points.at(-1) || point
  drawSegment(selectionContext, previous, point, stroke.size)
  drawSegment(previewContext, previous, point, stroke.size)
  stroke.points.push(point)
}

function startStroke(event: PointerEvent) {
  if (event.button !== 0 || props.processing) return
  const canvas = event.currentTarget as HTMLCanvasElement
  event.preventDefault()
  canvas.setPointerCapture(event.pointerId)
  updateBrushPreview(canvas, event)
  const stroke: Stroke = { mode: mode.value, size: normalizeCanvasMaskBrushSize(brushSize.value), points: [] }
  drawing.value = stroke
  drawStrokeSegment(stroke, canvasPoint(canvas, event))
  error.value = ''
}

function moveStroke(event: PointerEvent) {
  const canvas = event.currentTarget as HTMLCanvasElement
  updateBrushPreview(canvas, event)
  if (!drawing.value) return
  event.preventDefault()
  drawStrokeSegment(drawing.value, canvasPoint(canvas, event))
}

function finishStroke(event: PointerEvent) {
  const canvas = event.currentTarget as HTMLCanvasElement
  const stroke = drawing.value
  drawing.value = null
  if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId)
  if (!stroke?.points.length) return
  history.value = [...history.value, stroke]
  redo.value = []
}

function handlePointerLeave() {
  if (!drawing.value) brushPreview.value = null
}

function clearCanvases() {
  for (const canvas of [selectionCanvasRef.value, previewCanvasRef.value]) {
    canvas?.getContext('2d')?.clearRect(0, 0, canvas.width, canvas.height)
  }
}

function replayMask() {
  clearCanvases()
  for (const stroke of history.value) {
    const points = stroke.points
    const selectionContext = selectionCanvasRef.value?.getContext('2d', { willReadFrequently: true })
    const previewContext = previewCanvasRef.value?.getContext('2d')
    if (!selectionContext || !previewContext) return
    configureContext(selectionContext, stroke, false)
    configureContext(previewContext, stroke, true)
    points.forEach((point, index) => {
      const previous = points[index - 1] || point
      drawSegment(selectionContext, previous, point, stroke.size)
      drawSegment(previewContext, previous, point, stroke.size)
    })
  }
}

function undoMask() {
  const stroke = history.value.at(-1)
  if (!stroke || drawing.value) return
  history.value = history.value.slice(0, -1)
  redo.value = [...redo.value, stroke]
  replayMask()
}

function redoMask() {
  const stroke = redo.value.at(-1)
  if (!stroke || drawing.value) return
  redo.value = redo.value.slice(0, -1)
  history.value = [...history.value, stroke]
  replayMask()
}

function resetMask() {
  history.value = []
  redo.value = []
  drawing.value = null
  clearCanvases()
  error.value = ''
}

async function submit(generate: boolean) {
  if (generate && !prompt.value.trim()) {
    error.value = t('canvas.editors.maskPromptRequired')
    return
  }
  if (!selectionCanvasRef.value) return
  exporting.value = true
  try {
    const mask = await buildCanvasEditMask(selectionCanvasRef.value)
    emit('confirm', {
      prompt: prompt.value.trim(),
      mask,
      width: naturalWidth.value,
      height: naturalHeight.value,
      generate,
    })
  } catch (cause) {
    error.value = cause instanceof Error && cause.message === 'CANVAS_MASK_REQUIRED'
      ? t('canvas.editors.maskRequired')
      : t('canvas.editors.maskExportFailed')
  } finally {
    exporting.value = false
  }
}

function handleKeyboard(event: KeyboardEvent) {
  if (!props.show || event.altKey) return
  const target = event.target instanceof Element ? event.target : null
  if (target?.closest('input, textarea, [contenteditable="true"]')) return
  const key = event.key.toLowerCase()
  const modifier = event.ctrlKey || event.metaKey
  const undo = modifier && key === 'z' && !event.shiftKey
  const redoAction = modifier && (key === 'y' || (key === 'z' && event.shiftKey))
  if (!undo && !redoAction) return
  event.preventDefault()
  event.stopImmediatePropagation()
  if (redoAction) redoMask()
  else undoMask()
}

onMounted(() => window.addEventListener('keydown', handleKeyboard, true))
onBeforeUnmount(() => window.removeEventListener('keydown', handleKeyboard, true))
</script>

<style scoped>
.canvas-mask-editor { display: grid; grid-template-columns: minmax(0, 1fr) 300px; gap: 20px; min-width: 0; color: var(--color-text-primary); }
.canvas-mask-editor__viewport { position: relative; display: flex; min-width: 0; min-height: 440px; max-height: min(68vh, 720px); align-items: flex-start; justify-content: center; overflow: auto; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-bg-subtle); }
.canvas-mask-editor__stage { position: relative; flex: 0 0 auto; overflow: hidden; background: var(--color-bg-deep); transform-origin: top center; }
.canvas-mask-editor__stage img, .canvas-mask-editor__canvas { position: absolute; inset: 0; display: block; width: 100%; height: 100%; object-fit: contain; }
.canvas-mask-editor__canvas { cursor: crosshair; touch-action: none; opacity: .42; }
.canvas-mask-editor__brush-preview { position: absolute; z-index: 2; border: 1px solid var(--color-text-on-primary, #fff); border-radius: 50%; box-shadow: 0 0 0 1px var(--color-bg-deep); pointer-events: none; transform: translate(-50%, -50%); }
.canvas-mask-editor__loader { position: absolute; width: 1px; height: 1px; opacity: 0; pointer-events: none; }
.canvas-mask-editor__loading { display: grid; min-height: 440px; place-items: center; color: var(--color-text-muted); font-size: 13px; }
.canvas-mask-editor__controls { display: flex; min-width: 0; flex-direction: column; gap: 16px; }
.canvas-mask-editor__controls strong { font: 650 13px/1.4 var(--font-mono, monospace); }
.canvas-mask-editor__controls p { margin: 5px 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.55; }
.canvas-mask-editor__segments { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.canvas-mask-editor__segments button, .canvas-mask-editor__reset, .canvas-mask-editor__cancel, .canvas-mask-editor__secondary, .canvas-mask-editor__primary { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 7px; padding: 0 12px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface); color: var(--color-text-primary); font-size: 13px; font-weight: 650; transition: border-color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), color var(--motion-fast) var(--ease-standard); }
.canvas-mask-editor__segments button.is-active { border-color: var(--color-primary-border); background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-mask-editor__toolbar { display: grid; grid-template-columns: 36px 36px 1fr 36px 56px 36px; align-items: center; gap: 3px; padding: 3px; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: var(--color-surface-soft); }
.canvas-mask-editor__toolbar button { display: grid; height: 34px; place-items: center; border: 0; border-radius: 6px; background: transparent; color: var(--color-text-secondary); }
.canvas-mask-editor__toolbar button:not(:disabled):hover { background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-mask-editor__toolbar .canvas-mask-editor__zoom { font: 650 11px/1 var(--font-mono, monospace); }
.canvas-mask-editor__range, .canvas-mask-editor__prompt { display: grid; gap: 8px; color: var(--color-text-secondary); font-size: 13px; font-weight: 650; }
.canvas-mask-editor__range > span { display: flex; justify-content: space-between; gap: 12px; }
.canvas-mask-editor__prompt textarea { width: 100%; resize: vertical; min-height: 120px; padding: 10px 11px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface-soft); color: var(--color-text-primary); font: inherit; font-weight: 400; line-height: 1.5; }
.canvas-mask-editor__error { color: var(--color-danger) !important; font-weight: 650; }
.canvas-mask-editor__reset { align-self: flex-start; margin-top: auto; }
.canvas-mask-editor__footer { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: 9px; }
.canvas-mask-editor__cancel, .canvas-mask-editor__secondary, .canvas-mask-editor__primary { min-width: 104px; }
.canvas-mask-editor__primary { border-color: var(--color-primary); background: var(--color-primary); color: var(--color-text-on-primary, #fff); }
.canvas-mask-editor button:disabled { cursor: not-allowed; opacity: .5; }
.canvas-mask-editor button:focus-visible, .canvas-mask-editor textarea:focus-visible, .canvas-mask-editor input:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
@media (max-width: 820px) {
  .canvas-mask-editor { grid-template-columns: 1fr; }
  .canvas-mask-editor__viewport, .canvas-mask-editor__loading { min-height: 300px; }
  .canvas-mask-editor__controls { min-height: 360px; }
}
@media (max-width: 430px) {
  .canvas-mask-editor__viewport, .canvas-mask-editor__loading { min-height: 240px; }
  .canvas-mask-editor__toolbar { grid-template-columns: 36px 36px 1fr 36px 48px 36px; }
  .canvas-mask-editor__footer > button { min-width: 0; flex: 1 1 120px; }
}
</style>
