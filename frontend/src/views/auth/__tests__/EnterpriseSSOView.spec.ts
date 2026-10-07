import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive } from 'vue'
import EnterpriseSSOView from '../EnterpriseSSOView.vue'

const mock = vi.hoisted(() => ({ discover: vi.fn(), startLink: vi.fn(), recover: vi.fn(), startSSO: vi.fn(), getIdentityProvider: vi.fn(), getStatus: vi.fn(), loadWorkspaces: vi.fn(), replace: vi.fn(), showSuccess: vi.fn() }))
const route = reactive<{ query: Record<string, string> }>({ query: {} })
const auth = reactive({ isAuthenticated: false, user: null as null | { id: number; email: string } })
const workspaces = reactive({ workspaces: [] as Array<{ id: number; type: string; permissions: string[] }>, loadWorkspaces: mock.loadWorkspaces })
const provider = { workspace_id: 7, provider_id: 9, name: 'Enterprise Provider', is_default: true }
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace: mock.replace }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores', () => ({ useAuthStore: () => auth, useAppStore: () => ({ showSuccess: mock.showSuccess }) }))
vi.mock('@/stores/workspace', () => ({ useWorkspaceStore: () => workspaces }))
vi.mock('@/api/enterpriseSSO', () => ({ enterpriseSSOAPI: mock }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: mock }))
vi.mock('@/api/totp', () => ({ totpAPI: mock }))
let wrapper: VueWrapper | undefined
let locationBefore: Location
const assign = vi.fn()

async function render() {
  wrapper = mount(EnterpriseSSOView, { global: { stubs: { AuthLayout: { template: '<div><slot /></div>' }, RouterLink: { template: '<a><slot /></a>' } } } })
  await flushPromises()
  return wrapper
}

describe('EnterpriseSSOView', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    route.query = {}
    auth.isAuthenticated = false
    auth.user = null
    workspaces.workspaces = []
    mock.discover.mockResolvedValue({ enterprise_sso_available: true, providers: [provider, { ...provider, workspace_id: 8, provider_id: 10 }] })
    mock.startLink.mockResolvedValue({ authorization_url: 'https://id.example.com/authorize' })
    mock.startSSO.mockResolvedValue({ authorization_url: 'https://id.example.com/authorize' })
    mock.getStatus.mockResolvedValue({ enabled: false })
    locationBefore = window.location
    Object.defineProperty(window, 'location', { configurable: true, value: { ...locationBefore, assign } })
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; Object.defineProperty(window, 'location', { configurable: true, value: locationBefore }) })

  it('discovers only the requested workspace and keeps linking and recovery unavailable to guests', async () => {
    route.query = { workspace_id: '7', required: '1' }
    const view = await render()
    await view.find('#sso-work-email').setValue('person@example.com')
    await view.find('[data-testid="sso-discovery-form"]').trigger('submit')
    await flushPromises()
    expect(mock.discover).toHaveBeenCalledWith('person@example.com', expect.any(AbortSignal))
    expect(view.findAll('.sso-provider')).toHaveLength(1)
    expect(view.find('[data-testid="sso-prepare-link"]').exists()).toBe(false)
    expect(view.find('[data-testid="sso-recovery-open"]').exists()).toBe(false)
    await view.find('.sso-provider .btn-primary').trigger('click')
    await flushPromises()
    expect(mock.startSSO).toHaveBeenCalledWith(7, 9, '/workspaces/7/overview')
    expect(mock.startLink).not.toHaveBeenCalled()
    expect(assign).toHaveBeenCalledWith('https://id.example.com/authorize')
  })

  it('requires password and enabled TOTP before explicit linking and clears submitted proof', async () => {
    auth.isAuthenticated = true
    auth.user = { id: 3, email: 'person@example.com' }
    mock.getStatus.mockResolvedValue({ enabled: true })
    const view = await render()
    await view.find('[data-testid="sso-discovery-form"]').trigger('submit')
    await flushPromises()
    await view.find('[data-testid="sso-prepare-link"]').trigger('click')
    await view.find('#sso-link-password').setValue('password-proof')
    await view.find('[data-testid="sso-link-form"]').trigger('submit')
    expect(mock.startLink).not.toHaveBeenCalled()
    await view.find('#sso-link-totp').setValue('123456')
    await view.find('[data-testid="sso-link-form"]').trigger('submit')
    await flushPromises()
    expect(mock.startLink).toHaveBeenCalledWith(7, 9, '/workspaces/7/overview', { password: 'password-proof', totp_code: '123456' })
    expect(mock.startSSO).not.toHaveBeenCalled()
    expect(view.find('#sso-link-password').element).toHaveProperty('value', '')
  })

  it('loads the selected settings provider and opens a proof form without automatically linking', async () => {
    auth.isAuthenticated = true
    auth.user = { id: 3, email: 'person@example.com' }
    route.query = { workspace_id: '7', provider_id: '9', link: '1', return_to: '/workspaces/7/identity' }
    mock.getIdentityProvider.mockResolvedValue({ id: 9, workspace_id: 7, status: 'active', name: provider.name, is_default: true })
    const view = await render()
    expect(mock.getIdentityProvider).toHaveBeenCalledWith(7, 9)
    expect(view.find('[data-testid="sso-link-form"]').exists()).toBe(true)
    expect(mock.startLink).not.toHaveBeenCalled()
  })

  it('shows recovery only for a known owner in an SSO-required or provider-failure context', async () => {
    auth.isAuthenticated = true
    auth.user = { id: 3, email: 'person@example.com' }
    workspaces.workspaces = [{ id: 7, type: 'organization', permissions: ['owner.manage'] }]
    route.query = { workspace_id: '7' }
    const view = await render()
    expect(view.find('[data-testid="sso-recovery-open"]').exists()).toBe(false)
    route.query = { workspace_id: '7', required: '1' }
    await flushPromises()
    expect(view.find('[data-testid="sso-recovery-open"]').exists()).toBe(true)
    workspaces.workspaces[0].permissions = ['identity.manage']
    await flushPromises()
    expect(view.find('[data-testid="sso-recovery-open"]').exists()).toBe(false)
  })

  it('requires confirmation, password, TOTP and a reason to recover workspace access', async () => {
    auth.isAuthenticated = true
    auth.user = { id: 3, email: 'person@example.com' }
    workspaces.workspaces = [{ id: 7, type: 'organization', permissions: ['owner.manage'] }]
    route.query = { workspace_id: '7', required: '1' }
    mock.getStatus.mockResolvedValue({ enabled: true })
    mock.recover.mockResolvedValue({ require_sso: false })
    const view = await render()
    await view.find('[data-testid="sso-recovery-open"]').trigger('click')
    await view.find('#sso-recovery-password').setValue('password-proof')
    await view.find('#sso-recovery-totp').setValue('123456')
    await view.find('#sso-recovery-reason').setValue('The provider is unavailable')
    await view.find('[data-testid="sso-recovery-form"]').trigger('submit')
    expect(mock.recover).not.toHaveBeenCalled()
    await view.find('[data-testid="sso-recovery-form"] input[type="checkbox"]').setValue(true)
    await view.find('[data-testid="sso-recovery-form"]').trigger('submit')
    await flushPromises()
    expect(mock.recover).toHaveBeenCalledWith(7, { password: 'password-proof', totp_code: '123456', reason: 'The provider is unavailable', confirmed: true })
    expect(mock.replace).toHaveBeenCalledWith('/workspaces/7/identity')
    expect(view.find('[data-testid="sso-recovery-form"]').exists()).toBe(false)
  })

  it('clears proofs and ignores a pending linking result after changing tenant context', async () => {
    auth.isAuthenticated = true
    auth.user = { id: 3, email: 'person@example.com' }
    let finish: (value: unknown) => void = () => {}
    mock.startLink.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const view = await render()
    await view.find('[data-testid="sso-discovery-form"]').trigger('submit')
    await flushPromises()
    await view.find('[data-testid="sso-prepare-link"]').trigger('click')
    await view.find('#sso-link-password').setValue('password-proof')
    await view.find('[data-testid="sso-link-form"]').trigger('submit')
    route.query = { workspace_id: '8', required: '1' }
    await flushPromises()
    expect(view.find('[data-testid="sso-link-form"]').exists()).toBe(false)
    finish({ authorization_url: 'https://old-id.example.com/authorize' })
    await flushPromises()
    expect(assign).not.toHaveBeenCalled()
  })
})
