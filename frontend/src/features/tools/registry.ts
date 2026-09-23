/**
 * The canonical, UI-agnostic catalogue for the developer toolbox.
 *
 * Keeping this data separate from views means the home page, navigation and
 * route guards can consume the same source of truth without importing any
 * tool implementation (and therefore without pulling tool dependencies into
 * the initial bundle).
 */

export type ToolCategory =
  | 'security'
  | 'encoding'
  | 'developer'
  | 'api'
  | 'network'
  | 'text'

export type ToolId =
  | 'totp'
  | 'password'
  | 'uuid'
  | 'json'
  | 'base64'
  | 'url'
  | 'timestamp'
  | 'jwt'
  | 'hash'
  | 'hmac'
  | 'random-string'
  | 'regex'
  | 'http-status'
  | 'user-agent'
  | 'ip-cidr'
  | 'api-builder'
  | 'curl-converter'
  | 'json-to-struct'
  | 'cron'
  | 'diff'
  | 'markdown'
  | 'qrcode'

export interface ToolDefinition {
  readonly id: ToolId
  readonly route: `/tools/${ToolId}` | '/tools'
  readonly titleKey: string
  readonly descriptionKey: string
  readonly icon: 'shield' | 'key' | 'grid' | 'document' | 'lock' | 'type' | 'clock' | 'link' | 'server' | 'globe' | 'terminal' | 'beaker' | 'cube' | 'calendar' | 'refresh' | 'clipboard' | 'swap' | 'badge' | 'chart' | 'arrowRight'
  readonly category: ToolCategory
  /** The catalogue default. Runtime feature flags may further restrict it. */
  readonly enabled: true
  /** Lazy loader keeps the implementation out of the toolbox catalogue chunk. */
  readonly lazyComponent: () => Promise<{ default: Component }>
}

const lazyToolWorkspace = () => import('./ToolWorkspace.vue')

export const TOOL_REGISTRY: readonly ToolDefinition[] = [
  { id: 'totp', route: '/tools/totp', titleKey: 'tools.totp.title', descriptionKey: 'tools.totp.description', icon: 'shield', category: 'security', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'password', route: '/tools/password', titleKey: 'tools.password.title', descriptionKey: 'tools.password.description', icon: 'key', category: 'security', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'uuid', route: '/tools/uuid', titleKey: 'tools.uuid.title', descriptionKey: 'tools.uuid.description', icon: 'cube', category: 'developer', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'json', route: '/tools/json', titleKey: 'tools.json.title', descriptionKey: 'tools.json.description', icon: 'document', category: 'encoding', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'base64', route: '/tools/base64', titleKey: 'tools.base64.title', descriptionKey: 'tools.base64.description', icon: 'swap', category: 'encoding', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'url', route: '/tools/url', titleKey: 'tools.url.title', descriptionKey: 'tools.url.description', icon: 'link', category: 'encoding', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'timestamp', route: '/tools/timestamp', titleKey: 'tools.timestamp.title', descriptionKey: 'tools.timestamp.description', icon: 'clock', category: 'developer', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'jwt', route: '/tools/jwt', titleKey: 'tools.jwt.title', descriptionKey: 'tools.jwt.description', icon: 'badge', category: 'security', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'hash', route: '/tools/hash', titleKey: 'tools.hash.title', descriptionKey: 'tools.hash.description', icon: 'beaker', category: 'security', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'hmac', route: '/tools/hmac', titleKey: 'tools.hmac.title', descriptionKey: 'tools.hmac.description', icon: 'lock', category: 'security', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'random-string', route: '/tools/random-string', titleKey: 'tools.randomString.title', descriptionKey: 'tools.randomString.description', icon: 'refresh', category: 'security', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'regex', route: '/tools/regex', titleKey: 'tools.regex.title', descriptionKey: 'tools.regex.description', icon: 'type', category: 'developer', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'http-status', route: '/tools/http-status', titleKey: 'tools.httpStatus.title', descriptionKey: 'tools.httpStatus.description', icon: 'server', category: 'network', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'user-agent', route: '/tools/user-agent', titleKey: 'tools.userAgent.title', descriptionKey: 'tools.userAgent.description', icon: 'globe', category: 'api', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'ip-cidr', route: '/tools/ip-cidr', titleKey: 'tools.ipCidr.title', descriptionKey: 'tools.ipCidr.description', icon: 'globe', category: 'network', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'api-builder', route: '/tools/api-builder', titleKey: 'tools.apiBuilder.title', descriptionKey: 'tools.apiBuilder.description', icon: 'arrowRight', category: 'api', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'curl-converter', route: '/tools/curl-converter', titleKey: 'tools.curlConverter.title', descriptionKey: 'tools.curlConverter.description', icon: 'terminal', category: 'api', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'json-to-struct', route: '/tools/json-to-struct', titleKey: 'tools.jsonToStruct.title', descriptionKey: 'tools.jsonToStruct.description', icon: 'document', category: 'developer', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'cron', route: '/tools/cron', titleKey: 'tools.cron.title', descriptionKey: 'tools.cron.description', icon: 'calendar', category: 'developer', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'diff', route: '/tools/diff', titleKey: 'tools.diff.title', descriptionKey: 'tools.diff.description', icon: 'swap', category: 'text', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'markdown', route: '/tools/markdown', titleKey: 'tools.markdown.title', descriptionKey: 'tools.markdown.description', icon: 'clipboard', category: 'text', enabled: true, lazyComponent: lazyToolWorkspace },
  { id: 'qrcode', route: '/tools/qrcode', titleKey: 'tools.qrcode.title', descriptionKey: 'tools.qrcode.description', icon: 'grid', category: 'encoding', enabled: true, lazyComponent: lazyToolWorkspace },
] as const

export function getToolDefinition(id: string): ToolDefinition | undefined {
  return TOOL_REGISTRY.find((tool) => tool.id === id)
}

export function getToolsByCategory(category: ToolCategory): readonly ToolDefinition[] {
  return TOOL_REGISTRY.filter((tool) => tool.category === category)
}

export function searchTools(query: string): readonly ToolDefinition[] {
  const normalized = query.trim().toLocaleLowerCase()
  if (!normalized) return TOOL_REGISTRY
  return TOOL_REGISTRY.filter((tool) =>
    `${tool.id} ${tool.titleKey} ${tool.descriptionKey}`.toLocaleLowerCase().includes(normalized),
  )
}
import type { Component } from 'vue'
