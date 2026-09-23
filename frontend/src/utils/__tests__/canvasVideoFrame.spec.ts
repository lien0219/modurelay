import { describe, expect, it } from 'vitest'
import { resolveCanvasVideoFrameTime } from '@/utils/canvasVideoFrame'

describe('resolveCanvasVideoFrameTime', () => {
  it('resolves first, current, and final frame positions inside media bounds', () => {
    expect(resolveCanvasVideoFrameTime('first', 12, 6)).toBe(0)
    expect(resolveCanvasVideoFrameTime('current', 12, 6)).toBe(6)
    expect(resolveCanvasVideoFrameTime('current', 12, 50)).toBeCloseTo(11.999)
    expect(resolveCanvasVideoFrameTime('last', 12, 0)).toBeCloseTo(11.999)
  })

  it('rejects non-finite media timing as a zero-time frame', () => {
    expect(resolveCanvasVideoFrameTime('current', Number.NaN, Number.NaN)).toBe(0)
    expect(resolveCanvasVideoFrameTime('last', Number.POSITIVE_INFINITY, 4)).toBe(0)
  })
})
