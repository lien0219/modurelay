import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import LocaleSwitcher from '../LocaleSwitcher.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ locale: ref('en') })
}))

vi.mock('motion-v', () => ({
  AnimatePresence: { template: '<div><slot /></div>' },
  motion: { div: { template: '<div><slot /></div>' } }
}))

vi.mock('@/i18n', () => ({
  availableLocales: [
    { code: 'en', name: 'English', flag: 'EN' },
    { code: 'zh', name: '中文', flag: 'ZH' },
  ],
  setLocale: vi.fn(),
}))

vi.mock('@/composables/usePrefersReducedMotion', () => ({
  usePrefersReducedMotion: () => ref(true)
}))

describe('LocaleSwitcher', () => {
  it('opens the footer menu upward with accessible menu semantics', async () => {
    const wrapper = mount(LocaleSwitcher, {
      props: { placement: 'top-end' },
      global: { stubs: { Icon: { template: '<span />' } } },
    })

    await wrapper.get('button').trigger('click')

    const menu = wrapper.get('[role="menu"]')
    expect(menu.classes()).toContain('bottom-full')
    expect(menu.classes()).toContain('mb-1')
    expect(wrapper.get('button').attributes('aria-expanded')).toBe('true')
    expect(wrapper.findAll('[role="menuitemradio"]')).toHaveLength(2)
  })
})
