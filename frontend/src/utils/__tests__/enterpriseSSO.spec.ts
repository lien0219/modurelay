import { describe, expect, it, vi } from 'vitest'
import { clearEnterpriseSSONavigation, getEnterpriseSSONavigation, rememberEnterpriseSSONavigation, safeEnterpriseReturnTo, ssoRequiredRedirect } from '../enterpriseSSO'

describe('enterprise SSO navigation', () => {
  it.each(['/workspaces/7/identity', '/workspaces?sort=name', '/dashboard', '/profile?tab=security'])('allows a relative control-plane route: %s', value => {
    expect(safeEnterpriseReturnTo(value)).toBe(value)
  })

  it.each([undefined, ['https://evil.example'], 'https://evil.example', '//evil.example', '/\\evil.example', '/workspaces/../login', '/workspaces/%2e%2e/%2f%2fevil.example', '/workspaces/%255c%255cevil.example', '/workspaces/7\u0000', '/workspaces/7%0aevil', '/admin', '/login', '/workspaces/%'])('rejects an unsafe or unsupported destination: %s', value => {
    expect(safeEnterpriseReturnTo(value, '/workspaces/7/overview')).toBe('/workspaces/7/overview')
  })

  it('redirects only SSO_REQUIRED failures belonging to a workspace API request', () => {
    const path = ssoRequiredRedirect('/workspaces/7/teams', 403, 'SSO_REQUIRED', '/workspaces/7/teams')
    expect(path).toBe('/auth/sso?workspace_id=7&required=1&return_to=%2Fworkspaces%2F7%2Fteams')
    expect(ssoRequiredRedirect('/workspaces/7/teams', 'FORBIDDEN', '', '/dashboard')).toBeNull()
    expect(ssoRequiredRedirect('/auth/me', 'SSO_REQUIRED', '', '/dashboard')).toBeNull()
    expect(ssoRequiredRedirect('/workspaces/0/teams', 'SSO_REQUIRED', '', '/dashboard')).toBeNull()
  })

  it('does not redirect a late denial from another workspace into the current route', () => {
    expect(ssoRequiredRedirect('/workspaces/7/teams', 'SSO_REQUIRED', '', '/workspaces/8/teams')).toBeNull()
    expect(ssoRequiredRedirect('/workspaces/7/teams', 'SSO_REQUIRED', '', '/profile')).toBeNull()
  })

  it('keeps only an expiring safe navigation hint and clears it after completion', () => {
    sessionStorage.clear()
    const now = Date.now()
    const clock = vi.spyOn(Date, 'now').mockReturnValue(now)
    rememberEnterpriseSSONavigation(7, 9, '//evil.example')
    expect(getEnterpriseSSONavigation()).toMatchObject({ workspace_id: 7, provider_id: 9, return_to: '/workspaces/7/overview' })
    clock.mockReturnValue(now + 16 * 60_000)
    expect(getEnterpriseSSONavigation()).toBeNull()
    clock.mockRestore()
    clearEnterpriseSSONavigation()
    expect(getEnterpriseSSONavigation()).toBeNull()
  })
})
