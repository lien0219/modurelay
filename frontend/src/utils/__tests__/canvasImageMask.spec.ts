import { describe, expect, it } from 'vitest'
import {
  canvasMaskHasSelection,
  createCanvasEditMaskPixels,
  normalizeCanvasMaskBrushSize,
} from '@/utils/canvasImageMask'

describe('canvas image mask', () => {
  it('turns painted selection pixels into transparent edit regions', () => {
    const selection = new Uint8ClampedArray([
      0, 0, 0, 0,
      0, 0, 0, 128,
      0, 0, 0, 255,
    ])

    expect(Array.from(createCanvasEditMaskPixels(selection))).toEqual([
      255, 255, 255, 255,
      255, 255, 255, 127,
      255, 255, 255, 0,
    ])
    expect(canvasMaskHasSelection(selection)).toBe(true)
    expect(canvasMaskHasSelection(new Uint8ClampedArray(8))).toBe(false)
  })

  it('bounds and snaps brush sizes', () => {
    expect(normalizeCanvasMaskBrushSize(2)).toBe(8)
    expect(normalizeCanvasMaskBrushSize(53)).toBe(54)
    expect(normalizeCanvasMaskBrushSize(999)).toBe(160)
  })
})
