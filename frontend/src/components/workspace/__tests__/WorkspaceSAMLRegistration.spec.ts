import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import WorkspaceSAMLRegistration from '../WorkspaceSAMLRegistration.vue'
import type { WorkspaceIdentityProvider } from '@/api/workspace'

const api = vi.hoisted(() => ({ getSAMLServiceProvider: vi.fn(), rotateSAMLKeys: vi.fn() }))
vi.mock('@/api/workspace', () => ({ workspaceAPI: api }))

describe('SAML registration rotation isolation', () => {
  it('keeps the current rotation disabled when an earlier provider finishes', async () => {
    api.getSAMLServiceProvider.mockResolvedValue({ entity_id: 'https://relay.example/sp', acs_url: 'https://relay.example/acs', metadata_url: 'https://relay.example/metadata', signing_certificate: 'public certificate' })
    let finishA!: (value: WorkspaceIdentityProvider) => void
    let finishB!: (value: WorkspaceIdentityProvider) => void
    api.rotateSAMLKeys.mockImplementationOnce(() => new Promise<WorkspaceIdentityProvider>(resolve => { finishA = resolve }))
    api.rotateSAMLKeys.mockImplementationOnce(() => new Promise<WorkspaceIdentityProvider>(resolve => { finishB = resolve }))
    const provider = (id: number) => ({ id, name: `Provider ${id}`, type: 'saml', revision: 3 } as WorkspaceIdentityProvider)
    const wrapper = mount(WorkspaceSAMLRegistration, { props: { workspaceId: 1, provider: provider(9), canManage: true }, global: { plugins: [createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })] } })
    await flushPromises()
    await wrapper.get('[data-testid="saml-stage"]').trigger('click')
    await wrapper.setProps({ provider: provider(10) })
    await flushPromises()
    await wrapper.get('[data-testid="saml-stage"]').trigger('click')
    expect(wrapper.get('[data-testid="saml-stage"]').attributes('disabled')).toBeDefined()
    finishA(provider(9))
    await flushPromises()
    expect(wrapper.get('[data-testid="saml-stage"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="saml-stage"]').trigger('click')
    expect(api.rotateSAMLKeys).toHaveBeenCalledTimes(2)
    finishB({ ...provider(10), revision: 4 })
    await flushPromises()
    expect(wrapper.get('[data-testid="saml-stage"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.emitted('updated')).toEqual([[{ ...provider(10), revision: 4 }]])
    wrapper.unmount()
  })
})
