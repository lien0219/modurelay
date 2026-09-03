import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import enDistribution from '@/i18n/locales/en/distribution'
import zhDistribution from '@/i18n/locales/zh/distribution'
import DistributionView from '../DistributionView.vue'

const { showInfo } = vi.hoisted(() => ({
  showInfo: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showInfo }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('DistributionView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows the permission notice when the user requests agent access', async () => {
    const wrapper = mount(DistributionView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          DistributionPreview: { template: '<section><slot name="action" /></section>' },
          Icon: true,
        },
      },
    })

    const button = wrapper.get('button')
    expect(button.text()).toContain('distribution.cta')

    await button.trigger('click')

    expect(showInfo).toHaveBeenCalledOnce()
    expect(showInfo).toHaveBeenCalledWith('distribution.permissionNotice')
  })

  it('presents multiplier distribution without placeholder or commission copy', () => {
    const zhCopy = JSON.stringify(zhDistribution.distribution)
    const enCopy = JSON.stringify(enDistribution.distribution)

    expect(zhDistribution.distribution.permissionNotice).toBe('暂无权限，请联系管理员报名开启代理分销')
    expect(zhDistribution.distribution.features.agentMultiplier.title).toBe('代理专属倍率')
    expect(zhDistribution.distribution.features.retailMultiplier.title).toBe('自主分销倍率')
    expect(zhDistribution.distribution.features.spread.title).toBe('倍率差额空间')
    expect(zhCopy).not.toMatch(/筹备中|待开发|预告|规划|佣金/)
    expect(enCopy).not.toMatch(/in preparation|to be developed|preview|planned|commission/i)
  })
})
