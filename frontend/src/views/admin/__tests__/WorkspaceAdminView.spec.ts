import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en'
import zh from '@/i18n/locales/zh'
import WorkspaceAdminView from '../WorkspaceAdminView.vue'

const mocks = vi.hoisted(() => ({ adminList: vi.fn(), adminInspect: vi.fn(), adminSetStatus: vi.fn(), overview: vi.fn(), diagnostics: vi.fn(), jobs: vi.fn(), retryWebhook: vi.fn(), upgradeSessionMFA: vi.fn(), stepUp: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: mocks }))
vi.mock('@/api/enterpriseAdmin', () => ({ enterpriseAdminAPI: mocks }))
vi.mock('@/api', () => ({ totpAPI: mocks }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('@/stores', () => ({ useAppStore: () => mocks }))
const auth = reactive({ user: { id: 7, role: 'admin', totp_enabled: true }, token: 'token-a', isAdmin: true, adoptStoredSessionTokens: (tokens: { access_token: string }) => { auth.token = tokens.access_token } })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))

const at = '2026-10-09T00:00:00.123456789Z'
const workspace = (id: number) => ({ id, name: `Tenant ${id}`, slug: `tenant-${id}`, type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions: [], updated_at: at })
const count = (value: number | null, capped = false, freshness = 'fresh') => ({ value, available: value !== null, capped, freshness, observed_at: at, source: 'bounded_sql', coverage: 'inventory' })
const detail = (id: number) => ({ state: 'healthy', workspace: workspace(id), observed_at: at, counts: { members: count(3), projects: count(0, true), async_video: count(null) }, owner_valid: null, billing_owner_valid: true, allowed_operations: ['suspend'], security_policy: null, retention: [], recent_audit: [], identity: null, purge: null, jobs: null, issues: [] })
const deferred = <T,>() => { let resolve!: (value: T) => void; let reject!: (error: unknown) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no }); return { promise, resolve, reject } }
let wrapper: VueWrapper | undefined
async function render(locale = 'en') { wrapper = mount(WorkspaceAdminView, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale, messages: { en, zh } })] } }); await flushPromises(); return wrapper }
async function inspect(id: number) { await wrapper!.find(`[data-testid="inspect-${id}"]`).trigger('click'); await flushPromises() }
async function confirm() { await wrapper!.find('[name="operation-reason"]').setValue('Incident response'); await wrapper!.find('[name="operation-confirmation"]').setValue('suspend:1'); await wrapper!.find('[data-testid="admin-confirm-form"]').trigger('submit'); await flushPromises() }
beforeEach(() => {
  vi.resetAllMocks(); localStorage.clear(); auth.user = { id: 7, role: 'admin', totp_enabled: true }; auth.token = 'token-a'; auth.isAdmin = true
  localStorage.setItem('auth_token', 'token-a'); localStorage.setItem('auth_user', JSON.stringify({ id: 7 })); localStorage.setItem('refresh_token', 'refresh-a')
  mocks.adminList.mockResolvedValue({ items: [workspace(1), workspace(2)], page: 1, page_size: 20, pages: 3, total: 41, total_capped: false, observed_at: at })
  mocks.overview.mockResolvedValue({ state: 'healthy', observed_at: at, counts: { workspaces_active: count(2) }, worker_exceptions: count(0), unknown_worker_liveness: count(9), identity: null, issues: [] })
  mocks.jobs.mockResolvedValue({ state: 'unknown', observed_at: at, workers: [], webhook_failure_trend: [], export_rotation: null, issues: [] })
  mocks.diagnostics.mockImplementation(async (id: number) => detail(id))
  mocks.adminSetStatus.mockResolvedValue({ ...workspace(1), status: 'suspended' })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; document.body.innerHTML = '' })

describe('Global Admin diagnostic console', () => {
  it('uses labelled filters and stable server pagination', async () => {
    await render(); await wrapper!.find('[name="name_prefix"]').setValue('acme'); await wrapper!.find('[name="workspace_id"]').setValue('42'); await wrapper!.find('[name="owner_user_id"]').setValue('7'); await wrapper!.find('[name="billing_owner_user_id"]').setValue('8'); await wrapper!.find('[name="type"]').setValue('organization'); await wrapper!.find('[name="created_from"]').setValue('2026-10-01T00:00'); await wrapper!.find('[name="created_to"]').setValue('2026-10-09T00:00'); await wrapper!.find('[name="updated_from"]').setValue('2026-10-02T00:00'); await wrapper!.find('[name="updated_to"]').setValue('2026-10-09T00:00'); await wrapper!.find('[name="sort"]').setValue('updated_at'); await wrapper!.find('[data-testid="admin-search-form"]').trigger('submit'); await flushPromises()
    expect(mocks.adminList.mock.calls.at(-1)?.[0]).toMatchObject({ name_prefix: 'acme', workspace_id: 42, owner_user_id: 7, billing_owner_user_id: 8, type: 'organization', created_from: new Date('2026-10-01T00:00').toISOString(), created_to: new Date('2026-10-09T00:00').toISOString(), updated_from: new Date('2026-10-02T00:00').toISOString(), updated_to: new Date('2026-10-09T00:00').toISOString(), sort: 'updated_at', page: 1, page_size: 20 })
    await wrapper!.find('[data-testid="admin-next-page"]').trigger('click'); await flushPromises()
    expect(mocks.adminList.mock.calls.at(-1)?.[0].page).toBe(2)
    expect(wrapper!.find('label[for="admin-name_prefix"]').exists()).toBe(true)
  })
  it('renders the worker evidence fields and bounded job blockers', async () => {
    mocks.diagnostics.mockResolvedValueOnce({
      ...detail(1),
      jobs: {
        state: 'warning', observed_at: at, webhook_failure_trend: [], export_rotation: null, issues: [], workers: [{
          worker: 'export', state: 'warning', liveness: 'unknown', observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh', counts: {},
          due_lag_seconds: 12, stored_scan_lag_seconds: 9,
          oldest_pending: { value: at, age_seconds: 60, available: true, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          pending_alert_age: { value: at, age_seconds: 30, available: true, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          last_failure: { code: 'EXPORT_FAILED', source_time: at, available: true, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          latest_failure_time: { value: at, age_seconds: 5, available: true, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          last_successful_job_at: null, last_successful_scan_at: null, last_failed_at: at,
          jobs: [{ id: 'job-1', workspace_id: 1, state: 'blocked', created_at: at, finished_at: null, failed_at: at, failure_code: 'HOLD_ACTIVE', blockers: [{ code: 'HOLD_ACTIVE', reason: 'Protected hold', observed_at: at, source_time: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh', runbook: 'hold-runbook' }], source_time: at, observed_at: at, freshness: 'fresh' }],
          jobs_capped: false, issues: [{ code: 'LIVENESS_UNAVAILABLE', severity: 'warning', scope: 'export', observed_at: at, reason: 'No heartbeat', recommended_action: 'Use monitor', runbook: 'worker-runbook' }], runbook: 'worker-runbook'
        }]
      }
    })
    await render(); await inspect(1)
    const text = wrapper!.find('[data-testid="workspace-detail"]').text()
    expect(text).toContain('60'); expect(text).toContain('30'); expect(text).toContain('9'); expect(text).toContain('EXPORT_FAILED'); expect(text).toContain('HOLD_ACTIVE'); expect(text).toContain('Protected hold'); expect(text).toContain('worker-runbook')
  })
  it('keeps independent partial failures, unknown payer evidence and capped zero distinct from empty', async () => {
    mocks.overview.mockRejectedValue({ message: 'raw secret' }); await render(); await inspect(1)
    expect(wrapper!.find('[data-testid="overview-error"]').exists()).toBe(true)
    expect(wrapper!.find('[data-testid="workspace-detail"]').text()).toContain('At least 0')
    expect(wrapper!.find('[data-testid="workspace-detail"]').text()).toContain('Unknown')
    expect(wrapper!.text()).not.toContain('raw secret'); expect(wrapper!.find('pre').exists()).toBe(false)
  })
  it('fences A to B to A reads and obsolete errors', async () => {
    const a = deferred<ReturnType<typeof detail>>(); const b = deferred<ReturnType<typeof detail>>(); const a2 = deferred<ReturnType<typeof detail>>()
    mocks.diagnostics.mockReturnValueOnce(a.promise).mockReturnValueOnce(b.promise).mockReturnValueOnce(a2.promise)
    await render(); await inspect(1); await inspect(2); await inspect(1); a2.resolve(detail(1)); await flushPromises(); b.resolve(detail(2)); a.reject({ message: 'obsolete secret' }); await flushPromises()
    expect(wrapper!.find('[data-testid="workspace-detail"]').text()).toContain('Tenant 1'); expect(wrapper!.find('[data-testid="workspace-detail"]').text()).not.toContain('Tenant 2'); expect(mocks.showError).not.toHaveBeenCalled()
    expect(mocks.diagnostics.mock.calls[0][1].aborted).toBe(true)
  })
  it('hides workspace A actions while workspace B evidence is loading', async () => {
    const pendingB = deferred<ReturnType<typeof detail>>()
    mocks.diagnostics.mockImplementation(async (id: number) => id === 2 ? pendingB.promise : detail(id))
    await render(); await inspect(1)
    await wrapper!.find('[data-testid="inspect-2"]').trigger('click'); await flushPromises()
    expect(wrapper!.find('[data-testid="workspace-detail"]').exists()).toBe(false)
    expect(wrapper!.find('[data-testid="operation-suspend"]').exists()).toBe(false)
    pendingB.resolve(detail(2)); await flushPromises()
    expect(wrapper!.find('[data-testid="workspace-detail"]').text()).toContain('Tenant 2')
  })
  it('disables stale last-observed actions after a refresh failure', async () => {
    await render(); await inspect(1); mocks.diagnostics.mockRejectedValueOnce({ message: 'unsafe' }); await wrapper!.find('[data-testid="detail-refresh"]').trigger('click'); await flushPromises()
    expect(wrapper!.find('[data-testid="workspace-detail"]').text()).toContain('Last observed'); expect(wrapper!.find('[data-testid="operation-suspend"]').attributes()).toHaveProperty('disabled')
  })
  it('captures one UUID, exact version and reason, locks duplicate mutation and invalidates old reads', async () => {
    const pending = deferred<unknown>(); mocks.adminSetStatus.mockReturnValueOnce(pending.promise)
    await render(); await inspect(1); await wrapper!.find('[data-testid="operation-suspend"]').trigger('click'); await confirm(); await wrapper!.find('[data-testid="admin-confirm-form"]').trigger('submit'); await flushPromises()
    expect(mocks.adminSetStatus).toHaveBeenCalledTimes(1)
    const [id, body, config] = mocks.adminSetStatus.mock.calls[0]
    expect(id).toBe(1); expect(body).toMatchObject({ action: 'suspend', reason: 'Incident response', confirmation: 'suspend:1', expected_updated_at: at }); expect(body.idempotency_key).toMatch(/^[a-f0-9-]{36}$/); expect(config).toMatchObject({ preserveAuthSessionOnFailure: true, sessionProofAccessToken: 'token-a' })
    pending.resolve({ ...workspace(1), status: 'suspended' }); await flushPromises(); expect(mocks.showSuccess).toHaveBeenCalledTimes(1)
  })
  it('runs only server-eligible dead webhook retries with the captured delivery token', async () => {
    mocks.diagnostics.mockResolvedValueOnce({
      ...detail(1),
      jobs: {
        state: 'warning',
        observed_at: at,
        webhook_failure_trend: [],
        export_rotation: null,
        issues: [],
        workers: [{
          worker: 'webhook', state: 'warning', liveness: 'unknown', observed_at: at,
          source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh', counts: {},
          due_lag_seconds: 4, stored_scan_lag_seconds: null,
          oldest_pending: { value: null, age_seconds: null, available: false, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          pending_alert_age: { value: null, age_seconds: null, available: false, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          last_failure: { code: 'DELIVERY_FAILED', source_time: at, available: true, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          latest_failure_time: { value: at, age_seconds: 5, available: true, observed_at: at, source: 'bounded_sql', coverage: 'workspace', freshness: 'fresh' },
          last_successful_job_at: null, last_successful_scan_at: null, last_failed_at: at,
          jobs: [{ id: 'job-1', workspace_id: 1, state: 'dead', created_at: at, finished_at: null, failed_at: at, failure_code: 'DELIVERY_FAILED', webhook_id: 9, delivery_id: 10, attempts: 2, last_attempt_at: at, endpoint_enabled: true, retry_eligibility: 'eligible_requires_guarded_action', observed_at: at, freshness: 'fresh' }],
          jobs_capped: false, issues: [], runbook: 'webhook-retry'
        }]
      }
    })
    mocks.retryWebhook.mockResolvedValue({ id: 'op-1', action: 'retry_webhook', target_type: 'webhook_delivery', target_id: 10, workspace_id: 1, previous_status: 'dead', result_status: 'retrying', result_updated_at: at, created_at: at })
    await render(); await inspect(1)
    await wrapper!.find('[data-testid="retry-10"]').trigger('click')
    await wrapper!.find('[name="operation-reason"]').setValue('Retry after verified endpoint recovery')
    await wrapper!.find('[name="operation-confirmation"]').setValue('retry_webhook:10')
    await wrapper!.find('[data-testid="admin-confirm-form"]').trigger('submit'); await flushPromises()
    expect(mocks.retryWebhook).toHaveBeenCalledTimes(1)
    const [workspaceId, webhookId, deliveryId, body, config] = mocks.retryWebhook.mock.calls[0]
    expect([workspaceId, webhookId, deliveryId]).toEqual([1, 9, 10])
    expect(body).toMatchObject({ action: 'retry_webhook', reason: 'Retry after verified endpoint recovery', confirmation: 'retry_webhook:10', expected_attempts: 2, expected_last_attempt_at: at })
    expect(body.idempotency_key).toMatch(/^[a-f0-9-]{36}$/)
    expect(config).toMatchObject({ preserveAuthSessionOnFailure: true, sessionProofAccessToken: 'token-a' })
  })
  it('clears sensitive state and cancels pending requests when authorization is revoked', async () => {
    await render(); await inspect(1); auth.user.role = 'user'; auth.isAdmin = false; await flushPromises()
    expect(wrapper!.find('[data-testid="workspace-detail"]').exists()).toBe(false); expect(wrapper!.text()).not.toContain('Tenant 1'); expect(wrapper!.text()).not.toContain('Tenant 2'); expect(mocks.diagnostics.mock.calls[0][1].aborted).toBe(true)
  })
  it('clears sensitive state when the current admin session token changes', async () => {
    await render(); await inspect(1)
    localStorage.setItem('auth_token', 'token-b'); auth.token = 'token-b'; await flushPromises()
    expect(wrapper!.find('[data-testid="workspace-detail"]').exists()).toBe(false); expect(wrapper!.text()).not.toContain('Tenant 1'); expect(mocks.diagnostics.mock.calls[0][1].aborted).toBe(true)
  })
  it('cancels overview and jobs requests on authorization changes', async () => {
    const overviewPending = deferred<unknown>(); const jobsPending = deferred<unknown>(); mocks.overview.mockReturnValueOnce(overviewPending.promise); mocks.jobs.mockReturnValueOnce(jobsPending.promise)
    await render(); const overviewSignal = mocks.overview.mock.calls[0][0] as AbortSignal; const jobsSignal = mocks.jobs.mock.calls[0][0] as AbortSignal; auth.isAdmin = false; await flushPromises()
    expect(overviewSignal.aborted).toBe(true); expect(jobsSignal.aborted).toBe(true)
  })
  it('keeps confirmation and proof keyboard focus contained and returns it to the trigger', async () => {
    await render(); await inspect(1); const trigger = wrapper!.find('[data-testid="operation-suspend"]'); (trigger.element as HTMLElement).focus(); await trigger.trigger('click'); await flushPromises()
    const dialog = wrapper!.find('[data-testid="admin-operation-dialog"]'); const buttons = dialog.findAll('button'); (buttons.at(-1)!.element as HTMLElement).focus(); await buttons.at(-1)!.trigger('keydown', { key: 'Tab' }); expect(dialog.element.contains(document.activeElement)).toBe(true)
    await dialog.trigger('keydown', { key: 'Escape' }); await flushPromises(); expect(document.activeElement).toBe(trigger.element)
  })
  it('has Chinese labels and fixed localized operation errors', async () => {
    mocks.adminSetStatus.mockRejectedValue({ code: 'WORKSPACE_CONFLICT', message: 'SQL password' }); await render('zh'); await inspect(1); await wrapper!.find('[data-testid="operation-suspend"]').trigger('click'); await confirm()
    expect(wrapper!.text()).not.toContain('SQL password'); expect(mocks.showError.mock.calls[0][0]).not.toContain('WORKSPACE_CONFLICT')
  })
})
