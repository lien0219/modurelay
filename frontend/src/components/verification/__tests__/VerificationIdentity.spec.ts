import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import VerificationIdentity from '../VerificationIdentity.vue'

const mountIdentity = (props: InstanceType<typeof VerificationIdentity>['$props']) => mount(
  VerificationIdentity,
  {
    props,
    global: {
      stubs: {
        Icon: { template: '<svg data-fallback-icon="true" />' },
      },
    },
  },
)

describe('VerificationIdentity', () => {
  it.each(['amazon', 'discord', 'github', 'google', 'microsoft', 'openai', 'telegram', 'whatsapp'])(
    'renders %s with bundled icon data',
    (code) => {
      const wrapper = mountIdentity({ kind: 'platform', code, label: code })

      expect(wrapper.find('svg').exists()).toBe(true)
      expect(wrapper.find('img').exists()).toBe(false)
      expect(wrapper.html()).not.toContain('cdn.simpleicons.org')
    },
  )

  it('renders country flags from the local flag-icons bundle', () => {
    const wrapper = mountIdentity({ kind: 'country', code: 'US', label: 'United States' })

    expect(wrapper.get('.fi').classes()).toContain('fi-us')
    expect(wrapper.get('span').classes()).toContain('whitespace-nowrap')
    expect(wrapper.find('img').exists()).toBe(false)
  })

  it('rejects a legacy Simple Icons CDN override', () => {
    const wrapper = mountIdentity({
      kind: 'platform',
      code: 'other',
      label: 'Other',
      icon: 'https://cdn.simpleicons.org/other',
    })

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.get('[data-fallback-icon="true"]').exists()).toBe(true)
  })
})
