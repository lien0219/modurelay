import { describe, expect, it } from 'vitest'
import router from '../index'

describe('workspace security route', () => {
  it('registers the authenticated tenant security editor', () => {
    const route = router.getRoutes().find(item => item.name === 'WorkspaceSecurity')
    expect(route?.path).toBe('/workspaces/:workspaceId/security')
    expect(route?.meta.requiresAuth).toBe(true)
  })
})
