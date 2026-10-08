import { describe, expect, it } from 'vitest'
import { deniedInvitationWorkspace, workspaceSecurityReason, workspaceSecurityMessageKey, workspaceRecoveryPath } from '../workspaceSecurity'

describe('workspace security recovery decisions', () => {
  it('maps code and reason without displaying raw denial details', () => {
    expect(workspaceSecurityReason({ code: 403, reason: 'MFA_REQUIRED' })).toBe('MFA_REQUIRED')
    expect(workspaceSecurityMessageKey({ code: 'INVITATIONS_DISABLED', message: 'raw internal SQL detail' })).toBe('workspace.securityReasons.INVITATIONS_DISABLED')
    expect(workspaceSecurityMessageKey({ code: 'unknown', message: 'secret' })).toBe('workspace.securityActionError')
  })

  it('uses the validated target tenant for SSO recovery without adding invitation tokens', () => {
    expect(workspaceRecoveryPath(7, 'SSO_REQUIRED', '/workspaces/2/invitations')).toBe('/auth/sso?workspace_id=7&required=1&return_to=%2Fworkspaces%2F2%2Finvitations')
    expect(workspaceRecoveryPath(7, 'WORKSPACE_REAUTH_REQUIRED', 'https://evil.example')).toBe('/login?reauth=1&redirect=%2Fworkspaces%2F7%2Foverview')
  })

  it('accepts the backend decimal tenant metadata and rejects malformed target identifiers', () => {
    expect(deniedInvitationWorkspace({ metadata: { workspace_id: '7' } })).toBe(7)
    expect(deniedInvitationWorkspace({ metadata: { workspace_id: 7 } })).toBe(7)
    for (const workspace_id of ['0', '-1', '1e3', ' 7', '7 ', '9007199254740992', '1.5', '', {}, null]) {
      expect(deniedInvitationWorkspace({ metadata: { workspace_id } })).toBeNull()
    }
  })
})
