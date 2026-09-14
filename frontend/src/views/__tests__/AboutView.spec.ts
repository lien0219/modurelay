import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'

import AboutView from '../AboutView.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    template: '<div data-testid="app-layout"><slot /></div>',
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    props: ['name'],
    template: '<span data-testid="icon" :data-name="name" />',
  },
}))

enableAutoUnmount(afterEach)

describe('AboutView', () => {
  it('renders the enterprise service overview and contact details', () => {
    const wrapper = mount(AboutView)

    expect(wrapper.get('[data-testid="about-page"]')).toBeTruthy()
    expect(wrapper.text()).toContain('about.title')
    expect(wrapper.text()).toContain('about.services.aiProduct.title')
    expect(wrapper.text()).toContain('about.industries.ecommerce')
    expect(wrapper.text()).toContain('13017739011')
  })

  it('exposes a keyboard-accessible telephone link', () => {
    const wrapper = mount(AboutView)
    const link = wrapper.get('a[href="tel:13017739011"]')

    expect(link.attributes('aria-label')).toBe('about.phoneAria')
    expect(link.text()).toContain('13017739011')
  })
})
