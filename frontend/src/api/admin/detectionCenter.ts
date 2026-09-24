import { apiClient } from '../client'

export type DetectionProtocol = 'auto' | 'openai' | 'anthropic' | 'gemini'
export type DetectionMode = 'standard' | 'deep'
export type DetectionStatus = 'success' | 'failed' | 'partial' | 'inconclusive' | 'not_applicable' | 'unavailable'

export interface DetectionAttempt {
  protocol: string
  status: DetectionStatus | string
  http_status?: number
  latency_ms?: number
  message: string
}

export interface DetectedModel {
  id: string
  name: string
  provider?: string
  protocols: string[]
  capabilities?: Record<string, unknown>
}

export interface DetectionDiscoveryResponse {
  suggested_protocol: DetectionProtocol
  protocols: string[]
  models: DetectedModel[]
  attempts: DetectionAttempt[]
}

export interface DetectionEvidence {
  kind: string
  label: string
  expected?: string
  actual?: string
  http_status?: number
  duration_ms?: number
  response_excerpt?: string
}

export interface DetectionProbeResult {
  id: string
  name: string
  category: string
  status: DetectionStatus
  confidence: number
  summary: string
  failure_reason?: string
  reason_code?: string
  possible_causes?: string[]
  recommendations?: string[]
  evidence?: DetectionEvidence[]
}

export interface DetectionSummary {
  total: number
  success: number
  failed: number
  partial: number
  inconclusive: number
  not_applicable: number
  unavailable: number
}

export interface DetectionReport {
  report_id: string
  started_at: string
  completed_at: string
  base_url: string
  protocol: string
  model: string
  mode: DetectionMode
  summary: DetectionSummary
  probes: DetectionProbeResult[]
  notes?: string[]
}

export interface DetectionConnectionPayload {
  base_url: string
  api_key: string
  protocol: DetectionProtocol
}

export interface DetectionRunPayload extends DetectionConnectionPayload {
  model: string
  mode: DetectionMode
}

export async function discover(payload: DetectionConnectionPayload): Promise<DetectionDiscoveryResponse> {
  const { data } = await apiClient.post<DetectionDiscoveryResponse>('/admin/detection-center/discover', payload, { timeout: 60000 })
  return data
}

export async function run(payload: DetectionRunPayload): Promise<DetectionReport> {
  const { data } = await apiClient.post<DetectionReport>('/admin/detection-center/run', payload, { timeout: 600000 })
  return data
}

export default { discover, run }
