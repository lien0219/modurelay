import { describe, expect, it } from 'vitest'
import enCanvas from '@/i18n/locales/en/canvas'
import zhCanvas from '@/i18n/locales/zh/canvas'
import canvasViewSource from '@/views/user/CanvasView.vue?raw'
import canvasNodeSource from '@/components/canvas/CanvasNode.vue?raw'
import canvasMiniMapSource from '@/components/canvas/CanvasMiniMap.vue?raw'
import canvasImageAngleSource from '@/components/canvas/CanvasImageAngleDialog.vue?raw'
import canvasImageEditorSource from '@/components/canvas/CanvasImageEditorDialog.vue?raw'
import canvasImageMaskSource from '@/components/canvas/CanvasImageMaskDialog.vue?raw'

function leafKeys(value: unknown, prefix = ''): string[] {
  if (!value || typeof value !== 'object') return [prefix]
  return Object.entries(value as Record<string, unknown>)
    .flatMap(([key, child]) => leafKeys(child, prefix ? `${prefix}.${key}` : key))
    .sort()
}

function hasPath(value: unknown, path: string) {
  return path.split('.').every((segment) => {
    if (!value || typeof value !== 'object' || !(segment in value)) return false
    value = (value as Record<string, unknown>)[segment]
    return true
  })
}

describe('canvas locale parity', () => {
  it('keeps English and Chinese canvas keys aligned', () => {
    expect(leafKeys(enCanvas)).toEqual(leafKeys(zhCanvas))
  })

  it('defines every statically referenced canvas translation', () => {
    const sources = [
      canvasViewSource,
      canvasNodeSource,
      canvasMiniMapSource,
      canvasImageEditorSource,
      canvasImageMaskSource,
      canvasImageAngleSource,
    ].join('\n')
    const keys = [...sources.matchAll(/t\(['"](canvas\.[^'"]+)['"]/g)].map(match => match[1])
    expect([...new Set(keys)].filter(key => !hasPath(enCanvas, key))).toEqual([])
  })
})
