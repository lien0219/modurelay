import { describe, expect, it } from 'vitest'
import router from '../index'

describe('workspace webhook route', () => {
  it('registers the tenant webhook surface', () => {
    const route = router.getRoutes().find(item => item.name === 'WorkspaceWebhooks')
    expect(route?.path).toBe('/workspaces/:workspaceId/webhooks')
    expect(route?.meta.requiresAuth).toBe(true)
  })
})
