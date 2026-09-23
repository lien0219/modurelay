import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import SMSServiceLogo from './SMSServiceLogo.vue'

describe('SMSServiceLogo', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('shows the service initial without requesting an optional provider icon URL', () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)

    const wrapper = mount(SMSServiceLogo, {
      props: {
        icon: '/api/v1/sms/service-icons/custom-service',
        label: 'Custom service',
      },
    })

    expect(wrapper.text()).toBe('C')
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('continues to render supported inline images', () => {
    const wrapper = mount(SMSServiceLogo, {
      props: {
        icon: 'data:image/svg+xml,%3Csvg/%3E',
        label: 'Custom service',
      },
    })

    expect(wrapper.find('img').attributes('src')).toBe('data:image/svg+xml,%3Csvg/%3E')
  })
})
