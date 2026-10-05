import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import NotificationCenterBell from '../NotificationCenterBell.vue'
import { listNotifications, getUnreadNotificationCount, markNotificationRead, markAllNotificationsRead } from '@/api/notifications'
import resourceAPI from '@/api/resourceCenter'
import { createTestI18n } from '@/__tests__/utils/i18n'
import notificationEn from '@/i18n/locales/en/notifications'
import notificationZh from '@/i18n/locales/zh/notifications'
import type { UserNotification } from '@/types'

vi.mock('@/api/notifications', () => ({ listNotifications: vi.fn(), getUnreadNotificationCount: vi.fn(), markNotificationRead: vi.fn(), markAllNotificationsRead: vi.fn() }))
vi.mock('@/api/resourceCenter', () => ({ default: { listNotifications: vi.fn(), markNotificationRead: vi.fn(), markAllNotificationsRead: vi.fn() } }))
enableAutoUnmount(afterEach)

const notification: UserNotification = { id: 1, category: 'workspace', title: 'Workspace changed', body: 'A member joined', workspace_id: 7, target_path: '/workspaces/7/members', read_at: null, created_at: '2026-10-05T00:00:00Z' }
const resource = { id: 9, post_id: 25, kind: 'reply', actor: { id: 2, username: 'Ari', role: 'user', created_at: '' }, read: false, created_at: '2026-10-04T00:00:00Z' }
const page = (items: UserNotification[], value = 1, pages = 1) => ({ items, total: items.length, page: value, page_size: 20, pages })

async function render(props: Record<string, unknown> = {}, locale = 'en') {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  await router.push('/dashboard')
  const i18n = createTestI18n({
    en: { ...notificationEn, common: { close: 'Close', retry: 'Retry' }, resourceCenter: { notificationReply: '{actor} replied to your comment', notificationComment: '{actor} commented on your post' } },
    zh: { ...notificationZh, common: { close: '关闭', retry: '重试' }, resourceCenter: { notificationReply: '{actor} 回复了你的评论', notificationComment: '{actor} 评论了你的帖子' } },
  }, locale)
  const wrapper = mount(NotificationCenterBell, { props: { userKey: 1, ...props }, global: { plugins: [router, i18n], stubs: { Icon: true } } })
  await flushPromises()
  return { wrapper, router, i18n }
}

describe('NotificationCenterBell', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(listNotifications).mockResolvedValue(page([{ ...notification }]))
    vi.mocked(getUnreadNotificationCount).mockResolvedValue(1)
    vi.mocked(resourceAPI.listNotifications).mockResolvedValue([])
  })

  it('loads domain notifications, leaves announcements independent, and marks one read before navigating', async () => {
    const { wrapper, router } = await render()
    expect(listNotifications).not.toHaveBeenCalled()
    expect(wrapper.get('[role="status"]').text()).toBe('1 unread')
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Workspace changed')
    expect(resourceAPI.listNotifications).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="notification-item-1"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/workspaces/7/members')
    expect(wrapper.get('[role="status"]').text()).toBe('0 unread')
    expect(markNotificationRead).toHaveBeenCalledWith(1)
  })

  it('adapts forum notifications behind the feature flag and preserves their read and navigation APIs', async () => {
    vi.mocked(resourceAPI.listNotifications).mockResolvedValue([{ ...resource }])
    const { wrapper, router } = await render({ includeResource: true })
    expect(wrapper.get('[role="status"]').text()).toBe('2 unread')
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Ari replied to your comment')
    await wrapper.get('[data-testid="notification-resource-9"]').trigger('click')
    await flushPromises()
    expect(resourceAPI.markNotificationRead).toHaveBeenCalledWith(9)
    expect(router.currentRoute.value.path).toBe('/resource-center/posts/25')
  })

  it('filters forum items by category and unread state', async () => {
    vi.mocked(resourceAPI.listNotifications).mockResolvedValue([{ ...resource }, { ...resource, id: 10, read: true }])
    const { wrapper } = await render({ includeResource: true })
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('select[aria-label="Category"]').setValue('workspace')
    await flushPromises()
    expect(wrapper.find('[data-testid="notification-resource-9"]').exists()).toBe(false)
    await wrapper.get('select[aria-label="Category"]').setValue('resources')
    await flushPromises()
    await wrapper.get('[data-testid="notification-filter-unread"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="notification-resource-9"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="notification-resource-10"]').exists()).toBe(false)
  })

  it('loads another domain page without duplicating resource notifications', async () => {
    vi.mocked(listNotifications).mockImplementation(async query => query?.page === 2 ? page([{ ...notification, id: 2, title: 'Another update' }], 2, 2) : page([{ ...notification }], 1, 2))
    const { wrapper } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="notification-load-more"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Another update')
    expect(wrapper.findAll('[data-testid="notification-item-1"]')).toHaveLength(1)
  })

  it('shows a load error with retry instead of an empty-success state', async () => {
    vi.mocked(listNotifications).mockRejectedValueOnce(new Error('offline'))
    const { wrapper } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Notifications could not be loaded.')
    expect(wrapper.text()).not.toContain('No notifications')
    await wrapper.get('[data-testid="notification-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Workspace changed')
  })

  it('keeps an unread item open when its read update fails', async () => {
    vi.mocked(markNotificationRead).mockRejectedValueOnce(new Error('offline'))
    const { wrapper, router } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="notification-item-1"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(wrapper.get('[role="alert"]').text()).toContain('The read status could not be saved.')
    expect(wrapper.get('[data-testid="notification-item-1"]').classes()).toContain('is-unread')
  })

  it('keeps surviving source data and reports an unknown count when a source fails', async () => {
    vi.mocked(getUnreadNotificationCount).mockRejectedValue(new Error('offline'))
    vi.mocked(resourceAPI.listNotifications).mockRejectedValue(new Error('offline'))
    const { wrapper } = await render({ includeResource: true })
    expect(wrapper.get('[role="status"]').text()).toBe('Unread count unavailable')
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Workspace changed')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it('keeps failed resource reads unread after a partially successful bulk read', async () => {
    vi.mocked(resourceAPI.listNotifications).mockResolvedValue([{ ...resource }])
    vi.mocked(resourceAPI.markAllNotificationsRead).mockRejectedValue(new Error('offline'))
    const { wrapper } = await render({ includeResource: true })
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="notification-mark-all"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="notification-item-1"]').classes()).not.toContain('is-unread')
    expect(wrapper.get('[data-testid="notification-resource-9"]').classes()).toContain('is-unread')
    expect(wrapper.get('[role="status"]').text()).toBe('1 unread')
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
  })

  it.each(['//outside.example', '/\\outside.example', '/%2foutside.example'])('rejects unsafe explicit navigation %s and uses the event workspace route', async targetPath => {
    vi.mocked(listNotifications).mockResolvedValue(page([{ ...notification, target_path: targetPath, data: { event_type: 'member.joined' } }]))
    const { wrapper, router } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="notification-item-1"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/workspaces/7/members')
  })

  it('localizes budget threshold triggers and distinguishes rejected requests from finalized spend', async () => {
    vi.mocked(listNotifications).mockResolvedValue(page([
      { ...notification, id: 1, category: 'budget', title: undefined, body: undefined, title_key: 'notifications.budget.title', body_key: 'notifications.budget.body', data: { event_type: 'budget.hard_limit_reached', threshold: 100, reason_code: 'admission_rejected' } },
      { ...notification, id: 2, category: 'budget', title: undefined, body: undefined, title_key: 'notifications.budget.title', body_key: 'notifications.budget.body', data: { event_type: 'budget.hard_limit_reached', threshold: 100, reason_code: 'finalized_spend' } },
    ]))
    const { wrapper, i18n } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="notification-item-1"]').text()).toContain('Request rejected by the budget limit')
    expect(wrapper.get('[data-testid="notification-item-2"]').text()).toContain('Finalized spend crossed the threshold')
    expect(wrapper.text()).toContain('Threshold: 100%')
    i18n.global.locale.value = 'zh'
    await flushPromises()
    expect(wrapper.get('[data-testid="notification-item-1"]').text()).toContain('请求因预算限制被拒绝')
    expect(wrapper.get('[data-testid="notification-item-2"]').text()).toContain('已结算费用达到阈值')
    expect(wrapper.text()).toContain('阈值：100%')
  })

  it('scopes bulk-read to the selected workspace without marking global forum notifications', async () => {
    const { wrapper } = await render({ workspaceId: 7, projectId: 21, includeResource: true })
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('select[aria-label="Notification scope"]').setValue('workspace')
    await flushPromises()
    await wrapper.get('[data-testid="notification-mark-all"]').trigger('click')
    await flushPromises()
    expect(markAllNotificationsRead).toHaveBeenCalledWith({ workspace_id: 7 })
    expect(resourceAPI.markAllNotificationsRead).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="notification-mark-all"]').attributes('disabled')).toBeDefined()
  })

  it('ignores a stale list after the workspace context changes', async () => {
    let resolveOld!: (value: ReturnType<typeof page>) => void
    vi.mocked(listNotifications).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
    const { wrapper } = await render({ workspaceId: 7 })
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await wrapper.setProps({ workspaceId: 8 })
    await flushPromises()
    resolveOld(page([{ ...notification, title: 'Old workspace notice' }]))
    await flushPromises()
    expect(wrapper.text()).toContain('Workspace changed')
    expect(wrapper.text()).not.toContain('Old workspace notice')
  })

  it('ignores a late mark-read result after the user changes', async () => {
    let resolveRead!: () => void
    vi.mocked(markNotificationRead).mockImplementationOnce(() => new Promise(resolve => { resolveRead = resolve }))
    const { wrapper, router } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="notification-item-1"]').trigger('click')
    await wrapper.setProps({ userKey: 2 })
    resolveRead()
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('translates API message keys using the actual locale and updates open items after switching language', async () => {
    vi.mocked(listNotifications).mockResolvedValue(page([{ ...notification, title: undefined, body: undefined, title_key: 'notifications.member.title', body_key: 'notifications.member.body' }]))
    const { wrapper, i18n } = await render()
    await wrapper.get('button[aria-label="Notifications"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Member update')
    i18n.global.locale.value = 'zh'
    await flushPromises()
    expect(wrapper.text()).toContain('成员更新')
    expect(wrapper.get('[role="status"]').text()).toBe('1 条未读通知')
  })
})
