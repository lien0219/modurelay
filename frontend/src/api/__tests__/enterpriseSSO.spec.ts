import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { enterpriseSSOAPI } from '../enterpriseSSO'

vi.mock('../client', () => ({ apiClient: { post: vi.fn() } }))

describe('enterprise SSO API', () => {
  beforeEach(() => { vi.resetAllMocks(); vi.mocked(apiClient.post).mockResolvedValue({ data: {} }) })

  it('performs discovery without requesting or exposing user account state', async () => {
    const signal = new AbortController().signal
    await enterpriseSSOAPI.discover('person@example.com', signal)
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/discover', { email: 'person@example.com' }, { signal })
  })

  it('uses the explicit authenticated linking endpoint with password and TOTP proof', async () => {
    await enterpriseSSOAPI.startLink(7, 9, '/workspaces/7/identity', { password: 'proof', totp_code: '123456' })
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/link/start', { workspace_id: 7, provider_id: 9, return_to: '/workspaces/7/identity', password: 'proof', totp_code: '123456' })
  })

  it('exchanges browser-bound HttpOnly completion cookies without URL or request-body tokens', async () => {
    await enterpriseSSOAPI.exchange()
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/exchange')
  })

  it('returns a local two-factor challenge without treating it as a token completion', async () => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { requires_2fa: true } })
    await expect(enterpriseSSOAPI.exchange()).resolves.toEqual({ requires_2fa: true })
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/exchange')
  })

  it('submits only the authenticator code to finish the browser-bound exchange', async () => {
    await enterpriseSSOAPI.exchange('123456')
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/exchange', { totp_code: '123456' })
  })

  it('requires explicit confirmation and proof for owner recovery', async () => {
    await enterpriseSSOAPI.recover(7, { password: 'proof', totp_code: '123456', reason: 'Provider configuration outage', confirmed: true })
    expect(apiClient.post).toHaveBeenCalledWith('/auth/sso/recover', { workspace_id: 7, password: 'proof', totp_code: '123456', reason: 'Provider configuration outage', confirmed: true })
  })
})
