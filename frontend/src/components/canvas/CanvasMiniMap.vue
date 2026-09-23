<template>
  <div class="canvas-minimap glass-popover" :aria-label="t('canvas.minimap')" data-canvas-no-zoom>
    <svg
      ref="mapRef"
      viewBox="0 0 240 160"
      role="img"
      :aria-label="t('canvas.minimapNavigation')"
      @pointerdown="startNavigation"
      @pointermove="continueNavigation"
      @pointerup="stopNavigation"
      @pointercancel="stopNavigation"
    >
      <rect width="240" height="160" rx="8" class="canvas-minimap__background" />
      <rect
        v-for="node in nodes"
        :key="node.id"
        :x="toMapX(node.position.x)"
        :y="toMapY(node.position.y)"
        :width="Math.max(nodeSize(node).width * geometry.scale, 2)"
        :height="Math.max(nodeSize(node).height * geometry.scale, 2)"
        rx="1.5"
        class="canvas-minimap__node"
        :class="`canvas-minimap__node--${node.type}`"
      />
      <rect
        :x="viewportRect.x"
        :y="viewportRect.y"
        :width="viewportRect.width"
        :height="viewportRect.height"
        rx="3"
        class="canvas-minimap__viewport"
      />
    </svg>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ViewportTransform } from '@vue-flow/core'
import { CANVAS_NODE_DEFAULT_SIZE, type CanvasFlowNode } from '@/utils/canvasGraph'

const props = defineProps<{
  nodes: CanvasFlowNode[]
  viewport: ViewportTransform
  viewportSize: { width: number; height: number }
}>()
const emit = defineEmits<{ viewportChange: [viewport: ViewportTransform] }>()
const { t } = useI18n()
const mapRef = ref<SVGSVGElement | null>(null)
const dragging = ref(false)

function nodeSize(node: CanvasFlowNode) {
  const fallback = CANVAS_NODE_DEFAULT_SIZE[node.type]
  return {
    width: Number(node.width) || fallback.width,
    height: Number(node.height) || fallback.height,
  }
}

const geometry = computed(() => {
  if (!props.nodes.length) return { minX: -500, minY: -500, scale: 0.16, offsetX: 40, offsetY: 0 }
  let minX = Infinity
  let minY = Infinity
  let maxX = -Infinity
  let maxY = -Infinity
  for (const node of props.nodes) {
    const size = nodeSize(node)
    minX = Math.min(minX, node.position.x)
    minY = Math.min(minY, node.position.y)
    maxX = Math.max(maxX, node.position.x + size.width)
    maxY = Math.max(maxY, node.position.y + size.height)
  }
  minX -= 420
  minY -= 320
  maxX += 420
  maxY += 320
  const width = Math.max(maxX - minX, 1)
  const height = Math.max(maxY - minY, 1)
  const scale = Math.min(240 / width, 160 / height)
  return {
    minX,
    minY,
    scale,
    offsetX: (240 - width * scale) / 2,
    offsetY: (160 - height * scale) / 2,
  }
})

function toMapX(worldX: number) {
  return (worldX - geometry.value.minX) * geometry.value.scale + geometry.value.offsetX
}

function toMapY(worldY: number) {
  return (worldY - geometry.value.minY) * geometry.value.scale + geometry.value.offsetY
}

const viewportRect = computed(() => {
  const zoom = Math.max(props.viewport.zoom, 0.0001)
  const worldX = -props.viewport.x / zoom
  const worldY = -props.viewport.y / zoom
  return {
    x: toMapX(worldX),
    y: toMapY(worldY),
    width: Math.max((props.viewportSize.width / zoom) * geometry.value.scale, 4),
    height: Math.max((props.viewportSize.height / zoom) * geometry.value.scale, 4),
  }
})

function navigate(event: PointerEvent) {
  const box = mapRef.value?.getBoundingClientRect()
  if (!box) return
  const mapX = (event.clientX - box.left) * (240 / box.width)
  const mapY = (event.clientY - box.top) * (160 / box.height)
  const worldX = (mapX - geometry.value.offsetX) / geometry.value.scale + geometry.value.minX
  const worldY = (mapY - geometry.value.offsetY) / geometry.value.scale + geometry.value.minY
  emit('viewportChange', {
    x: props.viewportSize.width / 2 - worldX * props.viewport.zoom,
    y: props.viewportSize.height / 2 - worldY * props.viewport.zoom,
    zoom: props.viewport.zoom,
  })
}

function startNavigation(event: PointerEvent) {
  dragging.value = true
  mapRef.value?.setPointerCapture(event.pointerId)
  navigate(event)
}

function continueNavigation(event: PointerEvent) {
  if (dragging.value) navigate(event)
}

function stopNavigation(event: PointerEvent) {
  dragging.value = false
  if (mapRef.value?.hasPointerCapture(event.pointerId)) mapRef.value.releasePointerCapture(event.pointerId)
}
</script>

<style scoped>
.canvas-minimap { position: absolute; z-index: 24; bottom: 78px; left: 18px; width: 240px; height: 160px; overflow: hidden; padding: 0; border: 1px solid var(--glass-border); border-radius: 9px; box-shadow: var(--shadow-overlay); }
.canvas-minimap svg { display: block; width: 100%; height: 100%; cursor: crosshair; touch-action: none; }
.canvas-minimap__background { fill: var(--color-surface-overlay); }
.canvas-minimap__node { fill: var(--color-text-muted); opacity: .68; }
.canvas-minimap__node--image, .canvas-minimap__node--reference, .canvas-minimap__node--video, .canvas-minimap__node--audio { fill: var(--color-accent); }
.canvas-minimap__node--generation, .canvas-minimap__node--config { fill: var(--color-primary); }
.canvas-minimap__node--group { fill: var(--color-primary-soft); stroke: var(--color-primary); stroke-dasharray: 3 2; }
.canvas-minimap__viewport { fill: var(--color-primary-soft); fill-opacity: .55; stroke: var(--color-primary); stroke-width: 1.5; vector-effect: non-scaling-stroke; }
@media (max-width: 760px) { .canvas-minimap { bottom: 118px; left: 8px; width: 180px; height: 120px; } }
</style>
