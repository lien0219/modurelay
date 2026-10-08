import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { totpAPI } from '../totp'

vi.mock('../client', () => ({ apiClient: { post: vi.fn() } }))

describe('TOTP session upgrade', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('auth_user', JSON.stringify({ id: 7 }))
    localStorage.setItem('auth_token', 'current-access')
    localStorage.setItem('refresh_token', 'current-refresh')
    vi.resetAllMocks()
    Object.defineProperty(navigator, 'locks', { configurable: true, value: undefined })
  })

  it('uses the exact coordinated refresh snapshot and the token TTL instead of grant TTL', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { verified: true, expires_in: 900, access_token: 'mfa-access', refresh_token: 'mfa-refresh', token_type: 'Bearer', token_expires_in: 1800 } })
    const result = await totpAPI.upgradeSessionMFA('123456')
    expect(apiClient.post).toHaveBeenCalledWith('/user/totp/step-up', { code: '123456', refresh_token: 'current-refresh' }, { preserveAuthSessionOnFailure: true, sessionProofAccessToken: 'current-access' })
    expect(result.expires_in).toBe(1800)
    expect(localStorage.getItem('refresh_token')).toBe('mfa-refresh')
    expect(Number(localStorage.getItem('token_expires_at'))).toBeGreaterThan(Date.now() + 1700000)
  })

  it('does not turn a legacy grant-only response into session MFA', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { verified: true, expires_in: 900 } })
    await expect(totpAPI.upgradeSessionMFA('123456')).rejects.toThrow('Invalid session upgrade response')
    expect(localStorage.getItem('auth_token')).toBe('current-access')
  })
})
