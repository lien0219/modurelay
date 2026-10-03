import { afterEach, describe, expect, it, vi } from 'vitest'

import { mediaModelFamily, normalizeRoutedModelName, videoTransportKind } from '@/services/api/media-adapters'
import { providerAxios } from '@/services/api/provider-transport'
import {
  createVideoGenerationTask,
  pollVideoGenerationTask,
  waitForVideoGenerationTask,
  type VideoGenerationTask,
} from '@/services/api/video'
import type { AiConfig } from '@/stores/use-config-store'
import type { ReferenceImage } from '@/types/image'

const routedModel = '42:seedance-2.0'
const apiKey = 'canvas-test-key'

function videoConfig(videoMode = 'frames'): AiConfig {
  const modelOption = `relay::${routedModel}`
  return {
    channelMode: 'local',
    baseUrl: 'https://relay.example',
    apiKey,
    apiFormat: 'openai',
    channels: [{
      id: 'relay',
      name: 'Relay',
      baseUrl: 'https://relay.example',
      apiKey,
      apiFormat: 'openai',
      models: [{ name: routedModel, capability: 'video' }],
    }],
    model: modelOption,
    imageModel: '',
    videoModel: modelOption,
    textModel: '',
    audioModel: '',
    audioVoice: '',
    audioFormat: '',
    audioSpeed: '',
    audioInstructions: '',
    videoSeconds: '8',
    vquality: '1080',
    videoGenerateAudio: 'false',
    videoWatermark: 'true',
    videoMode,
    systemPrompt: '',
    reasoningEffort: 'auto',
    models: [modelOption],
    quality: 'auto',
    size: '16:9',
    background: 'auto',
    count: '1',
    canvasImageCount: '3',
    proxyEnabled: false,
    proxyUrl: '',
  }
}

function referenceImage(id: string): ReferenceImage {
  return {
    id,
    name: `${id}.png`,
    type: 'image/png',
    dataUrl: 'data:image/png;base64,aGVsbG8=',
  }
}

function openAITask(): VideoGenerationTask {
  return { id: 'task-123', provider: 'openai', model: `relay::${routedModel}` }
}

afterEach(() => vi.restoreAllMocks())

describe('Seedance through the Canvas OpenAI-compatible video API', () => {
  it('recognizes routed Seedance names for compatible JSON transport', () => {
    expect(normalizeRoutedModelName(routedModel)).toBe('seedance-2.0')
    expect(mediaModelFamily(routedModel)).toBe('seedance')
    expect(videoTransportKind(routedModel)).toBe('compatible-json')
    expect(videoTransportKind('generic-video-model')).toBe('multipart')
  })

  it('posts frame inputs and video parameters to the shared /v1/videos endpoint', async () => {
    const post = vi.spyOn(providerAxios, 'post').mockResolvedValue({ data: { id: 'task-123' } } as never)

    const task = await createVideoGenerationTask(
      videoConfig(),
      'A simple moving scene',
      [referenceImage('first'), referenceImage('last')],
    )

    expect(task).toEqual({ id: 'task-123', provider: 'openai', model: `relay::${routedModel}` })
    expect(post).toHaveBeenCalledOnce()
    const [url, payload] = post.mock.calls[0] as [string, Record<string, unknown>]
    expect(url).toBe('https://relay.example/v1/videos')
    expect(payload).toMatchObject({
      model: routedModel,
      prompt: 'A simple moving scene',
      duration: 8,
      resolution: '1080p',
      aspect_ratio: '16:9',
      audio: false,
      generate_audio: false,
      watermark: true,
      media: [
        { type: 'first_frame', url: 'data:image/png;base64,aGVsbG8=' },
        { type: 'last_frame', url: 'data:image/png;base64,aGVsbG8=' },
      ],
    })
    expect(post.mock.calls[0]?.[2]).toMatchObject({
      headers: { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
    })
  })

  it('sends reference images as compatible JSON media', async () => {
    const post = vi.spyOn(providerAxios, 'post').mockResolvedValue({ data: { task_id: 'task-reference' } } as never)

    await createVideoGenerationTask(videoConfig('reference'), 'Use this reference', [referenceImage('reference')])

    const payload = post.mock.calls[0]?.[1] as Record<string, unknown>
    expect(payload.model).toBe(routedModel)
    expect(payload.media).toEqual([{ type: 'reference_image', url: 'data:image/png;base64,aGVsbG8=' }])
  })

  it('polls the bound task and resolves a result URL from the task response', async () => {
    const get = vi.spyOn(providerAxios, 'get')
      .mockResolvedValueOnce({ data: { id: 'task-123', status: 'succeeded', content: { video_url: 'https://media.example/result.mp4' } } } as never)
      .mockRejectedValueOnce(new Error('Direct media fetch is unavailable'))

    const state = await pollVideoGenerationTask(videoConfig(), openAITask())

    expect(get.mock.calls[0]?.[0]).toBe('https://relay.example/v1/videos/task-123')
    expect(state).toEqual({
      status: 'completed',
      result: { url: 'https://media.example/result.mp4', mimeType: 'video/mp4' },
    })
  })

  it('surfaces provider task failures without requesting result content', async () => {
    const get = vi.spyOn(providerAxios, 'get').mockResolvedValueOnce({
      data: { id: 'task-123', status: 'failed', error: { message: 'Generation failed' } },
    } as never)

    await expect(pollVideoGenerationTask(videoConfig(), openAITask())).resolves.toEqual({
      status: 'failed',
      error: 'Generation failed',
    })
    expect(get).toHaveBeenCalledOnce()
  })

  it('honors cancellation and task deadlines while polling', async () => {
    const get = vi.spyOn(providerAxios, 'get').mockResolvedValue({
      data: { id: 'task-123', status: 'processing' },
    } as never)
    const controller = new AbortController()
    controller.abort()

    await expect(waitForVideoGenerationTask(videoConfig(), openAITask(), { signal: controller.signal }))
      .rejects.toMatchObject({ name: 'AbortError' })
    expect(get).not.toHaveBeenCalled()

    await expect(waitForVideoGenerationTask(videoConfig(), openAITask(), { deadlineAt: Date.now() - 1 }))
      .rejects.toMatchObject({ name: 'VideoTaskTimeout' })
  })
})
