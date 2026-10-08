import { safeEnterpriseReturnTo } from './enterpriseSSO'

const reasons = new Set(['SSO_REQUIRED', 'MFA_REQUIRED', 'MFA_ENROLLMENT_REQUIRED', 'WORKSPACE_REAUTH_REQUIRED', 'IDENTITY_PROVIDER_NOT_APPROVED', 'INVITATIONS_DISABLED', 'EXTERNAL_MEMBER_NOT_ALLOWED', 'MEMBER_DOMAIN_NOT_ALLOWED', 'WORKSPACE_SECURITY_POLICY_CONFLICT', 'SECURITY_POLICY_UNAVAILABLE', 'STEP_UP_REQUIRED', 'STEP_UP_TOTP_NOT_ENABLED', 'RECENT_AUTH_REQUIRED', 'SESSION_MFA_UPGRADE_INVALID', 'SESSION_MFA_UPGRADE_REUSED', 'AUTH_SESSION_CHANGED', 'TOTP_INVALID_CODE', 'TOTP_TOO_MANY_ATTEMPTS'])

let navigationGeneration = 0
let navigationPath = window.location.pathname + window.location.search
function changedNavigation() { ++navigationGeneration; navigationPath = window.location.pathname + window.location.search }
window.addEventListener('popstate', changedNavigation)
window.addEventListener('workspace-context-changed', changedNavigation)
export function workspaceNavigationGeneration(): number {
  const path = window.location.pathname + window.location.search
  if (path !== navigationPath) changedNavigation()
  return navigationGeneration
}

export function workspaceSecurityReason(error: unknown): string {
  const candidate = error as { code?: unknown; reason?: unknown } | null
  return [candidate?.reason, candidate?.code].find(value => typeof value === 'string' && reasons.has(value)) as string || ''
}

export function workspaceSecurityMessageKey(error: unknown): string {
  const reason = workspaceSecurityReason(error)
  return reason ? `workspace.securityReasons.${reason}` : 'workspace.securityActionError'
}

export function isWorkspaceAssuranceRequired(error: unknown): boolean {
  return ['SSO_REQUIRED', 'MFA_REQUIRED', 'MFA_ENROLLMENT_REQUIRED', 'WORKSPACE_REAUTH_REQUIRED', 'IDENTITY_PROVIDER_NOT_APPROVED', 'STEP_UP_REQUIRED', 'STEP_UP_TOTP_NOT_ENABLED', 'RECENT_AUTH_REQUIRED'].includes(workspaceSecurityReason(error))
}

export function workspaceRecoveryPath(workspaceId: number, reason: string, returnTo: unknown, requiresSSO = false): string {
  const path = safeEnterpriseReturnTo(returnTo, `/workspaces/${workspaceId}/overview`)
  if (requiresSSO || reason === 'SSO_REQUIRED' || reason === 'IDENTITY_PROVIDER_NOT_APPROVED') {
    const query = new URLSearchParams({ workspace_id: String(workspaceId), required: '1', return_to: path })
    return `/auth/sso?${query.toString()}`
  }
  return `/login?${new URLSearchParams({ reauth: '1', redirect: path })}`
}

// Only scopes UI work to the current browser session. It never supplies MFA evidence.
export function browserSessionIdentity(): string {
  const token = localStorage.getItem('auth_token') || ''
  let session = token
  try {
    const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/'))) as { sid?: unknown }
    if (typeof payload.sid === 'string' && payload.sid) session = payload.sid
  } catch { /* Opaque/legacy tokens still invalidate work when they change. */ }
  let userID: unknown = ''
  try { userID = JSON.parse(localStorage.getItem('auth_user') || '{}').id || '' } catch { /* Invalid user state scopes no trusted action. */ }
  return `${userID}:${session}`
}

export function deniedInvitationWorkspace(error: unknown): number | null {
  const id = (error as { metadata?: { workspace_id?: unknown } } | null)?.metadata?.workspace_id
  const candidate = typeof id === 'string' && /^[1-9][0-9]*$/.test(id) ? Number(id) : id
  return typeof candidate === 'number' && Number.isSafeInteger(candidate) && candidate > 0 ? candidate : null
}
