import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type { AdminWorkspaceFilter, AdminWorkspacePage, AdminOperationInput, AdminSensitiveConfig } from './enterpriseAdmin'

export interface Workspace {
  id: number
  name: string
  slug: string
  type: string
  status: string
  project_access_mode?: 'all_projects' | 'assigned_projects' | string
  owner_user_id: number
  billing_owner_user_id: number
  created_at?: string
  updated_at?: string
  permissions: string[]
}

export type WorkspaceFinopsAnomalyStatus = 'open' | 'acknowledged' | 'resolved'
export type WorkspaceFinopsAnomalySeverity = 'low' | 'medium' | 'high' | 'critical'
export type WorkspaceFinopsAnomalyDetector = 'spend_spike' | 'request_spike' | 'unit_cost_spike'

export interface WorkspaceFinopsAnomaly {
  id: number
  workspace_id: number
  project_id?: number
  scope_type: string
  scope_id?: number
  dimension_type: string
  dimension_value: string
  detector_type: WorkspaceFinopsAnomalyDetector | string
  detector_version: string
  window_start: string
  window_end: string
  observed_spend: number
  expected_spend: number
  spend_delta: number
  observed_requests: number
  expected_requests: number
  observed_unit_cost: number
  expected_unit_cost: number
  baseline_sample_count: number
  baseline_start?: string
  baseline_end?: string
  baseline_mad: number
  relative_increase: number
  score: number
  severity: WorkspaceFinopsAnomalySeverity | string
  fingerprint: string
  snapshot_id: number
  status: WorkspaceFinopsAnomalyStatus
  first_detected_at: string
  last_detected_at: string
  acknowledged_at?: string | null
  acknowledged_by_user_id?: number | null
  resolved_at?: string | null
  resolved_by_user_id?: number | null
  resolution_reason?: string
  version: number
  created_at: string
  updated_at: string
}

export interface WorkspaceFinopsAnomalyFilter {
  status?: WorkspaceFinopsAnomalyStatus
  severity?: WorkspaceFinopsAnomalySeverity
  detector_type?: WorkspaceFinopsAnomalyDetector
  dimension_type?: string
  start?: string
  end?: string
  page?: number
  page_size?: number
}

export interface WorkspaceFinopsAnomalyDetectorStatus {
  last_successful_scan?: string | null
  last_processed_bucket?: string | null
  last_failure_code?: string
  lag_seconds: number
  candidate_count: number
  finding_count: number
  scan_duration_ms: number
}

export interface EnterpriseDomain {
  id: number
  workspace_id: number
  domain: string
  normalized_domain: string
  status: string
  verification_method: string
  dns_host?: string
  verified_at?: string | null
  last_checked_at?: string | null
  last_error_code?: string
  created_at?: string
  updated_at?: string
}

export interface EnterpriseDomainCreateResult {
  domain: EnterpriseDomain
  verification_token: string
  verification_txt: string
}

export type WorkspaceIdentityValidationCode = 'SUCCESS' | 'DISCOVERY_FAILED' | 'ISSUER_MISMATCH' | 'ENDPOINT_INVALID' | 'CONFIGURATION_INVALID' | 'VALIDATION_FAILED' | 'PROVIDER_DISABLED'

export interface WorkspaceSAMLConfig {
  idp_entity_id: string
  sso_url: string
  signing_certificates: string[]
  metadata_xml?: string
  metadata_url?: string
  metadata_source: 'manual' | 'xml' | 'url'
  subject_attribute: string
  allow_unspecified_name_id: boolean
  email_attribute: string
  name_attribute: string
  groups_attribute: string
  authn_requests_signed: true
  sp_certificate?: string
  next_sp_certificate?: string
}

export interface WorkspaceSAMLServiceProvider {
  entity_id: string
  acs_url: string
  metadata_url: string
  signing_certificate: string
  next_signing_certificate?: string
  idp_initiated_supported: false
  slo_supported: false
}

export interface WorkspaceIdentityProvider {
  id: number
  workspace_id: number
  type: 'oidc' | 'saml'
  saml_public_id?: string
  saml?: WorkspaceSAMLConfig
  provider_key: string
  name: string
  status: string
  is_default: boolean
  issuer_url: string
  client_id: string
  has_client_secret: boolean
  token_auth_method?: 'client_secret_basic' | 'client_secret_post' | 'none'
  revision?: number
  scopes: string[]
  authorization_endpoint?: string
  token_endpoint?: string
  jwks_uri?: string
  userinfo_endpoint?: string
  discovery_enabled: boolean
  claim_mapping: Record<string, unknown>
  jit_config: {
    enabled: boolean
    default_role: string
    allowed_domains: string[]
    require_verified_email: boolean
  }
  created_at?: string
  updated_at?: string
  disabled_at?: string | null
  last_validated_at?: string | null
  last_validation_code?: WorkspaceIdentityValidationCode | null
}

export interface WorkspaceIdentityProviderInput {
  type?: 'oidc' | 'saml'
  saml?: WorkspaceSAMLConfig
  provider_key: string
  name: string
  issuer_url?: string
  client_id?: string
  client_secret?: string
  secret_action?: 'preserve' | 'replace' | 'remove'
  revision?: number
  token_auth_method?: 'client_secret_basic' | 'client_secret_post' | 'none'
  authorization_endpoint?: string
  token_endpoint?: string
  jwks_uri?: string
  userinfo_endpoint?: string
  scopes?: string[]
  is_default: boolean
  discovery_enabled: boolean
  claim_mapping?: Record<string, unknown>
  jit_config: {
    enabled: boolean
    default_role: string
    allowed_domains: string[]
    require_verified_email: boolean
  }
}

export type SCIMDefaultRole = 'viewer' | 'developer' | 'admin' | 'billing'

export interface SCIMConnector {
  id: number
  workspace_id: number
  revision: number
  public_endpoint_id: string
  name: string
  status: 'active' | 'disabled'
  default_role: SCIMDefaultRole
  group_mode: 'explicit'
  created_at: string
  updated_at: string
  disabled_at?: string | null
  last_sync_at?: string | null
  last_error_code?: string
  failure_count: number
  base_url: string
}

export interface SCIMToken {
  id: number
  connector_id: number
  token_prefix: string
  status: 'active' | 'revoked'
  created_at: string
  expires_at?: string | null
  revoked_at?: string | null
  last_used_at?: string | null
}

export interface SCIMGroupBinding {
  id: string
  display_name: string
  external_id?: string
  revision: number
  team_id: number | null
}

export interface WorkspaceIdentityMappings {
  roles: Array<{ claim_value: string; role: 'viewer' | 'developer' | 'billing' | 'admin'; priority: number }>
  teams: Array<{ claim_value: string; team_id: number }>
}

export interface WorkspaceSecurityDecision {
  allowed: boolean
  reason: string
  requires_sso: boolean
  requires_mfa: boolean
  requires_mfa_enrollment: boolean
  requires_reauthentication: boolean
  provider_allowed: boolean
  policy_revision: number
}

export interface WorkspaceSecurityPolicy {
  workspace_id: number
  require_sso: boolean
  require_mfa: boolean
  sso_grace_until?: string | null
  session_max_age_seconds: number | null
  invitation_policy: 'any' | 'verified_domains_only' | 'disabled'
  allow_external_members: boolean
  workspace_jit_enabled: boolean
  approved_identity_provider_mode: 'any_active' | 'selected'
  approved_identity_provider_ids: number[]
  revision: number
  updated_by_user_id?: number
  updated_at?: string
  verified_domains?: string[]
  external_member_count?: number
  decision?: WorkspaceSecurityDecision
  session_max_age_min_seconds?: number
  session_max_age_max_seconds?: number
  prerequisites?: string[]
  prerequisite_reason?: string
}

export type WorkspaceIdentityPolicy = WorkspaceSecurityPolicy
export type WorkspaceSecurityPolicyPatch = Partial<Pick<WorkspaceSecurityPolicy, 'require_sso' | 'require_mfa' | 'sso_grace_until' | 'session_max_age_seconds' | 'invitation_policy' | 'allow_external_members' | 'workspace_jit_enabled' | 'approved_identity_provider_mode' | 'approved_identity_provider_ids'>> & { expected_revision: number }

export type ProjectAccessMode = 'all_projects' | 'assigned_projects'
export type ProjectAccessSubjectType = 'member' | 'team'
export type ProjectAccessRole = 'viewer' | 'developer' | 'admin'

export interface WorkspaceTeam {
  id: number
  workspace_id: number
  name: string
  slug: string
  description: string
  status: string
  created_at?: string
  updated_at?: string
}

export interface WorkspaceTeamMember {
  team_id: number
  workspace_member_id: number
  user_id: number
  role: string
  status: string
  email?: string
  created_at?: string
}

export interface ProjectAccessGrant {
  id: number
  workspace_id: number
  project_id: number
  subject_type: ProjectAccessSubjectType
  subject_id: number
  role: ProjectAccessRole
  created_by_user_id: number
  created_at?: string
  updated_at?: string
}

export interface WorkspaceTeamInput {
  name: string
  slug: string
  description?: string
}

export interface ProjectAccessGrantInput {
  subject_type: ProjectAccessSubjectType
  subject_id: number
  role: ProjectAccessRole
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

export type AllocationEnvironment = 'production' | 'staging' | 'development' | 'testing' | 'unallocated' | `custom:${string}`

export interface WorkspaceCostCenter {
  id: number
  workspace_id: number
  code: string
  name: string
  description: string
  status: 'active' | 'archived' | string
}

export interface WorkspaceAllocationTag {
  id: number
  workspace_id: number
  key: string
  value: string
  description: string
  status: 'active' | 'archived' | string
}

export interface WorkspaceAllocationConfig {
  cost_center_id?: number | null
  environment: Exclude<AllocationEnvironment, 'unallocated'>
  tags?: Record<string, string>
  allocation_tags?: Record<string, string>
  policy_revision: number
}

export interface WorkspaceProjectAllocation {
  workspace_id: number
  project_id: number
  allocation: WorkspaceAllocationConfig
  updated_by_user_id?: number
}

export interface WorkspaceAllocationReportGroup {
  key: string
  cost: number
  request_count: number
}

export interface WorkspaceAllocationReport {
  workspace_id: number
  project_id?: number
  workspace_total: number
  allocated: number
  unallocated: number
  overlapping_tags: boolean
  cost_centers?: WorkspaceAllocationReportGroup[]
  environments?: WorkspaceAllocationReportGroup[]
  tags?: WorkspaceAllocationReportGroup[]
}

export interface WorkspaceAllocationFilter {
  start?: string
  end?: string
  timezone?: string
  cost_center_id?: number
  environment?: string
  tag_key?: string
  tag_value?: string
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
  updateProjectAccessMode: async (workspaceId: number, project_access_mode: ProjectAccessMode) => (await apiClient.patch<Workspace>(`/workspaces/${workspaceId}/project-access-mode`, { project_access_mode })).data,

  listDomains: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<EnterpriseDomain>>(`/workspaces/${workspaceId}/domains`, pageConfig(params))).data,
  createDomain: async (workspaceId: number, domain: string) => (await apiClient.post<EnterpriseDomainCreateResult>(`/workspaces/${workspaceId}/domains`, { domain })).data,
  verifyDomain: async (workspaceId: number, domainId: number) => (await apiClient.post<EnterpriseDomain>(`/workspaces/${workspaceId}/domains/${domainId}/verify`)).data,
  regenerateDomainToken: async (workspaceId: number, domainId: number) => (await apiClient.post<EnterpriseDomainCreateResult>(`/workspaces/${workspaceId}/domains/${domainId}/regenerate`)).data,
  revokeDomain: async (workspaceId: number, domainId: number) => (await apiClient.delete<EnterpriseDomain>(`/workspaces/${workspaceId}/domains/${domainId}`)).data,
  listSCIMConnectors: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<SCIMConnector[]>(`/workspaces/${workspaceId}/scim-connectors`, { signal })).data,
  createSCIMConnector: async (workspaceId: number, payload: { name: string; default_role: SCIMDefaultRole }) => (await apiClient.post<SCIMConnector>(`/workspaces/${workspaceId}/scim-connectors`, payload)).data,
  updateSCIMConnector: async (workspaceId: number, connectorId: number, payload: { name: string; default_role: SCIMDefaultRole; revision: number }) => (await apiClient.patch<SCIMConnector>(`/workspaces/${workspaceId}/scim-connectors/${connectorId}`, payload)).data,
  disableSCIMConnector: async (workspaceId: number, connectorId: number, revision: number) => (await apiClient.post(`/workspaces/${workspaceId}/scim-connectors/${connectorId}/disable`, { revision })).data,
  listSCIMTokens: async (workspaceId: number, connectorId: number, signal?: AbortSignal) => (await apiClient.get<SCIMToken[]>(`/workspaces/${workspaceId}/scim-connectors/${connectorId}/tokens`, { signal })).data,
  createSCIMToken: async (workspaceId: number, connectorId: number, payload: { expires_at?: string }) => (await apiClient.post<{ token: SCIMToken; secret: string }>(`/workspaces/${workspaceId}/scim-connectors/${connectorId}/tokens`, payload)).data,
  revokeSCIMToken: async (workspaceId: number, connectorId: number, tokenId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/scim-connectors/${connectorId}/tokens/${tokenId}`)).data,
  listSCIMGroups: async (workspaceId: number, connectorId: number, signal?: AbortSignal) => (await apiClient.get<SCIMGroupBinding[]>(`/workspaces/${workspaceId}/scim-connectors/${connectorId}/groups`, { signal })).data,
  bindSCIMGroup: async (workspaceId: number, connectorId: number, groupId: string, payload: { revision: number; team_id: number | null }) => (await apiClient.put(`/workspaces/${workspaceId}/scim-connectors/${connectorId}/groups/${encodeURIComponent(groupId)}/team`, payload)).data,
  listIdentityProviders: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<WorkspaceIdentityProvider>>(`/workspaces/${workspaceId}/identity-providers`, pageConfig(params))).data,
  getIdentityProvider: async (workspaceId: number, providerId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceIdentityProvider>(`/workspaces/${workspaceId}/identity-providers/${providerId}`, { signal })).data,
  createIdentityProvider: async (workspaceId: number, payload: WorkspaceIdentityProviderInput) => (await apiClient.post<WorkspaceIdentityProvider>(`/workspaces/${workspaceId}/identity-providers`, payload)).data,
  updateIdentityProvider: async (workspaceId: number, providerId: number, payload: WorkspaceIdentityProviderInput) => (await apiClient.patch<WorkspaceIdentityProvider>(`/workspaces/${workspaceId}/identity-providers/${providerId}`, payload)).data,
  disableIdentityProvider: async (workspaceId: number, providerId: number) => (await apiClient.post(`/workspaces/${workspaceId}/identity-providers/${providerId}/disable`)).data,
  getSAMLServiceProvider: async (workspaceId: number, providerId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceSAMLServiceProvider>(`/workspaces/${workspaceId}/identity-providers/${providerId}/saml-sp`, { signal })).data,
  rotateSAMLKeys: async (workspaceId: number, providerId: number, payload: { revision: number; action: 'stage' | 'promote' }) => (await apiClient.post<WorkspaceIdentityProvider>(`/workspaces/${workspaceId}/identity-providers/${providerId}/saml-keys/rotate`, payload)).data,
  getIdentityProviderMappings: async (workspaceId: number, providerId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceIdentityMappings>(`/workspaces/${workspaceId}/identity-providers/${providerId}/mappings`, { signal })).data,
  updateIdentityProviderMappings: async (workspaceId: number, providerId: number, payload: WorkspaceIdentityMappings) => (await apiClient.put(`/workspaces/${workspaceId}/identity-providers/${providerId}/mappings`, payload)).data,
  getIdentityPolicy: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceIdentityPolicy>(`/workspaces/${workspaceId}/security-policy`, { signal })).data,
  getSecurityPolicy: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceSecurityPolicy>(`/workspaces/${workspaceId}/security-policy`, { signal })).data,
  updateIdentityPolicy: async (workspaceId: number, payload: WorkspaceSecurityPolicyPatch) => (await apiClient.patch<WorkspaceIdentityPolicy>(`/workspaces/${workspaceId}/security-policy`, payload)).data,
  updateSecurityPolicy: async (workspaceId: number, payload: WorkspaceSecurityPolicyPatch) => (await apiClient.patch<WorkspaceSecurityPolicy>(`/workspaces/${workspaceId}/security-policy`, payload)).data,
  previewSecurityPolicy: async (workspaceId: number, payload: WorkspaceSecurityPolicyPatch, signal?: AbortSignal) => (await apiClient.post<WorkspaceSecurityPolicy>(`/workspaces/${workspaceId}/security-policy/preview`, payload, { signal })).data,
  startSSO: async (workspaceId: number, providerId: number, returnTo = `/workspaces/${workspaceId}/overview`) => (await apiClient.post<{ authorization_url: string; expires_at: string }>('/auth/sso/start', { workspace_id: workspaceId, provider_id: providerId, return_to: returnTo })).data,

  listTeams: async (workspaceId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<WorkspaceTeam>>(`/workspaces/${workspaceId}/teams`, pageConfig(params))).data,
  createTeam: async (workspaceId: number, payload: WorkspaceTeamInput) => (await apiClient.post<WorkspaceTeam>(`/workspaces/${workspaceId}/teams`, payload)).data,
  updateTeam: async (workspaceId: number, teamId: number, payload: WorkspaceTeamInput) => (await apiClient.patch<WorkspaceTeam>(`/workspaces/${workspaceId}/teams/${teamId}`, payload)).data,
  archiveTeam: async (workspaceId: number, teamId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/teams/${teamId}`)).data,
  listTeamMembers: async (workspaceId: number, teamId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<WorkspaceTeamMember>>(`/workspaces/${workspaceId}/teams/${teamId}/members`, pageConfig(params))).data,
  addTeamMember: async (workspaceId: number, teamId: number, workspaceMemberId: number) => (await apiClient.post(`/workspaces/${workspaceId}/teams/${teamId}/members`, { workspace_member_id: workspaceMemberId })).data,
  removeTeamMember: async (workspaceId: number, teamId: number, workspaceMemberId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/teams/${teamId}/members/${workspaceMemberId}`)).data,

  listProjectAccessGrants: async (workspaceId: number, projectId: number, params?: PageParams) => (await apiClient.get<BasePaginationResponse<ProjectAccessGrant>>(`/workspaces/${workspaceId}/projects/${projectId}/access-grants`, pageConfig(params))).data,
  createProjectAccessGrant: async (workspaceId: number, projectId: number, payload: ProjectAccessGrantInput) => (await apiClient.post<ProjectAccessGrant>(`/workspaces/${workspaceId}/projects/${projectId}/access-grants`, payload)).data,
  updateProjectAccessGrant: async (workspaceId: number, projectId: number, grantId: number, payload: ProjectAccessGrantInput) => (await apiClient.patch<ProjectAccessGrant>(`/workspaces/${workspaceId}/projects/${projectId}/access-grants/${grantId}`, payload)).data,
  deleteProjectAccessGrant: async (workspaceId: number, projectId: number, grantId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/projects/${projectId}/access-grants/${grantId}`)).data,

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

  listCostCenters: async (workspaceId: number, archived = false, signal?: AbortSignal) => (await apiClient.get<WorkspaceCostCenter[]>(`/workspaces/${workspaceId}/cost-centers`, { params: { include_archived: archived }, signal })).data,
  createCostCenter: async (workspaceId: number, payload: Pick<WorkspaceCostCenter, 'code' | 'name' | 'description'>) => (await apiClient.post<WorkspaceCostCenter>(`/workspaces/${workspaceId}/cost-centers`, payload)).data,
  updateCostCenter: async (workspaceId: number, centerId: number, payload: Pick<WorkspaceCostCenter, 'code' | 'name' | 'description'>) => (await apiClient.patch<WorkspaceCostCenter>(`/workspaces/${workspaceId}/cost-centers/${centerId}`, payload)).data,
  archiveCostCenter: async (workspaceId: number, centerId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/cost-centers/${centerId}`)).data,
  listAllocationTags: async (workspaceId: number, archived = false, signal?: AbortSignal) => (await apiClient.get<WorkspaceAllocationTag[]>(`/workspaces/${workspaceId}/allocation-tags`, { params: { include_archived: archived }, signal })).data,
  createAllocationTag: async (workspaceId: number, payload: Pick<WorkspaceAllocationTag, 'key' | 'value' | 'description'>) => (await apiClient.post<WorkspaceAllocationTag>(`/workspaces/${workspaceId}/allocation-tags`, payload)).data,
  updateAllocationTag: async (workspaceId: number, tagId: number, payload: Pick<WorkspaceAllocationTag, 'key' | 'value' | 'description'>) => (await apiClient.patch<WorkspaceAllocationTag>(`/workspaces/${workspaceId}/allocation-tags/${tagId}`, payload)).data,
  archiveAllocationTag: async (workspaceId: number, tagId: number) => (await apiClient.delete(`/workspaces/${workspaceId}/allocation-tags/${tagId}`)).data,
  getAllocationReport: async (workspaceId: number, params: WorkspaceAllocationFilter = {}, signal?: AbortSignal) => (await apiClient.get<WorkspaceAllocationReport>(`/workspaces/${workspaceId}/finops/allocation`, { params, signal })).data,
  getProjectAllocationReport: async (workspaceId: number, projectId: number, params: WorkspaceAllocationFilter = {}, signal?: AbortSignal) => (await apiClient.get<WorkspaceAllocationReport>(`/workspaces/${workspaceId}/projects/${projectId}/finops/allocation`, { params, signal })).data,
  getProjectAllocation: async (workspaceId: number, projectId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceProjectAllocation>(`/workspaces/${workspaceId}/projects/${projectId}/allocation`, { signal })).data,
  updateProjectAllocation: async (workspaceId: number, projectId: number, payload: WorkspaceAllocationConfig) => (await apiClient.put<WorkspaceProjectAllocation>(`/workspaces/${workspaceId}/projects/${projectId}/allocation`, payload)).data,

  listFinopsAnomalies: async (workspaceId: number, params: WorkspaceFinopsAnomalyFilter = {}, signal?: AbortSignal) => (await apiClient.get<BasePaginationResponse<WorkspaceFinopsAnomaly>>(`/workspaces/${workspaceId}/finops/anomalies`, { params, signal })).data,
  getFinopsAnomaly: async (workspaceId: number, anomalyId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceFinopsAnomaly>(`/workspaces/${workspaceId}/finops/anomalies/${anomalyId}`, { signal })).data,
  updateFinopsAnomaly: async (workspaceId: number, anomalyId: number, payload: { status: 'acknowledged' | 'resolved'; resolution_reason?: string; expected_version: number }) => (await apiClient.patch<WorkspaceFinopsAnomaly>(`/workspaces/${workspaceId}/finops/anomalies/${anomalyId}`, payload)).data,
  getFinopsAnomalyStatus: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceFinopsAnomalyDetectorStatus>(`/workspaces/${workspaceId}/finops/anomalies/status`, { signal })).data,
  listProjectFinopsAnomalies: async (workspaceId: number, projectId: number, params: WorkspaceFinopsAnomalyFilter = {}, signal?: AbortSignal) => (await apiClient.get<BasePaginationResponse<WorkspaceFinopsAnomaly>>(`/workspaces/${workspaceId}/projects/${projectId}/finops/anomalies`, { params, signal })).data,
  getProjectFinopsAnomaly: async (workspaceId: number, projectId: number, anomalyId: number, signal?: AbortSignal) => (await apiClient.get<WorkspaceFinopsAnomaly>(`/workspaces/${workspaceId}/projects/${projectId}/finops/anomalies/${anomalyId}`, { signal })).data,
  updateProjectFinopsAnomaly: async (workspaceId: number, projectId: number, anomalyId: number, payload: { status: 'acknowledged' | 'resolved'; resolution_reason?: string; expected_version: number }) => (await apiClient.patch<WorkspaceFinopsAnomaly>(`/workspaces/${workspaceId}/projects/${projectId}/finops/anomalies/${anomalyId}`, payload)).data,

  adminList: async ({ signal, ...params }: AdminWorkspaceFilter = {}) => (await apiClient.get<AdminWorkspacePage>('/admin/workspaces', { signal, params: { page: 1, page_size: 20, sort: 'id', direction: 'desc', ...params } })).data,
  adminInspect: async (workspaceId: number, signal?: AbortSignal) => (await apiClient.get<Workspace>(`/admin/workspaces/${workspaceId}`, { signal })).data,
  adminSetStatus: async (workspaceId: number, body: AdminOperationInput | string, config?: AdminSensitiveConfig) => {
    const input = typeof body === 'string' ? { status: body } as AdminOperationInput : body
    return (await apiClient.patch<Workspace>(`/admin/workspaces/${workspaceId}/status`, input, config)).data
  },
}

export type WorkspacePage<T> = BasePaginationResponse<T>
