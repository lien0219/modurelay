import { apiClient, buildGatewayUrl } from './client'
import i18n from '@/i18n'

export type CanvasNodeType = 'prompt' | 'reference' | 'generation' | 'image' | 'text' | 'video' | 'audio' | 'config' | 'group'
export type CanvasGenerationMode = 'image' | 'video' | 'audio' | 'text'

export const CANVAS_GROK_TTS_MODEL = 'grok-voice-tts'
export const CANVAS_GROK_TTS_VOICES = ['Ara'] as const

export const CANVAS_DOCUMENT_SCHEMA_VERSION = 2
export const CANVAS_DOCUMENT_MIN_SCHEMA_VERSION = 1

export interface CanvasNodeData {
  label: string
  prompt?: string
  content?: string
  composerContent?: string
  model?: string
  status?: string
  taskId?: string
  assetId?: number
  fileName?: string
  contentType?: string
  url?: string
  error?: string
  generationMode?: CanvasGenerationMode
  quality?: string
  size?: string
  background?: string
  count?: number
  seconds?: number
  aspectRatio?: string
  resolution?: string
  voice?: string
  language?: string
  format?: string
  speed?: number
  instructions?: string
  fontSize?: number
  naturalWidth?: number
  naturalHeight?: number
  durationMs?: number
  groupId?: string
  collapsed?: boolean
  [key: string]: unknown
}

export interface CanvasNode {
  id: string
  type: CanvasNodeType | string
  position: { x: number; y: number }
  width?: number
  height?: number
  data: CanvasNodeData
  [key: string]: unknown
}

export interface CanvasEdge {
  id: string
  source: string
  target: string
  sourceHandle?: string | null
  targetHandle?: string | null
  type?: string
  [key: string]: unknown
}

export interface CanvasDocument {
  schema_version: number
  nodes: CanvasNode[]
  edges: CanvasEdge[]
  viewport: { x: number; y: number; zoom: number }
  background_mode?: 'dots' | 'lines' | 'blank'
  show_image_info?: boolean
  [key: string]: unknown
}

export interface CanvasProject {
  id: number
  title: string
  document: CanvasDocument
  schema_version: number
  revision: number
  document_bytes: number
  created_at: string
  updated_at: string
  last_opened_at?: string
}

export interface CanvasRevision {
  id: number
  project_id: number
  revision: number
  source: string
  schema_version: number
  document: CanvasDocument
  created_at: string
}

export interface CanvasAsset {
  id: number
  project_id: number
  status: string
  source: string
  source_task_id?: string
  source_image_index?: number
  file_name: string
  content_type: string
  size_bytes: number
  sha256: string
  created_at: string
  updated_at: string
}

export interface ImageTask {
  id: string
  task_id: string
  object: string
  status: string
  image_url?: string
  result?: { data?: Array<{ url?: string; b64_json?: string }> }
  error?: unknown
  expires_at: number
}

export interface CanvasImageGenerationOptions {
  size?: string
  quality?: string
  background?: string
  count?: number
}

export interface CanvasImageEditOptions extends CanvasImageGenerationOptions {
  mask?: Blob
  maskFileName?: string
}

function gatewayResponseText(payload: unknown): string {
  if (!payload || typeof payload !== 'object') return ''
  const record = payload as Record<string, unknown>
  if (typeof record.output_text === 'string') return record.output_text
  if (typeof record.text === 'string') return record.text
  if (!Array.isArray(record.output)) return ''
  return record.output.flatMap((item) => {
    if (!item || typeof item !== 'object') return []
    const content = (item as Record<string, unknown>).content
    if (!Array.isArray(content)) return []
    return content.flatMap((part) => {
      if (!part || typeof part !== 'object') return []
      const value = (part as Record<string, unknown>).text
      return typeof value === 'string' ? [value] : []
    })
  }).join('\n').trim()
}

async function gatewayError(response: Response, fallback: string) {
  const body = await response.text()
  if (!body) return `${fallback} (${response.status})`
  try {
    const payload = JSON.parse(body) as { message?: string; error?: string | { message?: string } }
    if (typeof payload.error === 'string' && payload.error.trim()) return payload.error
    if (typeof payload.error === 'object' && payload.error?.message) return payload.error.message
    if (payload.message?.trim()) return payload.message
  } catch {
    return body.slice(0, 500)
  }
  return `${fallback} (${response.status})`
}

export const canvasAPI = {
  async listProjects(): Promise<CanvasProject[]> {
    const { data } = await apiClient.get<CanvasProject[]>('/canvas/projects')
    return data || []
  },
  async createProject(title?: string): Promise<CanvasProject> {
    const { data } = await apiClient.post<CanvasProject>('/canvas/projects', { title })
    return data
  },
  async getProject(id: number): Promise<CanvasProject> {
    const { data } = await apiClient.get<CanvasProject>(`/canvas/projects/${id}`)
    return data
  },
  async saveProject(id: number, revision: number, title: string, document: CanvasDocument): Promise<CanvasProject> {
    const { data } = await apiClient.put<CanvasProject>(`/canvas/projects/${id}`, { title, document }, { headers: { 'If-Match': String(revision) } })
    return data
  },
  async deleteProject(id: number): Promise<void> {
    await apiClient.delete(`/canvas/projects/${id}`)
  },
  async listRevisions(id: number): Promise<CanvasRevision[]> {
    const { data } = await apiClient.get<CanvasRevision[]>(`/canvas/projects/${id}/revisions`)
    return data || []
  },
  async checkpoint(id: number): Promise<CanvasRevision> {
    const { data } = await apiClient.post<CanvasRevision>(`/canvas/projects/${id}/checkpoints`)
    return data
  },
  async restore(id: number, revisionId: number, revision: number): Promise<CanvasProject> {
    const { data } = await apiClient.post<CanvasProject>(`/canvas/projects/${id}/revisions/${revisionId}/restore`, {}, { headers: { 'If-Match': String(revision) } })
    return data
  },
  async uploadAsset(projectId: number, file: File, idempotencyKey: string): Promise<CanvasAsset> {
    const body = new FormData()
    body.append('file', file)
    const { data } = await apiClient.post<CanvasAsset>(`/canvas/projects/${projectId}/assets`, body, { headers: { 'Content-Type': 'multipart/form-data', 'Idempotency-Key': idempotencyKey } })
    return data
  },
  async promoteTaskAsset(projectId: number, taskId: string, imageIndex = 0, idempotencyKey: string): Promise<CanvasAsset> {
    const { data } = await apiClient.post<CanvasAsset>(`/canvas/projects/${projectId}/assets/from-task`, { task_id: taskId, image_index: imageIndex }, { headers: { 'Idempotency-Key': idempotencyKey } })
    return data
  },
  async assetURL(assetId: number): Promise<string> {
    const { data } = await apiClient.get<{ url: string }>(`/canvas/assets/${assetId}/url`)
    return data.url
  },
  async deleteAsset(assetId: number): Promise<void> {
    await apiClient.delete(`/canvas/assets/${assetId}`)
  },
  async submitGeneration(apiKey: string, prompt: string, model = 'gpt-image-1', options: CanvasImageGenerationOptions = {}, signal?: AbortSignal): Promise<ImageTask> {
    const response = await fetch(buildGatewayUrl('/v1/images/generations/async'), {
      method: 'POST',
      headers: { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        model,
        prompt,
        size: options.size || '1024x1024',
        quality: options.quality || 'auto',
        n: Math.max(1, Math.min(4, Math.round(options.count || 1))),
        ...(options.background ? { background: options.background } : {}),
      }),
      signal,
    })
    if (!response.ok) throw new Error((await response.text()) || `${i18n.global.t('canvas.requestFailed')} (${response.status})`)
    return response.json()
  },
  async submitEdit(apiKey: string, prompt: string, model: string, image: Blob, fileName: string, options: CanvasImageEditOptions = {}, signal?: AbortSignal): Promise<ImageTask> {
    const body = new FormData()
    body.append('model', model)
    body.append('prompt', prompt)
    body.append('image', image, fileName || 'reference.png')
    if (options.mask) body.append('mask', options.mask, options.maskFileName || 'mask.png')
    body.append('size', options.size || '1024x1024')
    body.append('quality', options.quality || 'auto')
    body.append('n', String(Math.max(1, Math.min(4, Math.round(options.count || 1)))))
    if (options.background) body.append('background', options.background)
    const response = await fetch(buildGatewayUrl('/v1/images/edits/async'), {
      method: 'POST',
      headers: { Authorization: `Bearer ${apiKey}` },
      body,
      signal,
    })
    if (!response.ok) throw new Error((await response.text()) || `${i18n.global.t('canvas.requestFailed')} (${response.status})`)
    return response.json()
  },
  async getImageTask(apiKey: string, taskId: string, signal?: AbortSignal): Promise<ImageTask> {
    const response = await fetch(buildGatewayUrl(`/v1/images/tasks/${encodeURIComponent(taskId)}`), {
      headers: { Authorization: `Bearer ${apiKey}` },
      signal,
    })
    if (!response.ok) throw new Error((await response.text()) || `${i18n.global.t('canvas.taskRequestFailed')} (${response.status})`)
    return response.json()
  },
  async submitText(apiKey: string, prompt: string, model: string, signal?: AbortSignal): Promise<string> {
    const response = await fetch(buildGatewayUrl('/v1/responses'), {
      method: 'POST',
      headers: { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ model, input: prompt, stream: false }),
      signal,
    })
    if (!response.ok) throw new Error(await gatewayError(response, i18n.global.t('canvas.requestFailed')))
    const text = gatewayResponseText(await response.json())
    if (!text) throw new Error(i18n.global.t('canvas.emptyTextResponse'))
    return text
  },
  async submitSpeech(
    apiKey: string,
    input: { prompt: string; voice?: string; language?: string },
    signal?: AbortSignal,
  ): Promise<Blob> {
    const response = await fetch(buildGatewayUrl('/v1/tts'), {
      method: 'POST',
      headers: { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({
        text: input.prompt,
        language: input.language?.trim() || 'en',
        voice_id: input.voice?.trim() || CANVAS_GROK_TTS_VOICES[0],
      }),
      signal,
    })
    if (!response.ok) throw new Error(await gatewayError(response, i18n.global.t('canvas.audioGenerationFailed')))
    const blob = await response.blob()
    if (!blob.size || blob.type.includes('json')) throw new Error(i18n.global.t('canvas.audioGenerationFailed'))
    return blob
  },
}

export default canvasAPI
