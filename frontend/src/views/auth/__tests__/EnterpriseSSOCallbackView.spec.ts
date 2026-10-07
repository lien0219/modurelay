import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import EnterpriseSSOCallbackView from '../EnterpriseSSOCallbackView.vue'
import { rememberEnterpriseSSONavigation } from '@/utils/enterpriseSSO'

const mock = vi.hoisted(() => ({ exchange: vi.fn(), persistOAuthTokenContext: vi.fn(), setToken: vi.fn(), replace: vi.fn(), showSuccess: vi.fn() }))
const route = { query: {} as Record<string, string> }
vi.mock('vue-router', () => ({ useRoute: () => route, useRouter: () => ({ replace: mock.replace }) }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/enterpriseSSO', () => ({ enterpriseSSOAPI: mock }))
vi.mock('@/api/auth', () => ({ persistOAuthTokenContext: mock.persistOAuthTokenContext }))
vi.mock('@/stores', () => ({ useAuthStore: () => ({ setToken: mock.setToken }), useAppStore: () => ({ showSuccess: mock.showSuccess }) }))
let wrapper: VueWrapper | undefined
const completion = { access_token: 'server-token', refresh_token: 'refresh-token', expires_in: 3600, user: { id: 3 }, workspace_id: 7, provider_id: 9, return_to: '/workspaces/7/teams' }

function render() {
  wrapper = mount(EnterpriseSSOCallbackView, { global: { stubs: { AuthLayout: { template: '<div><slot /></div>' }, RouterLink: { template: '<a><slot /></a>' } } } })
  return wrapper
}

function expectExistingSessionPreserved() {
  expect(mock.persistOAuthTokenContext).not.toHaveBeenCalled()
  expect(mock.setToken).not.toHaveBeenCalled()
  expect(mock.replace).not.toHaveBeenCalled()
  expect(mock.showSuccess).not.toHaveBeenCalled()
  expect(localStorage.getItem('auth_token')).toBe('existing-token')
  expect(localStorage.getItem('refresh_token')).toBe('existing-refresh-token')
}

describe('EnterpriseSSOCallbackView', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    sessionStorage.clear()
    localStorage.setItem('auth_token', 'existing-token')
    localStorage.setItem('refresh_token', 'existing-refresh-token')
    route.query = {}
    mock.exchange.mockResolvedValue(completion)
  })
  afterEach(() => { wrapper?.unmount(); wrapper = undefined; window.location.hash = ''; localStorage.clear() })

  it('exchanges browser completion cookies and installs only server-issued tokens before navigation', async () => {
    window.location.hash = '#access_token=forged-fragment-token'
    render()
    await flushPromises()
    expect(mock.exchange).toHaveBeenCalledWith()
    expect(mock.persistOAuthTokenContext).toHaveBeenCalledWith(completion)
    expect(mock.setToken).toHaveBeenCalledWith('server-token')
    expect(mock.replace).toHaveBeenCalledWith('/workspaces/7/teams')
    expect(mock.setToken.mock.invocationCallOrder[0]).toBeLessThan(mock.replace.mock.invocationCallOrder[0])
  })

  it('falls back to the authenticated workspace when the server completion return path is unsafe', async () => {
    mock.exchange.mockResolvedValue({ ...completion, return_to: '//evil.example' })
    render()
    await flushPromises()
    expect(mock.replace).toHaveBeenCalledWith('/workspaces/7/overview')
  })

  it('presents an accessible authenticator challenge while preserving the existing session and navigation hint', async () => {
    rememberEnterpriseSSONavigation(7, 9, '/workspaces/7/identity')
    mock.exchange.mockResolvedValue({ requires_2fa: true })
    const view = render()
    await flushPromises()
    expect(view.text()).toContain('workspace.ssoTotpDescription')
    const input = view.get('input[autocomplete="one-time-code"]')
    expect(view.get(`label[for="${input.attributes('id')}"]`).text()).toBe('workspace.ssoTotpCode')
    expect(input.attributes('type')).toBe('text')
    expect(input.attributes('inputmode')).toBe('numeric')
    expect(input.attributes('pattern')).toBe('[0-9]{6}')
    expect(input.attributes('maxlength')).toBe('6')
    expect(view.find('[role="alert"]').exists()).toBe(false)
    expect(mock.exchange).toHaveBeenCalledWith()
    expectExistingSessionPreserved()
    expect(sessionStorage.getItem('modurelay.enterprise-sso-navigation')).not.toBeNull()
  })

  it('submits the code once and installs tokens only after successful two-factor completion', async () => {
    let finish: (value: unknown) => void = () => {}
    mock.exchange.mockResolvedValueOnce({ requires_2fa: true })
    mock.exchange.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const view = render()
    await flushPromises()
    await view.get('input').setValue('123456')
    await view.get('form').trigger('submit')
    await view.get('form').trigger('submit')
    expect(mock.exchange).toHaveBeenCalledTimes(2)
    expect(mock.exchange).toHaveBeenLastCalledWith('123456')
    expect(view.get('button[type="submit"]').attributes('disabled')).toBeDefined()
    expect(view.get('form').attributes('aria-busy')).toBe('true')
    expectExistingSessionPreserved()
    finish(completion)
    await flushPromises()
    expect(mock.persistOAuthTokenContext).toHaveBeenCalledWith(completion)
    expect(mock.setToken).toHaveBeenCalledWith('server-token')
    expect(mock.replace).toHaveBeenCalledWith('/workspaces/7/teams')
    expect(mock.setToken.mock.invocationCallOrder[0]).toBeLessThan(mock.replace.mock.invocationCallOrder[0])
    expect(mock.showSuccess).toHaveBeenCalledWith('workspace.ssoSignInSuccess')
  })

  it('keeps invalid two-factor errors local, clears the submitted code, and allows a successful retry', async () => {
    mock.exchange.mockResolvedValueOnce({ requires_2fa: true })
      .mockRejectedValueOnce({ status: 400, code: 'TOTP_INVALID_CODE', message: 'raw provider message' })
      .mockResolvedValueOnce(completion)
    const view = render()
    await flushPromises()
    await view.get('input').setValue('123456')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.get('[role="alert"]').text()).toBe('workspace.ssoTotpInvalidCode')
    expect(view.text()).not.toContain('raw provider message')
    expect((view.get('input').element as HTMLInputElement).value).toBe('')
    expect(view.get('input').attributes('aria-invalid')).toBe('true')
    expect(view.get('input').attributes('aria-describedby')).toContain(view.get('[role="alert"]').attributes('id'))
    expect(view.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
    expectExistingSessionPreserved()
    await view.get('input').setValue('654321')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(mock.exchange).toHaveBeenLastCalledWith('654321')
    expect(mock.setToken).toHaveBeenCalledTimes(1)
    expect(mock.replace).toHaveBeenCalledWith('/workspaces/7/teams')
  })

  it.each(['12345', '12345x'])('rejects a malformed authenticator code before sending it: %s', async code => {
    mock.exchange.mockResolvedValue({ requires_2fa: true })
    const view = render()
    await flushPromises()
    await view.get('input').setValue(code)
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(mock.exchange).toHaveBeenCalledTimes(1)
    expect(view.get('[role="alert"]').text()).toBe('workspace.ssoTotpInvalidCode')
    expectExistingSessionPreserved()
  })

  it('shows rate limiting without consuming or replacing the existing session', async () => {
    mock.exchange.mockResolvedValueOnce({ requires_2fa: true })
      .mockRejectedValueOnce({ status: 429, code: 'TOTP_TOO_MANY_ATTEMPTS' })
    const view = render()
    await flushPromises()
    await view.get('input').setValue('123456')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.get('[role="alert"]').text()).toBe('workspace.ssoTotpTooManyAttempts')
    expect(view.get('form').exists()).toBe(true)
    expectExistingSessionPreserved()
  })

  it('returns to sign-in recovery when the two-factor completion has expired', async () => {
    rememberEnterpriseSSONavigation(7, 9, '/workspaces/7/identity')
    mock.exchange.mockResolvedValueOnce({ requires_2fa: true })
      .mockRejectedValueOnce({ status: 401, code: 'OIDC_STATE_SESSION_MISMATCH' })
    const view = render()
    await flushPromises()
    await view.get('input').setValue('123456')
    await view.get('form').trigger('submit')
    await flushPromises()
    expect(view.find('form').exists()).toBe(false)
    expect(view.get('[role="alert"]').text()).toBe('workspace.ssoCompletionFailed')
    expect(view.get('a.btn-primary').attributes('to')).toContain('workspace_id=7')
    expectExistingSessionPreserved()
  })

  it('ignores a pending two-factor result after the callback view is unmounted', async () => {
    let finish: (value: unknown) => void = () => {}
    mock.exchange.mockResolvedValueOnce({ requires_2fa: true })
    mock.exchange.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const view = render()
    await flushPromises()
    await view.get('input').setValue('123456')
    await view.get('form').trigger('submit')
    view.unmount()
    wrapper = undefined
    finish(completion)
    await flushPromises()
    expectExistingSessionPreserved()
  })

  it('explains required account linking without adopting any credentials', async () => {
    route.query = { error: 'OIDC_ACCOUNT_LINK_REQUIRED', workspace_id: '7' }
    const view = render()
    await flushPromises()
    expect(view.text()).toContain('workspace.ssoLinkRequired')
    expect(mock.exchange).not.toHaveBeenCalled()
    expect(mock.setToken).not.toHaveBeenCalled()
  })

  it('keeps callback failures local and rejects an invalid completion tenant', async () => {
    mock.exchange.mockResolvedValue({ ...completion, workspace_id: 0 })
    const view = render()
    await flushPromises()
    expect(view.text()).toContain('workspace.ssoCompletionFailed')
    expect(mock.persistOAuthTokenContext).not.toHaveBeenCalled()
    expect(mock.setToken).not.toHaveBeenCalled()
    expect(mock.replace).not.toHaveBeenCalled()
  })

  it('restores the initiating workspace for recovery after a provider failure without granting access', async () => {
    rememberEnterpriseSSONavigation(7, 9, '/workspaces/7/identity')
    route.query = { error: 'OIDC_AUTHENTICATION_FAILED' }
    const view = render()
    await flushPromises()
    const retry = view.find('a.btn-primary')
    expect(retry.attributes('to')).toContain('workspace_id=7')
    expect(retry.attributes('to')).toContain('error=provider_failure')
    expect(mock.setToken).not.toHaveBeenCalled()
  })

  it('ignores a pending completion after the callback view is unmounted', async () => {
    let finish: (value: unknown) => void = () => {}
    mock.exchange.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    render().unmount()
    wrapper = undefined
    finish(completion)
    await flushPromises()
    expect(mock.persistOAuthTokenContext).not.toHaveBeenCalled()
    expect(mock.setToken).not.toHaveBeenCalled()
  })
})
