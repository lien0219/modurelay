import { afterEach, describe, expect, it, vi } from 'vitest'
import { CANVAS_GROK_TTS_VOICES, canvasAPI } from '@/api/canvas'

describe('canvasAPI.submitSpeech', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('uses the production Grok TTS endpoint and request contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(new Blob(['ID3audio'], { type: 'audio/mpeg' }), {
      status: 200,
      headers: { 'Content-Type': 'audio/mpeg' },
    }))
    vi.stubGlobal('fetch', fetchMock)

    const result = await canvasAPI.submitSpeech('canvas-key', {
      prompt: 'Read this',
      voice: CANVAS_GROK_TTS_VOICES[0],
      language: 'en',
    })

    expect(fetchMock).toHaveBeenCalledOnce()
    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toMatch(/\/v1\/tts$/)
    expect(request.headers).toEqual({
      Authorization: 'Bearer canvas-key',
      'Content-Type': 'application/json',
    })
    expect(JSON.parse(String(request.body))).toEqual({
      text: 'Read this',
      language: 'en',
      voice_id: 'Ara',
    })
    expect(result.type).toBe('audio/mpeg')
  })
})

describe('canvasAPI.submitEdit', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('sends a transparent PNG mask through the production multipart contract', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(JSON.stringify({ task_id: 'image-task-1', status: 'queued' }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }))
    vi.stubGlobal('fetch', fetchMock)
    const source = new Blob(['source'], { type: 'image/png' })
    const mask = new Blob(['mask'], { type: 'image/png' })

    await canvasAPI.submitEdit('canvas-key', 'Replace the selected area', 'gpt-image-1', source, 'source.png', {
      size: 'auto',
      count: 1,
      mask,
      maskFileName: 'selection-mask.png',
    })

    const [url, request] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect(url).toMatch(/\/v1\/images\/edits\/async$/)
    expect(request.headers).toEqual({ Authorization: 'Bearer canvas-key' })
    expect(request.body).toBeInstanceOf(FormData)
    const body = request.body as FormData
    expect(body.get('prompt')).toBe('Replace the selected area')
    expect(body.get('size')).toBe('auto')
    expect((body.get('image') as File).name).toBe('source.png')
    expect((body.get('mask') as File).name).toBe('selection-mask.png')
  })
})
