export function safeEnterpriseReturnTo(value: unknown, fallback = '/workspaces'): string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.length > 1024) return fallback
  let decoded = value
  for (let depth = 0; depth < 4; depth++) {
    if (decoded.startsWith('//') || decoded.includes('\\') || [...decoded].some(char => char.charCodeAt(0) < 32 || char.charCodeAt(0) === 127)) return fallback
    let next: string
    try { next = decodeURIComponent(decoded) } catch { return fallback }
    if (next === decoded) break
    if (depth === 3) return fallback
    decoded = next
  }
  if (/(?:^|\/)\.\.?($|\/)/.test(decoded.split(/[?#]/)[0])) return fallback
  const origin = 'https://modurelay.invalid'
  const target = new URL(decoded, origin)
  if (target.origin !== origin || !/^\/(?:workspaces|dashboard|profile)(?:\/|$)/.test(target.pathname)) return fallback
  return value
}

export function ssoRequiredRedirect(requestUrl: string, code: unknown, reason: unknown, returnTo: string): string | null {
  if (code !== 'SSO_REQUIRED' && reason !== 'SSO_REQUIRED') return null
  const match = requestUrl.match(/^\/?workspaces\/(\d+)(?:\/|$)/)
  if (!match || !Number.isSafeInteger(Number(match[1])) || Number(match[1]) <= 0) return null
  const routeWorkspace = returnTo.match(/^\/workspaces\/(\d+)(?:\/|$)/)
  if (!routeWorkspace || routeWorkspace[1] !== match[1]) return null
  const params = new URLSearchParams({ workspace_id: match[1], required: '1', return_to: safeEnterpriseReturnTo(returnTo, `/workspaces/${match[1]}/overview`) })
  return `/auth/sso?${params.toString()}`
}

const navigationHintKey = 'modurelay.enterprise-sso-navigation'
interface EnterpriseSSONavigationHint { workspace_id: number; provider_id: number; return_to: string; expires_at: number }

// This session-local hint restores only navigation after a provider failure. It grants no access.
export function rememberEnterpriseSSONavigation(workspaceId: number, providerId: number, returnTo: string): void {
  if (!Number.isSafeInteger(workspaceId) || workspaceId <= 0 || !Number.isSafeInteger(providerId) || providerId <= 0) return
  try { sessionStorage.setItem(navigationHintKey, JSON.stringify({ workspace_id: workspaceId, provider_id: providerId, return_to: safeEnterpriseReturnTo(returnTo, `/workspaces/${workspaceId}/overview`), expires_at: Date.now() + 15 * 60_000 })) }
  catch { /* Sign-in does not depend on browser navigation storage. */ }
}

export function getEnterpriseSSONavigation(): EnterpriseSSONavigationHint | null {
  try {
    const hint = JSON.parse(sessionStorage.getItem(navigationHintKey) || 'null') as EnterpriseSSONavigationHint | null
    if (!hint || !Number.isSafeInteger(hint.workspace_id) || hint.workspace_id <= 0 || !Number.isSafeInteger(hint.provider_id) || hint.provider_id <= 0 || !Number.isFinite(hint.expires_at) || hint.expires_at <= Date.now() || hint.expires_at > Date.now() + 15 * 60_000) return null
    return { workspace_id: hint.workspace_id, provider_id: hint.provider_id, return_to: safeEnterpriseReturnTo(hint.return_to, `/workspaces/${hint.workspace_id}/overview`), expires_at: hint.expires_at }
  } catch { return null }
}

export function clearEnterpriseSSONavigation(): void {
  try { sessionStorage.removeItem(navigationHintKey) } catch { /* Storage may be disabled. */ }
}
