import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import WorkspaceAccessRecovery from '../WorkspaceAccessRecovery.vue'
import { useAuthStore } from '@/stores/auth'

const api = vi.hoisted(() => ({ getStatus: vi.fn(), upgradeSessionMFA: vi.fn() }))
vi.mock('@/api/totp', () => ({ totpAPI: api }))
vi.mock('@/components/common/BaseDialog.vue', () => ({ default: { props: ['show'], template: '<div v-if="show"><slot /></div>' } }))
vi.mock('@/components/user/profile/TotpSetupModal.vue', () => ({ default: { template: '<button data-testid="complete-enrollment" @click="$emit(\'success\')">Enroll</button>' } }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))

async function render(reason = 'MFA_REQUIRED') {
  const pinia = createPinia()
  setActivePinia(pinia)
  const auth = useAuthStore()
  auth.token = 'global-session'
  auth.user = { id: 7 } as never
  localStorage.setItem('auth_token', 'global-session')
  localStorage.setItem('auth_user', JSON.stringify({ id: 7 }))
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/workspaces/1/security')
  const view = mount(WorkspaceAccessRecovery, { props: { workspaceId: 1, error: { code: reason }, returnTo: '/workspaces/1/security' }, global: { plugins: [pinia, router] } })
  await flushPromises()
  return { view, auth }
}

describe('workspace access recovery', () => {
  beforeEach(() => { localStorage.clear(); vi.resetAllMocks(); api.getStatus.mockResolvedValue({ enabled: true, feature_enabled: true }) })

  it('does not treat completed enrollment as verified session MFA', async () => {
    api.getStatus.mockResolvedValueOnce({ enabled: false, feature_enabled: true }).mockResolvedValue({ enabled: true, feature_enabled: true })
    const { view } = await render('MFA_ENROLLMENT_REQUIRED')
    await view.find('[data-testid="security-enroll-mfa"]').trigger('click')
    await view.find('[data-testid="complete-enrollment"]').trigger('click')
    await flushPromises()
    expect(view.emitted('verified')).toBeUndefined()
    expect(view.find('[data-testid="security-verify-mfa"]').exists()).toBe(true)
    view.unmount()
  })

  it('adopts the returned token pair before notifying the denied action', async () => {
    const { view, auth } = await render()
    api.upgradeSessionMFA.mockImplementation(async () => {
      localStorage.setItem('auth_token', 'mfa-access')
      localStorage.setItem('refresh_token', 'mfa-refresh')
      localStorage.setItem('token_expires_at', String(Date.now() + 1800000))
      return { access_token: 'mfa-access', refresh_token: 'mfa-refresh', expires_in: 1800, token_type: 'Bearer' }
    })
    await view.find('[data-testid="security-verify-mfa"]').trigger('click')
    await view.find('[name="workspace-mfa-code"]').setValue('123456')
    await view.find('[data-testid="security-mfa-form"]').trigger('submit')
    await flushPromises()
    expect(auth.token).toBe('mfa-access')
    expect(localStorage.getItem('refresh_token')).toBe('mfa-refresh')
    expect(view.emitted('verified')).toHaveLength(1)
    view.unmount()
  })

  it('keeps a failed proof recoverable without clearing the global session', async () => {
    api.upgradeSessionMFA.mockRejectedValue({ code: 'TOTP_INVALID_CODE' })
    const { view, auth } = await render()
    await view.find('[data-testid="security-verify-mfa"]').trigger('click')
    await view.find('[name="workspace-mfa-code"]').setValue('123456')
    await view.find('[data-testid="security-mfa-form"]').trigger('submit')
    await flushPromises()
    expect(auth.token).toBe('global-session')
    expect(view.find('[data-testid="security-mfa-error"]').exists()).toBe(true)
    expect(view.emitted('verified')).toBeUndefined()
    view.unmount()
  })

  it('uses backend SSO metadata for session age recovery without inferring the selected tenant', async () => {
    const { view } = await render('WORKSPACE_REAUTH_REQUIRED')
    await view.setProps({ workspaceId: 7, error: { code: 'WORKSPACE_REAUTH_REQUIRED', metadata: { requires_sso: 'true' } } })
    expect(view.find('a').attributes('href')).toBe('/auth/sso?workspace_id=7&required=1&return_to=%2Fworkspaces%2F1%2Fsecurity')
    view.unmount()
  })
})
