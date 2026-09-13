import { openDB, type DBSchema } from 'idb'
import type { CanvasDocument } from '@/api/canvas'

interface CanvasDraftDB extends DBSchema {
  drafts: { key: number; value: { projectId: number; document: CanvasDocument; title: string; savedAt: number } }
}

const dbPromise = openDB<CanvasDraftDB>('modurelay-canvas', 1, {
  upgrade(db) {
    if (!db.objectStoreNames.contains('drafts')) db.createObjectStore('drafts', { keyPath: 'projectId' })
  },
})

export async function saveCanvasDraft(projectId: number, title: string, document: CanvasDocument) {
  const db = await dbPromise
  await db.put('drafts', { projectId, title, document, savedAt: Date.now() })
}

export async function getCanvasDraft(projectId: number) {
  const db = await dbPromise
  return db.get('drafts', projectId)
}

export async function removeCanvasDraft(projectId: number) {
  const db = await dbPromise
  await db.delete('drafts', projectId)
}
