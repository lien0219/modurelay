import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import { createTestI18n } from '@/__tests__/utils/i18n'
import en from '@/i18n/locales/en'
import WorkspaceOverviewView from '../WorkspaceOverviewView.vue'

const lifecycle = vi.hoisted(() => ({
  getLifecycle: vi.fn(), updateRetention: vi.fn(), restoreWorkspace: vi.fn(),
  listExports: vi.fn(), createExport: vi.fn(), cancelExport: vi.fn(),
  authorizeDownload: vi.fn(), redeemDownload: vi.fn(),
  getDeletionPreflight: vi.fn(), createDeletionChallenge: vi.fn(),
  requestDeletion: vi.fn(), getDeletion: vi.fn(), cancelDeletion: vi.fn(), retryDeletion: vi.fn(),
}))
const workspace = vi.hoisted(() => ({
  getOverview: vi.fn(), getBudget: vi.fn(), listMembers: vi.fn(),
  archiveWorkspace: vi.fn(), getWorkspace: vi.fn(), listProjects: vi.fn(),
}))
vi.mock('@/api/workspaceLifecycle', () => ({ workspaceLifecycleAPI: lifecycle }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: workspace }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/components/auth/TotpStepUpDialog.vue', () => ({ default: { template: '<div />' } }))

const ownerPermissions = ['workspace.read', 'lifecycle.read', 'lifecycle.manage', 'workspace.archive', 'workspace.restore', 'export.create', 'export.read', 'export.download', 'export.cancel', 'deletion.request', 'deletion.cancel', 'deletion.retry']
const preflight = (eligible = true) => ({
  eligible, blocking_reasons: eligible ? [] : [{ code: 'ACTIVE_HOLDS', reason: 'Active budget reservations', count: 2 }],
  resource_counts: { projects: 4 }, protected_records: { usage_logs: 8 }, counts_capped: false,
  estimated_purge_scope: ['credentials'], retained_scope: ['financial_evidence', 'policy_ineligible_business_configuration'],
  earliest_purge_at: '2026-10-16T00:00:00Z', protected_evidence_retained: true,
})
const deletion = (overrides = {}) => ({
  id: 'delete-1', workspace_id: 1, state: 'pending', previous_status: 'active', phase: 'credentials',
  cursor: 0, progress: 0, attempts: 0, blocking_reasons: [], business_closed: false,
  protected_evidence_retained: true, earliest_purge_at: '2026-10-16T00:00:00Z',
  created_at: '2026-10-09T00:00:00Z', updated_at: '2026-10-09T00:00:00Z', completed_at: null,
  ...overrides,
})
const completedExport = { id: 'export-1', workspace_id: 1, state: 'completed', attempts: 1, progress: 12, size_bytes: 300, artifact_sha256: 'a'.repeat(64), data_cutoff: '2026-10-09T00:00:00Z', created_at: '2026-10-09T00:00:00Z', updated_at: '2026-10-09T00:00:00Z', completed_at: '2026-10-09T00:01:00Z', expires_at: '2099-10-16T00:00:00Z' }
const retention = [{ category: 'operational', retention_days: 30, minimum_days: 30, protected: false }, { category: 'financial', retention_days: 0, minimum_days: 0, protected: true }]
const summary = (overrides = {}) => ({ retention, deletion: null, capabilities: { export_enabled: true, purge_enabled: true }, ...overrides })
let wrapper: VueWrapper | undefined

async function render(permissions = ownerPermissions, status = 'active') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const store = useWorkspaceStore()
  store.workspaces = [
    { id: 1, name: 'ACME', slug: 'acme', type: 'organization', status, owner_user_id: 7, billing_owner_user_id: 7, permissions },
    { id: 2, name: 'Other tenant', slug: 'other', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions },
  ]
  store.selectedWorkspaceId = 1
  store.permissions = permissions
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/overview', component: WorkspaceOverviewView }, { path: '/login', component: { template: '<div />' } }] })
  await router.push('/workspaces/1/overview')
  await router.isReady()
  wrapper = mount(WorkspaceOverviewView, { attachTo: document.body, global: { plugins: [pinia, router, createTestI18n({ en })], stubs: { BaseDialog: { props: ['show', 'title'], template: '<div v-if="show" role="dialog" :aria-label="title"><slot /><slot name="footer" /></div>' } } } })
  await flushPromises()
  return { wrapper, store, router }
}

describe('Workspace lifecycle settings', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    workspace.getOverview.mockResolvedValue(null)
    workspace.getBudget.mockResolvedValue(null)
    workspace.listMembers.mockResolvedValue({ items: [] })
    workspace.listProjects.mockResolvedValue({ items: [] })
    lifecycle.getLifecycle.mockResolvedValue(summary())
    lifecycle.listExports.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 10, pages: 0 })
    lifecycle.getDeletionPreflight.mockResolvedValue(preflight())
    lifecycle.createDeletionChallenge.mockResolvedValue({ token: 'one-use-secret', expires_at: new Date(Date.now() + 60000).toISOString(), preflight: preflight() })
    lifecycle.requestDeletion.mockResolvedValue(deletion())
    lifecycle.cancelDeletion.mockResolvedValue(undefined)
    lifecycle.cancelExport.mockResolvedValue(undefined)
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; document.body.innerHTML = ''; vi.useRealTimers(); vi.unstubAllGlobals(); vi.restoreAllMocks() })

  it('renders blockers and prevents deletion when preflight is ineligible', async () => {
    lifecycle.getDeletionPreflight.mockResolvedValue(preflight(false))
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="deletion-preflight"]').exists()).toBe(true)
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Active budget reservations')
    expect(wrapper.get('[data-testid="request-deletion"]').attributes('disabled')).toBeDefined()
  })

  it('requires the exact current Workspace name and a fresh eligible challenge', async () => {
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="request-deletion"]').exists()).toBe(true)
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    const confirm = wrapper.get('[data-testid="confirm-deletion"]')
    expect(confirm.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME ')
    expect(confirm.attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME')
    expect(confirm.attributes('disabled')).toBeUndefined()
    await confirm.trigger('click')
    await flushPromises()
    expect(lifecycle.requestDeletion).toHaveBeenCalledWith(1, { name: 'ACME', token: 'one-use-secret' }, expect.any(AbortSignal))
    expect(wrapper.find('[data-testid="deletion-name"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('one-use-secret')
  })

  it('uses challenge preflight instead of an earlier eligible result', async () => {
    lifecycle.createDeletionChallenge.mockResolvedValue({ token: 'one-use-secret', expires_at: new Date(Date.now() + 60000).toISOString(), preflight: preflight(false) })
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="deletion-preflight"]').exists()).toBe(true)
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME')
    expect(wrapper.get('[data-testid="confirm-deletion"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('Active budget reservations')
  })

  it('disables confirmation when its one-use challenge expires', async () => {
    lifecycle.createDeletionChallenge.mockResolvedValue({ token: 'expired', expires_at: '2000-01-01T00:00:00Z', preflight: preflight() })
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="deletion-preflight"]').exists()).toBe(true)
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME')
    expect(wrapper.get('[data-testid="confirm-deletion"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('expired')
    expect(lifecycle.requestDeletion).not.toHaveBeenCalled()
  })

  it('shows completed business closure separately from retained evidence and current retention', async () => {
    lifecycle.getLifecycle.mockResolvedValue(summary({ deletion: deletion({ state: 'completed', business_closed: true }) }))
    const { wrapper } = await render(ownerPermissions, 'deleted')
    expect(wrapper.find('[data-testid="lifecycle-panel"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Business closed')
    expect(wrapper.text()).toContain('Protected financial and audit evidence is retained')
    expect(wrapper.text()).toContain('current retention')
    expect(wrapper.text()).not.toContain('physically erased')
    expect(wrapper.find('[data-testid="request-deletion"]').exists()).toBe(false)
  })

  it('offers downloads for completed exports with indefinite artifact retention', async () => {
    lifecycle.listExports.mockResolvedValue({ items: [{ ...completedExport, expires_at: null }], total: 1, page: 1, page_size: 10, pages: 1 })
    const { wrapper } = await render()
    expect(wrapper.get('[data-testid="download-export-export-1"]').attributes('disabled')).toBeUndefined()
  })

  it.each(['2000-01-01T00:00:00Z', 'invalid-expiry'])('rejects unavailable export downloads for expiry %s', async expires_at => {
    lifecycle.listExports.mockResolvedValue({ items: [{ ...completedExport, expires_at }], total: 1, page: 1, page_size: 10, pages: 1 })
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="download-export-export-1"]').exists()).toBe(false)
  })

  it('allows cancellation only before purge has made any progress', async () => {
    lifecycle.getLifecycle.mockResolvedValue(summary({ deletion: deletion() }))
    const { wrapper } = await render(ownerPermissions, 'pending_deletion')
    expect(wrapper.find('[data-testid="cancel-deletion"]').exists()).toBe(true)
    await wrapper.get('[data-testid="cancel-deletion"]').trigger('click')
    await flushPromises()
    expect(lifecycle.cancelDeletion).toHaveBeenCalledWith(1, 'delete-1', expect.any(AbortSignal))
  })

  it('hides cancellation after purge starts and exposes permitted failed retry', async () => {
    lifecycle.getLifecycle.mockResolvedValue(summary({ deletion: deletion({ state: 'failed', phase: 'configuration', cursor: 12, progress: 12, failure_code: 'PURGE_RETRY' }) }))
    const { wrapper } = await render(ownerPermissions, 'purging')
    expect(wrapper.find('[data-testid="retry-deletion"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="cancel-deletion"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('PURGE_RETRY')
  })

  it('keeps cancellation available before purge when cleanup is disabled', async () => {
    lifecycle.getLifecycle.mockResolvedValue(summary({ deletion: deletion(), capabilities: { export_enabled: false, purge_enabled: false } }))
    const { wrapper } = await render(ownerPermissions, 'pending_deletion')
    expect(wrapper.find('[data-testid="cancel-deletion"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="retry-deletion"]').exists()).toBe(false)
    await wrapper.get('[data-testid="cancel-deletion"]').trigger('click')
    await flushPromises()
    expect(lifecycle.cancelDeletion).toHaveBeenCalledWith(1, 'delete-1', expect.any(AbortSignal))
  })

  it('hides cancellation while a deletion worker is running even before its first batch', async () => {
    lifecycle.getLifecycle.mockResolvedValue(summary({ deletion: deletion({ state: 'running' }) }))
    const { wrapper } = await render(ownerPermissions, 'pending_deletion')
    expect(wrapper.find('[data-testid="cancel-deletion"]').exists()).toBe(false)
  })

  it('keeps read-only Admin lifecycle visible without Owner-only queries or actions', async () => {
    const { wrapper } = await render(['workspace.read', 'lifecycle.read'])
    expect(wrapper.find('[data-testid="lifecycle-panel"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="deletion-preflight"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="create-export"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="retention-save-operational"]').exists()).toBe(false)
    expect(lifecycle.listExports).not.toHaveBeenCalled()
    expect(lifecycle.getDeletionPreflight).not.toHaveBeenCalled()
  })

  it('shows a restore failure and its reauthentication action inside the open confirmation dialog', async () => {
    lifecycle.restoreWorkspace.mockRejectedValue({ reason: 'RECENT_AUTH_REQUIRED', message: 'Restore requires a fresh sign-in.' })
    const { wrapper } = await render(ownerPermissions, 'archived')
    await wrapper.get('[data-testid="restore-workspace"]').trigger('click')
    await flushPromises()
    const dialog = wrapper.findComponent({ name: 'ConfirmDialog' })
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(dialog.text()).toContain('Restore requires a fresh sign-in.')
    expect(dialog.find('a').attributes('href')).toContain('reauth=1')
  })

  it('hides lifecycle from roles without central lifecycle.read permission', async () => {
    const { wrapper } = await render(['workspace.read'])
    expect(wrapper.find('[data-testid="lifecycle-panel"]').exists()).toBe(false)
    expect(lifecycle.getLifecycle).not.toHaveBeenCalled()
  })

  it('ignores a late lifecycle read from a previous Workspace', async () => {
    let resolveOld!: (value: ReturnType<typeof summary>) => void
    lifecycle.getLifecycle.mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const { wrapper, store } = await render()
    await store.selectWorkspace(2)
    await flushPromises()
    resolveOld(summary({ deletion: deletion({ failure_code: 'OLD_TENANT_ONLY' }) }))
    await flushPromises()
    expect(wrapper.find('[data-testid="lifecycle-panel"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('OLD_TENANT_ONLY')
  })

  it('closes deletion modal and discards a late challenge on a Workspace switch', async () => {
    let resolveChallenge!: (value: unknown) => void
    lifecycle.createDeletionChallenge.mockImplementationOnce(() => new Promise(resolve => { resolveChallenge = resolve }))
    const { wrapper, store } = await render()
    expect(wrapper.find('[data-testid="deletion-preflight"]').exists()).toBe(true)
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await store.selectWorkspace(2)
    resolveChallenge({ token: 'stale-token', expires_at: '2099-01-01T00:00:00Z', preflight: preflight() })
    await flushPromises()
    expect(wrapper.find('[data-testid="deletion-name"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('stale-token')
    expect(lifecycle.requestDeletion).not.toHaveBeenCalled()
  })

  it('downloads a ZIP through authenticated redemption and promptly revokes the blob URL', async () => {
    lifecycle.listExports.mockResolvedValue({ items: [completedExport], total: 1, page: 1, page_size: 10, pages: 1 })
    lifecycle.authorizeDownload.mockResolvedValue({ token: 'download-secret', expires_at: '2099-01-01T00:00:00Z' })
    lifecycle.redeemDownload.mockResolvedValue(new Blob(['zip'], { type: 'application/zip' }))
    const createObjectURL = vi.fn(() => 'blob:local-export')
    const revokeObjectURL = vi.fn()
    const OriginalURL = URL
    vi.stubGlobal('URL', class extends OriginalURL { static createObjectURL = createObjectURL; static revokeObjectURL = revokeObjectURL })
    const clickedLinks: string[] = []
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function () { clickedLinks.push(this.href) })
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="download-export-export-1"]').exists()).toBe(true)
    await wrapper.get('[data-testid="download-export-export-1"]').trigger('click')
    await flushPromises()
    expect(lifecycle.redeemDownload).toHaveBeenCalledWith(1, 'export-1', 'download-secret', expect.any(AbortSignal))
    expect(clickedLinks).toEqual(['blob:local-export'])
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:local-export')
    expect(wrapper.html()).not.toContain('download-secret')
    expect(Object.values(localStorage).join(' ')).not.toContain('download-secret')
    expect(Object.values(sessionStorage).join(' ')).not.toContain('download-secret')
  })

  it('keeps action errors visible and supplies a sign-in action for recent-auth failures', async () => {
    lifecycle.createDeletionChallenge.mockRejectedValue({ reason: 'RECENT_AUTH_REQUIRED', message: 'Reauthenticate to continue.' })
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="deletion-preflight"]').exists()).toBe(true)
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Reauthenticate to continue.')
    expect(wrapper.find('[data-testid="lifecycle-sign-in"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="lifecycle-sign-in"]').attributes('href')).toContain('reauth=1')
    expect(lifecycle.requestDeletion).not.toHaveBeenCalled()
  })

  it('does not allow finite financial retention or days below the operational floor', async () => {
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="retention-days-operational"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="retention-days-financial"]').exists()).toBe(false)
    await wrapper.get('[data-testid="retention-days-operational"]').setValue('29')
    expect(wrapper.get('[data-testid="retention-save-operational"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="retention-days-operational"]').setValue('0')
    expect(wrapper.get('[data-testid="retention-save-operational"]').attributes('disabled')).toBeUndefined()
  })

  it('allows an Owner to change currently indefinite operational retention above its floor', async () => {
    lifecycle.getLifecycle.mockResolvedValue(summary({ retention: [{ category: 'operational', retention_days: 0, minimum_days: 30, protected: true }] }))
    lifecycle.updateRetention.mockResolvedValue([{ category: 'operational', retention_days: 45, minimum_days: 30, protected: false }])
    const { wrapper } = await render()
    await wrapper.get('[data-testid="retention-days-operational"]').setValue('45')
    await wrapper.get('[data-testid="retention-save-operational"]').trigger('click')
    await flushPromises()
    expect(lifecycle.updateRetention).toHaveBeenCalledWith(1, 'operational', 45, expect.any(AbortSignal))
    expect(wrapper.text()).toContain('45 days')
  })

  it('blocks duplicate deletion submissions while the first request is pending', async () => {
    let resolveDeletion!: (value: ReturnType<typeof deletion>) => void
    lifecycle.requestDeletion.mockImplementationOnce(() => new Promise(resolve => { resolveDeletion = resolve }))
    const { wrapper } = await render()
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME')
    const confirm = wrapper.get('[data-testid="confirm-deletion"]')
    await confirm.trigger('click')
    await confirm.trigger('click')
    expect(lifecycle.requestDeletion).toHaveBeenCalledTimes(1)
    expect(confirm.attributes('disabled')).toBeDefined()
    resolveDeletion(deletion())
    await flushPromises()
  })

  it('keeps a deletion request failure visible and requires a new challenge for retry', async () => {
    lifecycle.requestDeletion.mockRejectedValue({ reason: 'DELETION_CONFIRMATION_INVALID', message: 'Confirmation is expired or already used.' })
    const { wrapper } = await render()
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME')
    await wrapper.get('[data-testid="confirm-deletion"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Confirmation is expired or already used.')
    expect(wrapper.get('[data-testid="confirm-deletion"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
  })

  it('ignores a late mutation failure when switching away and back to the same Workspace', async () => {
    let rejectExport!: (error: unknown) => void
    lifecycle.createExport.mockImplementationOnce(() => new Promise((_resolve, reject) => { rejectExport = reject }))
    const { wrapper, store } = await render()
    await wrapper.get('[data-testid="create-export"]').trigger('click')
    await store.selectWorkspace(2)
    await store.selectWorkspace(1)
    await flushPromises()
    rejectExport({ message: 'OLD_GENERATION_ONLY' })
    await flushPromises()
    expect(wrapper.text()).not.toContain('OLD_GENERATION_ONLY')
    expect(wrapper.get('[data-testid="create-export"]').attributes('disabled')).toBeUndefined()
  })

  it('does not redeem a late download grant after switching Workspace', async () => {
    lifecycle.listExports.mockResolvedValue({ items: [completedExport], total: 1, page: 1, page_size: 10, pages: 1 })
    let resolveGrant!: (value: unknown) => void
    lifecycle.authorizeDownload.mockImplementationOnce(() => new Promise(resolve => { resolveGrant = resolve }))
    const { wrapper, store } = await render()
    await wrapper.get('[data-testid="download-export-export-1"]').trigger('click')
    await store.selectWorkspace(2)
    resolveGrant({ token: 'old-grant', expires_at: '2099-01-01T00:00:00Z' })
    await flushPromises()
    expect(lifecycle.redeemDownload).not.toHaveBeenCalled()
  })

  it('does not create a download URL when redemption finishes after switching Workspace', async () => {
    lifecycle.listExports.mockResolvedValue({ items: [completedExport], total: 1, page: 1, page_size: 10, pages: 1 })
    lifecycle.authorizeDownload.mockResolvedValue({ token: 'old-grant', expires_at: '2099-01-01T00:00:00Z' })
    let resolveBlob!: (value: Blob) => void
    lifecycle.redeemDownload.mockImplementationOnce(() => new Promise(resolve => { resolveBlob = resolve }))
    const createObjectURL = vi.fn(() => 'blob:old-tenant')
    const OriginalURL = URL
    vi.stubGlobal('URL', class extends OriginalURL { static createObjectURL = createObjectURL; static revokeObjectURL = vi.fn() })
    const { wrapper, store } = await render()
    await wrapper.get('[data-testid="download-export-export-1"]').trigger('click')
    await flushPromises()
    await store.selectWorkspace(2)
    resolveBlob(new Blob(['zip']))
    await flushPromises()
    expect(createObjectURL).not.toHaveBeenCalled()
  })

  it('cancels pending exports and exposes no download action before completion', async () => {
    lifecycle.listExports.mockResolvedValue({ items: [{ ...completedExport, state: 'running' }], total: 1, page: 1, page_size: 10, pages: 1 })
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="download-export-export-1"]').exists()).toBe(false)
    await wrapper.get('[data-testid="cancel-export-export-1"]').trigger('click')
    await flushPromises()
    expect(lifecycle.cancelExport).toHaveBeenCalledWith(1, 'export-1', expect.any(AbortSignal))
  })

  it('polls pending jobs at a bounded interval and stops after unmount', async () => {
    vi.useFakeTimers()
    lifecycle.getLifecycle.mockResolvedValue(summary({ deletion: deletion() }))
    const { wrapper } = await render(ownerPermissions, 'pending_deletion')
    expect(lifecycle.getLifecycle).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(14999)
    expect(lifecycle.getLifecycle).toHaveBeenCalledTimes(1)
    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(lifecycle.getLifecycle).toHaveBeenCalledTimes(2)
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(60000)
    expect(lifecycle.getLifecycle).toHaveBeenCalledTimes(2)
  })

  it('expires an initially valid challenge while its dialog is still open', async () => {
    vi.useFakeTimers()
    lifecycle.createDeletionChallenge.mockResolvedValue({ token: 'short-grant', expires_at: new Date(Date.now() + 1000).toISOString(), preflight: preflight() })
    const { wrapper } = await render()
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="deletion-name"]').setValue('ACME')
    expect(wrapper.get('[data-testid="confirm-deletion"]').attributes('disabled')).toBeUndefined()
    await vi.advanceTimersByTimeAsync(1001)
    expect(wrapper.get('[data-testid="confirm-deletion"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('confirmation expired')
  })

  it('keeps keyboard focus inside the deletion dialog', async () => {
    const { wrapper } = await render()
    await wrapper.get('[data-testid="deletion-preflight"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="request-deletion"]').trigger('click')
    await flushPromises()
    const input = wrapper.get('[data-testid="deletion-name"]')
    await input.setValue('ACME')
    ;(input.element as HTMLInputElement).focus()
    await input.trigger('keydown', { key: 'Tab', shiftKey: true })
    expect(document.activeElement).toBe(wrapper.get('[data-testid="confirm-deletion"]').element)
    await wrapper.get('[data-testid="confirm-deletion"]').trigger('keydown', { key: 'Tab' })
    expect(document.activeElement).toBe(input.element)
  })

  it('preserves unsaved retention input while export jobs are polled', async () => {
    vi.useFakeTimers()
    lifecycle.listExports.mockResolvedValue({ items: [{ ...completedExport, state: 'running' }], total: 1, page: 1, page_size: 10, pages: 1 })
    const { wrapper } = await render()
    await wrapper.get('[data-testid="retention-days-operational"]').setValue('45')
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect((wrapper.get('[data-testid="retention-days-operational"]').element as HTMLInputElement).value).toBe('45')
  })

  it('preserves parent settings and an open create form across unchanged-permission polling', async () => {
    vi.useFakeTimers()
    const permissions = [...ownerPermissions, 'workspace.update', 'member.read']
    workspace.listMembers.mockResolvedValue({ items: [{ user_id: 7, status: 'active', role: 'owner' }] })
    workspace.getWorkspace.mockImplementation(() => Promise.resolve({ id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: [...permissions] }))
    lifecycle.listExports.mockResolvedValue({ items: [{ ...completedExport, state: 'running' }], total: 1, page: 1, page_size: 10, pages: 1 })
    const { wrapper } = await render(permissions)
    const panel = (title: string) => wrapper.findAll('.workspace-form-panel').find(item => item.get('h2').text() === title)
    await panel(en.workspace.settings)!.get('input').setValue('Unsaved Workspace name')
    await wrapper.findAll('button').find(button => button.text() === en.workspace.createWorkspace)!.trigger('click')
    await panel(en.workspace.createWorkspace)!.get('input').setValue('Unsaved new Workspace')
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect((panel(en.workspace.settings)!.get('input').element as HTMLInputElement).value).toBe('Unsaved Workspace name')
    expect(panel(en.workspace.createWorkspace)).toBeDefined()
    expect((panel(en.workspace.createWorkspace)!.get('input').element as HTMLInputElement).value).toBe('Unsaved new Workspace')
  })

  it('returns to the newest export page and polls a job created from older history', async () => {
    vi.useFakeTimers()
    let created = false
    const pending = { ...completedExport, id: 'new-export', state: 'pending', progress: 0, expires_at: null }
    lifecycle.listExports.mockImplementation((_id, page) => Promise.resolve({ items: page === 1 && created ? [pending] : [{ ...completedExport, id: `history-${page}` }], total: 12, page, page_size: 10, pages: 2 }))
    lifecycle.createExport.mockImplementation(() => { created = true; return Promise.resolve(pending) })
    const { wrapper } = await render()
    await wrapper.findAll('button').find(button => button.text() === en.workspace.lifecycle.next)!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('history-2')
    await wrapper.get('[data-testid="create-export"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="cancel-export-new-export"]').exists()).toBe(true)
    expect(lifecycle.listExports).toHaveBeenLastCalledWith(1, 1, 10, expect.any(AbortSignal))
    const requests = lifecycle.listExports.mock.calls.length
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(lifecycle.listExports).toHaveBeenCalledTimes(requests + 1)
  })

  it('still applies server permission revocation during background polling', async () => {
    vi.useFakeTimers()
    let revoked = false
    workspace.getWorkspace.mockImplementation(() => Promise.resolve({ id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: revoked ? ['lifecycle.read'] : [...ownerPermissions] }))
    lifecycle.listExports.mockResolvedValue({ items: [{ ...completedExport, state: 'running' }], total: 1, page: 1, page_size: 10, pages: 1 })
    const { wrapper, store } = await render()
    expect(wrapper.get('[data-testid="create-export"]').exists()).toBe(true)
    revoked = true
    await vi.advanceTimersByTimeAsync(15000)
    await flushPromises()
    expect(store.permissions).toEqual(['lifecycle.read'])
    expect(wrapper.find('[data-testid="create-export"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="request-deletion"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="retention-save-operational"]').exists()).toBe(false)
  })
})
