import { describe, expect, it, vi } from 'vitest'
import type { CanvasDocument } from '@/api/canvas'
import {
  CANVAS_HANDLES,
  applyCanvasGroupSelection,
  applyCanvasUngroupSelection,
  canGroupCanvasNodes,
  findCanvasGroupDropTarget,
  findContainingCanvasGroupId,
  createCanvasEdge,
  createStarterCanvasDocument,
  findAvailableCanvasPosition,
  isCanvasConnectionValid,
  normalizeCanvasDocument,
  serializeCanvasDocument,
  snapCanvasNodesIntoGroup,
  type CanvasFlowEdge,
  type CanvasFlowNode,
} from '@/utils/canvasGraph'

vi.stubGlobal('crypto', { randomUUID: () => 'generated-id' })

const nodes: CanvasFlowNode[] = [
  { id: 'prompt-1', type: 'prompt', position: { x: 0, y: 0 }, data: { label: 'Prompt', prompt: 'A quiet studio' } },
  { id: 'reference-1', type: 'reference', position: { x: 0, y: 100 }, data: { label: 'Reference', assetId: 12 } },
  { id: 'generation-1', type: 'generation', position: { x: 300, y: 0 }, data: { label: 'Generate', model: 'gpt-image-1' } },
  { id: 'image-1', type: 'image', position: { x: 600, y: 0 }, data: { label: 'Result', assetId: 13 } },
  { id: 'config-1', type: 'config', position: { x: 300, y: 300 }, data: { label: 'Config' } },
  { id: 'config-2', type: 'config', position: { x: 600, y: 300 }, data: { label: 'Config 2' } },
  { id: 'group-1', type: 'group', position: { x: 900, y: 0 }, data: { label: 'Group' } },
]

describe('canvas graph connections', () => {
  it('accepts resource connections while protecting group and config invariants', () => {
    expect(isCanvasConnectionValid({ source: 'prompt-1', target: 'generation-1' }, nodes, [])).toBe(true)
    expect(isCanvasConnectionValid({ source: 'reference-1', target: 'generation-1' }, nodes, [])).toBe(true)
    expect(isCanvasConnectionValid({ source: 'generation-1', target: 'image-1' }, nodes, [])).toBe(true)
    expect(isCanvasConnectionValid({ source: 'prompt-1', target: 'image-1' }, nodes, [])).toBe(true)
    expect(isCanvasConnectionValid({ source: 'image-1', target: 'config-1' }, nodes, [])).toBe(true)
    expect(isCanvasConnectionValid({ source: 'config-1', target: 'config-2' }, nodes, [])).toBe(false)
    expect(isCanvasConnectionValid({ source: 'image-1', target: 'group-1' }, nodes, [])).toBe(false)
    expect(isCanvasConnectionValid({ source: 'prompt-1', target: 'prompt-1' }, nodes, [])).toBe(false)
  })

  it('normalizes legacy connections and rejects duplicate or occupied inputs', () => {
    const edge = createCanvasEdge({ source: 'prompt-1', target: 'generation-1' }, nodes, [], 'edge-1')
    expect(edge).toMatchObject({
      sourceHandle: CANVAS_HANDLES.promptOutput,
      targetHandle: CANVAS_HANDLES.generationPrompt,
      type: 'default',
    })

    expect(createCanvasEdge({ source: 'prompt-1', target: 'generation-1' }, nodes, [edge!] as CanvasFlowEdge[])).toBeNull()

    const secondPrompt = {
      id: 'prompt-2',
      type: 'prompt' as const,
      position: { x: 0, y: 200 },
      data: { label: 'Prompt 2', prompt: 'Alternative' },
    }
    expect(createCanvasEdge(
      { source: secondPrompt.id, target: 'generation-1' },
      [...nodes, secondPrompt],
      [edge!] as CanvasFlowEdge[],
    )).toBeNull()
  })

  it('ignores only the edge being revalidated by the controlled flow model', () => {
    const edge = createCanvasEdge({ source: 'prompt-1', target: 'generation-1' }, nodes, [], 'edge-1')!

    expect(isCanvasConnectionValid(edge, nodes, [edge], edge.id)).toBe(true)
    expect(isCanvasConnectionValid({ ...edge, id: 'edge-2' }, nodes, [edge], 'edge-2')).toBe(false)
  })
})

describe('canvas document normalization', () => {
  it('creates a connected three-node starter workflow', () => {
    const document = createStarterCanvasDocument({
      text: 'Text',
      config: 'Generator',
      image: 'Image',
    })

    expect(document.nodes).toHaveLength(3)
    expect(document.edges).toHaveLength(2)
    expect(new Set(document.nodes.map(node => node.id)).size).toBe(3)
    expect(new Set(document.edges.map(edge => edge.id)).size).toBe(2)
    expect(document.edges).toEqual([
      expect.objectContaining({
        source: document.nodes[0].id,
        target: document.nodes[1].id,
        sourceHandle: CANVAS_HANDLES.genericOutput,
        targetHandle: CANVAS_HANDLES.genericInput,
      }),
      expect.objectContaining({
        source: document.nodes[1].id,
        target: document.nodes[2].id,
        sourceHandle: CANVAS_HANDLES.genericOutput,
        targetHandle: CANVAS_HANDLES.genericInput,
      }),
    ])
  })

  it('keeps supported legacy edges and restores their handles', () => {
    const document: CanvasDocument = {
      schema_version: 1,
      nodes,
      edges: [
        { id: 'edge-1', source: 'prompt-1', target: 'generation-1' },
        { id: 'edge-invalid', source: 'prompt-1', target: 'image-1' },
      ],
      viewport: { x: 20, y: 30, zoom: 99 },
    }

    const normalized = normalizeCanvasDocument(document)
    expect(normalized.edges).toHaveLength(2)
    expect(normalized.edges[0]).toMatchObject({
      sourceHandle: CANVAS_HANDLES.promptOutput,
      targetHandle: CANVAS_HANDLES.generationPrompt,
    })
    expect(normalized.edges[1]).toMatchObject({
      sourceHandle: CANVAS_HANDLES.promptOutput,
      targetHandle: CANVAS_HANDLES.genericInput,
    })
    expect(normalized.viewport.zoom).toBe(5)
  })

  it('serializes only durable node data and viewport state', () => {
    const runtimeNodes: CanvasFlowNode[] = [
      ...nodes,
      {
        id: 'reference-2',
        type: 'reference',
        position: { x: Number.NaN, y: 10 },
        data: {
          label: 'Uploaded',
          assetId: 42,
          fileName: 'input.png',
          contentType: 'image/png',
          url: 'https://temporary.example/signed',
          previewUrl: 'blob:local',
          error: 'request detail',
          apiKey: 'must-not-survive',
        },
      },
    ]

    const document = serializeCanvasDocument(runtimeNodes, [], { x: 1, y: 2, zoom: 1.2 })
    const reference = document.nodes.find(node => node.id === 'reference-2')

    expect(reference?.position.x).toBe(0)
    expect(reference?.data).toEqual({
      label: 'Uploaded',
      assetId: 42,
      fileName: 'input.png',
      contentType: 'image/png',
    })
    expect(JSON.stringify(document)).not.toContain('temporary.example')
    expect(JSON.stringify(document)).not.toContain('must-not-survive')
    expect(document.schema_version).toBe(2)
  })
})

describe('canvas node placement', () => {
  it('keeps a clear preferred position and moves right when it is occupied', () => {
    expect(findAvailableCanvasPosition({ x: 100, y: 80 }, [])).toEqual({ x: 100, y: 80 })
    expect(findAvailableCanvasPosition(
      { x: 100, y: 80 },
      [{ position: { x: 100, y: 80 } }],
    )).toEqual({ x: 460, y: 80 })
  })

  it('skips nearby occupied slots without overlapping user-positioned nodes', () => {
    const position = findAvailableCanvasPosition(
      { x: 0, y: 0 },
      [
        { position: { x: 0, y: 0 } },
        { position: { x: 360, y: 0 } },
        { position: { x: 350, y: 300 } },
      ],
    )

    expect(position).toEqual({ x: 360, y: -304 })
  })
})

describe('canvas groups', () => {
  const groupedNodes: CanvasFlowNode[] = [
    { id: 'group-a', type: 'group', position: { x: 0, y: 0 }, width: 700, height: 500, data: { label: 'Group A' } },
    { id: 'text-a', type: 'text', position: { x: 40, y: 80 }, width: 200, height: 160, data: { label: 'A', groupId: 'group-a' } },
    { id: 'image-a', type: 'image', position: { x: 280, y: 80 }, width: 240, height: 180, data: { label: 'B', groupId: 'group-a' } },
    { id: 'text-b', type: 'text', position: { x: 760, y: 80 }, width: 200, height: 160, data: { label: 'C' } },
  ]

  it('detects drop targets, constrains members, and clears membership outside a group', () => {
    expect(findCanvasGroupDropTarget(new Set(['text-b']), groupedNodes)?.id).toBeUndefined()
    const moved = groupedNodes.map(node => node.id === 'text-b' ? { ...node, position: { x: 560, y: 320 } } : node)
    const target = findCanvasGroupDropTarget(new Set(['text-b']), moved)
    expect(target?.id).toBe('group-a')
    const snapped = snapCanvasNodesIntoGroup(new Set(['text-b']), moved, target!)
    const child = snapped.find(node => node.id === 'text-b')!
    expect(child.data.groupId).toBe('group-a')
    expect(child.position.x + Number(child.width)).toBeLessThanOrEqual(676)
    expect(findContainingCanvasGroupId({ ...child, position: { x: 900, y: 700 } }, snapped)).toBeUndefined()
  })

  it('flattens selected groups into a new group and releases only requested members', () => {
    expect(canGroupCanvasNodes(new Set(['group-a', 'text-b']), groupedNodes)).toBe(true)
    const group: CanvasFlowNode = { id: 'group-b', type: 'group', position: { x: 0, y: 0 }, data: { label: 'Group B' } }
    const grouped = applyCanvasGroupSelection(new Set(['group-a', 'text-b']), groupedNodes, [], group)!
    expect(grouped.nodes.some(node => node.id === 'group-a')).toBe(false)
    expect(grouped.nodes.filter(node => node.type !== 'group').every(node => node.data.groupId === 'group-b')).toBe(true)

    const released = applyCanvasUngroupSelection(new Set(['text-a']), groupedNodes, [])!
    expect(released.nodes.find(node => node.id === 'text-a')?.data.groupId).toBeUndefined()
    expect(released.nodes.find(node => node.id === 'image-a')?.data.groupId).toBe('group-a')
    expect(released.nodes.some(node => node.id === 'group-a')).toBe(true)
  })
})
