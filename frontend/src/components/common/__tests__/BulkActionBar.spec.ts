import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import BulkActionBar from '../BulkActionBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) =>
      params?.count != null ? `${key}:${params.count}` : key
  })
}))

vi.mock('motion-v', () => ({
  AnimatePresence: {
    name: 'AnimatePresence',
    template: '<div><slot /></div>'
  },
  motion: {
    div: {
      name: 'motion-div',
      template: '<div><slot /></div>'
    }
  }
}))

vi.mock('@/composables/usePrefersReducedMotion', () => ({
  usePrefersReducedMotion: () => ({ value: true })
}))

describe('BulkActionBar', () => {
  it('renders when show is true and hides when false', async () => {
    const wrapper = mount(BulkActionBar, {
      props: { show: true, selectedCount: 3 },
      slots: { default: '<button>Go</button>' }
    })

    expect(wrapper.text()).toContain('common.selectedCount:3')
    expect(wrapper.text()).toContain('Go')

    await wrapper.setProps({ show: false })
    expect(wrapper.find('.bulk-action-bar').exists()).toBe(false)
  })
})
