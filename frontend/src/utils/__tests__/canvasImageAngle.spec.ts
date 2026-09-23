import { describe, expect, it } from 'vitest'
import {
  buildCanvasImageAnglePrompt,
  canvasImageAnglePreviewTransform,
  normalizeCanvasImageAngle,
} from '@/utils/canvasImageAngle'

describe('canvas image angle', () => {
  it('bounds camera parameters', () => {
    expect(normalizeCanvasImageAngle({
      horizontalAngle: 90,
      pitchAngle: -80,
      cameraDistance: 4.84,
      wideAngle: 1 as unknown as boolean,
    })).toEqual({
      horizontalAngle: 60,
      pitchAngle: -45,
      cameraDistance: 4.8,
      wideAngle: true,
    })
  })

  it('builds a deterministic edit prompt and bounded preview transform', () => {
    const params = { horizontalAngle: -30, pitchAngle: 15, cameraDistance: 6.2, wideAngle: true }
    expect(buildCanvasImageAnglePrompt(params)).toContain('30 degrees to the left')
    expect(buildCanvasImageAnglePrompt(params)).toContain('15 degree top-down view')
    expect(buildCanvasImageAnglePrompt(params)).toContain('wide-angle lens')
    expect(canvasImageAnglePreviewTransform(params)).toContain('perspective(520px)')
  })
})
