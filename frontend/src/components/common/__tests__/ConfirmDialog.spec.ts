import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ConfirmDialog from '../ConfirmDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
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

describe('ConfirmDialog', () => {
  it('emits confirm once and blocks rapid double confirm', async () => {
    const wrapper = mount(ConfirmDialog, {
      props: {
        show: true,
        title: 'Delete',
        message: 'Sure?',
        danger: true
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: `
              <div v-if="show">
                <slot />
                <slot name="footer" />
              </div>
            `
          },
          Icon: true
        }
      }
    })

    const buttons = wrapper.findAll('button')
    const confirmBtn = buttons[buttons.length - 1]
    await confirmBtn.trigger('click')
    await confirmBtn.trigger('click')

    expect(wrapper.emitted('confirm')).toHaveLength(1)
  })

  it('shows destructive cue when danger is true', () => {
    const wrapper = mount(ConfirmDialog, {
      props: {
        show: true,
        title: 'Delete',
        message: 'Sure?',
        danger: true
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: `
              <div v-if="show">
                <slot />
                <slot name="footer" />
              </div>
            `
          },
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('common.destructiveAction')
  })
})
