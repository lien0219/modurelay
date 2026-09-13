import type { Connection, ViewportTransform } from '@vue-flow/core'
import type {
  CanvasDocument,
  CanvasEdge,
  CanvasNode,
  CanvasNodeData,
  CanvasNodeType,
} from '@/api/canvas'
import { CANVAS_DOCUMENT_SCHEMA_VERSION } from '@/api/canvas'

export const CANVAS_HANDLES = {
  promptOutput: 'prompt-output',
  referenceOutput: 'reference-output',
  generationPrompt: 'generation-prompt',
  generationReference: 'generation-reference',
  generationOutput: 'generation-output',
  imageInput: 'image-input',
  genericInput: 'node-input',
  genericOutput: 'node-output',
} as const

interface CanvasConnectionRule {
  source: CanvasNodeType
  target: CanvasNodeType
  sourceHandle: string
  targetHandle: string
}

const connectionRules: CanvasConnectionRule[] = [
  {
    source: 'prompt',
    target: 'generation',
    sourceHandle: CANVAS_HANDLES.promptOutput,
    targetHandle: CANVAS_HANDLES.generationPrompt,
  },
  {
    source: 'reference',
    target: 'generation',
    sourceHandle: CANVAS_HANDLES.referenceOutput,
    targetHandle: CANVAS_HANDLES.generationReference,
  },
  {
    source: 'generation',
    target: 'image',
    sourceHandle: CANVAS_HANDLES.generationOutput,
    targetHandle: CANVAS_HANDLES.imageInput,
  },
]

const nodeTypes = new Set<CanvasNodeType>(['prompt', 'reference', 'generation', 'image', 'text', 'video', 'audio', 'config', 'group'])
const transientDataKeys = new Set(['url', 'previewUrl', 'objectUrl', 'error', 'progress'])

export const CANVAS_NODE_DEFAULT_SIZE: Record<CanvasNodeType, { width: number; height: number }> = {
  prompt: { width: 288, height: 228 },
  reference: { width: 288, height: 260 },
  generation: { width: 288, height: 220 },
  image: { width: 320, height: 280 },
  text: { width: 300, height: 220 },
  video: { width: 360, height: 270 },
  audio: { width: 320, height: 180 },
  config: { width: 300, height: 268 },
  group: { width: 680, height: 440 },
}

export interface CanvasFlowNode {
  id: string
  type: CanvasNodeType
  position: { x: number; y: number }
  width?: number
  height?: number
  data: CanvasNodeData
  dragHandle?: string
  ariaLabel?: string
  selected?: boolean
  [key: string]: unknown
}

export interface CanvasFlowEdge extends CanvasEdge {
  interactionWidth?: number
  ariaLabel?: string
  selected?: boolean
  [key: string]: unknown
}

function canvasNodeWidth(node: CanvasFlowNode) {
  return Number(node.width) || CANVAS_NODE_DEFAULT_SIZE[node.type].width
}

function canvasNodeHeight(node: CanvasFlowNode) {
  return Number(node.height) || CANVAS_NODE_DEFAULT_SIZE[node.type].height
}

function canvasNodeBounds(nodes: CanvasFlowNode[]) {
  return nodes.reduce((bounds, node) => ({
    left: Math.min(bounds.left, node.position.x),
    top: Math.min(bounds.top, node.position.y),
    right: Math.max(bounds.right, node.position.x + canvasNodeWidth(node)),
    bottom: Math.max(bounds.bottom, node.position.y + canvasNodeHeight(node)),
  }), { left: Infinity, top: Infinity, right: -Infinity, bottom: -Infinity })
}

function selectedCanvasGroupIds(selectedIds: Set<string>, nodes: CanvasFlowNode[]) {
  return new Set(nodes.filter(node => selectedIds.has(node.id) && node.type === 'group').map(node => node.id))
}

export function collectCanvasGroupMembers(selectedIds: Set<string>, nodes: CanvasFlowNode[]) {
  const groupIds = selectedCanvasGroupIds(selectedIds, nodes)
  return nodes.filter(node => node.type !== 'group' && (
    selectedIds.has(node.id) || (Boolean(node.data.groupId) && groupIds.has(String(node.data.groupId)))
  ))
}

export function getCanvasGroupWrapRect(members: CanvasFlowNode[]) {
  const bounds = canvasNodeBounds(members)
  return {
    x: bounds.left - 24,
    y: bounds.top - 52,
    width: bounds.right - bounds.left + 48,
    height: bounds.bottom - bounds.top + 76,
  }
}

export function canGroupCanvasNodes(selectedIds: Set<string>, nodes: CanvasFlowNode[]) {
  const members = collectCanvasGroupMembers(selectedIds, nodes)
  if (members.length < 2) return false
  const groupId = members[0].data.groupId
  return !groupId || members.some(node => node.data.groupId !== groupId)
}

export function canUngroupCanvasNodes(selectedIds: Set<string>, nodes: CanvasFlowNode[]) {
  return nodes.some(node => selectedIds.has(node.id) && (node.type === 'group' || Boolean(node.data.groupId)))
}

function emptyCanvasGroupIds(nodes: CanvasFlowNode[], keepId?: string) {
  const used = new Set(nodes.flatMap(node => node.type !== 'group' && node.data.groupId ? [String(node.data.groupId)] : []))
  return new Set(nodes.filter(node => node.type === 'group' && node.id !== keepId && !used.has(node.id)).map(node => node.id))
}

function withoutCanvasNodes(nodes: CanvasFlowNode[], edges: CanvasFlowEdge[], removedIds: Set<string>) {
  return {
    nodes: nodes.filter(node => !removedIds.has(node.id)),
    edges: edges.filter(edge => !removedIds.has(edge.source) && !removedIds.has(edge.target)),
  }
}

export function applyCanvasGroupSelection(
  selectedIds: Set<string>,
  nodes: CanvasFlowNode[],
  edges: CanvasFlowEdge[],
  group: CanvasFlowNode,
) {
  const members = collectCanvasGroupMembers(selectedIds, nodes)
  if (members.length < 2) return null
  const memberIds = new Set(members.map(node => node.id))
  const flattenedGroupIds = selectedCanvasGroupIds(selectedIds, nodes)
  const updated = nodes
    .filter(node => !flattenedGroupIds.has(node.id))
    .map(node => memberIds.has(node.id) ? { ...node, data: { ...node.data, groupId: group.id } } : node)
  const insertAt = updated.findIndex(node => memberIds.has(node.id))
  const withGroup = insertAt < 0
    ? [...updated, group]
    : [...updated.slice(0, insertAt), group, ...updated.slice(insertAt)]
  const removed = new Set([...flattenedGroupIds, ...emptyCanvasGroupIds(withGroup, group.id)])
  const result = withoutCanvasNodes(withGroup, edges, removed)
  return { ...result, selectedIds: [group.id] }
}

export function applyCanvasUngroupSelection(
  selectedIds: Set<string>,
  nodes: CanvasFlowNode[],
  edges: CanvasFlowEdge[],
) {
  const flattenedGroupIds = selectedCanvasGroupIds(selectedIds, nodes)
  if (!flattenedGroupIds.size && !nodes.some(node => selectedIds.has(node.id) && node.data.groupId)) return null
  const releasedIds = new Set<string>()
  const updated = nodes
    .filter(node => !flattenedGroupIds.has(node.id))
    .map(node => {
      const groupId = String(node.data.groupId || '')
      if (!groupId || (!flattenedGroupIds.has(groupId) && !selectedIds.has(node.id))) return node
      releasedIds.add(node.id)
      return { ...node, data: { ...node.data, groupId: undefined } }
    })
  const removed = new Set([...flattenedGroupIds, ...emptyCanvasGroupIds(updated)])
  const result = withoutCanvasNodes(updated, edges, removed)
  return {
    ...result,
    selectedIds: result.nodes.filter(node => selectedIds.has(node.id) || releasedIds.has(node.id)).map(node => node.id),
  }
}

export function findCanvasGroupDropTarget(movedIds: Set<string>, nodes: CanvasFlowNode[]) {
  if (nodes.some(node => movedIds.has(node.id) && node.type === 'group')) return undefined
  const moving = nodes.filter(node => movedIds.has(node.id) && node.type !== 'group')
  if (!moving.length) return undefined
  return [...nodes].reverse().find(group => {
    if (group.type !== 'group' || movedIds.has(group.id)) return false
    const width = canvasNodeWidth(group)
    const height = canvasNodeHeight(group)
    return moving.some(node => {
      const centerX = node.position.x + canvasNodeWidth(node) / 2
      const centerY = node.position.y + canvasNodeHeight(node) / 2
      return centerX >= group.position.x && centerX <= group.position.x + width
        && centerY >= group.position.y && centerY <= group.position.y + height
    })
  })
}

export function findContainingCanvasGroupId(node: CanvasFlowNode, nodes: CanvasFlowNode[]) {
  const centerX = node.position.x + canvasNodeWidth(node) / 2
  const centerY = node.position.y + canvasNodeHeight(node) / 2
  return [...nodes].reverse().find(group => (
    group.type === 'group'
    && group.id !== node.id
    && centerX >= group.position.x
    && centerX <= group.position.x + canvasNodeWidth(group)
    && centerY >= group.position.y
    && centerY <= group.position.y + canvasNodeHeight(group)
  ))?.id
}

export function snapCanvasNodesIntoGroup(movedIds: Set<string>, nodes: CanvasFlowNode[], group: CanvasFlowNode) {
  const moving = nodes.filter(node => movedIds.has(node.id) && node.type !== 'group')
  if (!moving.length) return nodes
  const bounds = canvasNodeBounds(moving)
  const left = group.position.x + 24
  const top = group.position.y + 52
  const right = group.position.x + canvasNodeWidth(group) - 24
  const bottom = group.position.y + canvasNodeHeight(group) - 24
  const dx = bounds.right - bounds.left > right - left
    ? left - bounds.left
    : bounds.left < left ? left - bounds.left : bounds.right > right ? right - bounds.right : 0
  const dy = bounds.bottom - bounds.top > bottom - top
    ? top - bounds.top
    : bounds.top < top ? top - bounds.top : bounds.bottom > bottom ? bottom - bounds.bottom : 0
  return nodes.map(node => {
    if (!movedIds.has(node.id) || node.type === 'group') return node
    return {
      ...node,
      position: { x: node.position.x + dx, y: node.position.y + dy },
      data: { ...node.data, groupId: group.id },
    }
  })
}

const CANVAS_NODE_WIDTH = 288
const CANVAS_NODE_ESTIMATED_HEIGHT = 248
const CANVAS_NODE_COLUMN_STEP = 360
const CANVAS_NODE_ROW_STEP = 304

function positionsOverlap(
  first: { x: number; y: number },
  second: { x: number; y: number },
) {
  return Math.abs(first.x - second.x) < CANVAS_NODE_WIDTH + 40
    && Math.abs(first.y - second.y) < CANVAS_NODE_ESTIMATED_HEIGHT + 32
}

function ringOffsets(radius: number) {
  const offsets: Array<{ x: number; y: number }> = [{ x: radius, y: 0 }]
  for (let y = 1; y <= radius; y += 1) {
    offsets.push({ x: radius, y }, { x: radius, y: -y })
  }
  for (let x = radius - 1; x >= -radius; x -= 1) {
    offsets.push({ x, y: radius }, { x, y: -radius })
  }
  for (let y = radius - 1; y > -radius; y -= 1) offsets.push({ x: -radius, y })
  return offsets
}

export function findAvailableCanvasPosition(
  preferred: { x: number; y: number },
  nodes: Array<Pick<CanvasFlowNode, 'position'>>,
): { x: number; y: number } {
  const available = (candidate: { x: number; y: number }) => (
    nodes.every(node => !positionsOverlap(candidate, node.position))
  )
  if (available(preferred)) return preferred

  const maxRadius = Math.ceil(Math.sqrt(nodes.length + 1)) + 2
  for (let radius = 1; radius <= maxRadius; radius += 1) {
    for (const offset of ringOffsets(radius)) {
      const candidate = {
        x: preferred.x + offset.x * CANVAS_NODE_COLUMN_STEP,
        y: preferred.y + offset.y * CANVAS_NODE_ROW_STEP,
      }
      if (available(candidate)) return candidate
    }
  }

  return {
    x: preferred.x + (maxRadius + 1) * CANVAS_NODE_COLUMN_STEP,
    y: preferred.y,
  }
}

export function isCanvasNodeType(value: unknown): value is CanvasNodeType {
  return typeof value === 'string' && nodeTypes.has(value as CanvasNodeType)
}

export function getConnectionRule(
  sourceType: unknown,
  targetType: unknown,
): CanvasConnectionRule | undefined {
  if (!isCanvasNodeType(sourceType) || !isCanvasNodeType(targetType)) return undefined
  if (targetType === 'group' || (sourceType === 'config' && targetType === 'config')) return undefined
  const specific = connectionRules.find(rule => rule.source === sourceType && rule.target === targetType)
  if (specific) return specific
  const sourceHandle = ({
    prompt: CANVAS_HANDLES.promptOutput,
    reference: CANVAS_HANDLES.referenceOutput,
    generation: CANVAS_HANDLES.generationOutput,
  } as Partial<Record<CanvasNodeType, string>>)[sourceType] || CANVAS_HANDLES.genericOutput
  return {
    source: sourceType,
    target: targetType,
    sourceHandle,
    targetHandle: CANVAS_HANDLES.genericInput,
  }
}

function nodeTypeFor(nodes: Array<{ id: string; type?: string }>, id: string) {
  return nodes.find(node => node.id === id)?.type
}

export function normalizeCanvasConnection(
  connection: Connection | CanvasEdge,
  nodes: Array<{ id: string; type?: string }>,
): Connection | null {
  if (!connection.source || !connection.target || connection.source === connection.target) return null

  const rule = getConnectionRule(
    nodeTypeFor(nodes, connection.source),
    nodeTypeFor(nodes, connection.target),
  )
  if (!rule) return null

  const sourceHandle = connection.sourceHandle || rule.sourceHandle
  const targetHandle = connection.targetHandle || rule.targetHandle
  if (sourceHandle !== rule.sourceHandle || targetHandle !== rule.targetHandle) return null

  return {
    source: connection.source,
    target: connection.target,
    sourceHandle,
    targetHandle,
  }
}

export function isCanvasConnectionValid(
  connection: Connection | CanvasEdge,
  nodes: Array<{ id: string; type?: string }>,
  edges: Array<Pick<CanvasFlowEdge, 'source' | 'target' | 'sourceHandle' | 'targetHandle'> & { id?: string }>,
  ignoredEdgeId?: string,
): boolean {
  const normalized = normalizeCanvasConnection(connection, nodes)
  if (!normalized) return false

  return !edges.some(edge => {
    if (ignoredEdgeId && edge.id === ignoredEdgeId) return false

    const existing = normalizeCanvasConnection(edge as CanvasEdge, nodes)
    if (!existing) return false

    const duplicate =
      existing.source === normalized.source &&
      existing.target === normalized.target &&
      existing.sourceHandle === normalized.sourceHandle &&
      existing.targetHandle === normalized.targetHandle
    const occupiedInput = normalized.targetHandle !== CANVAS_HANDLES.genericInput
      && existing.target === normalized.target
      && existing.targetHandle === normalized.targetHandle
    return duplicate || occupiedInput
  })
}

export function createCanvasEdge(
  connection: Connection,
  nodes: Array<{ id: string; type?: string }>,
  edges: CanvasFlowEdge[],
  id = `edge-${crypto.randomUUID?.() || Date.now()}`,
): CanvasFlowEdge | null {
  const normalized = normalizeCanvasConnection(connection, nodes)
  if (!normalized || !isCanvasConnectionValid(normalized, nodes, edges)) return null

  return {
    id,
    ...normalized,
    type: 'default',
    interactionWidth: 20,
    ariaLabel: `${normalized.source} to ${normalized.target}`,
  }
}

export interface CanvasStarterLabels {
  text: string
  config: string
  image: string
}

export function createStarterCanvasDocument(labels: CanvasStarterLabels): CanvasDocument {
  const makeNodeId = (type: CanvasNodeType) => (
    `${type}-${globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`}`
  )
  const textId = makeNodeId('text')
  const configId = makeNodeId('config')
  const imageId = makeNodeId('image')
  const nodes: CanvasNode[] = [
    {
      id: textId,
      type: 'text',
      position: { x: 0, y: 36 },
      data: { label: labels.text, content: '', fontSize: 16 },
    },
    {
      id: configId,
      type: 'config',
      position: { x: 360, y: 0 },
      data: { label: labels.config, generationMode: 'image', model: 'gpt-image-1', size: '1024x1024', quality: 'auto', count: 1, status: 'ready' },
    },
    {
      id: imageId,
      type: 'image',
      position: { x: 720, y: 36 },
      data: { label: labels.image },
    },
  ]
  const edges = [
    createCanvasEdge(
      { source: textId, target: configId },
      nodes,
      [],
      `edge-${textId}-${configId}`,
    ),
    createCanvasEdge(
      { source: configId, target: imageId },
      nodes,
      [],
      `edge-${configId}-${imageId}`,
    ),
  ].filter((edge): edge is CanvasFlowEdge => Boolean(edge))

  return {
    schema_version: CANVAS_DOCUMENT_SCHEMA_VERSION,
    nodes,
    edges,
    viewport: { x: 80, y: 180, zoom: 0.82 },
  }
}

export function normalizeCanvasEdges(
  nodes: Array<{ id: string; type?: string }>,
  edges: CanvasEdge[],
): CanvasFlowEdge[] {
  const normalized: CanvasFlowEdge[] = []
  const edgeIds = new Set<string>()

  for (const edge of edges) {
    if (!edge || typeof edge.id !== 'string' || !edge.id || edgeIds.has(edge.id)) continue
    const connection = normalizeCanvasConnection(edge, nodes)
    if (!connection || !isCanvasConnectionValid(connection, nodes, normalized)) continue

    normalized.push({
      id: edge.id,
      ...connection,
      type: 'default',
      interactionWidth: 20,
      ariaLabel: `${connection.source} to ${connection.target}`,
    })
    edgeIds.add(edge.id)
  }

  return normalized
}

function cleanString(value: unknown, maxLength: number): string | undefined {
  if (typeof value !== 'string') return undefined
  const clean = value.slice(0, maxLength)
  return clean || undefined
}

function cleanAssetId(value: unknown): number | undefined {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : undefined
}

function cleanNumber(value: unknown, min: number, max: number): number | undefined {
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed)) return undefined
  return Math.min(max, Math.max(min, parsed))
}

function copyString(data: CanvasNodeData, input: CanvasNodeData, key: keyof CanvasNodeData, maxLength: number) {
  const value = cleanString(input[key], maxLength)
  if (value !== undefined) data[key] = value
}

function copyNumber(data: CanvasNodeData, input: CanvasNodeData, key: keyof CanvasNodeData, min: number, max: number) {
  const value = cleanNumber(input[key], min, max)
  if (value !== undefined) data[key] = value
}

export function serializeCanvasNodeData(type: CanvasNodeType, input: CanvasNodeData): CanvasNodeData {
  const label = cleanString(input.label, 160) || type
  const data: CanvasNodeData = { label }

  if (type === 'prompt') {
    data.prompt = cleanString(input.prompt, 32_000) || ''
  }
  if (type === 'text') {
    data.content = cleanString(input.content, 100_000) || ''
    copyNumber(data, input, 'fontSize', 10, 144)
  }
  if (type === 'reference' || type === 'image' || type === 'video' || type === 'audio') {
    const assetId = cleanAssetId(input.assetId)
    if (assetId) data.assetId = assetId
    copyString(data, input, 'fileName', 255)
    copyString(data, input, 'contentType', 128)
    copyString(data, input, 'taskId', 256)
    copyString(data, input, 'status', 64)
    copyNumber(data, input, 'naturalWidth', 1, 100_000)
    copyNumber(data, input, 'naturalHeight', 1, 100_000)
    copyNumber(data, input, 'durationMs', 0, 86_400_000)
  }
  if (type === 'image') {
    copyString(data, input, 'prompt', 32_000)
    copyString(data, input, 'model', 160)
    copyString(data, input, 'quality', 32)
    copyString(data, input, 'size', 32)
    copyString(data, input, 'background', 32)
  }
  if (type === 'generation' || type === 'config') {
    data.model = cleanString(input.model, 160) || 'gpt-image-1'
    data.status = cleanString(input.status, 64) || 'ready'
    copyString(data, input, 'taskId', 256)
    const mode = cleanString(input.generationMode, 16)
    if (mode === 'image' || mode === 'video' || mode === 'audio' || mode === 'text') data.generationMode = mode
    copyString(data, input, 'prompt', 32_000)
    copyString(data, input, 'composerContent', 32_000)
    copyString(data, input, 'quality', 32)
    copyString(data, input, 'size', 32)
    copyString(data, input, 'background', 32)
    copyString(data, input, 'aspectRatio', 32)
    copyString(data, input, 'resolution', 32)
    copyString(data, input, 'voice', 64)
    copyString(data, input, 'language', 16)
    copyString(data, input, 'format', 32)
    copyString(data, input, 'instructions', 8_000)
    copyNumber(data, input, 'count', 1, 10)
    copyNumber(data, input, 'seconds', 1, 60)
    copyNumber(data, input, 'speed', 0.25, 4)
  }
  copyString(data, input, 'groupId', 128)
  if (type === 'group' && typeof input.collapsed === 'boolean') data.collapsed = input.collapsed

  return data
}

function finiteNumber(value: unknown, fallback = 0) {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

export function normalizeCanvasNodes(input: CanvasNode[]): CanvasFlowNode[] {
  const nodes: CanvasFlowNode[] = []
  const ids = new Set<string>()

  for (const node of input) {
    if (!node || typeof node.id !== 'string' || !node.id || ids.has(node.id) || !isCanvasNodeType(node.type)) {
      continue
    }
    nodes.push({
      id: node.id,
      type: node.type,
      position: {
        x: finiteNumber(node.position?.x),
        y: finiteNumber(node.position?.y),
      },
      width: cleanNumber(node.width, 160, 2400) || CANVAS_NODE_DEFAULT_SIZE[node.type].width,
      height: cleanNumber(node.height, 120, 1800) || CANVAS_NODE_DEFAULT_SIZE[node.type].height,
      data: serializeCanvasNodeData(node.type, node.data || { label: node.type }),
      dragHandle: '.canvas-node__drag',
      ariaLabel: cleanString(node.data?.label, 160) || node.type,
    })
    ids.add(node.id)
  }

  return nodes
}

export function normalizeViewport(input?: Partial<ViewportTransform>): ViewportTransform {
  const zoom = finiteNumber(input?.zoom, 1)
  return {
    x: finiteNumber(input?.x),
    y: finiteNumber(input?.y),
    zoom: Math.min(5, Math.max(0.05, zoom)),
  }
}

export function normalizeCanvasDocument(document: CanvasDocument): {
  nodes: CanvasFlowNode[]
  edges: CanvasFlowEdge[]
  viewport: ViewportTransform
} {
  const nodes = normalizeCanvasNodes(Array.isArray(document.nodes) ? document.nodes : [])
  const edges = normalizeCanvasEdges(nodes, Array.isArray(document.edges) ? document.edges : [])
  return { nodes, edges, viewport: normalizeViewport(document.viewport) }
}

export function serializeCanvasDocument(
  nodes: CanvasFlowNode[],
  edges: CanvasFlowEdge[],
  viewport: ViewportTransform,
): CanvasDocument {
  const serializedNodes: CanvasNode[] = nodes
    .filter(node => isCanvasNodeType(node.type))
    .map(node => ({
      id: node.id,
      type: node.type as CanvasNodeType,
      position: {
        x: finiteNumber(node.position.x),
        y: finiteNumber(node.position.y),
      },
      width: cleanNumber(node.width, 160, 2400) || CANVAS_NODE_DEFAULT_SIZE[node.type as CanvasNodeType].width,
      height: cleanNumber(node.height, 120, 1800) || CANVAS_NODE_DEFAULT_SIZE[node.type as CanvasNodeType].height,
      data: serializeCanvasNodeData(node.type as CanvasNodeType, node.data),
    }))
  const serializedEdges = normalizeCanvasEdges(serializedNodes, edges as CanvasEdge[]).map(edge => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    sourceHandle: edge.sourceHandle,
    targetHandle: edge.targetHandle,
    type: 'default',
  }))

  return {
    schema_version: CANVAS_DOCUMENT_SCHEMA_VERSION,
    nodes: serializedNodes,
    edges: serializedEdges,
    viewport: normalizeViewport(viewport),
  }
}

export function hasTransientCanvasData(data: CanvasNodeData): boolean {
  return Object.keys(data).some(key => transientDataKeys.has(key))
}

export function connectedNode(
  targetId: string,
  targetHandle: string,
  nodes: CanvasFlowNode[],
  edges: CanvasFlowEdge[],
): CanvasFlowNode | undefined {
  const edge = edges.find(item => item.target === targetId && item.targetHandle === targetHandle)
  return edge ? nodes.find(node => node.id === edge.source) : undefined
}
