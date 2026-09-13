<template>
  <BaseDialog
    :show="show"
    :title="t('canvas.editors.angleTitle')"
    width="wide"
    :close-on-escape="!processing"
    :show-close-button="!processing"
    @close="emit('close')"
  >
    <div class="canvas-angle-editor" data-canvas-no-zoom>
      <section class="canvas-angle-editor__preview">
        <div>
          <img :src="source" alt="" draggable="false" :style="{ transform: previewTransform }" />
          <span aria-hidden="true"></span>
        </div>
        <button type="button" :disabled="processing" @click="reset"><Icon name="refresh" size="sm" aria-hidden="true" />{{ t('canvas.editors.reset') }}</button>
      </section>

      <section class="canvas-angle-editor__controls">
        <label v-for="control in numericControls" :key="control.key">
          <span>{{ control.label }}</span>
          <input v-model.number="params[control.key]" type="range" :min="control.min" :max="control.max" :step="control.step" />
          <output>{{ displayValue(control.key, control.suffix) }}</output>
        </label>
        <div class="canvas-angle-editor__lens">
          <span>{{ t('canvas.editors.lens') }}</span>
          <div role="group" :aria-label="t('canvas.editors.lens')">
            <button type="button" :class="{ 'is-active': !params.wideAngle }" :aria-pressed="!params.wideAngle" @click="params.wideAngle = false">{{ t('canvas.editors.standard') }}</button>
            <button type="button" :class="{ 'is-active': params.wideAngle }" :aria-pressed="params.wideAngle" @click="params.wideAngle = true">{{ t('canvas.editors.wide') }}</button>
          </div>
        </div>
        <p>{{ t('canvas.editors.angleDescription') }}</p>
      </section>
    </div>

    <template #footer>
      <div class="canvas-angle-editor__footer">
        <button type="button" class="canvas-angle-editor__cancel" :disabled="processing" @click="emit('close')">{{ t('common.cancel') }}</button>
        <button type="button" class="canvas-angle-editor__generate" :disabled="processing || !source" :aria-busy="processing || undefined" @click="confirm">
          <Icon name="sparkles" size="sm" aria-hidden="true" />{{ processing ? t('common.processing') : t('canvas.editors.aiGenerate') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import {
  DEFAULT_CANVAS_IMAGE_ANGLE,
  canvasImageAnglePreviewTransform,
  normalizeCanvasImageAngle,
  type CanvasImageAngleParams,
} from '@/utils/canvasImageAngle'

const props = defineProps<{ show: boolean; source: string; processing?: boolean }>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'confirm', params: CanvasImageAngleParams): void
}>()
const { t } = useI18n()
const params = reactive<CanvasImageAngleParams>({ ...DEFAULT_CANVAS_IMAGE_ANGLE })
const numericControls = computed<Array<{
  key: 'horizontalAngle' | 'pitchAngle' | 'cameraDistance'
  label: string
  min: number
  max: number
  step: number
  suffix: string
}>>(() => [
  { key: 'horizontalAngle', label: t('canvas.editors.horizontal'), min: -60, max: 60, step: 1, suffix: 'deg' },
  { key: 'pitchAngle', label: t('canvas.editors.pitch'), min: -45, max: 45, step: 1, suffix: 'deg' },
  { key: 'cameraDistance', label: t('canvas.editors.distance'), min: 1, max: 10, step: 0.1, suffix: '' },
])
const previewTransform = computed(() => canvasImageAnglePreviewTransform(params))

watch(() => [props.show, props.source] as const, ([show]) => {
  if (show) reset()
})

function reset() {
  Object.assign(params, DEFAULT_CANVAS_IMAGE_ANGLE)
}

function displayValue(key: 'horizontalAngle' | 'pitchAngle' | 'cameraDistance', suffix: string) {
  const value = Number(params[key])
  return `${Number.isInteger(value) ? value : value.toFixed(1)}${suffix}`
}

function confirm() {
  emit('confirm', normalizeCanvasImageAngle(params))
}
</script>

<style scoped>
.canvas-angle-editor { display: grid; grid-template-columns: minmax(260px, 1fr) minmax(330px, .9fr); gap: 22px; color: var(--color-text-primary); }
.canvas-angle-editor__preview { display: flex; min-height: 340px; justify-content: space-between; flex-direction: column; gap: 16px; padding: 16px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-bg-subtle); overflow: hidden; }
.canvas-angle-editor__preview > div { position: relative; display: grid; flex: 1; place-items: center; perspective: 520px; }
.canvas-angle-editor__preview img { position: relative; z-index: 1; display: block; width: min(210px, 72%); aspect-ratio: 1; border-radius: 8px; object-fit: cover; box-shadow: var(--shadow-lg); transition: transform var(--motion-base) var(--ease-standard); }
.canvas-angle-editor__preview > div > span { position: absolute; bottom: 14%; left: 50%; width: 108px; height: 34px; border-radius: 50%; background: color-mix(in srgb, var(--color-bg-deep) 28%, transparent); filter: blur(5px); transform: translateX(-50%); }
.canvas-angle-editor__preview button, .canvas-angle-editor__lens button, .canvas-angle-editor__cancel, .canvas-angle-editor__generate { display: inline-flex; min-height: 40px; align-items: center; justify-content: center; gap: 7px; padding: 0 12px; border: 1px solid var(--color-border); border-radius: 8px; background: var(--color-surface); color: var(--color-text-primary); font-size: 13px; font-weight: 650; transition: border-color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard), color var(--motion-fast) var(--ease-standard); }
.canvas-angle-editor__preview button { align-self: flex-start; }
.canvas-angle-editor__controls { display: flex; min-width: 0; flex-direction: column; gap: 22px; padding: 8px 0; }
.canvas-angle-editor__controls label { display: grid; grid-template-columns: 92px minmax(0, 1fr) 58px; align-items: center; gap: 12px; color: var(--color-text-secondary); font-size: 13px; font-weight: 650; }
.canvas-angle-editor__controls output { color: var(--color-text-primary); font: 650 12px/1 var(--font-mono, monospace); text-align: right; }
.canvas-angle-editor__lens { display: grid; grid-template-columns: 92px minmax(0, 1fr); align-items: center; gap: 12px; color: var(--color-text-secondary); font-size: 13px; font-weight: 650; }
.canvas-angle-editor__lens > div { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 5px; padding: 3px; border-radius: 8px; background: var(--color-surface-soft); }
.canvas-angle-editor__lens button { min-height: 34px; border-color: transparent; background: transparent; }
.canvas-angle-editor__lens button.is-active { border-color: var(--color-primary-border); background: var(--color-surface); color: var(--color-primary); box-shadow: var(--shadow-xs); }
.canvas-angle-editor__controls p { margin: auto 0 0; color: var(--color-text-muted); font-size: 12px; line-height: 1.55; }
.canvas-angle-editor__footer { display: flex; justify-content: flex-end; gap: 9px; }
.canvas-angle-editor__cancel, .canvas-angle-editor__generate { min-width: 104px; }
.canvas-angle-editor__generate { border-color: var(--color-primary); background: var(--color-primary); color: var(--color-text-on-primary, #fff); }
.canvas-angle-editor button:disabled { cursor: not-allowed; opacity: .5; }
.canvas-angle-editor button:focus-visible, .canvas-angle-editor input:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
@media (max-width: 720px) {
  .canvas-angle-editor { grid-template-columns: 1fr; }
  .canvas-angle-editor__preview { min-height: 280px; }
}
@media (max-width: 430px) {
  .canvas-angle-editor__controls label { grid-template-columns: 76px minmax(0, 1fr) 52px; gap: 8px; }
  .canvas-angle-editor__lens { grid-template-columns: 76px minmax(0, 1fr); gap: 8px; }
}
@media (prefers-reduced-motion: reduce) {
  .canvas-angle-editor__preview img { transition-duration: .01ms; }
}
</style>
