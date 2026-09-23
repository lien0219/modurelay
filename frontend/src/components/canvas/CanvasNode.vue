<template>
  <article
    class="canvas-node"
    :class="[
      `canvas-node--${type}`,
      {
        'canvas-node--selected': selected,
        'canvas-node--busy': isBusy,
         'canvas-node--error': isError,
         'canvas-node--collapsed': Boolean(data.collapsed),
         'canvas-node--drop-target': Boolean(data.isGroupDropTarget),
      },
    ]"
    :style="nodeStyle"
    :aria-label="label"
    :data-node-id="id"
    @contextmenu.prevent.stop="invokeContextMenu"
  >
    <Handle
      v-if="showGenericInput"
      :id="CANVAS_HANDLES.genericInput"
      type="target"
      :position="Position.Left"
      :connectable="connectable"
      class="canvas-handle canvas-handle--generic"
      :style="{ top: genericInputTop }"
      :aria-label="t('canvas.handles.genericInput')"
      :title="t('canvas.handles.genericInput')"
    />
    <Handle
      v-if="type === 'generation'"
      :id="CANVAS_HANDLES.generationPrompt"
      type="target"
      :position="Position.Left"
      :connectable="connectable"
      class="canvas-handle canvas-handle--prompt"
      :style="{ top: '46%' }"
      :aria-label="t('canvas.handles.promptInput')"
      :title="t('canvas.handles.promptInput')"
    />
    <Handle
      v-if="type === 'generation'"
      :id="CANVAS_HANDLES.generationReference"
      type="target"
      :position="Position.Left"
      :connectable="connectable"
      class="canvas-handle canvas-handle--reference"
      :style="{ top: '74%' }"
      :aria-label="t('canvas.handles.referenceInput')"
      :title="t('canvas.handles.referenceInput')"
    />
    <Handle
      v-if="type === 'image'"
      :id="CANVAS_HANDLES.imageInput"
      type="target"
      :position="Position.Left"
      :connectable="connectable"
      class="canvas-handle canvas-handle--image"
      :style="{ top: '52%' }"
      :aria-label="t('canvas.handles.imageInput')"
      :title="t('canvas.handles.imageInput')"
    />

    <div class="canvas-node__hover-tools nodrag nopan" role="toolbar" :aria-label="t('canvas.nodeToolbar')">
      <button type="button" :title="t('canvas.duplicate')" :aria-label="t('canvas.duplicate')" @click.stop="invokeAction('onDuplicate')">
        <Icon name="copy" size="xs" />
      </button>
      <button type="button" :title="t('canvas.properties')" :aria-label="t('canvas.properties')" @click.stop="invokeAction('onOpenProperties')">
        <Icon name="cog" size="xs" />
      </button>
      <button type="button" class="is-danger" :title="t('canvas.deleteNode')" :aria-label="t('canvas.deleteNode')" @click.stop="invokeAction('onDelete')">
        <Icon name="trash" size="xs" />
      </button>
    </div>

    <header class="canvas-node__header canvas-node__drag">
      <span class="canvas-node__icon" aria-hidden="true"><Icon :name="nodeIcon" size="sm" /></span>
      <span class="canvas-node__heading">
        <span class="canvas-node__kind">{{ kindLabel }}</span>
        <strong>{{ label }}</strong>
      </span>
      <span v-if="showsStatus" class="canvas-node__status" :class="`canvas-node__status--${statusTone}`">
        <span class="canvas-node__status-dot" aria-hidden="true"></span>{{ statusLabel }}
      </span>
      <button
        v-if="type === 'group'"
        type="button"
        class="canvas-node__header-action nodrag nopan"
        :title="data.collapsed ? t('canvas.expandGroup') : t('canvas.collapseGroup')"
        :aria-label="data.collapsed ? t('canvas.expandGroup') : t('canvas.collapseGroup')"
        @click.stop="toggleGroup"
      >
        <Icon :name="data.collapsed ? 'chevronDown' : 'chevronUp'" size="xs" />
      </button>
    </header>

    <div v-if="type === 'prompt' && !data.collapsed" class="canvas-node__content canvas-node__text-editor">
      <label class="sr-only" :for="`canvas-prompt-${id}`">{{ t('canvas.prompt') }}</label>
      <textarea
        :id="`canvas-prompt-${id}`"
        class="nodrag nopan nowheel"
        :value="String(data.prompt || '')"
        :placeholder="t('canvas.describeImage')"
        maxlength="32000"
        @input="handlePromptInput"
      ></textarea>
      <span class="canvas-node__meta">{{ promptLength }} / 32000</span>
    </div>

    <div v-else-if="type === 'text' && !data.collapsed" class="canvas-node__content canvas-node__text-editor">
      <label class="sr-only" :for="`canvas-text-${id}`">{{ t('canvas.text') }}</label>
      <textarea
        :id="`canvas-text-${id}`"
        class="nodrag nopan nowheel"
        :style="{ fontSize: `${fontSize}px` }"
        :value="String(data.content || '')"
        :placeholder="t('canvas.textPlaceholder')"
        maxlength="100000"
        @input="handleTextInput"
      ></textarea>
      <span class="canvas-node__meta">{{ contentLength }} / 100000</span>
    </div>

    <div v-else-if="isImageNode && !data.collapsed" class="canvas-node__content canvas-node__media">
      <img v-if="data.url" :src="String(data.url)" :alt="String(data.fileName || label)" draggable="false" />
      <div v-else class="canvas-node__media-empty">
        <Icon name="image" size="lg" aria-hidden="true" />
        <span>{{ data.assetId ? t('canvas.loadingPreview') : t('canvas.imageEmpty') }}</span>
        <button type="button" class="canvas-node__inline-action nodrag nopan" @click.stop="invokeAction('onUpload')">{{ t('canvas.uploadMedia') }}</button>
      </div>
      <span v-if="data.fileName" class="canvas-node__file">{{ data.fileName }}</span>
      <div v-if="data.url" class="canvas-node__media-actions nodrag nopan">
        <button type="button" :title="t('canvas.editImage')" :aria-label="t('canvas.editImage')" @click.stop="invokeAction('onEdit')"><Icon name="edit" size="xs" /></button>
        <button type="button" :title="t('canvas.editors.maskTitle')" :aria-label="t('canvas.editors.maskTitle')" @click.stop="invokeAction('onMaskEdit')"><Icon name="focus" size="xs" /></button>
        <button type="button" :title="t('canvas.editors.angleTitle')" :aria-label="t('canvas.editors.angleTitle')" @click.stop="invokeAction('onAngle')"><Icon name="compass" size="xs" /></button>
        <button type="button" :title="t('canvas.replaceMedia')" :aria-label="t('canvas.replaceMedia')" @click.stop="invokeAction('onUpload')"><Icon name="upload" size="xs" /></button>
        <button type="button" :title="t('canvas.downloadMedia')" :aria-label="t('canvas.downloadMedia')" @click.stop="invokeAction('onDownload')"><Icon name="download" size="xs" /></button>
      </div>
    </div>

    <div v-else-if="type === 'video' && !data.collapsed" class="canvas-node__content canvas-node__media canvas-node__video">
      <video v-if="data.url" :src="String(data.url)" controls playsinline preload="metadata" class="nodrag nopan nowheel"></video>
      <div v-else class="canvas-node__media-empty">
        <Icon name="play" size="lg" aria-hidden="true" />
        <span>{{ data.assetId ? t('canvas.loadingPreview') : t('canvas.videoEmpty') }}</span>
        <button type="button" class="canvas-node__inline-action nodrag nopan" @click.stop="invokeAction('onUpload')">{{ t('canvas.uploadMedia') }}</button>
      </div>
      <span v-if="data.fileName" class="canvas-node__file">{{ data.fileName }}</span>
      <div v-if="data.url" class="canvas-node__media-actions nodrag nopan">
        <button type="button" :title="t('canvas.replaceMedia')" :aria-label="t('canvas.replaceMedia')" @click.stop="invokeAction('onUpload')"><Icon name="upload" size="xs" /></button>
        <button type="button" :title="t('canvas.downloadMedia')" :aria-label="t('canvas.downloadMedia')" @click.stop="invokeAction('onDownload')"><Icon name="download" size="xs" /></button>
      </div>
    </div>

    <div v-else-if="type === 'audio' && !data.collapsed" class="canvas-node__content canvas-node__audio">
      <div class="canvas-node__audio-mark" aria-hidden="true"><Icon name="music" size="lg" /></div>
      <audio v-if="data.url" :src="String(data.url)" controls preload="metadata" class="nodrag nopan nowheel"></audio>
      <button v-else type="button" class="canvas-node__inline-action nodrag nopan" @click.stop="invokeAction('onUpload')">{{ t('canvas.uploadAudio') }}</button>
      <span v-if="data.fileName" class="canvas-node__audio-file">{{ data.fileName }}</span>
    </div>

    <div v-else-if="(type === 'generation' || type === 'config') && !data.collapsed" class="canvas-node__content canvas-node__generation">
      <div class="canvas-node__model">
        <span>{{ generationModeLabel }}</span><strong>{{ String(data.model || defaultModel) }}</strong>
      </div>
      <div v-if="type === 'generation'" class="canvas-node__inputs">
        <span :class="{ 'is-connected': Boolean(data.promptConnected) }"><Icon name="document" size="xs" aria-hidden="true" />{{ t('canvas.prompt') }}</span>
        <span :class="{ 'is-connected': Boolean(data.referenceConnected) }"><Icon name="link" size="xs" aria-hidden="true" />{{ t('canvas.reference') }}</span>
      </div>
      <p v-else class="canvas-node__config-summary">{{ configSummary }}</p>
      <p v-if="data.error" class="canvas-node__error" role="alert">{{ data.error }}</p>
      <button type="button" class="canvas-node__generate nodrag nopan" :disabled="isBusy || !canGenerate" :aria-busy="isBusy || undefined" @click.stop="invokeAction('onGenerate')">
        <Icon name="play" size="xs" aria-hidden="true" />{{ isBusy ? t('canvas.generating') : generationActionLabel }}
      </button>
    </div>

    <div v-else-if="type === 'group' && !data.collapsed" class="canvas-node__content canvas-node__group-body">
      <span><Icon name="group" size="sm" aria-hidden="true" />{{ t('canvas.groupHint') }}</span>
    </div>

    <Handle
      :id="sourceHandle"
      type="source"
      :position="Position.Right"
      :connectable="connectable"
      class="canvas-handle"
      :class="sourceHandleClass"
      :aria-label="sourceHandleLabel"
      :title="sourceHandleLabel"
    />

    <button
      v-if="selected && !data.collapsed"
      type="button"
      class="canvas-node__resize nodrag nopan"
      :aria-label="t('canvas.resizeNode')"
      :title="t('canvas.resizeNode')"
      @pointerdown.stop.prevent="startResize"
    ></button>
  </article>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount } from 'vue'
import { Handle, Position, type NodeProps } from '@vue-flow/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CanvasNodeData, CanvasNodeType } from '@/api/canvas'
import { CANVAS_HANDLES, CANVAS_NODE_DEFAULT_SIZE } from '@/utils/canvasGraph'

type NodeAction = 'onChange' | 'onUpload' | 'onGenerate' | 'onDownload' | 'onEdit' | 'onMaskEdit' | 'onAngle' | 'onDuplicate' | 'onDelete' | 'onOpenProperties' | 'onResize' | 'onContextMenu'

const props = defineProps<NodeProps<CanvasNodeData, Record<string, never>, CanvasNodeType>>()
const { t } = useI18n()

const type = computed<CanvasNodeType>(() => props.type || 'prompt')
const data = computed(() => props.data || { label: type.value })
const label = computed(() => String(data.value.label || type.value))
const nodeWidth = computed(() => Number(props.dimensions.width) || CANVAS_NODE_DEFAULT_SIZE[type.value].width)
const nodeHeight = computed(() => Number(props.dimensions.height) || CANVAS_NODE_DEFAULT_SIZE[type.value].height)
const nodeStyle = computed(() => ({
  width: `${nodeWidth.value}px`,
  minHeight: `${data.value.collapsed ? 46 : nodeHeight.value}px`,
  height: data.value.collapsed ? '46px' : `${nodeHeight.value}px`,
}))
const kindLabel = computed(() => ({
  prompt: t('canvas.prompt'),
  reference: t('canvas.reference'),
  generation: t('canvas.generate'),
  image: t('canvas.image'),
  text: t('canvas.text'),
  video: t('canvas.videoNode'),
  audio: t('canvas.audio'),
  config: t('canvas.config'),
  group: t('canvas.group'),
}[type.value]))
const nodeIcon = computed<'document' | 'link' | 'sparkles' | 'image' | 'type' | 'play' | 'music' | 'cog' | 'group'>(() => ({
  prompt: 'document',
  reference: 'link',
  generation: 'sparkles',
  image: 'image',
  text: 'type',
  video: 'play',
  audio: 'music',
  config: 'cog',
  group: 'group',
}[type.value] as 'document' | 'link' | 'sparkles' | 'image' | 'type' | 'play' | 'music' | 'cog' | 'group'))
const promptLength = computed(() => String(data.value.prompt || '').length)
const contentLength = computed(() => String(data.value.content || '').length)
const fontSize = computed(() => Math.min(144, Math.max(10, Number(data.value.fontSize) || 16)))
const normalizedStatus = computed(() => String(data.value.status || 'ready').toLowerCase())
const isBusy = computed(() => ['queued', 'processing', 'downloading'].includes(normalizedStatus.value))
const isError = computed(() => ['failed', 'error'].includes(normalizedStatus.value) || Boolean(data.value.error))
const showsStatus = computed(() => type.value === 'generation' || type.value === 'config' || Boolean(data.value.status && data.value.status !== 'ready'))
const statusTone = computed(() => {
  if (isError.value) return 'error'
  if (isBusy.value) return 'busy'
  if (['completed', 'succeeded'].includes(normalizedStatus.value)) return 'success'
  return 'ready'
})
const statusLabel = computed(() => {
  const known = ['ready', 'queued', 'processing', 'downloading', 'completed', 'succeeded', 'failed', 'error'] as const
  return known.includes(normalizedStatus.value as (typeof known)[number]) ? t(`canvas.status.${normalizedStatus.value}`) : normalizedStatus.value
})
const isImageNode = computed(() => type.value === 'image' || type.value === 'reference')
const showGenericInput = computed(() => !['prompt', 'reference', 'generation', 'image', 'group'].includes(type.value))
const genericInputTop = computed(() => type.value === 'generation' || type.value === 'image' ? '20%' : '50%')
const sourceHandle = computed(() => ({
  prompt: CANVAS_HANDLES.promptOutput,
  reference: CANVAS_HANDLES.referenceOutput,
  generation: CANVAS_HANDLES.generationOutput,
} as Partial<Record<CanvasNodeType, string>>)[type.value] || CANVAS_HANDLES.genericOutput)
const sourceHandleClass = computed(() => ({
  prompt: 'canvas-handle--prompt',
  reference: 'canvas-handle--reference',
  generation: 'canvas-handle--image',
} as Partial<Record<CanvasNodeType, string>>)[type.value] || 'canvas-handle--generic')
const sourceHandleLabel = computed(() => t(({
  prompt: 'canvas.handles.promptOutput',
  reference: 'canvas.handles.referenceOutput',
  generation: 'canvas.handles.generationOutput',
} as Partial<Record<CanvasNodeType, string>>)[type.value] || 'canvas.handles.genericOutput'))
const generationMode = computed(() => String(data.value.generationMode || (type.value === 'generation' ? 'image' : 'image')))
const generationModeLabel = computed(() => t(`canvas.generationModes.${generationMode.value}`))
const defaultModel = computed(() => generationMode.value === 'video' ? 'grok-imagine-video-1.5' : generationMode.value === 'audio' ? 'grok-voice-tts' : 'gpt-image-1')
const generationActionLabel = computed(() => t(`canvas.generateActions.${generationMode.value}`))
const canGenerate = computed(() => type.value === 'config' ? Boolean(String(data.value.prompt || data.value.composerContent || '').trim() || data.value.inputConnected) : Boolean(data.value.promptConnected))
const configSummary = computed(() => {
  if (generationMode.value === 'video') return `${String(data.value.aspectRatio || '16:9')} / ${String(data.value.resolution || '720p')} / ${Number(data.value.seconds) || 6}s`
  if (generationMode.value === 'audio') return `${String(data.value.voice || 'Ara')} / ${String(data.value.language || 'en')}`
  return `${String(data.value.size || '1024x1024')} / ${String(data.value.quality || 'auto')} / ${Number(data.value.count) || 1}`
})

function invokeAction(name: NodeAction, ...args: unknown[]) {
  const action = data.value[name]
  if (typeof action === 'function') action(props.id, ...args)
}

function invokeContextMenu(event: MouseEvent) {
  invokeAction('onContextMenu', event)
}

function handlePromptInput(event: Event) {
  data.value.prompt = (event.target as HTMLTextAreaElement).value
  invokeAction('onChange')
}

function handleTextInput(event: Event) {
  data.value.content = (event.target as HTMLTextAreaElement).value
  invokeAction('onChange')
}

function toggleGroup() {
  data.value.collapsed = !data.value.collapsed
  invokeAction('onChange')
}

let resizeCleanup: (() => void) | null = null

function startResize(event: PointerEvent) {
  const startX = event.clientX
  const startY = event.clientY
  const startWidth = nodeWidth.value
  const startHeight = nodeHeight.value
  const minWidth = type.value === 'group' ? 280 : 200
  const minHeight = type.value === 'group' ? 180 : 140
  const move = (next: PointerEvent) => invokeAction(
    'onResize',
    Math.min(2400, Math.max(minWidth, startWidth + next.clientX - startX)),
    Math.min(1800, Math.max(minHeight, startHeight + next.clientY - startY)),
  )
  const stop = () => {
    window.removeEventListener('pointermove', move)
    window.removeEventListener('pointerup', stop)
    window.removeEventListener('pointercancel', stop)
    resizeCleanup = null
    invokeAction('onChange')
  }
  resizeCleanup?.()
  resizeCleanup = stop
  window.addEventListener('pointermove', move)
  window.addEventListener('pointerup', stop, { once: true })
  window.addEventListener('pointercancel', stop, { once: true })
}

onBeforeUnmount(() => resizeCleanup?.())
</script>

<style scoped>
.canvas-node { position: relative; isolation: isolate; overflow: visible; border: 1px solid var(--color-border-strong); border-radius: 10px; background: var(--color-surface); box-shadow: var(--shadow-xs); color: var(--color-text-primary); transition: border-color var(--motion-fast) var(--ease-standard), box-shadow var(--motion-fast) var(--ease-standard); }
.canvas-node:hover { border-color: var(--color-primary-border); box-shadow: var(--shadow-sm); }
.canvas-node--selected { border-color: var(--color-primary); box-shadow: 0 0 0 2px var(--color-primary-ring), var(--shadow-md); }
.canvas-node--error { border-color: var(--color-danger); }
.canvas-node--group { z-index: -1; border-style: dashed; background: color-mix(in srgb, var(--color-primary-soft) 32%, transparent); }
.canvas-node--group.canvas-node--drop-target { border-color: var(--color-primary); background: color-mix(in srgb, var(--color-primary-soft) 58%, transparent); box-shadow: 0 0 0 3px var(--color-primary-ring); }
.canvas-node--group .canvas-node__header { border-bottom-style: dashed; background: color-mix(in srgb, var(--color-surface) 80%, transparent); }
.canvas-node__header { display: flex; min-height: 44px; align-items: center; gap: 9px; padding: 6px 8px; border-bottom: 1px solid var(--color-border-subtle); cursor: grab; user-select: none; }
.canvas-node__header:active { cursor: grabbing; }
.canvas-node--collapsed .canvas-node__header { border-bottom: 0; }
.canvas-node__icon { display: grid; width: 28px; height: 28px; flex: 0 0 28px; place-items: center; border-radius: 7px; color: var(--color-primary); }
.canvas-node--reference .canvas-node__icon, .canvas-node--image .canvas-node__icon, .canvas-node--video .canvas-node__icon, .canvas-node--audio .canvas-node__icon { color: var(--color-accent); }
.canvas-node__heading { display: flex; min-width: 0; flex: 1; flex-direction: column; gap: 3px; }
.canvas-node__heading strong { overflow: hidden; font-size: 13px; font-weight: 650; line-height: 1.25; text-overflow: ellipsis; white-space: nowrap; }
.canvas-node__kind { color: var(--color-text-muted); font: 600 10px/1.2 var(--font-mono, monospace); text-transform: uppercase; }
.canvas-node__header-action { display: grid; width: 30px; height: 30px; place-items: center; border: 0; border-radius: 7px; background: transparent; color: var(--color-text-muted); cursor: pointer; }
.canvas-node__header-action:hover { background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-node__content { min-height: 0; }
.canvas-node__text-editor { display: flex; height: calc(100% - 44px); min-height: 96px; flex-direction: column; padding: 12px; }
.canvas-node__text-editor textarea { min-height: 0; flex: 1; resize: none; border: 0; outline: 0; background: transparent; color: var(--color-text-primary); font: inherit; line-height: 1.58; }
.canvas-node__text-editor textarea::placeholder { color: var(--color-text-muted); }
.canvas-node__meta { margin-top: 8px; color: var(--color-text-muted); font: 500 10px/1 var(--font-mono, monospace); text-align: right; }
.canvas-node__media { position: relative; height: calc(100% - 44px); min-height: 120px; overflow: hidden; border-radius: 0 0 9px 9px; background: var(--color-surface-soft); }
.canvas-node__media img, .canvas-node__media video { display: block; width: 100%; height: 100%; object-fit: contain; background: var(--color-bg-deep); }
.canvas-node__media-empty { display: flex; width: 100%; height: 100%; min-height: 120px; align-items: center; justify-content: center; flex-direction: column; gap: 9px; color: var(--color-text-muted); font-size: 12px; }
.canvas-node__inline-action { min-height: 32px; padding: 0 10px; border: 1px solid var(--color-border); border-radius: 7px; background: var(--color-surface); color: var(--color-text-secondary); cursor: pointer; }
.canvas-node__inline-action:hover { border-color: var(--color-primary-border); color: var(--color-primary); }
.canvas-node__file { position: absolute; right: 8px; bottom: 8px; left: 8px; overflow: hidden; padding: 5px 8px; border: 1px solid var(--glass-border); border-radius: 7px; background: var(--glass-bg-strong); color: var(--color-text-primary); font-size: 11px; text-overflow: ellipsis; white-space: nowrap; backdrop-filter: blur(12px); }
.canvas-node__media-actions { position: absolute; top: 8px; right: 8px; display: flex; gap: 3px; padding: 3px; border: 1px solid var(--glass-border); border-radius: 8px; background: var(--glass-bg-strong); box-shadow: var(--shadow-sm); backdrop-filter: blur(12px); }
.canvas-node__media-actions button, .canvas-node__hover-tools button { display: grid; width: 28px; height: 28px; place-items: center; border: 0; border-radius: 6px; background: transparent; color: var(--color-text-secondary); cursor: pointer; }
.canvas-node__media-actions button:hover, .canvas-node__hover-tools button:hover { background: var(--color-primary-soft); color: var(--color-primary); }
.canvas-node__hover-tools button.is-danger:hover { background: color-mix(in srgb, var(--color-danger) 10%, transparent); color: var(--color-danger); }
.canvas-node__hover-tools { position: absolute; z-index: 5; top: -39px; left: 50%; display: flex; gap: 2px; padding: 3px; border: 1px solid var(--glass-border); border-radius: 8px; background: var(--glass-bg-strong); box-shadow: var(--shadow-md); opacity: 0; pointer-events: none; transform: translate(-50%, 4px); transition: opacity var(--motion-fast) var(--ease-standard), transform var(--motion-fast) var(--ease-standard); backdrop-filter: blur(14px); }
.canvas-node:hover .canvas-node__hover-tools, .canvas-node:focus-within .canvas-node__hover-tools, .canvas-node--selected .canvas-node__hover-tools { opacity: 1; pointer-events: auto; transform: translate(-50%, 0); }
.canvas-node__generation { display: flex; height: calc(100% - 44px); min-height: 120px; flex-direction: column; gap: 10px; padding: 13px; }
.canvas-node__model { display: flex; align-items: flex-start; justify-content: space-between; gap: 10px; padding-bottom: 10px; border-bottom: 1px solid var(--color-border-subtle); }
.canvas-node__model span { color: var(--color-text-muted); font-size: 10px; text-transform: uppercase; }
.canvas-node__model strong { max-width: 66%; overflow: hidden; font: 600 11px/1.35 var(--font-mono, monospace); text-overflow: ellipsis; white-space: nowrap; }
.canvas-node__inputs { display: flex; gap: 6px; }
.canvas-node__inputs span { display: inline-flex; align-items: center; gap: 4px; padding: 5px 7px; border: 1px solid var(--color-border-subtle); border-radius: 6px; color: var(--color-text-muted); font-size: 10px; }
.canvas-node__inputs span.is-connected { border-color: color-mix(in srgb, var(--color-success) 28%, var(--color-border)); color: var(--color-success); }
.canvas-node__config-summary { margin: 0; color: var(--color-text-secondary); font: 500 11px/1.5 var(--font-mono, monospace); }
.canvas-node__error { margin: 0; color: var(--color-danger); font-size: 11px; line-height: 1.4; }
.canvas-node__generate { display: inline-flex; min-height: 34px; align-items: center; justify-content: center; gap: 7px; margin-top: auto; border: 0; border-radius: 7px; background: var(--color-primary); color: var(--color-text-on-primary, #fff); font-size: 12px; font-weight: 650; cursor: pointer; }
.canvas-node__generate:disabled { opacity: .48; cursor: not-allowed; }
.canvas-node__audio { display: flex; height: calc(100% - 44px); min-height: 92px; align-items: center; flex-direction: column; gap: 10px; padding: 16px; }
.canvas-node__audio-mark { color: var(--color-accent); }
.canvas-node__audio audio { width: 100%; min-height: 40px; }
.canvas-node__audio-file { max-width: 100%; overflow: hidden; color: var(--color-text-muted); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.canvas-node__group-body { display: grid; height: calc(100% - 44px); place-items: center; color: var(--color-text-muted); font-size: 12px; pointer-events: none; }
.canvas-node__group-body span { display: inline-flex; align-items: center; gap: 7px; }
.canvas-node__status { display: inline-flex; align-items: center; gap: 5px; color: var(--color-text-muted); font-size: 9px; text-transform: uppercase; }
.canvas-node__status-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.canvas-node__status--busy { color: var(--color-warning); }
.canvas-node__status--success { color: var(--color-success); }
.canvas-node__status--error { color: var(--color-danger); }
.canvas-node__resize { position: absolute; z-index: 8; right: -6px; bottom: -6px; width: 14px; height: 14px; border: 2px solid var(--color-surface); border-radius: 4px; background: var(--color-primary); box-shadow: var(--shadow-xs); cursor: nwse-resize; }
.canvas-handle { width: 11px; height: 11px; border: 2px solid var(--color-surface); background: var(--color-primary); }
.canvas-handle--reference, .canvas-handle--image { background: var(--color-accent); }
.canvas-handle--generic { background: var(--color-text-secondary); }
.canvas-handle:hover { transform: scale(1.2); }
button:focus-visible, textarea:focus-visible, audio:focus-visible, video:focus-visible { outline: 2px solid var(--color-primary); outline-offset: 2px; }
@media (prefers-reduced-motion: reduce) { .canvas-node, .canvas-node__hover-tools { transition-duration: .01ms; } }
</style>
