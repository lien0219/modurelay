export type CanvasImageEditOperation =
  | {
      mode: 'transform'
      cropWidth: number
      cropHeight: number
      rotation: 0 | 90 | 180 | 270
      flipX: boolean
      flipY: boolean
      scale: 1 | 2 | 4
    }
  | { mode: 'split'; rows: number; columns: number }

export interface CanvasImageEditResult {
  blob: Blob
  width: number
  height: number
  suffix: string
}

const MAX_OUTPUT_EDGE = 8192
const MAX_OUTPUT_PIXELS = 64_000_000

function clampInteger(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, Math.round(Number.isFinite(value) ? value : min)))
}

export function normalizeCanvasImageEditOperation(operation: CanvasImageEditOperation): CanvasImageEditOperation {
  if (operation.mode === 'split') {
    return {
      mode: 'split',
      rows: clampInteger(operation.rows, 1, 6),
      columns: clampInteger(operation.columns, 1, 6),
    }
  }
  const validRotation = [0, 90, 180, 270].includes(operation.rotation) ? operation.rotation : 0
  const validScale = [1, 2, 4].includes(operation.scale) ? operation.scale : 1
  return {
    mode: 'transform',
    cropWidth: clampInteger(operation.cropWidth, 10, 100),
    cropHeight: clampInteger(operation.cropHeight, 10, 100),
    rotation: validRotation as 0 | 90 | 180 | 270,
    flipX: Boolean(operation.flipX),
    flipY: Boolean(operation.flipY),
    scale: validScale as 1 | 2 | 4,
  }
}

async function loadDrawable(source: string) {
  const response = await fetch(source)
  if (!response.ok) throw new Error('CANVAS_IMAGE_READ_FAILED')
  const blob = await response.blob()
  if (!blob.size) throw new Error('CANVAS_IMAGE_READ_FAILED')
  if ('createImageBitmap' in window) {
    const bitmap = await createImageBitmap(blob)
    return {
      source: bitmap as CanvasImageSource,
      width: bitmap.width,
      height: bitmap.height,
      dispose: () => bitmap.close(),
    }
  }

  const objectURL = URL.createObjectURL(blob)
  const image = new Image()
  image.decoding = 'async'
  image.src = objectURL
  await image.decode()
  return {
    source: image as CanvasImageSource,
    width: image.naturalWidth,
    height: image.naturalHeight,
    dispose: () => URL.revokeObjectURL(objectURL),
  }
}

function createOutputCanvas(width: number, height: number) {
  const safeWidth = Math.max(1, Math.round(width))
  const safeHeight = Math.max(1, Math.round(height))
  if (safeWidth > MAX_OUTPUT_EDGE || safeHeight > MAX_OUTPUT_EDGE || safeWidth * safeHeight > MAX_OUTPUT_PIXELS) {
    throw new Error('CANVAS_IMAGE_OUTPUT_TOO_LARGE')
  }
  const canvas = document.createElement('canvas')
  canvas.width = safeWidth
  canvas.height = safeHeight
  const context = canvas.getContext('2d')
  if (!context) throw new Error('CANVAS_IMAGE_EDITOR_UNAVAILABLE')
  context.imageSmoothingEnabled = true
  context.imageSmoothingQuality = 'high'
  return { canvas, context }
}

function canvasBlob(canvas: HTMLCanvasElement) {
  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(blob => blob ? resolve(blob) : reject(new Error('CANVAS_IMAGE_EXPORT_FAILED')), 'image/png')
  })
}

export async function editCanvasImage(source: string, input: CanvasImageEditOperation): Promise<CanvasImageEditResult[]> {
  const operation = normalizeCanvasImageEditOperation(input)
  const drawable = await loadDrawable(source)
  try {
    if (operation.mode === 'split') {
      const results: CanvasImageEditResult[] = []
      for (let row = 0; row < operation.rows; row += 1) {
        const top = Math.round((row * drawable.height) / operation.rows)
        const bottom = Math.round(((row + 1) * drawable.height) / operation.rows)
        for (let column = 0; column < operation.columns; column += 1) {
          const left = Math.round((column * drawable.width) / operation.columns)
          const right = Math.round(((column + 1) * drawable.width) / operation.columns)
          const width = Math.max(1, right - left)
          const height = Math.max(1, bottom - top)
          const { canvas, context } = createOutputCanvas(width, height)
          context.drawImage(drawable.source, left, top, width, height, 0, 0, width, height)
          results.push({ blob: await canvasBlob(canvas), width, height, suffix: `r${row + 1}-c${column + 1}` })
        }
      }
      return results
    }

    const cropWidth = Math.max(1, Math.round(drawable.width * operation.cropWidth / 100))
    const cropHeight = Math.max(1, Math.round(drawable.height * operation.cropHeight / 100))
    const cropX = Math.round((drawable.width - cropWidth) / 2)
    const cropY = Math.round((drawable.height - cropHeight) / 2)
    const swapsAxes = operation.rotation === 90 || operation.rotation === 270
    const width = (swapsAxes ? cropHeight : cropWidth) * operation.scale
    const height = (swapsAxes ? cropWidth : cropHeight) * operation.scale
    const { canvas, context } = createOutputCanvas(width, height)
    context.translate(canvas.width / 2, canvas.height / 2)
    context.rotate(operation.rotation * Math.PI / 180)
    context.scale(operation.flipX ? -1 : 1, operation.flipY ? -1 : 1)
    context.drawImage(
      drawable.source,
      cropX,
      cropY,
      cropWidth,
      cropHeight,
      -cropWidth * operation.scale / 2,
      -cropHeight * operation.scale / 2,
      cropWidth * operation.scale,
      cropHeight * operation.scale,
    )
    return [{ blob: await canvasBlob(canvas), width: canvas.width, height: canvas.height, suffix: 'edited' }]
  } finally {
    drawable.dispose()
  }
}
