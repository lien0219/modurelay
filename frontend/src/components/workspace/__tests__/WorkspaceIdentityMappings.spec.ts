import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import WorkspaceIdentityMappings from '../WorkspaceIdentityMappings.vue'
import type { WorkspaceIdentityProvider } from '@/api/workspace'

const mock = vi.hoisted(() => ({ getIdentityProviderMappings: vi.fn(), updateIdentityProviderMappings: vi.fn(), listTeams: vi.fn(), showSuccess: vi.fn(), showError: vi.fn() }))
const permissions = reactive({ values: ['identity.read', 'identity.manage', 'team.read'] })
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: mock }))
vi.mock('@/stores/workspace', () => ({ useWorkspaceStore: () => ({ get permissions() { return permissions.values }, can: (permission: string) => permissions.values.includes(permission) }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mock }))
const provider = { id: 9, workspace_id: 7, name: 'Enterprise Provider' } as WorkspaceIdentityProvider
let wrapper: VueWrapper | undefined

async function render() {
  wrapper = mount(WorkspaceIdentityMappings, { props: { workspaceId: 7, provider } })
  await flushPromises()
  return wrapper
}

describe('WorkspaceIdentityMappings', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    permissions.values = ['identity.read', 'identity.manage', 'team.read']
    mock.getIdentityProviderMappings.mockResolvedValue({ roles: [], teams: [] })
    mock.listTeams.mockResolvedValue({ items: [{ id: 4, name: 'Engineering', status: 'active' }, { id: 5, name: 'Archived team', status: 'archived' }] })
    mock.updateIdentityProviderMappings.mockResolvedValue({})
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined })

  it('maps groups to non-owner roles and active Phase A teams in the same workspace', async () => {
    const view = await render()
    await view.find('[data-testid="identity-add-role-mapping"]').trigger('click')
    await view.find('[data-testid="identity-add-team-mapping"]').trigger('click')
    await view.find('input[name="role-group-0"]').setValue(' engineering ')
    await view.find('select[name="mapped-role-0"]').setValue('developer')
    await view.find('input[type="number"]').setValue(10)
    await view.find('input[name="team-group-0"]').setValue(' engineering ')
    await view.find('select[name="mapped-team-0"]').setValue('4')
    expect(view.findAll('select[name="mapped-role-0"] option').map(option => option.attributes('value'))).not.toContain('owner')
    expect(view.find('select[name="mapped-team-0"]').text()).not.toContain('Archived team')
    await view.find('[data-testid="identity-mapping-form"]').trigger('submit')
    await flushPromises()
    expect(mock.updateIdentityProviderMappings).toHaveBeenCalledWith(7, 9, { roles: [{ claim_value: 'engineering', role: 'developer', priority: 10 }], teams: [{ claim_value: 'engineering', team_id: 4 }] })
    expect(view.emitted('saved')).toHaveLength(1)
  })

  it('keeps mappings readable but does not expose mutation actions without management permission', async () => {
    permissions.values = ['identity.read']
    mock.getIdentityProviderMappings.mockResolvedValue({ roles: [{ claim_value: 'finance', role: 'billing', priority: 0 }], teams: [{ claim_value: 'finance', team_id: 4 }] })
    const view = await render()
    expect(view.find('input[name="role-group-0"]').attributes()).toHaveProperty('disabled')
    expect(view.find('[data-testid="identity-add-role-mapping"]').exists()).toBe(false)
    expect(view.find('button[type="submit"]').exists()).toBe(false)
    expect(mock.listTeams).not.toHaveBeenCalled()
    await view.find('form').trigger('submit')
    expect(mock.updateIdentityProviderMappings).not.toHaveBeenCalled()
  })

  it('blocks blank group mappings and unselected team targets', async () => {
    const view = await render()
    await view.find('[data-testid="identity-add-team-mapping"]').trigger('click')
    await view.find('form').trigger('submit')
    expect(mock.updateIdentityProviderMappings).not.toHaveBeenCalled()
    expect(view.find('[role="alert"]').text()).toBe('workspace.identityMappingInvalid')
  })

  it('can select Phase A teams beyond the first page without replacing existing mappings', async () => {
    mock.listTeams.mockResolvedValueOnce({ items: [{ id: 4, name: 'Engineering', status: 'active' }], pages: 2 }).mockResolvedValueOnce({ items: [{ id: 104, name: 'Platform', status: 'active' }], pages: 2 })
    const view = await render()
    await view.find('[data-testid="identity-add-team-mapping"]').trigger('click')
    await view.find('[data-testid="identity-load-more-teams"]').trigger('click')
    await flushPromises()
    expect(mock.listTeams).toHaveBeenLastCalledWith(7, expect.objectContaining({ page: 2, page_size: 100 }))
    expect(view.find('select[name="mapped-team-0"]').text()).toContain('Engineering')
    expect(view.find('select[name="mapped-team-0"]').text()).toContain('Platform')
    expect(view.find('[data-testid="identity-load-more-teams"]').exists()).toBe(false)
  })

  it('does not overwrite the new tenant with a stale mapping response', async () => {
    let finish: (value: unknown) => void = () => {}
    mock.getIdentityProviderMappings.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    wrapper = mount(WorkspaceIdentityMappings, { props: { workspaceId: 7, provider } })
    await wrapper.setProps({ workspaceId: 8, provider: { ...provider, workspace_id: 8, id: 10 } })
    await flushPromises()
    finish({ roles: [{ claim_value: 'old-tenant-admin', role: 'admin', priority: 100 }], teams: [] })
    await flushPromises()
    expect(wrapper.html()).not.toContain('old-tenant-admin')
    expect(mock.getIdentityProviderMappings).toHaveBeenLastCalledWith(8, 10, expect.any(AbortSignal))
  })
})
