import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'

export interface Workspace {
  id: number
  name: string
  slug: string
  type: string
  status: string
  owner_user_id: number
  billing_owner_user_id: number
  created_at?: string
  updated_at?: string
  permissions: string[]
}

export interface Project {
  id: number
  workspace_id: number
  name: string
  slug: string
  description: string
  status: string
  is_default: boolean
  created_by_user_id?: number
  allowed_group_ids?: number[] | null
  allowed_models?: string[] | null
  created_at?: string
  updated_at?: string
}

export interface WorkspaceMember {
  id: number
  workspace_id: number
  user_id: number
  role: string
  status: string
  invited_by_user_id?: number | null
  joined_at?: string
  created_at?: string
  updated_at?: string
  email?: string
  username?: string
}

export interface WorkspaceInvitation {
  id: number
  workspace_id: number
  email: string
  role: string
  invited_by_user_id: number
  expires_at: string
  accepted_at?: string | null
  revoked_at?: string | null
  created_at?: string
}

export interface WorkspaceAudit {
  id: number
  workspace_id: number
  project_id?: number | null
  actor_user_id: number
  action: string
  target_type: string
  target_id: number
  metadata?: Record<string, unknown>
  created_at?: string
}

export interface ProjectKey {
  id: number
  project_id?: number | null
  name: string
  key: string
  status: string
  group_id?: number | null
  ip_whitelist?: string[] | null
  ip_blacklist?: string[] | null
  quota?: number
  quota_used?: number
  usage?: number
  usage_5h?: number
  usage_1d?: number
  usage_7d?: number
  rate_limit_5h?: number
  rate_limit_1d?: number
  rate_limit_7d?: number
  expires_at?: string | null
  created_at?: string
  updated_at?: string
}

export interface WorkspaceAvailableGroup {
  id: number
  name: string
  platform?: string
  status?: string
}

export interface WorkspaceBudget {
  workspace_id?: number
  project_id?: number
  // The FinOps endpoints return the policy under `policy`. Keep the older
  // scope aliases while deployments roll forward so the UI remains tolerant
  // of an already deployed response shape.
  policy?: { amount?: number; hard_limit?: boolean; enabled?: boolean; timezone?: string }
  workspace?: { amount?: number; hard_limit?: boolean; enabled?: boolean; timezone?: string }
  project?: { amount?: number; hard_limit?: boolean; enabled?: boolean; timezone?: string }
  reserved?: number
  spent?: number
  remaining?: number
  over_budget?: boolean
  period_start?: string
  period_end?: string
}

export interface WorkspaceUsageBreakdown {
  id: string
  name: string
  requests: number
  spend: number
}

export interface ServiceAccountUsageBreakdown extends WorkspaceUsageBreakdown {
  tokens: number
  models: string[]
}

export interface WorkspaceDailySpendPoint {
  date: string
  requests: number
  spend: number
}

export interface WorkspaceOverview {
  summary?: {
    workspace_id?: number
    project_id?: number
    requests?: number
    spend?: number
    reserved?: number
    members?: number
    projects?: number
    api_keys?: number
    start?: string
    end?: string
    timezone?: string
  }
  projects?: WorkspaceUsageBreakdown[] | null
  platforms?: WorkspaceUsageBreakdown[] | null
  models?: WorkspaceUsageBreakdown[] | null
  api_keys?: WorkspaceUsageBreakdown[] | null
  service_accounts?: ServiceAccountUsageBreakdown[] | null
  daily_spend?: WorkspaceDailySpendPoint[] | null
  workspace?: Workspace
  members?: number
  requests?: number
  spend?: number
  budget?: WorkspaceBudget
  [key: string]: unknown
}

export interface WorkspaceUsage {
  workspace_id?: number
  project_id?: number
  requests?: number
  spend?: number
  reserved?: number
  members?: number
  projects_count?: number
  api_keys?: number
  start?: string
  end?: string
  timezone?: string
  summary?: WorkspaceUsage
  projects_breakdown?: Array<Record<string, unknown>>
  platforms?: Array<Record<string, unknown>>
  models?: Array<Record<string, unknown>>
  api_keys_breakdown?: Array<Record<string, unknown>>
  items?: Array<Record<string, unknown>>
  trend?: Array<Record<string, unknown>>
  by_project?: Array<Record<string, unknown>>
  by_platform?: Array<Record<string, unknown>>
  by_model?: Array<Record<string, unknown>>
  total?: number
  [key: string]: unknown
}

type PageParams = { page?: number; page_size?: number; signal?: AbortSignal }

const pageConfig = (params: PageParams = {}) => ({
  signal: params.signal,
  params: { page: params.page ?? 1, page_size: params.page_size ?? 50 },
})

export const workspaceAPI = {
  listWorkspaces: async (params?: PageParams) => (await apiClient.get<BasePaginationResponse<Workspace>>('/workspaces', pageConfig(params))).data,
  createWorkspace: async (payload: { name: string; slug: string }) => (await apiClient.post<Workspace>('/workspaces', payload)).data,
  getWorkspace: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<Workspace>(`/workspaces/${workspaceId}`, { signal })).data,
  updateWorkspace: async (workspaceId: number, payload: { name?: string; slug?: string; billing_owner_user_id?: number }) => (await apiClient.patch<Workspace>(`/workspaces/${workspaceId}`, payload)).data,
  archiveWorkspace: async (workspaceId: number) => (await apiClient.delete(`/workspaces/${workspaceId}`)).data,

  listProjects: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<Project>>(`/workspaces/${workspaceId}/projects`, pageConfig(params))).data,
  createProject: async (workspaceId: number, payload: Partial<Project>) => (await apiClient.post<Project>(`/workspaces/${workspaceId}/projects`, payload)).data,
  getProject: async (workspaceId: number, projectId: number, signal?: AbortSignal) => (await apiClient.get<Project>(`/workspaces/${workspaceId}/projects/${projectId}`, { signal })).data,
  updateProject: async (workspaceId: number, projectId: number, payload: Partial<Project>) => (await apiClient.patch<Project>(`/workspaces/${workspaceId}/projects/${projectId}`, payload)).data,
  archiveProject: async (workspaceId: number, projectId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/projects/${projectId}`)).data,

  listMembers: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<WorkspaceMember>>(`/workspaces/${workspaceId}/members`, pageConfig(params))).data,
  updateMember: async (workspaceId: number, memberId: number, payload: { role?: string; status?: string }) => (await apiClient.patch(`/workspaces/${workspaceId}/members/${memberId}`, payload)).data,
  removeMember: async (workspaceId: number, memberId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/members/${memberId}`)).data,
  listInvitations: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<WorkspaceInvitation>>(`/workspaces/${workspaceId}/invitations`, pageConfig(params))).data,
  createInvitation: async (workspaceId: number, payload: { email: string; role: string; expires_in_hours?: number }) => (await apiClient.post<{ invitation: WorkspaceInvitation; token: string }>(`/workspaces/${workspaceId}/invitations`, payload)).data,
  revokeInvitation: async (workspaceId: number, invitationId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/invitations/${invitationId}`)).data,
  acceptInvitation: async (token: string) => (await apiClient.post<Workspace>('/workspace-invitations/accept', { token })).data,
  listAudit: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<WorkspaceAudit>>(`/workspaces/${workspaceId}/audit`, pageConfig(params))).data,

  listKeys: async (workspaceId: number, projectId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<ProjectKey>>(`/workspaces/${workspaceId}/projects/${projectId}/keys`, pageConfig(params))).data,
  createKey: async (workspaceId: number, projectId: number, payload: Record<string, unknown>) => (await apiClient.post<ProjectKey>(`/workspaces/${workspaceId}/projects/${projectId}/keys`, payload)).data,
  getKey: async (workspaceId: number, projectId: number, keyId: number) => (await apiClient.get<ProjectKey>(`/workspaces/${workspaceId}/projects/${projectId}/keys/${keyId}`)).data,
  updateKey: async (workspaceId: number, projectId: number, keyId: number, payload: Record<string, unknown>) => (await apiClient.patch<ProjectKey>(`/workspaces/${workspaceId}/projects/${projectId}/keys/${keyId}`, payload)).data,
  revokeKey: async (workspaceId: number, projectId: number, keyId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/projects/${projectId}/keys/${keyId}`)).data,
  listAvailableGroups: async (workspaceId: number, projectId: number) => (await apiClient.get<WorkspaceAvailableGroup[]>(`/workspaces/${workspaceId}/projects/${projectId}/groups/available`)).data,

  getOverview: async (workspaceId: number, signal?: AbortSignal, params: Record<string, unknown> = {}) => (await apiClient.get<WorkspaceOverview>(`/workspaces/${workspaceId}/overview`, { signal, params })).data,
  getBudget: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceBudget>(`/workspaces/${workspaceId}/budget`, { signal })).data,
  updateBudget: async (workspaceId: number, payload: Record<string, unknown>) => (await apiClient.put<WorkspaceBudget>(`/workspaces/${workspaceId}/budget`, payload)).data,
  getUsage: async (workspaceId: number, params: Record<string, unknown> = {}, signal?: AbortSignal) => (await apiClient.get<WorkspaceUsage>(`/workspaces/${workspaceId}/usage`, { params, signal })).data,
  getProjectOverview: async (workspaceId: number, projectId: number, signal?: AbortSignal, params: Record<string, unknown> = {}) => (await apiClient.get<WorkspaceOverview>(`/workspaces/${workspaceId}/projects/${projectId}/overview`, { signal, params })).data,
  getProjectBudget: async (workspaceId: number, projectId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceBudget>(`/workspaces/${workspaceId}/projects/${projectId}/budget`, { signal })).data,
  updateProjectBudget: async (workspaceId: number, projectId: number, payload: Record<string, unknown>) => (await apiClient.put<WorkspaceBudget>(`/workspaces/${workspaceId}/projects/${projectId}/budget`, payload)).data,
  getProjectUsage: async (workspaceId: number, projectId: number, params: Record<string, unknown> = {}, signal?: AbortSignal) => (await apiClient.get<WorkspaceUsage>(`/workspaces/${workspaceId}/projects/${projectId}/usage`, { params, signal })).data,

  adminList: async (params?: PageParams) => (await apiClient.get<BasePaginationResponse<Workspace>>('/admin/workspaces', pageConfig(params))).data,
  adminInspect: async (workspaceId: number) => (await apiClient.get<Workspace>(`/admin/workspaces/${workspaceId}`)).data,
  adminSetStatus: async (workspaceId: number, status: string) => (await apiClient.patch<Workspace>(`/admin/workspaces/${workspaceId}/status`, { status })).data,
}

export type WorkspacePage<T> = BasePaginationResponse<T>
