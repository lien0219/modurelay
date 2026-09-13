import { describe, expect, it } from 'vitest'
import { normalizeCanvasImageEditOperation, type CanvasImageEditOperation } from '@/utils/canvasImageEditor'

describe('canvas image editor parameters', () => {
  it('bounds crop, rotation, and output scale values', () => {
    const normalized = normalizeCanvasImageEditOperation({
      mode: 'transform',
      cropWidth: 1,
      cropHeight: 140,
      rotation: 45,
      flipX: 1,
      flipY: 0,
      scale: 8,
    } as unknown as CanvasImageEditOperation)

    expect(normalized).toEqual({
      mode: 'transform',
      cropWidth: 10,
      cropHeight: 100,
      rotation: 0,
      flipX: true,
      flipY: false,
      scale: 1,
    })
  })

  it('limits split output to a 6 by 6 grid', () => {
    expect(normalizeCanvasImageEditOperation({ mode: 'split', rows: 0, columns: 99 })).toEqual({
      mode: 'split',
      rows: 1,
      columns: 6,
    })
  })
})
