import { apiClient } from './client'

export type PolicyScope = 'workspace' | 'project' | 'service_account'

export interface Policy {
  scope: PolicyScope
  scope_id: number
  revision: number
  allowed_models: string[] | null
  allowed_platforms: string[] | null
  rpm_limit: number | null
  daily_request_limit: number | null
  monthly_request_limit: number | null
  daily_token_limit: number | null
  monthly_token_limit: number | null
  enabled?: boolean
}

export interface EffectivePolicy {
  layers: Partial<Record<PolicyScope | 'group' | 'credential', Policy | null>>
  revisions: Partial<Record<PolicyScope | 'group' | 'credential', number>>
  rpm_limit: number | null
  daily_request_limit: number | null
  monthly_request_limit: number | null
  daily_token_limit: number | null
  monthly_token_limit: number | null
}

export type PolicyUpdate = Pick<Policy, 'allowed_models' | 'allowed_platforms' | 'rpm_limit' | 'daily_request_limit' | 'monthly_request_limit' | 'daily_token_limit' | 'monthly_token_limit'> & {
  expected_revision: number
}

const policyUpdate = (path: string, input: PolicyUpdate) => apiClient.patch<Policy>(path, input).then(response => response.data)
const policyGet = (path: string, signal?: AbortSignal) => apiClient.get<Policy>(path, { signal }).then(response => response.data)

export const policyAPI = {
  getWorkspace: (workspaceId: number, signal?: AbortSignal) => policyGet(`/workspaces/${workspaceId}/policy`, signal),
  updateWorkspace: (workspaceId: number, input: PolicyUpdate) => policyUpdate(`/workspaces/${workspaceId}/policy`, input),
  getProject: (workspaceId: number, projectId: number, signal?: AbortSignal) => policyGet(`/workspaces/${workspaceId}/projects/${projectId}/policy`, signal),
  updateProject: (workspaceId: number, projectId: number, input: PolicyUpdate) => policyUpdate(`/workspaces/${workspaceId}/projects/${projectId}/policy`, input),
  getServiceAccount: (workspaceId: number, projectId: number, serviceAccountId: number, signal?: AbortSignal) => policyGet(`/workspaces/${workspaceId}/projects/${projectId}/service-accounts/${serviceAccountId}/policy`, signal),
  updateServiceAccount: (workspaceId: number, projectId: number, serviceAccountId: number, input: PolicyUpdate) => policyUpdate(`/workspaces/${workspaceId}/projects/${projectId}/service-accounts/${serviceAccountId}/policy`, input),
  getEffective: (workspaceId: number, projectId: number, serviceAccountId?: number, signal?: AbortSignal) => {
    const path = serviceAccountId
      ? `/workspaces/${workspaceId}/projects/${projectId}/service-accounts/${serviceAccountId}/effective-policy`
      : `/workspaces/${workspaceId}/projects/${projectId}/effective-policy`
    return apiClient.get<EffectivePolicy>(path, { signal }).then(response => response.data)
  },
}
