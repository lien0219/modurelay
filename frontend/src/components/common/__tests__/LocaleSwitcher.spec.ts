import { ref } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import LocaleSwitcher from '../LocaleSwitcher.vue'

const { setLocale } = vi.hoisted(() => ({ setLocale: vi.fn() }))

vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: ref('zh') }) }))

vi.mock('motion-v', () => ({
  AnimatePresence: { template: '<div><slot /></div>' },
  motion: { div: { template: '<div><slot /></div>' } }
}))

vi.mock('@/i18n', () => ({
  setLocale,
  availableLocales: [
    { code: 'en', name: 'English', flag: '🇺🇸' },
    { code: 'zh', name: '中文', flag: '🇨🇳' }
  ]
}))

vi.mock('@/composables/usePrefersReducedMotion', () => ({
  usePrefersReducedMotion: () => ref(true)
}))

describe('LocaleSwitcher', () => {
  const reload = vi.fn()

  beforeEach(() => {
    setLocale.mockReset().mockResolvedValue(undefined)
    reload.mockReset()
    vi.stubGlobal('location', { ...window.location, reload })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

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

  const mountSwitcher = () =>
    mount(LocaleSwitcher, { global: { stubs: { Icon: true, transition: false } } })

  const choose = async (wrapper: ReturnType<typeof mountSwitcher>, name: string) => {
    await wrapper.get('button').trigger('click')
    const option = wrapper.findAll('button').find((button) => button.text().includes(name))
    await option!.trigger('click')
    await flushPromises()
  }

  // 分组名称等内容由后端按请求语言返回，已加载的数据要靠整页刷新换成新语言
  it('reloads the page after switching to another language', async () => {
    const wrapper = mountSwitcher()
    await choose(wrapper, 'English')
    expect(setLocale).toHaveBeenCalledWith('en')
    expect(reload).toHaveBeenCalledTimes(1)
    expect(setLocale.mock.invocationCallOrder[0]).toBeLessThan(reload.mock.invocationCallOrder[0])
  })

  it('does nothing when the current language is chosen again', async () => {
    const wrapper = mountSwitcher()
    await choose(wrapper, '中文')
    expect(setLocale).not.toHaveBeenCalled()
    expect(reload).not.toHaveBeenCalled()
  })
})
