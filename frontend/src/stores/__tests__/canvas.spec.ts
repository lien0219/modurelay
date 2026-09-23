import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { CanvasDocument, CanvasProject } from '@/api/canvas'
import { useCanvasStore } from '@/stores/canvas'

const api = vi.hoisted(() => ({
  getProject: vi.fn(),
  saveProject: vi.fn(),
  listProjects: vi.fn(),
  createProject: vi.fn(),
  deleteProject: vi.fn(),
}))

vi.mock('@/api/canvas', () => ({ canvasAPI: api }))

const document = (label: string): CanvasDocument => ({
  schema_version: 1,
  nodes: [{ id: 'prompt-1', type: 'prompt', position: { x: 0, y: 0 }, data: { label, prompt: label } }],
  edges: [],
  viewport: { x: 0, y: 0, zoom: 1 },
})

const project = (revision: number, nextDocument = document('initial')): CanvasProject => ({
  id: 7,
  title: 'Canvas',
  document: nextDocument,
  schema_version: 1,
  revision,
  document_bytes: 100,
  created_at: '2026-09-12T00:00:00Z',
  updated_at: '2026-09-12T00:00:00Z',
})

describe('canvas store save queue', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    api.getProject.mockResolvedValue(project(1))
  })

  it('persists the newest edit queued while an earlier revision is saving', async () => {
    let resolveFirst!: (value: CanvasProject) => void
    const firstResponse = new Promise<CanvasProject>(resolve => { resolveFirst = resolve })
    api.saveProject
      .mockReturnValueOnce(firstResponse)
      .mockResolvedValueOnce(project(3, document('second')))

    const store = useCanvasStore()
    await store.open(7)

    const firstSave = store.save('First', document('first'))
    await vi.waitFor(() => expect(api.saveProject).toHaveBeenCalledTimes(1))
    const secondSave = store.save('Second', document('second'))
    resolveFirst(project(2, document('first')))
    await Promise.all([firstSave, secondSave])

    expect(api.saveProject).toHaveBeenCalledTimes(2)
    expect(api.saveProject.mock.calls[0][1]).toBe(1)
    expect(api.saveProject.mock.calls[1][1]).toBe(2)
    expect(api.saveProject.mock.calls[1][2]).toBe('Second')
    expect(store.project?.revision).toBe(3)
    expect(store.saving).toBe(false)
  })

  it('clears the saving state after a failed write so the user can retry', async () => {
    api.saveProject
      .mockRejectedValueOnce(new Error('revision conflict'))
      .mockResolvedValueOnce(project(2, document('retry')))
    const store = useCanvasStore()
    await store.open(7)

    await expect(store.save('Failed', document('failed'))).rejects.toThrow('revision conflict')
    expect(store.saving).toBe(false)
    await store.save('Retry', document('retry'))

    expect(api.saveProject).toHaveBeenCalledTimes(2)
    expect(store.project?.revision).toBe(2)
  })
})
