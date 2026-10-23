// Fixed HTTP dimensions mirror the bounded instance metric labels. Values are
// validated again by the API; model names, IDs and arbitrary paths are not labels.
export const monitoringProtocols = ['http', 'sse', 'websocket'] as const
export const monitoringRoutes = ['responses', 'messages', 'chat_completions', 'gemini', 'embeddings', 'images', 'audio', 'models', 'other'] as const
export const httpDimensionKeys = ['protocol', 'route', 'status_code'] as const
export function isHTTPMonitoringMetric(metric: string) {
  return ['qps', 'rpm', 'request_count', 'qps_started', 'rpm_started', 'request_count_started'].includes(metric) || metric.startsWith('http_') || metric.startsWith('ttft_')
}
export function httpDimensionsFor(metric: string, source = 'app', externalAdapter = ''): readonly typeof httpDimensionKeys[number][] {
  if (!isHTTPMonitoringMetric(metric)) return []
  if (metric.endsWith('_started')) return ['route']
  const dimensions = metric.includes('_p95_') || metric.includes('_p99_') ? ['protocol', 'route'] as const : httpDimensionKeys
  return source === 'external_ingress' && externalAdapter === 'nginx' ? dimensions.filter(key => key === 'status_code') : dimensions
}
