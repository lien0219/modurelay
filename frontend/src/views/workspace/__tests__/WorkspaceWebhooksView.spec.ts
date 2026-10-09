import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import WorkspaceWebhooksView from '../WorkspaceWebhooksView.vue'
import { useWorkspaceStore } from '@/stores/workspace'
import { useAuthStore } from '@/stores/auth'
import { webhooksAPI, type WorkspaceWebhook, type WorkspaceWebhookDelivery } from '@/api/webhooks'
import { createTestI18n } from '@/__tests__/utils/i18n'
import workspaceEn from '@/i18n/locales/en/workspace'
import workspaceZh from '@/i18n/locales/zh/workspace'
import webhookEn from '@/i18n/locales/en/workspaceWebhooks'
import webhookZh from '@/i18n/locales/zh/workspaceWebhooks'
import commonEn from '@/i18n/locales/en/common'
import commonZh from '@/i18n/locales/zh/common'

vi.mock('@/api/webhooks', () => ({ webhooksAPI: {
  list: vi.fn(), create: vi.fn(), update: vi.fn(), remove: vi.fn(), rotateSecret: vi.fn(), test: vi.fn(), listDeliveries: vi.fn(), retry: vi.fn(),
} }))
vi.mock('@/components/layout/AppLayout.vue', () => ({ default: { template: '<main><slot /></main>' } }))
const toast = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/stores/app', () => ({ useAppStore: () => toast }))

enableAutoUnmount(afterEach)
afterEach(() => vi.restoreAllMocks())

const permissions = ['webhook.read', 'webhook.create', 'webhook.update', 'webhook.delete', 'webhook.secret.rotate', 'webhook.test', 'webhook.delivery.read', 'webhook.delivery.retry']
const webhook: WorkspaceWebhook = { id: 1, workspace_id: 7, name: 'Deploy', url: 'https://example.com/hook', enabled: true, event_types: ['project.created'], created_at: '2026-10-05T00:00:00Z', updated_at: '2026-10-05T00:00:00Z' }
const delivery: WorkspaceWebhookDelivery = { id: 10, webhook_id: 1, event_id: 'evt_test', event_type: 'project.created', status: 'dead', attempts: 3, response_status: 500, response_preview: '<b>provider response</b>', last_error: 'HTTP 500', created_at: '2026-10-05T00:00:00Z' }
const deliveryPage = (items: WorkspaceWebhookDelivery[], page = 1, pages = 1) => ({ items, total: items.length, page, page_size: 20, pages })
const commonKeys = ['loading', 'saving', 'cancel', 'actions', 'edit', 'refresh', 'retry', 'copied', 'copyFailed', 'enabled', 'disabled', 'close', 'back', 'next'] as const
const messages = {
  en: { workspace: { ...workspaceEn.workspace, ...webhookEn.workspace }, common: Object.fromEntries(commonKeys.map(key => [key, commonEn.common[key]])) },
  zh: { workspace: { ...workspaceZh.workspace, ...webhookZh.workspace }, common: Object.fromEntries(commonKeys.map(key => [key, commonZh.common[key]])) },
}

async function render(allowed = permissions, locale = 'en') {
  const store = useWorkspaceStore()
  store.$patch({ workspaces: [7, 8].map(id => ({ id, name: `Workspace ${id}`, slug: `workspace-${id}`, type: 'organization', status: 'active', owner_user_id: 1, billing_owner_user_id: 1, permissions: allowed })), selectedWorkspaceId: 7, permissions: [...allowed] })
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/workspaces/7/webhooks')
  const i18n = createTestI18n(messages, locale)
  const wrapper = mount(WorkspaceWebhooksView, { global: { plugins: [router, i18n] } })
  await flushPromises()
  return { wrapper, store, router, i18n }
}

async function createForm(wrapper: VueWrapper) {
  await wrapper.get('[data-testid="webhook-create-toggle"]').trigger('click')
  await wrapper.get('input[name="webhook-name"]').setValue('Build')
  await wrapper.get('input[name="webhook-url"]').setValue('https://example.com/build')
  await wrapper.get('input[name="webhook-events"][value="project.created"]').setValue(true)
}

describe('WorkspaceWebhooksView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.resetAllMocks()
    useAuthStore().$patch({ user: { id: 1, username: 'Ari', email: 'ari@example.com', role: 'user', balance: 0, concurrency: 1, status: 'active', allowed_groups: null, balance_notify_enabled: false, balance_notify_threshold: null, balance_notify_extra_emails: [], created_at: '', updated_at: '' } })
    vi.mocked(webhooksAPI.list).mockResolvedValue([{ ...webhook }])
    vi.mocked(webhooksAPI.listDeliveries).mockResolvedValue(deliveryPage([]))
  })

  it('lets an endpoint explicitly subscribe to administrator retry events', async () => {
    vi.mocked(webhooksAPI.create).mockResolvedValue({ webhook: { ...webhook, id: 2, event_types: ['webhook.administrator_retried'] }, secret: 'whsec_once' })
    const { wrapper } = await render()
    await wrapper.get('[data-testid="webhook-create-toggle"]').trigger('click')
    await wrapper.get('input[name="webhook-name"]').setValue('Retry observer')
    await wrapper.get('input[name="webhook-url"]').setValue('https://example.com/retry')
    await wrapper.get('input[name="webhook-events"][value="webhook.administrator_retried"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(webhooksAPI.create).toHaveBeenCalledWith(7, { name: 'Retry observer', url: 'https://example.com/retry', event_types: ['webhook.administrator_retried'] })
  })

  it('renders actual Chinese copy and shows and clears a newly created secret', async () => {
    vi.mocked(webhooksAPI.create).mockResolvedValue({ webhook: { ...webhook, id: 2 }, secret: 'whsec_once' })
    const { wrapper } = await render(permissions, 'zh')
    expect(wrapper.text()).toContain('创建 Webhook')
    expect(wrapper.text()).not.toContain('workspace.createWebhook')
    await createForm(wrapper)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(webhooksAPI.create).toHaveBeenCalledWith(7, { name: 'Build', url: 'https://example.com/build', event_types: ['project.created'] })
    expect(wrapper.get('[data-testid="webhook-secret"]').text()).toContain('whsec_once')
    await wrapper.get('[data-testid="webhook-secret-dismiss"]').trigger('click')
    expect(wrapper.text()).not.toContain('whsec_once')
  })

  it('does not load or show the webhook tab without read permission', async () => {
    const { wrapper } = await render([])
    expect(webhooksAPI.list).not.toHaveBeenCalled()
    expect(webhooksAPI.listDeliveries).not.toHaveBeenCalled()
    expect(wrapper.find('a[href="/workspaces/7/webhooks"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('You do not have access to webhooks in this workspace.')
  })

  it('keeps mutation actions hidden for a reader', async () => {
    const { wrapper } = await render(['webhook.read'])
    expect(wrapper.text()).toContain('Deploy')
    expect(webhooksAPI.listDeliveries).not.toHaveBeenCalled()
    expect(wrapper.find('select[aria-label="Webhook deliveries"]').exists()).toBe(false)
    for (const action of ['create-toggle', 'edit-1', 'rotate-1', 'test-1', 'delete-1']) expect(wrapper.find(`[data-testid="webhook-${action}"]`).exists()).toBe(false)
  })

  it('lets a delivery reader inspect records without retry or endpoint mutation permissions', async () => {
    vi.mocked(webhooksAPI.listDeliveries).mockResolvedValue(deliveryPage([delivery]))
    const { wrapper } = await render(['webhook.read', 'webhook.delivery.read'])
    expect(wrapper.text()).toContain('evt_test')
    expect(wrapper.find('[data-testid="webhook-retry-10"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="webhook-rotate-1"]').exists()).toBe(false)
  })

  it('ignores a list response from the previous workspace', async () => {
    let resolveList!: (result: WorkspaceWebhook[]) => void
    vi.mocked(webhooksAPI.list).mockImplementationOnce(() => new Promise(resolve => { resolveList = resolve }))
    const { wrapper, store } = await render()
    store.selectedWorkspaceId = 8
    await flushPromises()
    resolveList([{ ...webhook, name: 'Previous workspace endpoint' }])
    await flushPromises()
    expect(wrapper.text()).not.toContain('Previous workspace endpoint')
    expect(wrapper.text()).toContain('Deploy')
  })

  it('closes an open form when create permission is revoked', async () => {
    const { wrapper, store } = await render()
    await createForm(wrapper)
    store.permissions = ['webhook.read']
    await flushPromises()
    expect(wrapper.find('form').exists()).toBe(false)
    expect(webhooksAPI.create).not.toHaveBeenCalled()
  })

  it('ignores a late create secret after switching workspaces', async () => {
    let resolveCreate!: (result: Awaited<ReturnType<typeof webhooksAPI.create>>) => void
    vi.mocked(webhooksAPI.create).mockImplementationOnce(() => new Promise(resolve => { resolveCreate = resolve }))
    const { wrapper, store } = await render()
    await createForm(wrapper)
    await wrapper.get('form').trigger('submit')
    store.selectedWorkspaceId = 8
    await flushPromises()
    resolveCreate({ webhook: { ...webhook, id: 2 }, secret: 'whsec_previous_workspace' })
    await flushPromises()
    expect(wrapper.text()).not.toContain('whsec_previous_workspace')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(toast.showSuccess).not.toHaveBeenCalled()
  })

  it('ignores a late rotated secret after the user changes and prevents duplicate rotation', async () => {
    let resolveRotate!: (result: Awaited<ReturnType<typeof webhooksAPI.rotateSecret>>) => void
    vi.mocked(webhooksAPI.rotateSecret).mockImplementationOnce(() => new Promise(resolve => { resolveRotate = resolve }))
    const { wrapper } = await render()
    await wrapper.get('[data-testid="webhook-rotate-1"]').trigger('click')
    await wrapper.get('[data-testid="webhook-rotate-1"]').trigger('click')
    expect(webhooksAPI.rotateSecret).toHaveBeenCalledTimes(1)
    const auth = useAuthStore()
    auth.$patch({ user: { ...auth.user!, id: 2 } })
    await flushPromises()
    resolveRotate({ webhook, secret: 'whsec_previous_user' })
    await flushPromises()
    expect(wrapper.text()).not.toContain('whsec_previous_user')
    expect(toast.showSuccess).not.toHaveBeenCalled()
  })

  it('rejects a non-HTTPS URL with an inline field error', async () => {
    const { wrapper } = await render()
    await createForm(wrapper)
    await wrapper.get('input[name="webhook-url"]').setValue('http://example.com/hook')
    await wrapper.get('form').trigger('submit')
    expect(webhooksAPI.create).not.toHaveBeenCalled()
    expect(wrapper.get('#webhook-url-error').text()).toContain('HTTPS')
    expect(wrapper.get('input[name="webhook-url"]').attributes('aria-describedby')).toContain('webhook-url-error')
  })

  it('updates an existing webhook and accurately labels a disabled endpoint', async () => {
    vi.mocked(webhooksAPI.list).mockResolvedValue([{ ...webhook, enabled: false }])
    vi.mocked(webhooksAPI.update).mockResolvedValue({ ...webhook, name: 'Updated', enabled: true })
    const { wrapper } = await render()
    expect(wrapper.text()).toContain('Disabled')
    await wrapper.get('[data-testid="webhook-edit-1"]').trigger('click')
    await wrapper.get('input[name="webhook-name"]').setValue('Updated')
    await wrapper.get('input[name="webhook-enabled"]').setValue(true)
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    expect(webhooksAPI.update).toHaveBeenCalledWith(7, 1, { name: 'Updated', url: webhook.url, event_types: ['project.created'], enabled: true })
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('loads the selected webhook once and lets another endpoint show its deliveries', async () => {
    vi.mocked(webhooksAPI.list).mockResolvedValue([webhook, { ...webhook, id: 2, name: 'Build' }])
    vi.mocked(webhooksAPI.listDeliveries).mockImplementation(async (_workspace, id) => deliveryPage([{ ...delivery, webhook_id: id, event_id: `evt_${id}` }]))
    const { wrapper } = await render()
    expect(webhooksAPI.listDeliveries).toHaveBeenCalledTimes(1)
    await wrapper.get('select[aria-label="Webhook deliveries"]').setValue('2')
    await flushPromises()
    expect(wrapper.text()).toContain('evt_2')
    expect(wrapper.text()).not.toContain('evt_1')
    expect(wrapper.get('details pre').text()).toBe('<b>provider response</b>')
    expect(wrapper.find('details b').exists()).toBe(false)
    expect(wrapper.find('time[datetime="2026-10-05T00:00:00Z"]').exists()).toBe(true)
  })

  it('ignores a delivery response from a previously selected endpoint', async () => {
    let resolveDeliveries!: (result: ReturnType<typeof deliveryPage>) => void
    vi.mocked(webhooksAPI.list).mockResolvedValue([webhook, { ...webhook, id: 2, name: 'Build' }])
    vi.mocked(webhooksAPI.listDeliveries).mockImplementationOnce(() => new Promise(resolve => { resolveDeliveries = resolve }))
    vi.mocked(webhooksAPI.listDeliveries).mockResolvedValue(deliveryPage([{ ...delivery, webhook_id: 2, event_id: 'evt_current' }]))
    const { wrapper } = await render()
    await wrapper.get('select[aria-label="Webhook deliveries"]').setValue('2')
    await flushPromises()
    resolveDeliveries(deliveryPage([{ ...delivery, event_id: 'evt_previous' }]))
    await flushPromises()
    expect(wrapper.text()).toContain('evt_current')
    expect(wrapper.text()).not.toContain('evt_previous')
  })

  it('sends a test delivery without requiring delivery-read permission', async () => {
    vi.mocked(webhooksAPI.test).mockResolvedValue({ ...delivery, status: 'pending' })
    const { wrapper } = await render(['webhook.read', 'webhook.test'])
    await wrapper.get('[data-testid="webhook-test-1"]').trigger('click')
    await flushPromises()
    expect(webhooksAPI.test).toHaveBeenCalledWith(7, 1)
    expect(webhooksAPI.listDeliveries).not.toHaveBeenCalled()
    expect(toast.showSuccess).toHaveBeenCalledWith('Test delivery queued.')
  })

  it('retries a delivery with its workspace and endpoint and refreshes its status', async () => {
    vi.mocked(webhooksAPI.listDeliveries).mockResolvedValueOnce(deliveryPage([delivery])).mockResolvedValue(deliveryPage([{ ...delivery, status: 'pending' }]))
    vi.mocked(webhooksAPI.retry).mockResolvedValue({ ...delivery, status: 'pending' })
    const { wrapper } = await render()
    await wrapper.get('[data-testid="webhook-retry-10"]').trigger('click')
    await flushPromises()
    expect(webhooksAPI.retry).toHaveBeenCalledWith(7, 1, 10)
    expect(wrapper.text()).toContain('Pending')
    expect(wrapper.find('[data-testid="webhook-retry-10"]').exists()).toBe(false)
  })

  it('deletes a confirmed endpoint and refreshes the list', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    vi.mocked(webhooksAPI.list).mockResolvedValueOnce([webhook]).mockResolvedValue([])
    const { wrapper } = await render()
    await wrapper.get('[data-testid="webhook-delete-1"]').trigger('click')
    await flushPromises()
    expect(webhooksAPI.remove).toHaveBeenCalledWith(7, 1)
    expect(wrapper.text()).toContain('No webhooks configured.')
    expect(wrapper.find('select[aria-label="Webhook deliveries"]').exists()).toBe(false)
  })

  it('paginates deliveries and retries only terminal records', async () => {
    vi.mocked(webhooksAPI.listDeliveries).mockImplementation(async (_workspace, _id, query) => deliveryPage([{ ...delivery, id: query?.page === 2 ? 11 : 10, status: query?.page === 2 ? 'pending' : 'dead' }], query?.page || 1, 2))
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="webhook-retry-10"]').exists()).toBe(true)
    await wrapper.get('[data-testid="webhook-deliveries-next"]').trigger('click')
    await flushPromises()
    expect(webhooksAPI.listDeliveries).toHaveBeenLastCalledWith(7, 1, expect.objectContaining({ page: 2 }))
    expect(wrapper.find('[data-testid="webhook-retry-11"]').exists()).toBe(false)
  })

  it('does not offer manual retry for a successful delivery', async () => {
    vi.mocked(webhooksAPI.listDeliveries).mockResolvedValue(deliveryPage([{ ...delivery, status: 'succeeded' }]))
    const { wrapper } = await render()
    expect(wrapper.find('[data-testid="webhook-retry-10"]').exists()).toBe(false)
  })

  it('shows a load error with retry instead of a successful empty list', async () => {
    vi.mocked(webhooksAPI.list).mockRejectedValueOnce(new Error('offline'))
    const { wrapper } = await render()
    expect(wrapper.get('[role="alert"]').text()).toContain('Webhooks could not be loaded.')
    expect(wrapper.text()).not.toContain('No webhooks configured.')
    await wrapper.get('[data-testid="webhook-list-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Deploy')
  })
})
