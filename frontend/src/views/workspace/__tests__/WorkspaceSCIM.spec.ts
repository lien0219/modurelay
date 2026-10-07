import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import WorkspaceIdentityView from '../WorkspaceIdentityView.vue'
const api = vi.hoisted(() => ({ listWorkspaces: vi.fn(), listProjects: vi.fn(), listDomains: vi.fn(), listIdentityProviders: vi.fn(), getIdentityPolicy: vi.fn(), listSCIMConnectors: vi.fn(), createSCIMConnector: vi.fn(), updateSCIMConnector: vi.fn(), disableSCIMConnector: vi.fn(), listSCIMTokens: vi.fn(), createSCIMToken: vi.fn(), revokeSCIMToken: vi.fn(), listSCIMGroups: vi.fn(), bindSCIMGroup: vi.fn(), listTeams: vi.fn() }))
const app = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => app }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
const permissions = ['workspace.read', 'provisioning.read', 'provisioning.manage', 'provisioning.token.rotate', 'team.read']
const connector = { id: 4, workspace_id: 7, revision: 3, public_endpoint_id: 'public-id', name: 'Directory A', status: 'active', default_role: 'viewer', group_mode: 'explicit', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z', failure_count: 0, base_url: 'https://configured.example/scim/v2/public-id' }
const token = { id: 9, connector_id: 4, token_prefix: 'scim_old', status: 'active', created_at: '2026-01-01T00:00:00Z', last_used_at: '2026-02-01T00:00:00Z' }
const group = { id: 'group-id', display_name: 'Engineering', external_id: 'directory-group', revision: 2, team_id: null }
const workspace = { id: 7, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 1, billing_owner_user_id: 1, permissions }
let wrapper: VueWrapper | undefined
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(done => { resolve = done }); return { promise, resolve } }
async function render(access = permissions, type = 'organization') {
  const pinia = createPinia(); setActivePinia(pinia)
  api.listWorkspaces.mockResolvedValue({ items: [{ ...workspace, permissions: access, type }, { ...workspace, id: 8, permissions: access, type }] })
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/identity', component: WorkspaceIdentityView }] })
  await router.push('/workspaces/7/identity')
  wrapper = mount(WorkspaceIdentityView, { global: { plugins: [pinia, router], stubs: { BaseDialog: { props: ['show', 'title'], template: '<div v-if="show" role="dialog"><h3>{{title}}</h3><slot/><slot name="footer"/></div>' } } } })
  await flushPromises(); return wrapper
}
async function open(view: VueWrapper) { expect(view.find('[data-testid="scim-open"]').exists()).toBe(true); await view.find('[data-testid="scim-open"]').trigger('click'); await flushPromises() }
async function tab(view: VueWrapper, name: string) { await view.find(`[data-testid="scim-tab-${name}"]`).trigger('click'); await flushPromises() }
describe('Workspace SCIM management', () => {
  beforeEach(() => {
    vi.resetAllMocks(); localStorage.clear(); sessionStorage.clear()
    api.listProjects.mockResolvedValue({ items: [] }); api.listDomains.mockResolvedValue({ items: [] }); api.listIdentityProviders.mockResolvedValue({ items: [] }); api.getIdentityPolicy.mockResolvedValue({ workspace_id: 7, require_sso: false, revision: 1 })
    api.listSCIMConnectors.mockResolvedValue([connector, { ...connector, id: 5, name: 'Directory B' }])
    api.listSCIMTokens.mockResolvedValue([token]); api.listSCIMGroups.mockResolvedValue([group]); api.listTeams.mockResolvedValue({ items: [{ id: 11, name: 'Engineering', status: 'active' }, { id: 12, name: 'Archived team', status: 'archived' }], pages: 1 })
    api.createSCIMConnector.mockResolvedValue({ ...connector, id: 6, name: 'New directory' }); api.updateSCIMConnector.mockResolvedValue({ ...connector, name: 'Edited', revision: 4 }); api.disableSCIMConnector.mockResolvedValue(null)
    api.createSCIMToken.mockResolvedValue({ token: { ...token, id: 10, token_prefix: 'scim_new' }, secret: 'one-time-secret' }); api.revokeSCIMToken.mockResolvedValue(null); api.bindSCIMGroup.mockResolvedValue(null)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks() })
  it('shows provisioning with centralized read permission independently of identity permission', async () => {
    const view = await render(); await open(view)
    expect(api.listSCIMConnectors).toHaveBeenCalledWith(7, expect.any(AbortSignal))
    expect(api.listDomains).not.toHaveBeenCalled(); expect(view.text()).toContain('Directory A')
  })
  it.each([['workspace.read'], ['workspace.read', 'provisioning.manage'], ['workspace.read', 'provisioning.token.rotate']])('hides provisioning and performs no reads without provisioning.read: %s', async (...access) => {
    const view = await render(access as string[])
    expect(view.find('[data-testid="scim-open"]').exists()).toBe(false); expect(api.listSCIMConnectors).not.toHaveBeenCalled()
  })
  it('does not expose SCIM in a personal Workspace', async () => {
    const view = await render(permissions, 'personal'); expect(view.find('[data-testid="scim-open"]').exists()).toBe(false)
  })
  it('creates and edits connectors with non-owner role and revision', async () => {
    const view = await render(); await open(view)
    await view.find('[data-testid="scim-add-connector"]').trigger('click')
    await view.find('[name="scim_name"]').setValue('New directory'); await view.find('[name="scim_default_role"]').setValue('developer')
    expect(view.findAll('[name="scim_default_role"] option').map(item => item.attributes('value'))).not.toContain('owner')
    await view.find('[data-testid="scim-connector-form"]').trigger('submit'); await flushPromises()
    expect(api.createSCIMConnector).toHaveBeenCalledWith(7, { name: 'New directory', default_role: 'developer' })
    await view.find('[name="scim_connector"]').setValue('4'); await flushPromises()
    await view.find('[data-testid="scim-edit-connector"]').trigger('click'); await view.find('[name="scim_name"]').setValue('Edited')
    await view.find('[data-testid="scim-connector-form"]').trigger('submit'); await flushPromises()
    expect(api.updateSCIMConnector).toHaveBeenCalledWith(7, 4, { name: 'Edited', default_role: 'viewer', revision: 3 })
  })
  it('uses the supplied base URL and never guesses a URL when operator configuration is absent', async () => {
    api.listSCIMConnectors.mockResolvedValue([{ ...connector, base_url: '' }]); const view = await render(); await open(view)
    expect(view.text()).toContain('workspace.scimBaseURLUnavailable'); expect(view.find('[data-testid="scim-copy-base"]').exists()).toBe(false)
    expect(view.html()).not.toContain('localhost/scim')
  })
  it('shows only safe outcome copy for arbitrary provider errors', async () => {
    api.listSCIMConnectors.mockResolvedValue([{ ...connector, failure_count: 3, last_error_code: 'secret=raw-credential', last_sync_at: '2026-01-02T00:00:00Z' }]); const view = await render(); await open(view)
    expect(view.html()).not.toContain('raw-credential'); expect(view.text()).toContain('workspace.scimSyncFailed'); expect(view.find('[data-testid="scim-failure-count"]').text()).toContain('3')
  })
  it('creates a replacement token while retaining the old active token, then clears the secret on dismiss', async () => {
    const view = await render(); await open(view); await tab(view, 'tokens')
    await view.find('[data-testid="scim-token-form"]').trigger('submit'); await flushPromises()
    expect(api.createSCIMToken).toHaveBeenCalledWith(7, 4, {}); expect(api.revokeSCIMToken).not.toHaveBeenCalled()
    expect(view.text()).toContain('scim_old'); expect(view.text()).toContain('scim_new'); expect(view.text()).toContain('one-time-secret')
    expect(Object.keys(localStorage).map(key => localStorage.getItem(key)).join()).not.toContain('one-time-secret'); expect(Object.keys(sessionStorage).map(key => sessionStorage.getItem(key)).join()).not.toContain('one-time-secret'); expect(JSON.stringify(app.showSuccess.mock.calls)).not.toContain('one-time-secret')
    await view.find('[data-testid="scim-secret-dismiss"]').trigger('click'); expect(view.html()).not.toContain('one-time-secret')
  })
  it.each(['connector', 'workspace', 'tab', 'section', 'permission'])('clears a displayed token on %s change', async kind => {
    const view = await render(); await open(view); await tab(view, 'tokens'); await view.find('[data-testid="scim-token-form"]').trigger('submit'); await flushPromises()
    expect(view.text()).toContain('one-time-secret')
    if (kind === 'connector') await view.find('[name="scim_connector"]').setValue('5')
    if (kind === 'workspace') await useWorkspaceStore().selectWorkspace(8)
    if (kind === 'tab') await tab(view, 'groups')
    if (kind === 'section') await view.find('[data-testid="scim-close"]').trigger('click')
    if (kind === 'permission') useWorkspaceStore().permissions = ['workspace.read']
    await flushPromises(); expect(view.html()).not.toContain('one-time-secret')
  })
  it('rejects stale token responses and does not unlock a new pending request in another connector', async () => {
    const old = deferred<unknown>(); const next = deferred<unknown>(); api.createSCIMToken.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise)
    const view = await render(); await open(view); await tab(view, 'tokens'); await view.find('[data-testid="scim-token-form"]').trigger('submit')
    expect(view.find('[data-testid="scim-create-token"]').attributes()).toHaveProperty('disabled')
    await view.find('[name="scim_connector"]').setValue('5'); await flushPromises(); await view.find('[data-testid="scim-token-form"]').trigger('submit')
    old.resolve({ token: { ...token, id: 10 }, secret: 'old-connector-secret' }); await flushPromises()
    expect(view.html()).not.toContain('old-connector-secret'); expect(view.find('[data-testid="scim-create-token"]').attributes()).toHaveProperty('disabled')
    next.resolve({ token: { ...token, id: 11, connector_id: 5 }, secret: 'current-secret' }); await flushPromises(); expect(view.text()).toContain('current-secret')
  })
  it('revokes only the confirmed selected token and keeps expired token history readable', async () => {
    api.listSCIMTokens.mockResolvedValue([token, { ...token, id: 8, token_prefix: 'expired', expires_at: '2020-01-01T00:00:00Z' }]); const view = await render(); await open(view); await tab(view, 'tokens')
    expect(view.text()).toContain('workspace.scimTokenStates.expired'); vi.mocked(window.confirm).mockReturnValue(false)
    await view.find('[data-testid="scim-revoke-9"]').trigger('click'); expect(api.revokeSCIMToken).not.toHaveBeenCalled()
    vi.mocked(window.confirm).mockReturnValue(true); await view.find('[data-testid="scim-revoke-9"]').trigger('click'); await flushPromises(); expect(api.revokeSCIMToken).toHaveBeenCalledWith(7, 4, 9)
  })
  it('explicitly binds and unbinds returned groups without matching team names', async () => {
    const view = await render(); await open(view); await tab(view, 'groups')
    expect(api.bindSCIMGroup).not.toHaveBeenCalled(); const select = view.find('[name="scim-team-group-id"]'); expect(select.element).toHaveProperty('value', '')
    expect(select.text()).not.toContain('Archived team'); await select.setValue('11'); await view.find('[data-testid="scim-bind-group-id"]').trigger('click'); await flushPromises()
    expect(api.bindSCIMGroup).toHaveBeenCalledWith(7, 4, 'group-id', { revision: 2, team_id: 11 })
    api.listSCIMGroups.mockResolvedValue([{ ...group, revision: 3, team_id: 11 }]); await view.find('[data-testid="scim-refresh"]').trigger('click'); await flushPromises()
    await view.find('[data-testid="scim-unbind-group-id"]').trigger('click'); await flushPromises(); expect(api.bindSCIMGroup).toHaveBeenLastCalledWith(7, 4, 'group-id', { revision: 3, team_id: null })
  })
  it('requires explicit connector disable confirmation and stops writes while preserving history', async () => {
    const view = await render(); await open(view); vi.mocked(window.confirm).mockReturnValue(false)
    await view.find('[data-testid="scim-disable-connector"]').trigger('click'); expect(api.disableSCIMConnector).not.toHaveBeenCalled()
    vi.mocked(window.confirm).mockReturnValue(true); await view.find('[data-testid="scim-disable-connector"]').trigger('click'); await flushPromises()
    expect(api.disableSCIMConnector).toHaveBeenCalledWith(7, 4, 3); expect(view.find('[data-testid="scim-edit-connector"]').exists()).toBe(false)
    await tab(view, 'tokens'); expect(view.text()).toContain('scim_old'); expect(view.find('[data-testid="scim-token-form"]').exists()).toBe(false)
    await tab(view, 'groups'); expect(view.text()).toContain('Engineering'); expect(view.find('[data-testid="scim-bind-group-id"]').exists()).toBe(false)
  })
  it('separates management permission from token rotation permission', async () => {
    const view = await render(['workspace.read', 'provisioning.read', 'provisioning.token.rotate']); await open(view)
    expect(view.find('[data-testid="scim-edit-connector"]').exists()).toBe(false); expect(view.find('[data-testid="scim-add-connector"]').exists()).toBe(false)
    await tab(view, 'tokens'); expect(view.find('[data-testid="scim-token-form"]').exists()).toBe(true)
    await tab(view, 'groups'); expect(view.find('[data-testid="scim-bind-group-id"]').exists()).toBe(false)
  })
  it('sends optional token expiry as UTC and blocks expired input', async () => {
    const view = await render(); await open(view); await tab(view, 'tokens')
    await view.find('[name="scim_expiry"]').setValue('2020-01-01T00:00')
    await view.find('[data-testid="scim-token-form"]').trigger('submit'); expect(api.createSCIMToken).not.toHaveBeenCalled()
    await view.find('[name="scim_expiry"]').setValue('2030-01-01T09:30')
    await view.find('[data-testid="scim-token-form"]').trigger('submit'); await flushPromises()
    expect(api.createSCIMToken).toHaveBeenCalledWith(7, 4, { expires_at: new Date('2030-01-01T09:30').toISOString() })
  })
  it('ignores stale token results and finalizers after switching Workspace and starting a new mutation', async () => {
    const old = deferred<unknown>(); const next = deferred<unknown>(); api.createSCIMToken.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise)
    const view = await render(); await open(view); await tab(view, 'tokens'); await view.find('[data-testid="scim-token-form"]').trigger('submit')
    await useWorkspaceStore().selectWorkspace(8); await flushPromises(); await open(view); await tab(view, 'tokens')
    await view.find('[data-testid="scim-token-form"]').trigger('submit')
    old.resolve({ token: { ...token, id: 10 }, secret: 'old-workspace-secret' }); await flushPromises()
    expect(view.html()).not.toContain('old-workspace-secret'); expect(view.find('[data-testid="scim-create-token"]').attributes()).toHaveProperty('disabled')
    next.resolve({ token: { ...token, id: 11 }, secret: 'new-workspace-secret' }); await flushPromises()
    expect(view.text()).toContain('new-workspace-secret'); expect(api.createSCIMToken).toHaveBeenLastCalledWith(8, 4, {})
  })
  it('drops an old Workspace connector listing after the new Workspace has loaded', async () => {
    const old = deferred<unknown>(); api.listSCIMConnectors.mockReturnValueOnce(old.promise)
    const view = await render(); await open(view); await useWorkspaceStore().selectWorkspace(8); await flushPromises(); await open(view)
    old.resolve([{ ...connector, name: 'Old tenant directory' }]); await flushPromises()
    expect(view.text()).not.toContain('Old tenant directory'); expect(view.text()).toContain('Directory A')
  })
  it('ignores token completion after unmount and never persists or toasts the secret', async () => {
    const old = deferred<unknown>(); api.createSCIMToken.mockReturnValueOnce(old.promise)
    const view = await render(); await open(view); await tab(view, 'tokens'); await view.find('[data-testid="scim-token-form"]').trigger('submit')
    view.unmount(); wrapper = undefined; old.resolve({ token: { ...token, id: 10 }, secret: 'unmounted-secret' }); await flushPromises()
    expect(document.body.textContent).not.toContain('unmounted-secret')
    expect(JSON.stringify(app.showSuccess.mock.calls)).not.toContain('unmounted-secret')
    expect(Object.keys(localStorage).map(key => localStorage.getItem(key)).join()).not.toContain('unmounted-secret')
    expect(Object.keys(sessionStorage).map(key => sessionStorage.getItem(key)).join()).not.toContain('unmounted-secret')
  })
  it('keeps the new connector busy when an old group-binding finalizer resolves', async () => {
    const old = deferred<unknown>(); const next = deferred<unknown>(); api.bindSCIMGroup.mockReturnValueOnce(old.promise).mockReturnValueOnce(next.promise)
    const view = await render(); await open(view); await tab(view, 'groups'); await view.find('[name="scim-team-group-id"]').setValue('11')
    await view.find('[data-testid="scim-bind-group-id"]').trigger('click')
    await view.find('[name="scim_connector"]').setValue('5'); await flushPromises(); await view.find('[name="scim-team-group-id"]').setValue('11')
    await view.find('[data-testid="scim-bind-group-id"]').trigger('click')
    const listCalls = api.listSCIMGroups.mock.calls.length
    old.resolve(null); await flushPromises()
    expect(api.listSCIMGroups.mock.calls.length).toBe(listCalls)
    expect(view.find('[data-testid="scim-bind-group-id"]').attributes()).toHaveProperty('disabled')
    next.resolve(null); await flushPromises(); expect(view.find('[data-testid="scim-bind-group-id"]').attributes()).not.toHaveProperty('disabled')
  })
  it('loads active Teams beyond the first page without creating or name matching a Team', async () => {
    api.listTeams.mockResolvedValue({ items: [{ id: 11, name: 'Engineering', status: 'active' }], pages: 2 })
    const view = await render(); await open(view); await tab(view, 'groups')
    api.listTeams.mockResolvedValueOnce({ items: [{ id: 13, name: 'Platform', status: 'active' }, { id: 14, name: 'Archived', status: 'archived' }], pages: 2 })
    await view.find('[data-testid="scim-load-more-teams"]').trigger('click'); await flushPromises()
    expect(api.listTeams).toHaveBeenLastCalledWith(7, expect.objectContaining({ page: 2, page_size: 100 }))
    expect(view.find('[name="scim-team-group-id"]').text()).toContain('Platform'); expect(view.find('[name="scim-team-group-id"]').text()).not.toContain('Archived')
    expect(api.bindSCIMGroup).not.toHaveBeenCalled()
  })
  it.each([[{ status: 409, message: 'secret=private' }, 'workspace.scimConflict'], [{ reason: 'RECENT_AUTH_REQUIRED', message: 'secret=private' }, 'workspace.scimRecentAuthRequired']])('shows actionable safe mutation guidance for %s', async (error, message) => {
    api.createSCIMToken.mockRejectedValueOnce(error); const view = await render(); await open(view); await tab(view, 'tokens')
    await view.find('[data-testid="scim-token-form"]').trigger('submit'); await flushPromises()
    expect(view.find('[role="alert"]').text()).toContain(message); expect(view.html()).not.toContain('secret=private')
    expect(view.find('[data-testid="scim-create-token"]').attributes()).not.toHaveProperty('disabled')
  })
  it('allows provisioning readers to inspect all history without exposing any mutation controls', async () => {
    const view = await render(['workspace.read', 'provisioning.read']); await open(view)
    expect(view.text()).toContain('https://configured.example/scim/v2/public-id'); expect(view.find('[data-testid="scim-add-connector"]').exists()).toBe(false)
    await tab(view, 'tokens'); expect(view.text()).toContain('scim_old'); expect(view.find('[data-testid="scim-token-form"]').exists()).toBe(false); expect(view.find('[data-testid="scim-revoke-9"]').exists()).toBe(false)
    await tab(view, 'groups'); expect(view.text()).toContain('Engineering'); expect(view.find('[name="scim-team-group-id"]').attributes()).toHaveProperty('disabled')
    expect(api.listTeams).not.toHaveBeenCalled()
  })
  it('copies only the supplied Base URL and bearer when explicitly requested', async () => {
    const copy = vi.fn().mockResolvedValue(undefined); Object.defineProperty(navigator, 'clipboard', { value: { writeText: copy }, configurable: true })
    const view = await render(); await open(view); await view.find('[data-testid="scim-copy-base"]').trigger('click'); await flushPromises()
    expect(copy).toHaveBeenCalledWith(connector.base_url)
    await tab(view, 'tokens'); await view.find('[data-testid="scim-token-form"]').trigger('submit'); await flushPromises()
    await view.find('[data-testid="scim-secret-copy"]').trigger('click'); await flushPromises(); expect(copy).toHaveBeenLastCalledWith('one-time-secret')
    expect(view.find('[role="dialog"]').text()).toContain('workspace.scimCopied')
  })
  it('shows safe error and retry instead of raw backend credentials', async () => {
    api.listSCIMConnectors.mockRejectedValueOnce({ message: 'bearer=raw-secret' }); const view = await render(); await open(view)
    expect(view.find('[role="alert"]').text()).toContain('workspace.scimLoadError'); expect(view.html()).not.toContain('raw-secret')
    await view.find('[data-testid="scim-retry"]').trigger('click'); await flushPromises(); expect(view.text()).toContain('Directory A')
  })
})
