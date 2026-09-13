<template>
  <div class="canvas-project-preview" :class="{ 'is-empty': !previewNodes.length }" aria-hidden="true">
    <svg
      v-if="previewNodes.length"
      viewBox="0 0 320 160"
      preserveAspectRatio="xMidYMid meet"
    >
      <path
        v-for="edge in previewEdges"
        :key="edge.id"
        class="canvas-project-preview__edge"
        :d="edge.path"
      />
      <g
        v-for="node in previewNodes"
        :key="node.id"
        class="canvas-project-preview__node"
        :class="`canvas-project-preview__node--${node.type}`"
        :transform="`translate(${node.x} ${node.y})`"
      >
        <rect x="-38" y="-18" width="76" height="36" rx="7" />
        <circle class="canvas-project-preview__port" cx="-38" cy="0" r="3" />
        <circle class="canvas-project-preview__port" cx="38" cy="0" r="3" />
        <text x="0" y="1" text-anchor="middle" dominant-baseline="middle">
          {{ node.label }}
        </text>
      </g>
    </svg>

    <div v-else class="canvas-project-preview__empty">
      <Icon name="sparkles" size="lg" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { CanvasDocument } from '@/api/canvas'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  document: CanvasDocument
}>()

type PreviewNode = {
  id: string
  type: string
  label: string
  x: number
  y: number
}

const previewNodes = computed<PreviewNode[]>(() => {
  const nodes = props.document.nodes.slice(0, 8)
  if (!nodes.length) return []

  const xs = nodes.map(node => Number(node.position?.x) || 0)
  const ys = nodes.map(node => Number(node.position?.y) || 0)
  const minX = Math.min(...xs)
  const maxX = Math.max(...xs)
  const minY = Math.min(...ys)
  const maxY = Math.max(...ys)
  const width = Math.max(maxX - minX, 1)
  const height = Math.max(maxY - minY, 1)

  return nodes.map(node => ({
    id: node.id,
    type: node.type,
    label: String(node.data?.label || node.type).slice(0, 8),
    x: minX === maxX ? 160 : 52 + ((Number(node.position?.x) || 0) - minX) / width * 216,
    y: minY === maxY ? 80 : 38 + ((Number(node.position?.y) || 0) - minY) / height * 84,
  }))
})

const previewEdges = computed(() => {
  const positions = new Map(previewNodes.value.map(node => [node.id, node]))
  return props.document.edges.slice(0, 12).flatMap(edge => {
    const source = positions.get(edge.source)
    const target = positions.get(edge.target)
    if (!source || !target) return []
    const sourceX = source.x + 38
    const targetX = target.x - 38
    const curve = Math.max(24, Math.abs(targetX - sourceX) * 0.45)
    return [{
      id: edge.id,
      path: `M ${sourceX} ${source.y} C ${sourceX + curve} ${source.y}, ${targetX - curve} ${target.y}, ${targetX} ${target.y}`,
    }]
  })
})
</script>

<style scoped>
.canvas-project-preview {
  position: relative;
  width: 100%;
  aspect-ratio: 2 / 1;
  overflow: hidden;
  background: var(--color-bg-subtle);
  color: var(--color-text-muted);
}

.canvas-project-preview::after {
  position: absolute;
  inset: 0;
  border: 1px solid color-mix(in srgb, var(--color-border-subtle) 78%, transparent);
  content: '';
  pointer-events: none;
}

.canvas-project-preview svg {
  display: block;
  width: 100%;
  height: 100%;
}

.canvas-project-preview__edge {
  fill: none;
  stroke: var(--color-primary-border);
  stroke-linecap: round;
  stroke-width: 2;
  vector-effect: non-scaling-stroke;
}

.canvas-project-preview__node rect {
  fill: var(--color-surface-raised);
  stroke: var(--color-border-strong);
  stroke-width: 1;
}

.canvas-project-preview__node text {
  fill: var(--color-text-secondary);
  font-family: inherit;
  font-size: 9px;
  font-weight: 650;
  letter-spacing: 0;
}

.canvas-project-preview__node--prompt rect,
.canvas-project-preview__node--generation rect {
  fill: var(--color-primary-soft);
  stroke: var(--color-primary-border);
}

.canvas-project-preview__node--reference rect {
  fill: var(--color-accent-soft);
  stroke: color-mix(in srgb, var(--color-accent) 42%, var(--color-border));
}

.canvas-project-preview__port {
  fill: var(--color-primary);
  stroke: var(--color-surface);
  stroke-width: 1.5;
}

.canvas-project-preview__empty {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  color: var(--color-text-muted);
}

.canvas-project-preview__empty::before,
.canvas-project-preview__empty::after {
  position: absolute;
  width: 42px;
  height: 28px;
  border: 1px solid var(--color-border);
  border-radius: 7px;
  background: var(--color-surface-soft);
  content: '';
}

.canvas-project-preview__empty::before { transform: translateX(-54px); }
.canvas-project-preview__empty::after { transform: translateX(54px); }

.canvas-project-preview__empty :deep(svg) {
  position: relative;
  z-index: 1;
  color: var(--color-primary);
}
</style>
