import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import type { Component } from 'vue'
import { useWorkspaceStore } from '@/stores/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import WorkspaceMembersView from '../WorkspaceMembersView.vue'
import WorkspaceInvitationsView from '../WorkspaceInvitationsView.vue'
import WorkspaceAuditView from '../WorkspaceAuditView.vue'
import WorkspaceOverviewView from '../WorkspaceOverviewView.vue'
import WorkspaceProjectsView from '../WorkspaceProjectsView.vue'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(), listProjects: vi.fn(), getWorkspace: vi.fn(), getOverview: vi.fn(), getBudget: vi.fn(), listMembers: vi.fn(), listInvitations: vi.fn(), listAudit: vi.fn(),
  acceptInvitation: vi.fn(), createInvitation: vi.fn(), revokeInvitation: vi.fn(), updateMember: vi.fn(), removeMember: vi.fn(), updateWorkspace: vi.fn(), createWorkspace: vi.fn(),
  createProject: vi.fn(), updateProject: vi.fn(), archiveProject: vi.fn(),
}))
vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key.startsWith('workspace.') ? workspaceMessages.workspace[key.slice(10) as keyof typeof workspaceMessages.workspace] ?? key : key }),
}))

const permissions = ['workspace.read', 'workspace.update', 'project.read', 'project.create', 'project.update', 'project.archive', 'member.read', 'member.update', 'member.remove', 'member.invite', 'invitation.read', 'usage.read', 'budget.read', 'audit.read']
const workspace = (id: number) => ({ id, name: `Workspace ${id}`, slug: `workspace-${id}`, type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions })
const member = (name = 'New member', role = 'developer') => ({ id: 2, workspace_id: 1, user_id: 8, role, status: 'active', username: name })
let wrapper: VueWrapper | undefined

async function render(component: Component, section: string) {
  const pinia = createPinia()
  setActivePinia(pinia)
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/:section', component }, { path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push(`/workspaces/1/${section}`)
  wrapper = mount(component, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return { wrapper, router }
}

describe('workspace views keep their tenant context', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.listWorkspaces.mockResolvedValue({ items: [workspace(1), workspace(2)] })
    api.listProjects.mockResolvedValue({ items: [] })
    api.getWorkspace.mockImplementation(async (id: number) => workspace(id))
    api.listMembers.mockResolvedValue({ items: [member()] })
    api.listInvitations.mockResolvedValue({ items: [{ id: 5, workspace_id: 2, email: 'new@example.com', role: 'viewer', invited_by_user_id: 7, expires_at: '2027-01-01T00:00:00Z' }] })
    api.listAudit.mockResolvedValue({ items: [{ id: 9, workspace_id: 2, actor_user_id: 7, action: 'new.record', target_type: 'project', target_id: 20, created_at: '2026-10-01T00:00:00Z' }] })
    api.getOverview.mockResolvedValue({ summary: { members: 1, requests: 7, spend: 12.5 }, projects: [], platforms: [], models: [], api_keys: [] })
    api.getBudget.mockResolvedValue({ policy: { amount: 100 }, spent: 12.5, reserved: 0, remaining: 87.5 })
    api.acceptInvitation.mockResolvedValue(workspace(2))
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  const cases = [
    { section: 'members', component: WorkspaceMembersView, request: 'listMembers' as const, oldResponse: { items: [member('Old member')] }, current: 'New member', stale: 'Old member' },
    { section: 'invitations', component: WorkspaceInvitationsView, request: 'listInvitations' as const, oldResponse: { items: [{ id: 5, workspace_id: 1, email: 'old@example.com', role: 'viewer', invited_by_user_id: 7, expires_at: '2027-01-01T00:00:00Z' }] }, current: 'new@example.com', stale: 'old@example.com' },
    { section: 'audit', component: WorkspaceAuditView, request: 'listAudit' as const, oldResponse: { items: [{ id: 9, workspace_id: 1, actor_user_id: 7, action: 'old.record', target_type: 'project', target_id: 20, created_at: '2026-10-01T00:00:00Z' }] }, current: 'new.record', stale: 'old.record' },
    { section: 'overview', component: WorkspaceOverviewView, request: 'getOverview' as const, oldResponse: { summary: { members: 1, requests: 7, spend: 99 } }, current: '$12.50', stale: '$99.00' },
  ]

  it.each(cases)('$section ignores a response from the previous workspace', async ({ section, component, request, oldResponse, current, stale }) => {
    let resolveOld!: (value: unknown) => void
    api[request].mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const { wrapper, router } = await render(component, section)
    await router.push(`/workspaces/2/${section}`)
    await flushPromises()
    resolveOld(oldResponse)
    await flushPromises()
    expect(wrapper.text()).toContain(current)
    expect(wrapper.text()).not.toContain(stale)
  })

  it('lets an invited user accept a token and selects the returned workspace', async () => {
    const { wrapper, router } = await render(WorkspaceInvitationsView, 'invitations')
    await wrapper.find('input[name="invitation-token"]').setValue(' invitation-token ')
    await wrapper.find('form[data-testid="accept-invitation-form"]').trigger('submit')
    await flushPromises()
    expect(api.acceptInvitation).toHaveBeenCalledWith('invitation-token')
    expect(router.currentRoute.value.path).toBe('/workspaces/2/overview')
  })

  it('does not offer owner changes without the server owner.manage permission', async () => {
    api.listMembers.mockResolvedValue({ items: [member('Workspace owner', 'owner')] })
    const { wrapper } = await render(WorkspaceMembersView, 'members')
    const row = wrapper.find('tbody tr')
    expect(row.find('select').exists()).toBe(false)
    expect(row.findAll('button')).toHaveLength(0)
  })

  it('shows an inaccessible workspace route without displaying another workspace data', async () => {
    const { wrapper, router } = await render(WorkspaceMembersView, 'members')
    await router.push('/workspaces/999/members')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('New member')
  })

  it('creates a project with an explicit deny-all group policy and exact model allowlist', async () => {
    const { wrapper } = await render(WorkspaceProjectsView, 'projects')
    await wrapper.findAll('button').find(button => button.text() === 'Create project')!.trigger('click')
    const form = wrapper.find('form')
    await form.findAll('input')[0].setValue('Production')
    await form.findAll('input')[1].setValue('production')
    await form.find('select[name="group-policy"]').setValue('deny')
    await form.find('select[name="model-policy"]').setValue('restricted')
    await form.find('textarea[name="allowed-models"]').setValue('gpt-test\n gpt-test, claude-model')
    await form.trigger('submit')
    await flushPromises()
    expect(api.createProject).toHaveBeenCalledWith(1, { name: 'Production', slug: 'production', description: '', allowed_group_ids: [], allowed_models: ['gpt-test', 'claude-model'] })
  })

  it('preserves the difference between inherited and deny-all policies when editing a project', async () => {
    api.listProjects.mockResolvedValue({ items: [{ id: 10, workspace_id: 1, name: 'Production', slug: 'production', description: '', status: 'active', is_default: false, allowed_group_ids: [], allowed_models: null }] })
    const { wrapper } = await render(WorkspaceProjectsView, 'projects')
    await wrapper.find('tbody tr button').trigger('click')
    const form = wrapper.find('form')
    expect(form.find('select[name="group-policy"]').element.value).toBe('deny')
    expect(form.find('select[name="model-policy"]').element.value).toBe('inherit')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateProject).toHaveBeenCalledWith(1, 10, { name: 'Production', slug: 'production', description: '', allowed_group_ids: [], allowed_models: null })
  })
})
