import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ValidationMessage from '../ValidationMessage.vue'

vi.mock('motion-v', () => ({
  AnimatePresence: {
    name: 'AnimatePresence',
    template: '<div><slot /></div>'
  },
  motion: {
    p: {
      name: 'motion-p',
      template: '<p><slot /></p>'
    }
  }
}))

vi.mock('@/composables/usePrefersReducedMotion', () => ({
  usePrefersReducedMotion: () => ({ value: true })
}))

describe('ValidationMessage', () => {
  it('renders error message with alert role', () => {
    const wrapper = mount(ValidationMessage, {
      props: { message: 'Required', tone: 'error' }
    })
    expect(wrapper.text()).toContain('Required')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('hides when message is empty', () => {
    const wrapper = mount(ValidationMessage, {
      props: { message: '', tone: 'error' }
    })
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })
})
