export interface CanvasImageAngleParams {
  horizontalAngle: number
  pitchAngle: number
  cameraDistance: number
  wideAngle: boolean
}

export const DEFAULT_CANVAS_IMAGE_ANGLE: CanvasImageAngleParams = {
  horizontalAngle: 0,
  pitchAngle: 9,
  cameraDistance: 4.8,
  wideAngle: false,
}

function clamp(value: number, min: number, max: number, step = 1) {
  const finite = Number.isFinite(value) ? value : min
  return Math.min(max, Math.max(min, Math.round(finite / step) * step))
}

export function normalizeCanvasImageAngle(input: CanvasImageAngleParams): CanvasImageAngleParams {
  return {
    horizontalAngle: clamp(input.horizontalAngle, -60, 60),
    pitchAngle: clamp(input.pitchAngle, -45, 45),
    cameraDistance: Number(clamp(input.cameraDistance, 1, 10, 0.1).toFixed(1)),
    wideAngle: Boolean(input.wideAngle),
  }
}

export function canvasImageAnglePreviewTransform(input: CanvasImageAngleParams) {
  const params = normalizeCanvasImageAngle(input)
  const scale = 1.08 - params.cameraDistance * 0.035 - (params.wideAngle ? 0.08 : 0)
  return `perspective(520px) rotateY(${params.horizontalAngle * -0.45}deg) rotateX(${params.pitchAngle * 0.35}deg) scale(${Math.max(0.72, Math.min(1.08, scale))})`
}

export function buildCanvasImageAnglePrompt(input: CanvasImageAngleParams) {
  const params = normalizeCanvasImageAngle(input)
  const horizontal = params.horizontalAngle === 0
    ? 'front view'
    : `${Math.abs(params.horizontalAngle)} degrees to the ${params.horizontalAngle > 0 ? 'right' : 'left'}`
  const pitch = params.pitchAngle === 0
    ? 'eye level'
    : `${Math.abs(params.pitchAngle)} degree ${params.pitchAngle > 0 ? 'top-down' : 'low-angle'} view`
  const lens = params.wideAngle ? 'wide-angle lens' : 'standard lens'
  return `Recreate the source image from a new camera viewpoint: ${horizontal}, ${pitch}, camera distance ${params.cameraDistance.toFixed(1)}, ${lens}. Preserve the subject identity, materials, lighting, scene content, and visual style while changing only the camera viewpoint.`
}
