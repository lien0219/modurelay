import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export interface ServiceAccount {
  id: number
  workspace_id: number
  project_id: number
  name: string
  slug: string
  description: string
  status: 'active' | 'disabled'
  created_by_user_id: number | null
  disabled_at?: string | null
  created_at: string
  updated_at: string
}
export interface ServiceAccountCredential {
  id: number
  service_account_id: number
  project_id: number
  name: string
  key_suffix: string
  status: string
  group_id?: number | null
  ip_whitelist?: string[] | null
  ip_blacklist?: string[] | null
  quota: number
  quota_used: number
  expires_at?: string | null
  rate_limit_5h?: number
  rate_limit_1d?: number
  rate_limit_7d?: number
  usage_5h?: number
  usage_1d?: number
  usage_7d?: number
  last_used_at?: string | null
  created_at?: string
  updated_at?: string
}
export interface CredentialInput {
  name: string
  group_id?: number | null
  quota?: number
  expires_in_days?: number
  expires_at?: string | null
  ip_whitelist?: string[]
  ip_blacklist?: string[]
  rate_limit_5h?: number
  rate_limit_1d?: number
  rate_limit_7d?: number
}
export interface CredentialSecret { credential: ServiceAccountCredential; secret: string; previous_credential_id?: number }
type PageParams = { page?: number; page_size?: number; signal?: AbortSignal }
const pageConfig = (p: PageParams = {}) => ({ params: { page: p.page ?? 1, page_size: p.page_size ?? 50 }, signal: p.signal })
const path = (w: number, p: number, s?: number) => `/workspaces/${w}/projects/${p}/service-accounts${s === undefined ? '' : `/${s}`}`
const credentialsPath = (w: number, p: number, s: number, c?: number) => `${path(w, p, s)}/credentials${c === undefined ? '' : `/${c}`}`
const idempotency = (key: string) => ({ headers: { 'Idempotency-Key': key } })

export const serviceAccountsAPI = {
  list: async (w: number, p: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<ServiceAccount>>(path(w, p), pageConfig(params))).data,
  get: async (w: number, p: number, s: number, signal?: AbortSignal) => (await apiClient.get<ServiceAccount>(path(w, p, s), { signal })).data,
  create: async (w: number, p: number, input: { name: string; slug: string; description: string }) => (await apiClient.post<ServiceAccount>(path(w, p), input)).data,
  update: async (w: number, p: number, s: number, input: { name?: string; description?: string }) => (await apiClient.patch<ServiceAccount>(path(w, p, s), input)).data,
  setStatus: async (w: number, p: number, s: number, status: 'active' | 'disabled') => (await apiClient.post<ServiceAccount>(`${path(w, p, s)}/${status === 'active' ? 'enable' : 'disable'}`)).data,
  credentials: async (w: number, p: number, s: number, signal?: AbortSignal) => (await apiClient.get<ServiceAccountCredential[]>(credentialsPath(w, p, s), { signal })).data,
  createCredential: async (w: number, p: number, s: number, input: CredentialInput, key: string) => (await apiClient.post<CredentialSecret>(credentialsPath(w, p, s), input, idempotency(key))).data,
  updateCredential: async (w: number, p: number, s: number, c: number, input: CredentialInput) => (await apiClient.patch<ServiceAccountCredential>(credentialsPath(w, p, s, c), input)).data,
  revoke: async (w: number, p: number, s: number, c: number) => (await apiClient.post<ServiceAccountCredential>(`${credentialsPath(w, p, s, c)}/revoke`)).data,
  rotate: async (w: number, p: number, s: number, c: number, key: string) => (await apiClient.post<CredentialSecret>(`${credentialsPath(w, p, s, c)}/rotate`, undefined, idempotency(key))).data,
  adminList: async (params: PageParams & { search?: string } = {}) => (await apiClient.get<BasePaginationResponse<ServiceAccount>>('/admin/service-accounts', { ...pageConfig(params), params: { ...pageConfig(params).params, search: params.search } })).data,
  adminInspect: async (s: number, signal?: AbortSignal) => (await apiClient.get<{ service_account: ServiceAccount; credentials: ServiceAccountCredential[] }>(`/admin/service-accounts/${s}`, { signal })).data,
  adminDisable: async (s: number) => (await apiClient.post(`/admin/service-accounts/${s}/disable`)).data,
  adminRevoke: async (s: number, c: number) => (await apiClient.post(`/admin/service-accounts/${s}/credentials/${c}/revoke`)).data,
}
