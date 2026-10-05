<template>
  <div ref="root" class="notification-center relative">
    <button
      type="button"
      class="notification-center__trigger relative flex h-9 w-9 items-center justify-center rounded-lg text-[color:var(--color-text-secondary)]"
      :class="{ 'text-[color:var(--color-primary)]': unreadCount !== null && unreadCount > 0 }"
      :aria-label="t('notifications.title')"
      :aria-expanded="open"
      aria-haspopup="dialog"
      @click="toggle"
    >
      <Icon name="bell" size="md" />
      <span v-if="unreadCount !== null && unreadCount > 0" class="notification-center__badge" aria-hidden="true">{{ displayUnreadCount }}</span>
    </button>

    <span class="sr-only" role="status" aria-live="polite" aria-atomic="true">
      {{ unreadLabel }}
    </span>

    <div v-if="open" class="notification-center__popover glass-popover scrollable-popover" role="dialog" :aria-label="t('notifications.title')">
      <header class="notification-center__header">
        <div class="min-w-0">
          <h2>{{ t('notifications.title') }}</h2>
          <p>{{ unreadLabel }}</p>
        </div>
        <div class="notification-center__header-actions">
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            data-testid="notification-mark-all"
            :disabled="loading || mutating || unreadCount === 0"
            @click="markAllRead"
          >
            {{ t('notifications.markAllRead') }}
          </button>
          <button type="button" class="btn btn-icon btn-ghost" :aria-label="t('common.close')" @click="open = false">
            <Icon name="x" size="sm" />
          </button>
        </div>
      </header>

      <div class="notification-center__filters" role="group" :aria-label="t('notifications.filters')">
        <button type="button" class="notification-center__filter" :class="{ 'is-active': filter === 'all' }" :aria-pressed="filter === 'all'" @click="setFilter('all')">
          {{ t('notifications.all') }}
        </button>
        <button type="button" class="notification-center__filter" data-testid="notification-filter-unread" :class="{ 'is-active': filter === 'unread' }" :aria-pressed="filter === 'unread'" @click="setFilter('unread')">
          {{ t('notifications.unread') }}
        </button>
        <select v-model="category" class="notification-center__category" :aria-label="t('notifications.category')">
          <option value="">{{ t('notifications.allCategories') }}</option>
          <option v-for="option in categories" :key="option" :value="option">{{ categoryLabel(option) }}</option>
        </select>
        <select v-model="scope" class="notification-center__category" :aria-label="t('notifications.scope')">
          <option value="all">{{ t('notifications.allScopes') }}</option>
          <option v-if="workspaceId" value="workspace">{{ t('notifications.currentWorkspace') }}</option>
          <option v-if="workspaceId && projectId" value="project">{{ t('notifications.currentProject') }}</option>
        </select>
      </div>

      <div v-if="loadError || countError || readError" class="notification-center__error" role="alert">
        <span>{{ readError ? t('notifications.readError') : t('notifications.loadError') }}</span>
        <button v-if="loadError || countError" type="button" class="btn btn-ghost btn-sm" data-testid="notification-retry" :disabled="loading" @click="reload">{{ t('common.retry') }}</button>
      </div>
      <div v-if="loading && !items.length" class="notification-center__state" role="status">{{ t('notifications.loading') }}</div>
      <div v-else-if="items.length" class="notification-center__list" :aria-busy="loading">
        <button
          v-for="item in items"
          :key="`${item.source}-${item.id}`"
          :data-testid="`notification-${item.source === 'resource' ? 'resource' : 'item'}-${item.id}`"
          type="button"
          class="notification-center__item"
          :class="{ 'is-unread': !item.read }"
          :disabled="mutating"
          @click="openItem(item)"
        >
          <span class="notification-center__item-mark" aria-hidden="true"></span>
          <span class="min-w-0 flex-1 text-left">
            <strong>{{ item.title }}</strong>
            <span v-if="item.body" class="notification-center__body">{{ item.body }}</span>
            <span v-if="item.details" class="notification-center__body">{{ item.details }}</span>
            <time>{{ formatDate(item.created_at) }}</time>
          </span>
          <Icon name="chevronRight" size="sm" class="notification-center__item-arrow" />
        </button>
        <button v-if="hasMore" type="button" class="notification-center__load-more" data-testid="notification-load-more" :disabled="loadingMore || loading || mutating" @click="loadMore">
          {{ loadingMore ? t('notifications.loading') : t('notifications.loadMore') }}
        </button>
      </div>
      <p v-else-if="!loadError" class="notification-center__state">{{ t('notifications.empty') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { listNotifications, getUnreadNotificationCount, markNotificationRead, markAllNotificationsRead } from '@/api/notifications'
import resourceAPI from '@/api/resourceCenter'
import type { ResourceNotification, UserNotification } from '@/types'

interface Props {
  includeResource?: boolean
  workspaceId?: number | null
  projectId?: number | null
  userKey?: number | string | null
}

interface CenterItem {
  source: 'domain' | 'resource'
  id: number | string
  category: string
  title: string
  body: string
  details?: string
  read: boolean
  created_at: string
  targetPath?: string | null
  domain?: UserNotification
  resource?: ResourceNotification
}

const props = withDefaults(defineProps<Props>(), { includeResource: false, workspaceId: null, projectId: null, userKey: null })
const { t, te, locale } = useI18n()
const router = useRouter()
const root = ref<HTMLElement | null>(null)
const open = ref(false)
const filter = ref<'all' | 'unread'>('all')
const category = ref('')
const scope = ref<'all' | 'workspace' | 'project'>('all')
const page = ref(1)
const pages = ref(1)
const domainNotifications = ref<UserNotification[]>([])
const resourceNotifications = ref<ResourceNotification[]>([])
const domainUnread = ref<number | null>(null)
const resourceUnread = ref<number | null>(0)
const loadError = ref(false)
const countError = ref(false)
const readError = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const mutating = ref(false)
let controller: AbortController | null = null
let unreadController: AbortController | null = null
let generation = 0
let unreadGeneration = 0
let contextGeneration = 0

const includesResources = computed(() => props.includeResource && scope.value === 'all')
const unreadCount = computed(() => domainUnread.value === null || resourceUnread.value === null ? null : domainUnread.value + resourceUnread.value)
const unreadLabel = computed(() => unreadCount.value === null ? t('notifications.unreadUnavailable') : t('notifications.unreadCount', { count: unreadCount.value }))
const displayUnreadCount = computed(() => (unreadCount.value || 0) > 99 ? '99+' : String(unreadCount.value || 0))
const hasMore = computed(() => page.value < pages.value)
const categories = computed(() => [...new Set(['system', 'workspace', 'project', 'api_key', 'budget', 'billing', 'quota', 'security', ...(props.includeResource ? ['resources'] : []), ...domainNotifications.value.map(item => item.category)])])
const items = computed(() => [
  ...domainNotifications.value.map(domainItem),
  ...(includesResources.value ? resourceNotifications.value.map(resourceItem) : []),
].filter(item => (!category.value || item.category === category.value) && (filter.value !== 'unread' || !item.read))
  .sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)))

function isUnread(item: UserNotification): boolean {
  return !(item.read === true || item.is_read === true || item.read_at)
}

function localized(key: string | undefined, fallback: string | undefined, data?: Record<string, unknown> | null): string {
  if (fallback) return fallback
  if (!key) return ''
  const translated = t(key, data || {})
  return translated === key ? key.split('.').pop()?.replace(/[_-]+/g, ' ') || key : translated
}

function domainItem(item: UserNotification): CenterItem {
  const data = item.data || {}
  const explicitPath = item.target_path || (typeof data.target_path === 'string' ? data.target_path : typeof data.path === 'string' ? data.path : null)
  let targetPath = safeLocalPath(explicitPath) ? explicitPath : null
  if (!targetPath && item.workspace_id) {
    const base = `/workspaces/${item.workspace_id}`
    const eventType = typeof data.event_type === 'string' ? data.event_type : ''
    if (eventType.startsWith('member.')) targetPath = `${base}/members`
    else if (item.category === 'budget' || item.category === 'billing') targetPath = `${base}/finops`
    else if (eventType === 'webhook.test') targetPath = `${base}/webhooks`
    else if (item.project_id) targetPath = `${base}/projects/${item.project_id}`
    else targetPath = `${base}/overview`
  }
  return {
    source: 'domain', id: item.id, category: item.category, title: localized(item.title_key, item.title, data), body: localized(item.body_key, item.body, data),
    details: eventDetails(data),
    read: !isUnread(item), created_at: item.created_at, targetPath, domain: item,
  }
}

function eventDetails(data: Record<string, unknown>): string {
  const details: string[] = []
  const eventKey = typeof data.event_type === 'string' ? `notifications.events.${data.event_type}` : ''
  if (eventKey && te(eventKey)) details.push(t(eventKey))
  if (typeof data.threshold === 'number' && Number.isFinite(data.threshold)) details.push(t('notifications.threshold', { threshold: new Intl.NumberFormat(locale.value).format(data.threshold) }))
  const reasonKey = typeof data.reason_code === 'string' ? `notifications.reasons.${data.reason_code}` : ''
  if (reasonKey && te(reasonKey)) details.push(t(reasonKey))
  return details.join(' · ')
}

function resourceItem(item: ResourceNotification): CenterItem {
  return {
    source: 'resource', id: item.id, category: 'resources',
    title: item.kind === 'reply' ? t('resourceCenter.notificationReply', { actor: item.actor.username }) : t('resourceCenter.notificationComment', { actor: item.actor.username }),
    body: '', read: item.read, created_at: item.created_at, targetPath: `/resource-center/posts/${item.post_id}`, resource: item,
  }
}

function categoryLabel(value: string): string {
  const translated = t(`notifications.categories.${value}`)
  return translated === `notifications.categories.${value}` ? value : translated
}

function safeLocalPath(value?: string | null): value is string {
  if (!value) return false
  try {
    const decoded = decodeURIComponent(value)
    return decoded.startsWith('/') && !decoded.startsWith('//') && !decoded.includes('\\') && Array.from(decoded).every(character => character.charCodeAt(0) >= 32 && character.charCodeAt(0) !== 127)
  }
  catch { return false }
}

function scopeQuery(): { workspace_id?: number; project_id?: number } {
  if (scope.value === 'workspace' && props.workspaceId) return { workspace_id: props.workspaceId }
  if (scope.value === 'project' && props.workspaceId && props.projectId) return { workspace_id: props.workspaceId, project_id: props.projectId }
  return {}
}

function query(filterPage = 1) {
  return {
    page: filterPage,
    page_size: 20,
    category: category.value || undefined,
    unread: filter.value === 'unread' ? true : undefined,
    ...scopeQuery(),
  }
}

async function refreshUnread(): Promise<void> {
  const current = ++unreadGeneration
  const context = contextGeneration
  unreadController?.abort()
  unreadController = new AbortController()
  const [domainResult, resourceResult] = await Promise.allSettled([
    getUnreadNotificationCount({ ...scopeQuery(), signal: unreadController.signal }),
    includesResources.value ? resourceAPI.listNotifications() : Promise.resolve([] as ResourceNotification[]),
  ])
  if (current !== unreadGeneration || context !== contextGeneration) return
  domainUnread.value = domainResult.status === 'fulfilled' ? domainResult.value : null
  resourceUnread.value = resourceResult.status === 'fulfilled' ? resourceResult.value.filter(item => !item.read).length : null
  countError.value = domainResult.status === 'rejected' || resourceResult.status === 'rejected'
}

async function load(reset = true): Promise<void> {
  const current = ++generation
  const unreadCurrent = ++unreadGeneration
  const context = contextGeneration
  controller?.abort()
  unreadController?.abort()
  const nextController = new AbortController()
  controller = nextController
  if (reset) { page.value = 1; pages.value = 1 }
  loadError.value = false
  readError.value = false
  if (reset) loading.value = true
  else loadingMore.value = true
  try {
    const [domainResult, resourceResult, countResult] = await Promise.allSettled([
      listNotifications({ ...query(reset ? 1 : page.value + 1), signal: nextController.signal }),
      reset && includesResources.value ? resourceAPI.listNotifications() : Promise.resolve(resourceNotifications.value),
      getUnreadNotificationCount({ ...scopeQuery(), signal: nextController.signal }),
    ])
    if (current !== generation || context !== contextGeneration) return
    if (domainResult.status === 'fulfilled') {
      const domainPage = domainResult.value
      const nextItems = reset ? domainPage.items || [] : [...domainNotifications.value, ...(domainPage.items || [])]
      domainNotifications.value = [...new Map(nextItems.map(item => [item.id, item])).values()]
      page.value = domainPage.page || (reset ? 1 : page.value + 1)
      pages.value = domainPage.pages || 1
    }
    if (resourceResult.status === 'fulfilled' && reset) resourceNotifications.value = resourceResult.value
    loadError.value = domainResult.status === 'rejected' || resourceResult.status === 'rejected'
    if (unreadCurrent === unreadGeneration) {
      domainUnread.value = countResult.status === 'fulfilled' ? countResult.value : null
      resourceUnread.value = !includesResources.value ? 0 : resourceResult.status === 'fulfilled' ? resourceResult.value.filter(item => !item.read).length : null
      countError.value = countResult.status === 'rejected' || (includesResources.value && resourceResult.status === 'rejected')
    }
  } finally {
    if (current === generation) { loading.value = false; loadingMore.value = false }
  }
}

async function reload(): Promise<void> { await load(true) }
async function loadMore(): Promise<void> { if (!loadingMore.value && hasMore.value) await load(false) }
async function toggle(): Promise<void> { open.value = !open.value; if (open.value) await load(true) }
function setFilter(value: 'all' | 'unread'): void { filter.value = value }

function stopReads(): void {
  ++generation; ++unreadGeneration
  controller?.abort(); unreadController?.abort()
  loading.value = false; loadingMore.value = false
}

async function openItem(item: CenterItem): Promise<void> {
  if (mutating.value) return
  const context = contextGeneration
  mutating.value = true
  readError.value = false
  try {
    if (!item.read) {
      if (item.source === 'domain') await markNotificationRead(item.id)
      else if (item.resource) await resourceAPI.markNotificationRead(item.resource.id)
      if (context !== contextGeneration) return
      stopReads()
      if (item.source === 'domain') {
        domainNotifications.value = domainNotifications.value.map(value => value.id === item.id ? { ...value, read: true, read_at: new Date().toISOString() } : value)
        if (domainUnread.value !== null) domainUnread.value = Math.max(0, domainUnread.value - 1)
      } else {
        resourceNotifications.value = resourceNotifications.value.map(value => value.id === item.id ? { ...value, read: true } : value)
        if (resourceUnread.value !== null) resourceUnread.value = Math.max(0, resourceUnread.value - 1)
      }
    }
    if (context !== contextGeneration) return
    open.value = false
    if (safeLocalPath(item.targetPath)) await router.push(item.targetPath)
  } catch { if (context === contextGeneration) readError.value = true }
  finally { if (context === contextGeneration) mutating.value = false }
}

async function markAllRead(): Promise<void> {
  if (unreadCount.value === 0 || mutating.value) return
  const context = contextGeneration
  const includeResource = includesResources.value
  mutating.value = true
  readError.value = false
  try {
    const [domainResult, resourceResult] = await Promise.allSettled([
      markAllNotificationsRead(scopeQuery()),
      includeResource ? resourceAPI.markAllNotificationsRead() : Promise.resolve(),
    ])
    if (context !== contextGeneration) return
    stopReads()
    if (domainResult.status === 'fulfilled') {
      domainNotifications.value = domainNotifications.value.map(item => ({ ...item, read: true }))
      domainUnread.value = 0
    }
    if (includeResource && resourceResult.status === 'fulfilled') {
      resourceNotifications.value = resourceNotifications.value.map(item => ({ ...item, read: true }))
      resourceUnread.value = 0
    }
    readError.value = domainResult.status === 'rejected' || resourceResult.status === 'rejected'
  } finally { if (context === contextGeneration) mutating.value = false }
}

function formatDate(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

function handleOutside(event: MouseEvent): void { if (root.value && !root.value.contains(event.target as Node)) open.value = false }
function handleEscape(event: KeyboardEvent): void { if (event.key === 'Escape') open.value = false }

watch(() => [props.workspaceId, props.projectId, props.userKey, props.includeResource, scope.value, category.value, filter.value], () => {
  ++contextGeneration
  stopReads()
  domainNotifications.value = []; resourceNotifications.value = []
  domainUnread.value = null; resourceUnread.value = includesResources.value ? null : 0
  loadError.value = false; countError.value = false; readError.value = false; mutating.value = false
  if ((scope.value === 'workspace' && !props.workspaceId) || (scope.value === 'project' && (!props.workspaceId || !props.projectId))) { scope.value = 'all'; return }
  if (open.value) void load(true)
  else void refreshUnread()
}, { flush: 'sync' })
onMounted(() => { document.addEventListener('click', handleOutside); document.addEventListener('keydown', handleEscape); void refreshUnread() })
onBeforeUnmount(() => { ++contextGeneration; stopReads(); document.removeEventListener('click', handleOutside); document.removeEventListener('keydown', handleEscape) })
</script>

<style scoped>
.notification-center__trigger { transition: color var(--motion-fast) var(--ease-standard), background-color var(--motion-fast) var(--ease-standard); }
.notification-center__trigger:hover, .notification-center__trigger:focus-visible { background: var(--color-surface-soft); }
.notification-center__badge { position: absolute; right: -3px; top: -4px; min-width: 16px; border: 2px solid var(--color-surface); border-radius: 999px; background: var(--color-danger); color: #fff; padding: 0 3px; font-size: 9px; font-weight: 700; line-height: 14px; text-align: center; }
.notification-center__popover { position: absolute; right: 0; top: calc(100% + 8px); z-index: 50; width: min(390px, calc(100vw - 20px)); overflow: hidden; border: 1px solid var(--glass-border); border-radius: 12px; background: var(--color-surface-overlay); box-shadow: var(--shadow-overlay); }
.notification-center__header { display: flex; min-width: 0; align-items: flex-start; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--color-border-subtle); padding: 14px 16px; }
.notification-center__header h2 { margin: 0; color: var(--color-text-primary); font-size: 15px; font-weight: 700; }
.notification-center__header p { margin: 4px 0 0; color: var(--color-text-muted); font-size: 12px; }
.notification-center__header-actions { display: flex; flex-shrink: 0; align-items: center; gap: 4px; }
.notification-center__filters { display: flex; flex-wrap: wrap; gap: 5px; border-bottom: 1px solid var(--color-border-subtle); padding: 9px 12px; }
.notification-center__filter { border: 1px solid transparent; border-radius: 7px; color: var(--color-text-secondary); padding: 5px 8px; font-size: 12px; }
.notification-center__filter:hover, .notification-center__filter:focus-visible { border-color: var(--color-border); color: var(--color-text-primary); }
.notification-center__filter.is-active { border-color: var(--color-primary-border); background: var(--color-primary-soft); color: var(--color-primary); font-weight: 700; }
.notification-center__category { min-width: 0; flex: 1 1 120px; height: 30px; border: 1px solid var(--color-border); border-radius: 7px; background: var(--color-surface); color: var(--color-text-secondary); padding: 0 8px; font-size: 12px; }
.notification-center__list { max-height: min(430px, 60vh); overflow-y: auto; }
.notification-center__item { display: flex; width: 100%; min-width: 0; align-items: flex-start; gap: 9px; border-bottom: 1px solid var(--color-border-subtle); padding: 12px 14px; text-align: left; }
.notification-center__item:hover, .notification-center__item:focus-visible { background: var(--color-surface-soft); }
.notification-center__item-mark { width: 7px; height: 7px; flex: 0 0 7px; margin-top: 5px; border-radius: 50%; background: var(--color-border-strong); }
.notification-center__item.is-unread { background: color-mix(in srgb, var(--color-primary-soft) 55%, transparent); }
.notification-center__item.is-unread .notification-center__item-mark { background: var(--color-primary); }
.notification-center__item strong, .notification-center__body, .notification-center__item time { display: block; min-width: 0; overflow-wrap: anywhere; }
.notification-center__item strong { color: var(--color-text-primary); font-size: 13px; font-weight: 650; }
.notification-center__body { margin-top: 3px; color: var(--color-text-secondary); font-size: 12px; }
.notification-center__item time { margin-top: 5px; color: var(--color-text-muted); font-size: 11px; }
.notification-center__item-arrow { flex-shrink: 0; margin-top: 2px; color: var(--color-text-muted); }
.notification-center__state { margin: 0; padding: 34px 16px; color: var(--color-text-muted); font-size: 13px; text-align: center; }
.notification-center__load-more { width: 100%; border-top: 1px solid var(--color-border-subtle); color: var(--color-primary); padding: 10px; font-size: 12px; }
.notification-center__load-more:disabled { opacity: .55; }
.notification-center__error { display: flex; align-items: center; justify-content: space-between; gap: 8px; border-bottom: 1px solid var(--color-border-subtle); padding: 10px 14px; color: var(--color-danger); font-size: 12px; }
@media (max-width: 480px) { .notification-center__popover { position: fixed; right: 10px; top: 68px; } }
</style>
