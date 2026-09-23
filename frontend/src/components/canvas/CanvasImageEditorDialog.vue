<template>
  <BaseDialog
    :show="show"
    :title="t('canvas.editors.title')"
    width="wide"
    :close-on-escape="!processing"
    :show-close-button="!processing"
    @close="emit('close')"
  >
    <div class="canvas-image-editor" data-canvas-no-zoom>
      <div class="canvas-image-editor__tabs" role="tablist" :aria-label="t('canvas.editors.mode')">
        <button v-for="item in modes" :key="item.id" type="button" role="tab" :aria-selected="mode === item.id" :class="{ 'is-active': mode === item.id }" @click="mode = item.id">
          <Icon :name="item.icon" size="sm" aria-hidden="true" />{{ item.label }}
        </button>
      </div>

      <div class="canvas-image-editor__workspace">
        <div class="canvas-image-editor__preview">
          <div class="canvas-image-editor__stage">
            <img :src="source" alt="" draggable="false" :style="previewStyle" @load="readImageSize" />
            <div v-if="mode === 'crop'" class="canvas-image-editor__crop-frame" :style="cropFrameStyle" aria-hidden="true"></div>
            <div v-if="mode === 'split'" class="canvas-image-editor__split-grid" :style="splitGridStyle" aria-hidden="true"></div>
          </div>
          <span>{{ sourceSizeLabel }}</span>
        </div>

        <div class="canvas-image-editor__controls">
          <template v-if="mode === 'crop'">
            <label><span>{{ t('canvas.editors.cropWidth') }} <output>{{ cropWidth }}%</output></span><input v-model.number="cropWidth" type="range" min="10" max="100" step="1" /></label>
            <label><span>{{ t('canvas.editors.cropHeight') }} <output>{{ cropHeight }}%</output></span><input v-model.number="cropHeight" type="range" min="10" max="100" step="1" /></label>
          </template>
          <template v-else-if="mode === 'rotate'">
            <div class="canvas-image-editor__button-grid">
              <button type="button" @click="rotate(-90)"><Icon name="undo" size="sm" />{{ t('canvas.editors.rotateLeft') }}</button>
              <button type="button" @click="rotate(90)"><Icon name="redo" size="sm" />{{ t('canvas.editors.rotateRight') }}</button>
            </div>
            <label class="canvas-image-editor__check"><input v-model="flipX" type="checkbox" />{{ t('canvas.editors.flipHorizontal') }}</label>
            <label class="canvas-image-editor__check"><input v-model="flipY" type="checkbox" />{{ t('canvas.editors.flipVertical') }}</label>
          </template>
          <template v-else-if="mode === 'upscale'">
            <span class="canvas-image-editor__label">{{ t('canvas.editors.scale') }}</span>
            <div class="canvas-image-editor__segments">
              <button v-for="value in ([1, 2, 4] as const)" :key="value" type="button" :class="{ 'is-active': scale === value }" @click="scale = value">{{ value }}x</button>
            </div>
            <p>{{ t('canvas.editors.upscaleHint') }}</p>
          </template>
          <template v-else>
            <label><span>{{ t('canvas.editors.rows') }}</span><input v-model.number="rows" type="number" min="1" max="6" /></label>
            <label><span>{{ t('canvas.editors.columns') }}</span><input v-model.number="columns" type="number" min="1" max="6" /></label>
            <p>{{ t('canvas.editors.splitCount', { count: splitCount }) }}</p>
          </template>

          <div class="canvas-image-editor__output">
            <span>{{ t('canvas.editors.outputSize') }}</span>
            <strong>{{ outputSizeLabel }}</strong>
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="canvas-image-editor__footer">
        <button type="button" class="canvas-image-editor__cancel" :disabled="processing" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="canvas-image-editor__apply" :disabled="processing || !source" :aria-busy="processing || undefined" @click="apply">
          <Icon name="check" size="sm" aria-hidden="true" />{{ processing ? t('common.processing') : t('canvas.editors.apply') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { CanvasImageEditOperation } from '@/utils/canvasImageEditor'

type EditorMode = 'crop' | 'rotate' | 'upscale' | 'split'

const props = defineProps<{ show: boolean; source: string; processing?: boolean }>()
const emit = defineEmits<{ (event: 'close'): void; (event: 'apply', operation: CanvasImageEditOperation): void }>()
const { t } = useI18n()
const mode = ref<EditorMode>('crop')
const cropWidth = ref(100)
const cropHeight = ref(100)
const rotation = ref<0 | 90 | 180 | 270>(0)
const flipX = ref(false)
const flipY = ref(false)
const scale = ref<1 | 2 | 4>(2)
const rows = ref(2)
const columns = ref(2)
const naturalWidth = ref(0)
const naturalHeight = ref(0)

const modes = computed<Array<{ id: EditorMode; label: string; icon: 'focus' | 'refresh' | 'fitScreen' | 'grid' }>>(() => [
  { id: 'crop', label: t('canvas.editors.crop'), icon: 'focus' },
  { id: 'rotate', label: t('canvas.editors.rotate'), icon: 'refresh' },
  { id: 'upscale', label: t('canvas.editors.upscale'), icon: 'fitScreen' },
  { id: 'split', label: t('canvas.editors.split'), icon: 'grid' },
])
const splitCount = computed(() => Math.min(6, Math.max(1, Math.round(rows.value || 1))) * Math.min(6, Math.max(1, Math.round(columns.value || 1))))
const sourceSizeLabel = computed(() => naturalWidth.value ? `${naturalWidth.value} x ${naturalHeight.value} px` : t('canvas.editors.loading'))
const outputSizeLabel = computed(() => {
  if (!naturalWidth.value) return t('canvas.editors.loading')
  if (mode.value === 'split') return t('canvas.editors.pieces', { count: splitCount.value })
  let width = naturalWidth.value
  let height = naturalHeight.value
  if (mode.value === 'crop') {
    width *= cropWidth.value / 100
    height *= cropHeight.value / 100
  }
  if (mode.value === 'rotate' && (rotation.value === 90 || rotation.value === 270)) [width, height] = [height, width]
  if (mode.value === 'upscale') {
    width *= scale.value
    height *= scale.value
  }
  return `${Math.max(1, Math.round(width))} x ${Math.max(1, Math.round(height))} px`
})
const previewStyle = computed(() => mode.value === 'rotate' ? { transform: `rotate(${rotation.value}deg) scale(${flipX.value ? -1 : 1}, ${flipY.value ? -1 : 1})` } : undefined)
const cropFrameStyle = computed(() => ({ width: `${cropWidth.value}%`, height: `${cropHeight.value}%` }))
const splitGridStyle = computed(() => ({
  backgroundSize: `${100 / Math.max(1, columns.value)}% ${100 / Math.max(1, rows.value)}%`,
}))

watch(() => props.show, (show) => {
  if (!show) return
  mode.value = 'crop'
  cropWidth.value = 100
  cropHeight.value = 100
  rotation.value = 0
  flipX.value = false
  flipY.value = false
  scale.value = 2
  rows.value = 2
  columns.value = 2
})

function readImageSize(event: Event) {
  const image = event.target as HTMLImageElement
  naturalWidth.value = image.naturalWidth
  naturalHeight.value = image.naturalHeight
}

function rotate(delta: -90 | 90) {
  rotation.value = ((rotation.value + delta + 360) % 360) as 0 | 90 | 180 | 270
}

function apply() {
  if (mode.value === 'split') {
    emit('apply', { mode: 'split', rows: rows.value, columns: columns.value })
    return
  }
  emit('apply', {
    mode: 'transform',
    cropWidth: mode.value === 'crop' ? cropWidth.value : 100,
    cropHeight: mode.value === 'crop' ? cropHeight.value : 100,
    rotation: mode.value === 'rotate' ? rotation.value : 0,
    flipX: mode.value === 'rotate' && flipX.value,
    flipY: mode.value === 'rotate' && flipY.value,
    scale: mode.value === 'upscale' ? scale.value : 1,
  })
}
</script>

<style scoped>
.canvas-image-editor { display: grid; gap: 18px; color: var(--color-text-primary); }
.canvas-image-editor__tabs { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 6px; padding: 4px; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: var(--color-surface-soft); }
.canvas-image-editor__tabs button,
.canvas-image-editor__button-grid button,
.canvas-image-editor__segments button,
.canvas-image-editor__cancel,
.canvas-image-editor__apply { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 7px; border: 1px solid transparent; border-radius: 8px; font-size: 13px; font-weight: 650; transition: border-color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), color var(--motion-fast) var(--ease-standard); }
.canvas-image-editor__tabs button { color: var(--color-text-secondary); }
.canvas-image-editor__tabs button.is-active { border-color: var(--color-primary-border); background: var(--color-surface); color: var(--color-primary); box-shadow: var(--shadow-xs); }
.canvas-image-editor__workspace { display: grid; grid-template-columns: minmax(0, 1.35fr) minmax(260px, .65fr); gap: 20px; min-width: 0; }
.canvas-image-editor__preview { display: grid; min-width: 0; gap: 9px; color: var(--color-text-muted); font-size: 12px; text-align: center; }
.canvas-image-editor__stage { position: relative; display: grid; min-height: 360px; place-items: center; overflow: hidden; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-bg-subtle); }
.canvas-image-editor__stage img { display: block; max-width: 100%; max-height: 440px; object-fit: contain; transition: transform var(--motion-base) var(--ease-standard); }
.canvas-image-editor__crop-frame { position: absolute; inset: 50% auto auto 50%; border: 2px solid var(--color-primary); box-shadow: 0 0 0 999px color-mix(in srgb, var(--color-bg-deep) 46%, transparent); transform: translate(-50%, -50%); pointer-events: none; }
.canvas-image-editor__split-grid { position: absolute; inset: 0; background-image: linear-gradient(to right, var(--color-primary) 1px, transparent 1px), linear-gradient(to bottom, var(--color-primary) 1px, transparent 1px); pointer-events: none; }
.canvas-image-editor__controls { display: flex; min-width: 0; flex-direction: column; gap: 16px; }
.canvas-image-editor__controls label:not(.canvas-image-editor__check) { display: grid; gap: 8px; color: var(--color-text-secondary); font-size: 13px; font-weight: 650; }
.canvas-image-editor__controls label > span { display: flex; justify-content: space-between; gap: 12px; }
.canvas-image-editor__controls input[type='number'] { min-height: 40px; padding: 0 11px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface-soft); color: var(--color-text-primary); }
.canvas-image-editor__controls input:focus-visible,
.canvas-image-editor button:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
.canvas-image-editor__button-grid,
.canvas-image-editor__segments { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
.canvas-image-editor__segments { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.canvas-image-editor__button-grid button,
.canvas-image-editor__segments button,
.canvas-image-editor__cancel { border-color: var(--color-border); background: var(--color-surface); color: var(--color-text-primary); }
.canvas-image-editor__segments button.is-active { border-color: var(--color-primary-border); background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-image-editor__check { display: flex; min-height: 40px; align-items: center; gap: 9px; color: var(--color-text-secondary); font-size: 13px; }
.canvas-image-editor__controls p { margin: 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.55; }
.canvas-image-editor__label { color: var(--color-text-secondary); font-size: 13px; font-weight: 650; }
.canvas-image-editor__output { display: grid; gap: 5px; margin-top: auto; padding: 12px; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: var(--color-surface-soft); }
.canvas-image-editor__output span { color: var(--color-text-muted); font-size: 12px; }
.canvas-image-editor__output strong { font-size: 14px; overflow-wrap: anywhere; }
.canvas-image-editor__footer { display: flex; justify-content: flex-end; gap: 10px; }
.canvas-image-editor__cancel,
.canvas-image-editor__apply { min-width: 96px; padding: 0 14px; }
.canvas-image-editor__apply { background: var(--color-primary); color: white; }
.canvas-image-editor button:disabled { cursor: not-allowed; opacity: .58; }
@media (max-width: 720px) {
  .canvas-image-editor__tabs { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .canvas-image-editor__workspace { grid-template-columns: 1fr; }
  .canvas-image-editor__stage { min-height: 240px; }
}
@media (prefers-reduced-motion: reduce) {
  .canvas-image-editor__stage img { transition-duration: .01ms; }
}
</style>
