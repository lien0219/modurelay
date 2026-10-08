import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import WorkspaceSecurityView from '../WorkspaceSecurityView.vue'

const api = vi.hoisted(() => ({ listWorkspaces: vi.fn(), listProjects: vi.fn(), getSecurityPolicy: vi.fn(), listIdentityProviders: vi.fn(), previewSecurityPolicy: vi.fn(), updateSecurityPolicy: vi.fn() }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/common/ConfirmDialog.vue', () => ({ default: { props: ['show', 'confirming'], template: '<div v-if="show" data-testid="policy-confirm"><slot /><button :disabled="confirming" @click="$emit(\'confirm\')">Confirm</button><button @click="$emit(\'cancel\')">Cancel</button></div>' } }))
vi.mock('@/components/workspace/WorkspaceAccessRecovery.vue', () => ({ default: { template: '<button data-testid="recover-policy-access" @click="$emit(\'verified\')">Verify</button>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

const permissions = ['workspace.read', 'workspace_security.read', 'workspace_security.update', 'identity.read']
const workspace = { id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions }
const decision = { allowed: true, reason: 'allowed', requires_sso: false, requires_mfa: false, requires_mfa_enrollment: false, requires_reauthentication: false, provider_allowed: true, policy_revision: 2 }
const policy = { workspace_id: 1, require_sso: false, require_mfa: false, sso_grace_until: null, session_max_age_seconds: null, invitation_policy: 'any', allow_external_members: true, workspace_jit_enabled: true, approved_identity_provider_mode: 'any_active', approved_identity_provider_ids: [], revision: 2, verified_domains: ['example.com'], external_member_count: 3, decision }
let wrapper: VueWrapper | undefined

async function render(items = [workspace]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  api.listWorkspaces.mockResolvedValue({ items })
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/security', component: WorkspaceSecurityView }, { path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/workspaces/1/security')
  wrapper = mount(WorkspaceSecurityView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { view: wrapper, router }
}

describe('WorkspaceSecurityView', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.listProjects.mockResolvedValue({ items: [] })
    api.getSecurityPolicy.mockImplementation(async (id: number) => ({ ...policy, workspace_id: id }))
    api.listIdentityProviders.mockResolvedValue({ items: [{ id: 9, workspace_id: 1, name: 'Primary', status: 'active' }], pages: 1 })
    api.previewSecurityPolicy.mockResolvedValue({ ...policy, require_mfa: true })
    api.updateSecurityPolicy.mockResolvedValue({ ...policy, require_mfa: true, revision: 3 })
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('renders every control and preserves null age until explicitly supplied', async () => {
    const { view } = await render()
    for (const name of ['require_sso', 'require_mfa', 'sso_grace_until', 'session_max_age_seconds', 'invitation_policy', 'allow_external_members', 'workspace_jit_enabled', 'approved_identity_provider_mode']) expect(view.find(`[name="${name}"]`).exists()).toBe(true)
    await view.find('[name="require_mfa"]').setValue(true)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    expect(api.previewSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ expected_revision: 2, require_mfa: true, session_max_age_seconds: null, sso_grace_until: null }), expect.any(AbortSignal))
    expect(api.updateSecurityPolicy).not.toHaveBeenCalled()
    await view.find('[data-testid="identity-save-policy"]').trigger('click')
    await view.find('[data-testid="policy-confirm"] button').trigger('click')
    await flushPromises()
    expect(api.updateSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ expected_revision: 2, require_mfa: true, session_max_age_seconds: null }))
  })

  it.each(['2026-10-09T12:34:56.123456Z', '2026-10-09T20:34:56.123+08:00'])('preserves the exact unchanged grace deadline %s when another field is saved', async deadline => {
    api.getSecurityPolicy.mockResolvedValue({ ...policy, sso_grace_until: deadline })
    const { view } = await render()
    await view.find('[name="require_mfa"]').setValue(true)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    expect(api.previewSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ sso_grace_until: deadline }), expect.any(AbortSignal))
    await view.find('[data-testid="identity-save-policy"]').trigger('click')
    await view.find('[data-testid="policy-confirm"] button').trigger('click')
    await flushPromises()
    expect(api.updateSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ sso_grace_until: deadline }))
  })

  it.each(['', '2026-10-10T14:25'])('applies an explicitly changed grace input %s', async input => {
    api.getSecurityPolicy.mockResolvedValue({ ...policy, sso_grace_until: '2026-10-09T12:34:56.123456Z' })
    const { view } = await render()
    await view.find('[name="sso_grace_until"]').setValue(input)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    const expected = input ? new Date(input).toISOString() : null
    expect(api.previewSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ sso_grace_until: expected }), expect.any(AbortSignal))
    await view.find('[data-testid="identity-save-policy"]').trigger('click')
    await view.find('[data-testid="policy-confirm"] button').trigger('click')
    await flushPromises()
    expect(api.updateSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ sso_grace_until: expected }))
  })

  it.each(['0', '899', '2592001', '900.5'])('rejects invalid session age %s without previewing', async value => {
    const { view } = await render()
    await view.find('[name="session_max_age_seconds"]').setValue(value)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    expect(api.previewSecurityPolicy).not.toHaveBeenCalled()
    expect(view.find('[data-testid="security-validation"]').exists()).toBe(true)
  })

  it('keeps policy readable without update permission and never loads identity management data', async () => {
    const { view } = await render([{ ...workspace, permissions: ['workspace.read', 'workspace_security.read'] }])
    expect(view.find('[name="require_mfa"]').element.matches(':disabled')).toBe(true)
    expect(view.find('[data-testid="identity-save-policy"]').exists()).toBe(false)
    expect(api.listIdentityProviders).not.toHaveBeenCalled()
  })

  it('offers session recovery when policy reading is denied and reloads after proof', async () => {
    api.getSecurityPolicy.mockRejectedValueOnce({ status: 403, code: 'MFA_REQUIRED' })
    const { view } = await render([{ ...workspace, permissions: ['workspace.read', 'workspace_security.read'] }])
    expect(view.find('[data-testid="recover-policy-access"]').exists()).toBe(true)
    expect(view.find('[data-testid="security-form"]').exists()).toBe(false)
    await view.find('[data-testid="recover-policy-access"]').trigger('click')
    await flushPromises()
    expect(view.find('[name="require_mfa"]').exists()).toBe(true)
    expect(api.getSecurityPolicy).toHaveBeenCalledTimes(2)
  })

  it('does not label read-only approved provider IDs as inactive without provider details', async () => {
    api.getSecurityPolicy.mockResolvedValue({ ...policy, approved_identity_provider_mode: 'selected', approved_identity_provider_ids: [9] })
    const { view } = await render([{ ...workspace, permissions: ['workspace.read', 'workspace_security.read'] }])
    const provider = view.find('[data-testid="security-provider-9"]')
    expect(provider.text()).toContain('workspace.securityProviderDetailsUnavailable')
    expect(provider.text()).not.toContain('workspace.securityProviderUnavailable')
  })

  it('does not load organization policy for a personal workspace', async () => {
    const { view } = await render([{ ...workspace, type: 'personal' }])
    expect(api.getSecurityPolicy).not.toHaveBeenCalled()
    expect(view.text()).toContain('workspace.securityPersonalUnsupported')
  })

  it('retains approved provider IDs beyond the first page and warns about unavailable selected providers', async () => {
    api.getSecurityPolicy.mockResolvedValue({ ...policy, approved_identity_provider_mode: 'selected', approved_identity_provider_ids: [9, 60, 80] })
    api.listIdentityProviders.mockImplementation(async (_id: number, params: { page: number }) => params.page === 1 ? { items: [{ id: 9, name: 'Primary', status: 'active' }], pages: 2 } : { items: [{ id: 60, name: 'Second page', status: 'active' }], pages: 2 })
    const { view } = await render()
    expect(view.find('[data-testid="security-provider-60"]').exists()).toBe(true)
    expect(view.find('[data-testid="security-provider-80"]').exists()).toBe(true)
    await view.find('[name="require_mfa"]').setValue(true)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    expect(api.previewSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ approved_identity_provider_ids: [9, 60, 80] }), expect.any(AbortSignal))
  })

  it('lets the Owner explicitly remove a previously selected unavailable provider', async () => {
    api.getSecurityPolicy.mockResolvedValue({ ...policy, approved_identity_provider_mode: 'selected', approved_identity_provider_ids: [9, 80] })
    const { view } = await render()
    const unavailable = view.find('[data-testid="security-provider-80"] input')
    expect(unavailable.attributes()).not.toHaveProperty('disabled')
    await unavailable.setValue(false)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    expect(api.previewSecurityPolicy).toHaveBeenCalledWith(1, expect.objectContaining({ approved_identity_provider_ids: [9] }), expect.any(AbortSignal))
  })

  it('invalidates a pending confirmation when the candidate changes', async () => {
    const { view } = await render()
    await view.find('[name="require_mfa"]').setValue(true)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    await view.find('[data-testid="identity-save-policy"]').trigger('click')
    expect(view.find('[data-testid="policy-confirm"]').exists()).toBe(true)
    await view.find('[name="invitation_policy"]').setValue('disabled')
    expect(view.find('[data-testid="policy-confirm"]').exists()).toBe(false)
    expect(view.find('[data-testid="identity-save-policy"]').attributes()).toHaveProperty('disabled')
  })

  it('requires explicit refresh after a revision conflict instead of retrying', async () => {
    api.updateSecurityPolicy.mockRejectedValue({ status: 409, code: 'WORKSPACE_SECURITY_POLICY_CONFLICT' })
    const { view } = await render()
    await view.find('[name="require_mfa"]').setValue(true)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await flushPromises()
    await view.find('[data-testid="identity-save-policy"]').trigger('click')
    await view.find('[data-testid="policy-confirm"] button').trigger('click')
    await flushPromises()
    expect(view.find('[data-testid="security-conflict"]').exists()).toBe(true)
    expect(api.updateSecurityPolicy).toHaveBeenCalledTimes(1)
    expect(api.getSecurityPolicy).toHaveBeenCalledTimes(1)
  })

  it('ignores an old preview after switching away and back', async () => {
    let finish!: (value: typeof policy) => void
    api.previewSecurityPolicy.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const { view } = await render([workspace, { ...workspace, id: 2 }])
    await view.find('[name="require_mfa"]').setValue(true)
    await view.find('[data-testid="security-preview"]').trigger('click')
    await useWorkspaceStore().selectWorkspace(2)
    await useWorkspaceStore().selectWorkspace(1)
    await flushPromises()
    finish({ ...policy, decision: { ...decision, allowed: false, reason: 'stale-preview' } })
    await flushPromises()
    expect(view.find('[data-testid="security-preview-result"]').exists()).toBe(false)
  })

  it('will not submit for a route that has not adopted the selected workspace', async () => {
    const { view } = await render([workspace, { ...workspace, id: 2 }])
    await useWorkspaceStore().selectWorkspace(2)
    await flushPromises()
    expect(view.find('[data-testid="security-form"]').exists()).toBe(false)
    expect(api.updateSecurityPolicy).not.toHaveBeenCalled()
  })
})
