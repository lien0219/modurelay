import { buildGatewayUrl } from './client'
import { gatewayResponseError } from './gatewayError'
import i18n from '@/i18n'

export interface VideoGenerationInput {
  model: string
  prompt: string
  duration: number
  aspect_ratio: string
  resolution: string
}

export interface VideoGenerationTask {
  request_id?: string
  id?: string
  status?: string
  progress?: number
  model?: string
  video?: {
    url?: string
    duration?: number
  }
  url?: string
  video_url?: string
  download_url?: string
  error?: unknown
}

async function gatewayRequest(
  path: string,
  apiKey: string,
  init: RequestInit,
  fallback: string,
) {
  const response = await fetch(buildGatewayUrl(path), {
    ...init,
    headers: {
      Authorization: `Bearer ${apiKey}`,
      ...init.headers,
    },
  })
  if (!response.ok) throw await gatewayResponseError(response, fallback)
  return response
}

export const videoAPI = {
  async submit(apiKey: string, input: VideoGenerationInput, signal?: AbortSignal): Promise<VideoGenerationTask> {
    const response = await gatewayRequest('/v1/videos/generations', apiKey, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(input),
      signal,
    }, i18n.global.t('canvas.video.requestFailed'))
    return response.json()
  },

  async status(apiKey: string, requestId: string, signal?: AbortSignal): Promise<VideoGenerationTask> {
    const response = await gatewayRequest(`/v1/videos/${encodeURIComponent(requestId)}`, apiKey, {
      method: 'GET',
      signal,
    }, i18n.global.t('canvas.video.statusFailed'))
    return response.json()
  },

  async content(apiKey: string, requestId: string, signal?: AbortSignal): Promise<Blob> {
    const response = await gatewayRequest(`/v1/videos/${encodeURIComponent(requestId)}/content`, apiKey, {
      method: 'GET',
      headers: { Accept: 'video/*, application/octet-stream' },
      signal,
    }, i18n.global.t('canvas.video.downloadFailed'))
    return response.blob()
  },
}

export default videoAPI
