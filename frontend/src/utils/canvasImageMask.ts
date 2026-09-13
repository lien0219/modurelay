export const CANVAS_MASK_MIN_BRUSH_SIZE = 8
export const CANVAS_MASK_MAX_BRUSH_SIZE = 160

export interface CanvasImageMaskPayload {
  prompt: string
  mask: Blob
  width: number
  height: number
  generate: boolean
}

export function normalizeCanvasMaskBrushSize(value: number) {
  const finite = Number.isFinite(value) ? value : 100
  return Math.min(CANVAS_MASK_MAX_BRUSH_SIZE, Math.max(CANVAS_MASK_MIN_BRUSH_SIZE, Math.round(finite / 2) * 2))
}

export function createCanvasEditMaskPixels(selectionPixels: Uint8ClampedArray) {
  const mask = new Uint8ClampedArray(selectionPixels.length)
  for (let index = 0; index < selectionPixels.length; index += 4) {
    mask[index] = 255
    mask[index + 1] = 255
    mask[index + 2] = 255
    mask[index + 3] = 255 - selectionPixels[index + 3]
  }
  return mask
}

export function canvasMaskHasSelection(selectionPixels: Uint8ClampedArray) {
  for (let index = 3; index < selectionPixels.length; index += 4) {
    if (selectionPixels[index] > 0) return true
  }
  return false
}

function canvasBlob(canvas: HTMLCanvasElement) {
  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(blob => blob ? resolve(blob) : reject(new Error('CANVAS_MASK_EXPORT_FAILED')), 'image/png')
  })
}

export async function buildCanvasEditMask(selectionCanvas: HTMLCanvasElement) {
  const selectionContext = selectionCanvas.getContext('2d', { willReadFrequently: true })
  if (!selectionContext) throw new Error('CANVAS_IMAGE_EDITOR_UNAVAILABLE')
  const selection = selectionContext.getImageData(0, 0, selectionCanvas.width, selectionCanvas.height)
  if (!canvasMaskHasSelection(selection.data)) throw new Error('CANVAS_MASK_REQUIRED')

  const canvas = document.createElement('canvas')
  canvas.width = selectionCanvas.width
  canvas.height = selectionCanvas.height
  const context = canvas.getContext('2d')
  if (!context) throw new Error('CANVAS_IMAGE_EDITOR_UNAVAILABLE')
  context.putImageData(new ImageData(createCanvasEditMaskPixels(selection.data), canvas.width, canvas.height), 0, 0)
  return canvasBlob(canvas)
}
