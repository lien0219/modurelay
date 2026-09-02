import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import zhAdminDistribution from '@/i18n/locales/zh/admin/distribution'
import DistributionManagementView from '../DistributionManagementView.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

describe('DistributionManagementView', () => {
  it('renders the multiplier architecture without interactive controls', () => {
    const wrapper = mount(DistributionManagementView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
        },
      },
    })

    expect(wrapper.text()).toContain('admin.distribution.headline')
    expect(wrapper.text()).toContain('admin.distribution.model.spread.formula')
    expect(wrapper.text()).toContain('admin.distribution.architecture.qualification.title')
    expect(wrapper.text()).toContain('admin.distribution.architecture.governance.title')
    expect(wrapper.findAll('.distribution-admin__architecture-grid > article')).toHaveLength(6)
    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.find('input').exists()).toBe(false)
    expect(wrapper.find('form').exists()).toBe(false)
  })

  it('documents a spread model that is separate from commission accounting', () => {
    const distribution = zhAdminDistribution.distribution

    expect(distribution.status).toBe('待开发')
    expect(distribution.headline).toContain('不走佣金模式')
    expect(distribution.model.agent.formula).toBe('代理成本 = B × A')
    expect(distribution.model.retail.formula).toBe('客户计费 = B × R')
    expect(distribution.model.spread.formula).toBe('差额 = B × (R - A)')
    expect(distribution.principles.independentLedger).toBe('与邀请返利账本隔离')
  })
})
