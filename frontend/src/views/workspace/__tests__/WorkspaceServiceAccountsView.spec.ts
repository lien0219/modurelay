import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import WorkspaceServiceAccountsView from '../WorkspaceServiceAccountsView.vue'
import { serviceAccountsAPI } from '@/api/serviceAccounts'
import { workspaceAPI } from '@/api/workspace'
import { useWorkspaceStore } from '@/stores/workspace'
import { createTestI18n } from '@/__tests__/utils/i18n'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'

vi.mock('@/api/serviceAccounts', () => ({ serviceAccountsAPI: { list: vi.fn(), get: vi.fn(), credentials: vi.fn(), createCredential: vi.fn(), rotate: vi.fn(), revoke: vi.fn(), create: vi.fn(), update: vi.fn(), setStatus: vi.fn() } }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: { getProject: vi.fn(), listAvailableGroups: vi.fn(), getProjectOverview: vi.fn() } }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: { props: ['show', 'title'], template: '<div v-if="show" role="dialog"><slot /><slot name="footer" /></div>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
enableAutoUnmount(afterEach)
const account = { id: 11, workspace_id: 7, project_id: 9, name: 'Deploy', slug: 'deploy', description: '', status: 'active' as const, created_by_user_id: null, created_at: '', updated_at: '' }
const credential = { id: 13, service_account_id: 11, project_id: 9, name: 'CI', key_suffix: 'abcdef', status: 'active', quota: 0, quota_used: 0 }
const permissions = ['service_account.read', 'service_account.create', 'service_account.update', 'service_account.disable', 'service_account.credential.read', 'service_account.credential.create', 'service_account.credential.update', 'service_account.credential.rotate', 'service_account.credential.revoke', 'usage.read']
async function render(allowed = permissions, locale = 'en') {
  const store = useWorkspaceStore()
  store.$patch({ selectedWorkspaceId: 7, permissions: allowed, workspaces: [{ id: 7, name: 'Team', slug: 'team', type: 'organization', status: 'active', owner_user_id: 1, billing_owner_user_id: 1, permissions: allowed }] })
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/projects/:projectId/service-accounts/:serviceAccountId?', component: WorkspaceServiceAccountsView }, { path: '/workspaces/:workspaceId/projects/:projectId', component: { template: '<main />' } }] })
  await router.push('/workspaces/7/projects/9/service-accounts/11')
  const wrapper = mount(WorkspaceServiceAccountsView, { global: { plugins: [router, createTestI18n({ en, zh }, locale)] } })
  await flushPromises()
  return { wrapper, router, store }
}
describe('WorkspaceServiceAccountsView', () => {
  beforeEach(() => {
    setActivePinia(createPinia()); vi.resetAllMocks()
    vi.mocked(serviceAccountsAPI.get).mockResolvedValue(account)
    vi.mocked(serviceAccountsAPI.credentials).mockResolvedValue([credential])
    vi.mocked(serviceAccountsAPI.rotate).mockResolvedValue({ credential: { ...credential, id: 14 }, secret: 'sk-once', previous_credential_id: 13 })
    vi.mocked(workspaceAPI.getProject).mockResolvedValue({ id: 9, workspace_id: 7, name: 'App', slug: 'app', description: '', status: 'active', is_default: false })
    vi.mocked(workspaceAPI.listAvailableGroups).mockResolvedValue([])
    vi.mocked(workspaceAPI.getProjectOverview).mockResolvedValue({ summary: { requests: 5, spend: 2 } })
  })
  it('shows a masked credential and destroys the rotated secret on close', async () => {
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('abcdef')
    await wrapper.get('[data-testid="sa-rotate-13"]').trigger('click')
    await wrapper.get('[data-testid="sa-confirm"]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('sk-once')
    expect(wrapper.text()).toContain('old credential')
    await wrapper.get('[data-testid="sa-secret-close"]').trigger('click')
    expect(wrapper.text()).not.toContain('sk-once')
    expect(serviceAccountsAPI.revoke).not.toHaveBeenCalled()
  })
  it('keeps credentials and management controls hidden from usage-only readers', async () => {
    const { wrapper } = await render(['service_account.read', 'usage.read'])
    expect(serviceAccountsAPI.credentials).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="sa-edit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="sa-create-credential"]').exists()).toBe(false)
    expect(workspaceAPI.getProjectOverview).toHaveBeenCalledWith(7, 9, expect.anything(), { service_account_id: 11 })
  })
  it('blocks management in archived projects and renders translated Chinese labels', async () => {
    vi.mocked(workspaceAPI.getProject).mockResolvedValue({ id: 9, workspace_id: 7, name: 'App', slug: 'app', description: '', status: 'archived', is_default: false })
    const { wrapper } = await render(permissions, 'zh')
    expect(wrapper.text()).toContain('服务账号')
    expect(wrapper.find('[data-testid="sa-rotate-13"]').exists()).toBe(false)
  })
  it('discards a late secret response after route navigation', async () => {
    let resolve!: (value: { credential: typeof credential; secret: string }) => void
    vi.mocked(serviceAccountsAPI.rotate).mockReturnValue(new Promise(r => { resolve = r }))
    const { wrapper, router } = await render()
    await wrapper.get('[data-testid="sa-rotate-13"]').trigger('click'); await wrapper.get('[data-testid="sa-confirm"]').trigger('click')
    await router.push('/workspaces/7/projects/9/service-accounts/12'); await flushPromises()
    resolve({ credential, secret: 'sk-late' }); await flushPromises()
    expect(wrapper.text()).not.toContain('sk-late')
  })
})
