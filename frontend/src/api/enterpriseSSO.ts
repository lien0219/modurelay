import { apiClient } from './client'
import type { User } from '@/types'

export interface EnterpriseSSOProvider {
  workspace_id: number
  provider_id: number
  type?: 'oidc' | 'saml'
  saml_public_id?: string
  name: string
  is_default: boolean
}

export interface EnterpriseSSODiscovery {
  enterprise_sso_available: boolean
  providers: EnterpriseSSOProvider[]
}

export interface EnterpriseSSOCompletion {
  requires_2fa?: false
  access_token: string
  refresh_token?: string
  expires_in?: number
  user: User
  workspace_id: number
  provider_id: number
  return_to?: string
}

export interface EnterpriseSSOTotpChallenge {
  requires_2fa: true
}

export type EnterpriseSSOExchangeResult = EnterpriseSSOCompletion | EnterpriseSSOTotpChallenge

export const enterpriseSSOAPI = {
  discover: async (email: string, signal?: AbortSignal) => (await apiClient.post<EnterpriseSSODiscovery>('/auth/sso/discover', { email }, { signal })).data,
  startLink: async (workspaceId: number, providerId: number, returnTo: string, proof?: { password?: string; totp_code?: string }) => (await apiClient.post<{ authorization_url: string; expires_at: string }>('/auth/sso/link/start', { workspace_id: workspaceId, provider_id: providerId, return_to: returnTo, ...proof })).data,
  exchange: async (totpCode?: string): Promise<EnterpriseSSOExchangeResult> => {
    const response = totpCode === undefined
      ? await apiClient.post<EnterpriseSSOExchangeResult>('/auth/sso/exchange')
      : await apiClient.post<EnterpriseSSOExchangeResult>('/auth/sso/exchange', { totp_code: totpCode })
    return response.data
  },
  recover: async (workspaceId: number, payload: { password: string; totp_code: string; reason: string; confirmed: true }) => (await apiClient.post('/auth/sso/recover', { workspace_id: workspaceId, ...payload })).data,
}
