import { describe, expect, it } from 'vitest'
import router from '../index'

describe('policy routes', () => {
  it('resolves the static service-account policy path before the optional detail route', () => {
    expect(router.resolve('/workspaces/7/projects/9/service-accounts/11/policy').name).toBe('ServiceAccountPolicy')
  })

  it('registers workspace and project policy surfaces as authenticated routes', () => {
    for (const name of ['WorkspacePolicy', 'ProjectPolicy']) {
      const route = router.getRoutes().find(item => item.name === name)
      expect(route?.meta.requiresAuth).toBe(true)
    }
  })

  it('keeps the direct API key page as an independent authenticated route', () => {
    const keyRoute = router.getRoutes().find(item => item.name === 'Keys')
    expect(keyRoute?.path).toBe('/keys')
    expect(keyRoute?.meta.requiresAuth).toBe(true)
    expect(keyRoute?.meta.requiresAdmin).toBe(false)
    expect(router.resolve('/keys').matched.some(route => route.name === 'Workspace')).toBe(false)
  })
})
