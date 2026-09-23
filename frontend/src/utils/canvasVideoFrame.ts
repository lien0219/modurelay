export type CanvasVideoFramePosition = 'first' | 'current' | 'last'

export function resolveCanvasVideoFrameTime(
  position: CanvasVideoFramePosition,
  duration: number,
  currentTime: number,
) {
  const endTime = Number.isFinite(duration) ? Math.max(0, duration - 0.001) : 0
  if (position === 'first') return 0
  if (position === 'last') return endTime
  return Math.min(Math.max(0, Number.isFinite(currentTime) ? currentTime : 0), endTime)
}

function waitForVideo(video: HTMLVideoElement, eventName: 'loadedmetadata' | 'loadeddata' | 'seeked') {
  return new Promise<void>((resolve, reject) => {
    const cleanup = () => {
      video.removeEventListener(eventName, succeed)
      video.removeEventListener('error', fail)
    }
    const succeed = () => {
      cleanup()
      resolve()
    }
    const fail = () => {
      cleanup()
      reject(new Error('CANVAS_VIDEO_FRAME_READ_FAILED'))
    }
    video.addEventListener(eventName, succeed, { once: true })
    video.addEventListener('error', fail, { once: true })
  })
}

export async function captureCanvasVideoFrame(
  source: string,
  position: CanvasVideoFramePosition,
  currentTime = 0,
): Promise<{ blob: Blob; width: number; height: number }> {
  const video = document.createElement('video')
  video.crossOrigin = 'anonymous'
  video.muted = true
  video.playsInline = true
  video.preload = 'auto'

  try {
    const metadataLoaded = waitForVideo(video, 'loadedmetadata')
    video.src = source
    video.load()
    await metadataLoaded

    if (!video.videoWidth || !video.videoHeight) throw new Error('CANVAS_VIDEO_FRAME_READ_FAILED')
    const targetTime = resolveCanvasVideoFrameTime(position, video.duration, currentTime)
    if (targetTime > 0) {
      const seeked = waitForVideo(video, 'seeked')
      video.currentTime = targetTime
      await seeked
    } else if (video.readyState < HTMLMediaElement.HAVE_CURRENT_DATA) {
      await waitForVideo(video, 'loadeddata')
    }

    const canvas = document.createElement('canvas')
    canvas.width = video.videoWidth
    canvas.height = video.videoHeight
    const context = canvas.getContext('2d')
    if (!context) throw new Error('CANVAS_IMAGE_EDITOR_UNAVAILABLE')
    context.drawImage(video, 0, 0)
    const blob = await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob(result => result ? resolve(result) : reject(new Error('CANVAS_IMAGE_EXPORT_FAILED')), 'image/png')
    })
    return { blob, width: canvas.width, height: canvas.height }
  } finally {
    video.removeAttribute('src')
    video.load()
  }
}
