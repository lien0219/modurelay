import { describe, expect, it, vi } from 'vitest'
import { readCanvasClipboard } from '@/utils/canvasClipboard'

describe('readCanvasClipboard', () => {
  it('prefers a real clipboard image over text', async () => {
    const image = new Blob(['png'], { type: 'image/png' })
    const result = await readCanvasClipboard({
      read: vi.fn().mockResolvedValue([{ types: ['text/plain', 'image/png'], getType: vi.fn().mockResolvedValue(image) }]),
      readText: vi.fn().mockResolvedValue('ignored'),
    } as unknown as Clipboard)

    expect(result?.kind).toBe('image')
    expect(result?.kind === 'image' && result.file.name).toBe('clipboard-image.png')
  })

  it('returns trimmed text when no image exists', async () => {
    const result = await readCanvasClipboard({
      read: vi.fn().mockResolvedValue([]),
      readText: vi.fn().mockResolvedValue('  a canvas note  '),
    } as unknown as Clipboard)

    expect(result).toEqual({ kind: 'text', text: 'a canvas note' })
  })
})
