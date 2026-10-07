import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { useWorkspaceStore } from '@/stores/workspace'
import workspaceMessages from '@/i18n/locales/en/workspace'
import identityMessages from '@/i18n/locales/en/workspaceIdentity'
import WorkspaceIdentityView from '../WorkspaceIdentityView.vue'
import Pagination from '@/components/common/Pagination.vue'

const api = vi.hoisted(() => ({
  listWorkspaces: vi.fn(), listProjects: vi.fn(), listDomains: vi.fn(), createDomain: vi.fn(), verifyDomain: vi.fn(), regenerateDomainToken: vi.fn(), revokeDomain: vi.fn(),
  getSAMLServiceProvider: vi.fn(), rotateSAMLKeys: vi.fn(), listIdentityProviders: vi.fn(), createIdentityProvider: vi.fn(), updateIdentityProvider: vi.fn(), disableIdentityProvider: vi.fn(), getIdentityPolicy: vi.fn(), updateIdentityPolicy: vi.fn(), startSSO: vi.fn(),
}))

vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))
vi.mock('@/components/workspace/WorkspaceFrame.vue', () => ({ default: { template: '<main><slot /></main>' } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => {
    const path = key.replace(/^workspace\./, '').split('.')
    let value: unknown = { ...workspaceMessages.workspace, ...identityMessages.workspace }
    for (const part of path) value = (value as Record<string, unknown>)?.[part]
    if (typeof value !== 'string') return key
    return value.replace(/\{(\w+)\}/g, (_, name: string) => String(params?.[name] ?? `{${name}}`))
  } }),
}))

const permissions = ['workspace.read', 'identity.read', 'identity.manage', 'workspace_sso.update']
const workspace = { id: 1, name: 'ACME', slug: 'acme', type: 'organization', status: 'active', owner_user_id: 7, billing_owner_user_id: 7, permissions }
const provider = { id: 9, workspace_id: 1, type: 'oidc', provider_key: 'entra', name: 'Entra ID', status: 'active', is_default: true, issuer_url: 'https://login.example.com', client_id: 'client', has_client_secret: true, token_auth_method: 'client_secret_basic', revision: 3, scopes: ['openid', 'email'], discovery_enabled: true, claim_mapping: { email_verified: 'verified_email' }, jit_config: { enabled: false, default_role: 'viewer', allowed_domains: [], require_verified_email: true } }

let wrapper: VueWrapper | undefined

async function render(items = [workspace]) {
  const pinia = createPinia()
  setActivePinia(pinia)
  api.listWorkspaces.mockResolvedValue({ items })
  api.listProjects.mockResolvedValue({ items: [] })
  await useWorkspaceStore().loadWorkspaces()
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/workspaces/:workspaceId/identity', component: WorkspaceIdentityView }, { path: '/auth/sso', component: { template: '<div />' } }] })
  await router.push('/workspaces/1/identity')
  wrapper = mount(WorkspaceIdentityView, { global: { plugins: [pinia, router] } })
  await flushPromises()
  return wrapper
}

describe('WorkspaceIdentityView', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetAllMocks()
    api.listDomains.mockResolvedValue({ items: [] })
    api.listIdentityProviders.mockResolvedValue({ items: [provider] })
    api.getIdentityPolicy.mockResolvedValue({ workspace_id: 1, require_sso: false, sso_grace_until: null, revision: 2 })
    api.createDomain.mockResolvedValue({ domain: { id: 4, workspace_id: 1, domain: 'example.com', normalized_domain: 'example.com', status: 'pending', verification_method: 'dns' }, verification_token: 'token', verification_txt: 'modurelay-verification=token' })
    api.createIdentityProvider.mockResolvedValue(provider)
    api.updateIdentityProvider.mockResolvedValue(provider)
    api.updateIdentityPolicy.mockResolvedValue({ workspace_id: 1, require_sso: true, revision: 3 })
  })

  afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks() })

  it('creates SAML without OIDC fields and safely splits PEM certificates', async () => {
    const view = await render()
    await view.find('[data-testid="identity-add-provider"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    await form.find('select[name="type"]').setValue('saml')
    expect(form.find('input[name="client_id"]').exists()).toBe(false)
    await form.find('input[name="name"]').setValue('SAML IdP')
    await form.find('input[name="provider_key"]').setValue('saml-idp')
    await form.find('input[name="idp_entity_id"]').setValue('urn:example:idp')
    await form.find('input[name="sso_url"]').setValue('https://id.example.com/sso')
    const cert = '-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----'
    await form.find('textarea[name="signing_certificates"]').setValue(cert + '\n' + cert)
    await form.trigger('submit')
    await flushPromises()
    const payload = api.createIdentityProvider.mock.calls[0][1]
    expect(payload).toMatchObject({ type: 'saml', saml: { idp_entity_id: 'urn:example:idp', signing_certificates: [cert, cert], authn_requests_signed: true, subject_attribute: '', allow_unspecified_name_id: false } })
    for (const key of ['issuer_url', 'client_id', 'client_secret', 'secret_action', 'scopes', 'claim_mapping', 'token_auth_method']) expect(payload).not.toHaveProperty(key)
  })

  it('preserves SAML configuration on edit and keeps provider type immutable', async () => {
    const cert = '-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----'
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, type: 'saml', saml_public_id: 'public-id', saml: { idp_entity_id: 'urn:idp', sso_url: 'https://id.example.com/sso', signing_certificates: [cert], metadata_source: 'url', metadata_url: 'https://id.example.com/metadata', subject_attribute: 'immutable_id', allow_unspecified_name_id: false, email_attribute: 'mail', name_attribute: 'displayName', groups_attribute: 'roles', authn_requests_signed: true } }] })
    const view = await render()
    expect(view.text()).toContain('urn:idp')
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    expect(form.find('select[name="type"]').attributes()).toHaveProperty('disabled')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).toHaveBeenCalledWith(1, 9, expect.objectContaining({ revision: 3, saml: expect.objectContaining({ metadata_url: 'https://id.example.com/metadata', signing_certificates: [cert], subject_attribute: 'immutable_id' }) }))
    expect(api.getSAMLServiceProvider).not.toHaveBeenCalled()
  })

  it('shows SAML registration and requires confirmation before promotion', async () => {
    const samlProvider = { ...provider, type: 'saml', saml_public_id: 'public-id' }
    const registration = { entity_id: 'urn:sp', acs_url: 'https://relay.example.com/acs', metadata_url: 'https://relay.example.com/metadata', signing_certificate: 'current-cert', next_signing_certificate: 'next-cert', idp_initiated_supported: false, slo_supported: false }
    api.listIdentityProviders.mockResolvedValue({ items: [samlProvider] })
    api.getSAMLServiceProvider.mockResolvedValue(registration)
    api.rotateSAMLKeys.mockResolvedValue({ ...samlProvider, revision: 4 })
    const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false)
    const view = await render()
    await view.find('[data-testid="identity-saml-sp-9"]').trigger('click')
    await flushPromises()
    expect(view.text()).toContain('https://relay.example.com/acs')
    await view.find('[data-testid="saml-promote"]').trigger('click')
    expect(api.rotateSAMLKeys).not.toHaveBeenCalled()
    confirm.mockReturnValue(true)
    await view.find('[data-testid="saml-promote"]').trigger('click')
    await flushPromises()
    expect(api.rotateSAMLKeys).toHaveBeenCalledWith(1, 9, { revision: 3, action: 'promote' })
  })

  it('lets read-only users view SAML registration without key rotation controls', async () => {
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, type: 'saml' }] })
    api.getSAMLServiceProvider.mockResolvedValue({ entity_id: 'urn:sp', acs_url: 'https://relay.example.com/acs', metadata_url: 'https://relay.example.com/metadata', signing_certificate: 'cert' })
    const view = await render([{ ...workspace, permissions: ['workspace.read', 'identity.read'] }])
    await view.find('[data-testid="identity-saml-sp-9"]').trigger('click')
    await flushPromises()
    expect(view.text()).toContain('urn:sp')
    expect(view.find('[data-testid="saml-stage"]').exists()).toBe(false)
    expect(view.find('[data-testid="saml-promote"]').exists()).toBe(false)
  })

  it('preserves explicitly empty optional SAML attributes during ordinary edits', async () => {
    const cert = '-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----'
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, type: 'saml', saml: { idp_entity_id: 'urn:idp', sso_url: 'https://id.example.com/sso', signing_certificates: [cert], metadata_source: 'manual', email_attribute: 'mail', name_attribute: '', groups_attribute: '' } }] })
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    await view.find('[data-testid="identity-provider-form"]').trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).toHaveBeenCalledWith(1, 9, expect.objectContaining({ saml: expect.objectContaining({ name_attribute: '', groups_attribute: '' }) }))
  })

  it('imports pasted metadata on save without requiring fabricated manual or OIDC fields', async () => {
    const view = await render()
    await view.find('[data-testid="identity-add-provider"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    await form.find('select[name="type"]').setValue('saml')
    await form.find('input[name="name"]').setValue('XML IdP')
    await form.find('input[name="provider_key"]').setValue('xml-idp')
    await form.find('select[name="metadata_source"]').setValue('xml')
    await form.find('textarea[name="metadata_xml"]').setValue('<EntityDescriptor entityID="urn:idp"/>')
    expect(api.createIdentityProvider).not.toHaveBeenCalled()
    await form.trigger('submit')
    await flushPromises()
    expect(api.createIdentityProvider).toHaveBeenCalledWith(1, expect.objectContaining({ saml: expect.objectContaining({ metadata_source: 'xml', metadata_xml: '<EntityDescriptor entityID="urn:idp"/>', idp_entity_id: '', signing_certificates: [] }) }))
  })

  it('blocks malformed certificate fragments rather than silently discarding them', async () => {
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, type: 'saml', saml: { idp_entity_id: 'urn:idp', sso_url: 'https://id.example.com/sso', signing_certificates: ['-----BEGIN CERTIFICATE-----\nYWJj\n-----END CERTIFICATE-----'], metadata_source: 'manual' } }] })
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    await form.find('textarea[name="signing_certificates"]').setValue('-----BEGIN CERTIFICATE-----broken')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).not.toHaveBeenCalled()
  })

  it('renders provider state and saves a new verified domain request', async () => {
    const view = await render()
    expect(view.text()).toContain('Entra ID')
    await view.find('[data-testid="identity-domain-form"] input').setValue('example.com')
    await view.find('[data-testid="identity-domain-form"]').trigger('submit')
    await flushPromises()
    expect(api.createDomain).toHaveBeenCalledWith(1, 'example.com')
    expect(view.text()).toContain('modurelay-verification=token')
  })

  it.each([
    ['SUCCESS', 'Validated'],
    ['DISCOVERY_FAILED', 'Discovery failed'],
    ['ISSUER_MISMATCH', 'Issuer mismatch'],
    ['ENDPOINT_INVALID', 'Invalid endpoint'],
    ['CONFIGURATION_INVALID', 'Invalid configuration'],
    ['VALIDATION_FAILED', 'Validation failed'],
    ['PROVIDER_DISABLED', 'Provider disabled'],
  ])('shows the last validation time and a safe localized %s result', async (code, label) => {
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, last_validated_at: '2026-10-07T08:30:00Z', last_validation_code: code }] })
    const view = await render()
    const result = view.find('[data-testid="identity-validation-9"]')
    expect(result.text()).toContain('Last validation')
    expect(result.text()).toContain(label)
    expect(result.find('time').attributes('datetime')).toBe('2026-10-07T08:30:00Z')
    expect(result.find('time').text()).toContain('2026')
  })

  it('shows an unrecorded state for nullable validation fields and never renders raw endpoint errors', async () => {
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, last_validated_at: null, last_validation_code: null }, { ...provider, id: 10, last_validated_at: null, last_validation_code: 'https://id.example.com/token?client_secret=private-value' }] })
    const view = await render()
    expect(view.find('[data-testid="identity-validation-9"]').text()).toContain('Not yet recorded')
    expect(view.find('[data-testid="identity-validation-10"]').text()).toContain('Not yet recorded')
    expect(view.find('[data-testid="identity-validation-9"] time').exists()).toBe(false)
    expect(view.html()).not.toContain('private-value')
  })

  it('creates a preset provider with explicit secret and non-owner JIT configuration', async () => {
    const view = await render()
    await view.find('[data-testid="identity-add-provider"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    await form.find('select[name="preset"]').setValue('google')
    await form.find('input[name="client_id"]').setValue('google-client')
    await form.find('input[name="client_secret"]').setValue('new-client-secret')
    await form.find('input[name="jit_enabled"]').setValue(true)
    await form.find('select[name="default_role"]').setValue('developer')
    await form.find('input[name="allowed_domains"]').setValue('example.com')
    await view.find('[data-testid="identity-provider-form"]').trigger('submit')
    await flushPromises()
    expect(api.createIdentityProvider).toHaveBeenCalledWith(1, expect.objectContaining({ name: 'Google Workspace', issuer_url: 'https://accounts.google.com', client_secret: 'new-client-secret', secret_action: 'replace', jit_config: { enabled: true, default_role: 'developer', allowed_domains: ['example.com'], require_verified_email: true } }))
    expect(api.createIdentityProvider.mock.calls[0][1].claim_mapping).not.toHaveProperty('groups')
    expect(form.find('select[name="default_role"]').findAll('option').some(option => option.attributes('value') === 'owner')).toBe(false)
    expect(view.find('input[name="client_secret"]').exists()).toBe(false)
  })

  it('preserves a saved secret, token authentication method, claims and revision when editing', async () => {
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    expect(form.find('input[name="client_secret"]').exists()).toBe(false)
    await form.find('input[name="name"]').setValue('Renamed provider')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).toHaveBeenCalledWith(1, 9, expect.objectContaining({ name: 'Renamed provider', secret_action: 'preserve', token_auth_method: 'client_secret_basic', revision: 3, claim_mapping: expect.objectContaining({ email_verified: 'verified_email' }) }))
    expect(api.updateIdentityProvider.mock.calls[0][2]).not.toHaveProperty('client_secret')
    expect(view.find('[data-testid="identity-provider-form"]').exists()).toBe(false)
  })

  it('uses explicit endpoints when discovery is disabled', async () => {
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, discovery_enabled: false, authorization_endpoint: 'https://id.example.com/authorize', token_endpoint: 'https://id.example.com/token', jwks_uri: 'https://id.example.com/keys', userinfo_endpoint: 'https://id.example.com/userinfo' }] })
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    expect(form.find('input[name="jwks_uri"]').element).toHaveProperty('value', 'https://id.example.com/keys')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider.mock.calls[0][2]).toMatchObject({ discovery_enabled: false, authorization_endpoint: 'https://id.example.com/authorize', token_endpoint: 'https://id.example.com/token', jwks_uri: 'https://id.example.com/keys', userinfo_endpoint: 'https://id.example.com/userinfo' })
  })

  it('replaces a saved provider secret only through the explicit replacement action', async () => {
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    await view.find('select[name="secret_action"]').setValue('replace')
    await view.find('input[name="client_secret"]').setValue('replacement-secret')
    await view.find('[data-testid="identity-provider-form"]').trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).toHaveBeenCalledWith(1, 9, expect.objectContaining({ secret_action: 'replace', client_secret: 'replacement-secret', token_auth_method: 'client_secret_basic' }))
    expect(view.find('input[name="client_secret"]').exists()).toBe(false)
  })

  it('requires explicit confirmation before removing the secret and switching to a public client', async () => {
    const confirm = vi.spyOn(window, 'confirm').mockReturnValueOnce(false).mockReturnValueOnce(true)
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    await view.find('select[name="secret_action"]').setValue('remove')
    await view.find('[data-testid="identity-provider-form"]').trigger('submit')
    expect(confirm).toHaveBeenCalled()
    expect(api.updateIdentityProvider).not.toHaveBeenCalled()
    await view.find('[data-testid="identity-provider-form"]').trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider.mock.calls[0][2]).toMatchObject({ secret_action: 'remove', token_auth_method: 'none' })
    expect(api.updateIdentityProvider.mock.calls[0][2]).not.toHaveProperty('client_secret')
  })

  it('blocks public-client edits that would keep or replace a saved secret', async () => {
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    for (const action of ['preserve', 'replace']) {
      await form.find('select[name="secret_action"]').setValue(action)
      await form.find('select[name="token_auth_method"]').setValue('none')
      await form.trigger('submit')
      await flushPromises()
      expect(api.updateIdentityProvider).not.toHaveBeenCalled()
      expect(form.find('[role="alert"]').text()).toContain('Choose Remove the saved secret')
      expect(form.find('select[name="secret_action"]').attributes('aria-invalid')).toBe('true')
    }
    await form.find('select[name="secret_action"]').setValue('remove')
    expect(form.find('[role="alert"]').exists()).toBe(false)
  })

  it('requires explicit secret replacement when a public provider becomes confidential', async () => {
    api.listIdentityProviders.mockResolvedValue({ items: [{ ...provider, token_auth_method: 'none', has_client_secret: false }] })
    const view = await render()
    await view.find('[data-testid="identity-edit-provider-9"]').trigger('click')
    const form = view.find('[data-testid="identity-provider-form"]')
    await form.find('select[name="token_auth_method"]').setValue('client_secret_basic')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).not.toHaveBeenCalled()
    expect(form.find('[role="alert"]').text()).toContain('This provider has no saved secret')
    await form.find('select[name="secret_action"]').setValue('replace')
    expect(form.find('[role="alert"]').exists()).toBe(false)
    await form.find('input[name="client_secret"]').setValue('new-confidential-secret')
    await form.trigger('submit')
    await flushPromises()
    expect(api.updateIdentityProvider).toHaveBeenCalledWith(1, 9, expect.objectContaining({ token_auth_method: 'client_secret_basic', secret_action: 'replace', client_secret: 'new-confidential-secret' }))
  })

  it('supports DNS recheck and confirmed revocation of a verified domain', async () => {
    api.listDomains.mockResolvedValue({ items: [{ id: 4, domain: 'example.com', normalized_domain: 'example.com', status: 'verified' }] })
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    const view = await render()
    await view.find('[data-testid="identity-verify-domain-4"]').trigger('click')
    await flushPromises()
    expect(api.verifyDomain).toHaveBeenCalledWith(1, 4)
    await view.find('[data-testid="identity-revoke-domain-4"]').trigger('click')
    await flushPromises()
    expect(api.revokeDomain).toHaveBeenCalledWith(1, 4)
  })

  it('saves SSO policy and explains the server-enforced owner linking prerequisites', async () => {
    const view = await render()
    expect(view.text()).toContain('Link your owner account')
    expect(view.find('[data-testid="identity-link-provider-9"]').attributes('href')).toContain('link=1')
    await view.find('input[name="require_sso"]').setValue(true)
    await view.find('[data-testid="identity-save-policy"]').trigger('click')
    await flushPromises()
    expect(api.updateIdentityPolicy).toHaveBeenCalledWith(1, { require_sso: true, sso_grace_until: null })
  })

  it('keeps organization identity settings read-only without management permissions', async () => {
    const view = await render([{ ...workspace, permissions: ['workspace.read', 'identity.read'] }])
    expect(view.text()).toContain('Entra ID')
    expect(view.find('[data-testid="identity-domain-form"]').exists()).toBe(false)
    expect(view.find('[data-testid="identity-add-provider"]').exists()).toBe(false)
    expect(view.find('input[name="require_sso"]').attributes()).toHaveProperty('disabled')
  })

  it('can navigate beyond the first page of domains and identity providers', async () => {
    api.listDomains.mockResolvedValue({ items: [], total: 75, page: 1, page_size: 50, pages: 2 })
    api.listIdentityProviders.mockResolvedValue({ items: [provider], total: 75, page: 1, page_size: 50, pages: 2 })
    const view = await render()
    const pages = view.findAllComponents(Pagination)
    pages[0].vm.$emit('update:page', 2)
    await flushPromises()
    expect(api.listDomains).toHaveBeenLastCalledWith(1, expect.objectContaining({ page: 2, page_size: 50 }))
    view.findAllComponents(Pagination)[1].vm.$emit('update:page', 2)
    await flushPromises()
    expect(api.listIdentityProviders).toHaveBeenLastCalledWith(1, expect.objectContaining({ page: 2, page_size: 50 }))
  })

  it('does not load enterprise settings for a personal workspace', async () => {
    const view = await render([{ ...workspace, type: 'personal' }])
    expect(view.text()).toContain('organization workspaces')
    expect(api.listDomains).not.toHaveBeenCalled()
  })

  it('drops an old domain token after switching away and back before its creation finishes', async () => {
    let finish: (value: unknown) => void = () => {}
    api.createDomain.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const view = await render([workspace, { ...workspace, id: 2, name: 'Other' }])
    await view.find('[data-testid="identity-domain-form"] input').setValue('example.com')
    await view.find('[data-testid="identity-domain-form"]').trigger('submit')
    await useWorkspaceStore().selectWorkspace(2)
    await flushPromises()
    await useWorkspaceStore().selectWorkspace(1)
    await flushPromises()
    finish({ domain: { id: 4, normalized_domain: 'example.com' }, verification_txt: 'modurelay-verification=stale-secret' })
    await flushPromises()
    expect(view.text()).not.toContain('stale-secret')
  })

  it('clears provider secret input immediately when switching workspaces', async () => {
    const view = await render([workspace, { ...workspace, id: 2, name: 'Other' }])
    await view.find('[data-testid="identity-add-provider"]').trigger('click')
    await view.find('input[name="client_secret"]').setValue('sensitive-secret')
    await useWorkspaceStore().selectWorkspace(2)
    await flushPromises()
    expect(view.find('[data-testid="identity-provider-form"]').exists()).toBe(false)
    expect(view.html()).not.toContain('sensitive-secret')
  })
})
