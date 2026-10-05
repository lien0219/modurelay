import { beforeEach, describe, expect, it, vi } from 'vitest'
import { markAllNotificationsRead } from '../notifications'

const post = vi.hoisted(() => vi.fn())
vi.mock('../client', () => ({ apiClient: { post } }))

describe('notification API scope', () => {
  beforeEach(() => post.mockResolvedValue({ data: { marked: 2 } }))

  it('sends bulk-read scope in the query used by the backend instead of reading another tenant scope', async () => {
    await markAllNotificationsRead({ workspace_id: 7, project_id: 21 })
    expect(post).toHaveBeenCalledWith('/notifications/read-all', undefined, { params: { workspace_id: 7, project_id: 21 } })
  })
})
