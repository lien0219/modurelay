import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { policyAPI } from '@/api/policies'
import { useWorkspaceStore } from '@/stores/workspace'
import WorkspacePolicyView from '../WorkspacePolicyView.vue'

vi.mock('@/api/policies', () => ({ policyAPI: {
  getWorkspace: vi.fn(), updateWorkspace: vi.fn(),
  getProject: vi.fn(), updateProject: vi.fn(),
  getServiceAccount: vi.fn(), updateServiceAccount: vi.fn(), getEffective: vi.fn(),
} }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string, values?: Record<string, unknown>) => values ? `${key}:${String(values.revision)}` : key }),
}))

const projectPolicy = {
  scope: 'project' as const, scope_id: 9, revision: 4,
  allowed_models: null, allowed_platforms: null,
  rpm_limit: null, daily_request_limit: null, monthly_request_limit: null,
  daily_token_limit: null, monthly_token_limit: null,
}
const effectivePolicy = {
  layers: { workspace: { ...projectPolicy, scope: 'workspace' as const, scope_id: 7, revision: 2, allowed_models: ['gpt-6'] }, project: projectPolicy },
  revisions: { workspace: 2, project: 4 },
  rpm_limit: 20, daily_request_limit: 100, monthly_request_limit: null, daily_token_limit: null, monthly_token_limit: null,
}
let wrapper: VueWrapper | undefined

async function render(path = '/workspaces/7/projects/9/policy') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = useWorkspaceStore()
  store.$patch({ selectedWorkspaceId: 7, permissions: ['policy.read', 'workspace_policy.update', 'project_policy.update', 'service_account_policy.update'] })
  const router = createRouter({ history: createMemoryHistory(), routes: [
    { path: '/workspaces/:workspaceId/policy', component: WorkspacePolicyView },
    { path: '/workspaces/:workspaceId/projects/:projectId', component: { template: '<main />' } },
    { path: '/workspaces/:workspaceId/projects/:projectId/policy', component: WorkspacePolicyView },
    { path: '/workspaces/:workspaceId/projects/:projectId/service-accounts', component: { template: '<main />' } },
    { path: '/workspaces/:workspaceId/projects/:projectId/service-accounts/:serviceAccountId', component: { template: '<main />' } },
    { path: '/workspaces/:workspaceId/projects/:projectId/service-accounts/:serviceAccountId/policy', component: WorkspacePolicyView },
  ] })
  await router.push(path)
  await router.isReady()
  wrapper = mount(WorkspacePolicyView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { router, wrapper }
}

describe('WorkspacePolicyView', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(policyAPI.getWorkspace).mockResolvedValue({ ...projectPolicy, scope: 'workspace', scope_id: 7, revision: 0 })
    vi.mocked(policyAPI.getProject).mockResolvedValue(projectPolicy)
    vi.mocked(policyAPI.getServiceAccount).mockResolvedValue({ ...projectPolicy, scope: 'service_account', scope_id: 11 })
    vi.mocked(policyAPI.updateProject).mockResolvedValue({ ...projectPolicy, revision: 5, allowed_models: ['gpt-6'] })
    vi.mocked(policyAPI.updateWorkspace).mockResolvedValue({ ...projectPolicy, scope: 'workspace', scope_id: 7, revision: 1 })
    vi.mocked(policyAPI.updateServiceAccount).mockResolvedValue({ ...projectPolicy, scope: 'service_account', scope_id: 11, revision: 5 })
    vi.mocked(policyAPI.getEffective).mockResolvedValue(effectivePolicy)
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('loads project and effective policy, showing source constraints and effective numeric minima', async () => {
    const { wrapper } = await render()
    expect(policyAPI.getProject).toHaveBeenCalledWith(7, 9, expect.any(AbortSignal))
    expect(policyAPI.getEffective).toHaveBeenCalledWith(7, 9, undefined, expect.any(AbortSignal))
    expect(wrapper.get('[data-testid="effective-policy-readonly"]').text()).toContain('gpt-6')
    expect(wrapper.get('[data-testid="effective-policy-readonly"]').text()).toContain('20')
  })

  it('creates the first project policy using revision zero and preserves inherited nulls', async () => {
    vi.mocked(policyAPI.getProject).mockResolvedValue({ ...projectPolicy, revision: 0 })
    const { wrapper } = await render()
    await wrapper.get('[name="allowed-models-mode"]').setValue('restricted')
    await wrapper.get('textarea').setValue('gpt-6')
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(policyAPI.updateProject).toHaveBeenCalledWith(7, 9, expect.objectContaining({
      expected_revision: 0,
      allowed_models: ['gpt-6'],
      allowed_platforms: null,
      rpm_limit: null,
    }))
  })

  it('uses the service account scope route and policy endpoint for machine identity settings', async () => {
    await render('/workspaces/7/projects/9/service-accounts/11/policy')
    expect(policyAPI.getServiceAccount).toHaveBeenCalledWith(7, 9, 11, expect.any(AbortSignal))
    expect(policyAPI.getEffective).toHaveBeenCalledWith(7, 9, 11, expect.any(AbortSignal))
  })
})
