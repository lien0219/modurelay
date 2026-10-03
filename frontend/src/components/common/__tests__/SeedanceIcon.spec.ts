import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import PlatformIcon from '../PlatformIcon.vue'
import ModelIcon from '../ModelIcon.vue'

describe('Seedance official icon', () => {
  it('uses a bundled brand asset for the platform', () => {
    const wrapper = mount(PlatformIcon, { props: { platform: 'seedance' } })
    expect(wrapper.get('img').attributes('src')).toContain('/assets/platforms/seedance.svg')
    expect(wrapper.get('img').attributes('src')).not.toMatch(/^https?:\/\//)
  })

  it.each(['seedance-2.0', 'doubao-seedance-1-5-pro-251215', '48:seedance-2.0'])('uses the same asset for %s', model => {
    const wrapper = mount(ModelIcon, { props: { model } })
    expect(wrapper.get('img').attributes('src')).toContain('/assets/platforms/seedance.svg')
    expect(wrapper.find('.model-icon-fallback').exists()).toBe(false)
  })
})
