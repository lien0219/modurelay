import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PolicySettingsPanel from '../PolicySettingsPanel.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, values?: Record<string, unknown>) => values ? `${key}:${String(Object.values(values)[0])}` : key }) }))

const policy = {
  scope: 'project' as const,
  scope_id: 9,
  revision: 4,
  allowed_models: null,
  allowed_platforms: [],
  rpm_limit: null,
  daily_request_limit: 100,
  monthly_request_limit: null,
  daily_token_limit: null,
  monthly_token_limit: null,
}

describe('PolicySettingsPanel', () => {
  it('renders tri-state allowlists and emits null instead of converting inherit to deny-all', async () => {
    const wrapper = mount(PolicySettingsPanel, {
      props: { policy, editable: true, scopeLabel: 'Project policy' },
      global: { mocks: { $t: (key: string) => key } },
    })

    expect(wrapper.find('[name="allowed-models-mode"]').element.value).toBe('inherit')
    expect(wrapper.find('[name="allowed-platforms-mode"]').element.value).toBe('deny')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('save')?.[0]?.[0]).toMatchObject({
      expected_revision: 4,
      allowed_models: null,
      allowed_platforms: [],
      daily_request_limit: 100,
    })
  })

  it('shows effective policy as read-only provenance and does not expose save controls when locked', () => {
    const wrapper = mount(PolicySettingsPanel, {
      props: {
        policy,
        editable: false,
        scopeLabel: 'Effective policy',
        effective: {
          ...policy,
          scope: undefined,
          scope_id: undefined,
          revision: undefined,
          layers: {
            workspace: { ...policy, scope: 'workspace', allowed_models: ['gpt-6'], allowed_platforms: ['openai'] },
            project: { ...policy, scope: 'project', allowed_models: ['claude-4'], allowed_platforms: null },
          },
          revisions: { workspace: 2, project: 4 },
          rpm_limit: 25,
          daily_request_limit: 100,
          monthly_request_limit: null,
          daily_token_limit: null,
          monthly_token_limit: null,
        },
      },
      global: { mocks: { $t: (key: string) => key } },
    })

    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.text()).toContain('25')
    expect(wrapper.text()).toContain('workspace')
    expect(wrapper.text()).toContain('gpt-6')
    expect(wrapper.text()).toContain('claude-4')
    expect(wrapper.find('[data-testid="effective-policy-readonly"]').attributes('aria-readonly')).toBe('true')
  })

  it('shows the stored scope policy when the viewer cannot edit and no effective policy was requested', () => {
    const wrapper = mount(PolicySettingsPanel, {
      props: { policy: { ...policy, allowed_models: ['gpt-6'], rpm_limit: 40 }, editable: false, scopeLabel: 'Workspace policy' },
      global: { mocks: { $t: (key: string) => key } },
    })

    expect(wrapper.find('form').exists()).toBe(false)
    expect(wrapper.text()).toContain('gpt-6')
    expect(wrapper.text()).toContain('40')
    expect(wrapper.text()).toContain('workspace.policyModes.deny')
    expect(wrapper.find('[data-testid="policy-readonly"]').exists()).toBe(true)
  })

  it('rejects non-positive-integer limits instead of silently serializing them as inherited', async () => {
    const wrapper = mount(PolicySettingsPanel, {
      props: { policy, editable: true, scopeLabel: 'Project policy' },
      global: { mocks: { $t: (key: string) => key } },
    })

    await wrapper.find('input[type="number"]').setValue('1.5')
    await wrapper.find('form').trigger('submit')
    expect(wrapper.emitted('save')).toBeUndefined()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })
})
