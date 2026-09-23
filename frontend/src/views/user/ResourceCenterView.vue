<template>
  <AppLayout>
    <main class="forum-page">
      <div class="forum-shell">
        <section class="forum-hero">
          <div class="forum-hero__copy">
            <div class="forum-hero__topline">
              <span class="forum-kicker">{{ t('resourceCenter.heroEyebrow') }}</span>
              <button type="button" class="forum-hero__publish" @click="composerOpen = true">
                <Icon name="plus" size="sm" aria-hidden="true" />
                {{ t('resourceCenter.newPost') }}
              </button>
            </div>
            <h1>{{ t('resourceCenter.heroTitle') }}</h1>
            <p>{{ t('resourceCenter.heroDescription') }}</p>

            <div class="forum-hero__stats">
              <div>
                <strong>{{ postsPage.total }}</strong>
                <span>{{ t('resourceCenter.statsPosts') }}</span>
              </div>
              <div>
                <strong>{{ categories.length }}</strong>
                <span>{{ t('resourceCenter.statsCategories') }}</span>
              </div>
              <div>
                <strong>{{ unreadCount }}</strong>
                <span>{{ t('resourceCenter.statsUnread') }}</span>
              </div>
            </div>
          </div>

          <div class="forum-hero__visual" aria-hidden="true">
            <span class="forum-bubble forum-bubble--one"><Icon name="chatBubble" size="lg" /></span>
            <span class="forum-bubble forum-bubble--two"><Icon name="user" size="md" /></span>
            <span class="forum-bubble forum-bubble--three"><Icon name="check" size="md" /></span>
            <div class="forum-core">
              <span class="forum-core__mark">M</span>
            </div>
            <span class="forum-visual-copy">
              <strong>Share</strong>
              <strong>Learn</strong>
              <strong>Build Together</strong>
            </span>
          </div>
        </section>

        <section class="forum-disclaimer">
          <Icon name="exclamationTriangle" size="sm" aria-hidden="true" />
          <p>{{ t('resourceCenter.disclaimer') }}</p>
        </section>

        <section v-if="notificationOpen" class="forum-notifications">
          <header>
            <div>
              <span class="forum-section-kicker">{{ t('resourceCenter.activityEyebrow') }}</span>
              <h2>{{ t('resourceCenter.notifications') }}</h2>
            </div>
            <div class="forum-notifications__actions">
              <button type="button" :disabled="!unreadCount" @click="markAllRead">
                {{ t('resourceCenter.markAllRead') }}
              </button>
              <button type="button" :aria-label="t('resourceCenter.close')" @click="notificationOpen = false">
                <Icon name="x" size="sm" />
              </button>
            </div>
          </header>

          <div v-if="notifications.length" class="forum-notifications__list">
            <button
              v-for="item in notifications"
              :key="item.id"
              type="button"
              class="forum-notification"
              :class="{ 'is-read': item.read }"
              @click="openNotification(item)"
            >
              <span class="forum-notification__dot"></span>
              <span>
                <strong>
                  {{
                    item.kind === 'reply'
                      ? t('resourceCenter.notificationReply', { actor: item.actor.username })
                      : t('resourceCenter.notificationComment', { actor: item.actor.username })
                  }}
                </strong>
                <time>{{ formatDate(item.created_at) }}</time>
              </span>
              <Icon name="chevronRight" size="sm" aria-hidden="true" />
            </button>
          </div>
          <div v-else class="forum-notifications__empty">
            <Icon name="bell" size="md" aria-hidden="true" />
            <span>{{ t('resourceCenter.noNotifications') }}</span>
          </div>
        </section>

        <div class="forum-layout">
          <section class="forum-feed">
            <div class="forum-controls">
              <div class="forum-categories" role="tablist" :aria-label="t('resourceCenter.categories')">
                <button
                  type="button"
                  class="forum-category"
                  :class="{ 'is-active': selectedCategory === 0 }"
                  role="tab"
                  :aria-selected="selectedCategory === 0"
                  @click="selectCategory(0)"
                >
                  {{ t('resourceCenter.allCategories') }}
                  <span>{{ postsPage.total }}</span>
                </button>
                <button
                  v-for="category in categories"
                  :key="category.id"
                  type="button"
                  class="forum-category"
                  :class="{ 'is-active': selectedCategory === category.id }"
                  role="tab"
                  :aria-selected="selectedCategory === category.id"
                  @click="selectCategory(category.id)"
                >
                  {{ category.name }}
                </button>
              </div>

              <div class="forum-search-row">
                <label class="forum-search">
                  <Icon name="search" size="sm" aria-hidden="true" />
                  <input
                    v-model="query"
                    type="search"
                    :placeholder="t('resourceCenter.searchPlaceholder')"
                    @keyup.enter="searchPosts"
                  />
                </label>
                <select v-model="sortBy" class="forum-sort" :aria-label="t('resourceCenter.sortLabel')" @change="searchPosts">
                  <option value="latest">{{ t('resourceCenter.latest') }}</option>
                  <option value="popular">{{ t('resourceCenter.popular') }}</option>
                </select>
                <button type="button" class="forum-search-button" :aria-label="t('resourceCenter.search')" @click="searchPosts">
                  <Icon name="search" size="sm" />
                  <span>{{ t('resourceCenter.search') }}</span>
                </button>
              </div>
            </div>

            <div v-if="loading" class="forum-loading" aria-live="polite">
              <span v-for="i in 4" :key="i" class="forum-post forum-post--skeleton">
                <span class="forum-skeleton forum-skeleton--tag"></span>
                <span class="forum-skeleton forum-skeleton--title"></span>
                <span class="forum-skeleton forum-skeleton--body"></span>
                <span class="forum-skeleton forum-skeleton--meta"></span>
              </span>
            </div>

            <section v-else-if="!posts.length" class="forum-empty">
              <span class="forum-empty__icon"><Icon name="chatBubble" size="lg" aria-hidden="true" /></span>
              <h2>{{ t('resourceCenter.emptyTitle') }}</h2>
              <p>{{ t('resourceCenter.noPosts') }}</p>
              <button type="button" @click="composerOpen = true">
                <Icon name="plus" size="sm" />
                {{ t('resourceCenter.newPost') }}
              </button>
            </section>

            <div v-else class="forum-posts">
              <article v-for="post in posts" :key="post.id" class="forum-post">
                <div class="forum-post__main">
                  <RouterLink :to="'/resource-center/posts/' + post.id" class="forum-post__content">
                    <div class="forum-post__topline">
                      <span class="forum-post__category">{{ post.category.name }}</span>
                      <span v-if="post.author.role === 'admin'" class="forum-post__official">
                        <Icon name="badge" size="xs" aria-hidden="true" />
                        {{ t('resourceCenter.official') }}
                      </span>
                      <time>{{ formatDate(post.created_at) }}</time>
                    </div>
                    <h2>{{ post.title }}</h2>
                    <ResourceRichContent class="forum-post__excerpt" :content="post.content" />
                  </RouterLink>

                  <div class="forum-post__actions">
                    <button
                      type="button"
                      :class="{ 'is-active': post.liked }"
                      :aria-label="post.liked ? t('resourceCenter.liked') : t('resourceCenter.like')"
                      @click="togglePostLike(post)"
                    >
                      <Icon name="check" size="sm" />
                    </button>
                    <button
                      v-if="canDeletePost(post)"
                      type="button"
                      class="is-danger"
                      :title="t('admin.resourceCenter.deletePost')"
                      @click="removePost(post)"
                    >
                      <Icon name="trash" size="sm" />
                    </button>
                  </div>
                </div>

                <footer class="forum-post__footer">
                  <span class="forum-author">
                    <span class="forum-author__avatar">{{ avatarInitial(post.author.username) }}</span>
                    <span>{{ post.author.username }}</span>
                  </span>
                  <span class="forum-post__metric">
                    <Icon name="chatBubble" size="xs" aria-hidden="true" />
                    {{ post.comment_count }} {{ t('resourceCenter.comments') }}
                  </span>
                  <span class="forum-post__metric">
                    <Icon name="check" size="xs" aria-hidden="true" />
                    {{ post.like_count }} {{ t('resourceCenter.likes') }}
                  </span>
                  <span class="forum-post__metric">{{ post.view_count }} {{ t('resourceCenter.views') }}</span>
                </footer>
              </article>
            </div>

            <div v-if="postsPage.total" class="forum-pagination">
              <Pagination
                :page="postsPage.page"
                :total="postsPage.total"
                :page-size="postsPage.page_size"
                :show-page-size-selector="false"
                @update:page="changePage"
              />
            </div>
          </section>

          <aside class="forum-sidebar">
            <section class="forum-side-card">
              <div class="forum-side-card__heading">
                <span class="forum-side-card__icon"><Icon name="chatBubble" size="sm" /></span>
                <div>
                  <span class="forum-section-kicker">{{ t('resourceCenter.guideEyebrow') }}</span>
                  <h2>{{ t('resourceCenter.guideTitle') }}</h2>
                </div>
              </div>
              <ul class="forum-guide-list">
                <li>
                  <span><Icon name="user" size="sm" /></span>
                  <div><strong>{{ t('resourceCenter.guideRespectTitle') }}</strong><p>{{ t('resourceCenter.guideRespectDescription') }}</p></div>
                </li>
                <li>
                  <span><Icon name="check" size="sm" /></span>
                  <div><strong>{{ t('resourceCenter.guideVerifyTitle') }}</strong><p>{{ t('resourceCenter.guideVerifyDescription') }}</p></div>
                </li>
                <li>
                  <span><Icon name="exclamationTriangle" size="sm" /></span>
                  <div><strong>{{ t('resourceCenter.guideSafetyTitle') }}</strong><p>{{ t('resourceCenter.guideSafetyDescription') }}</p></div>
                </li>
              </ul>
            </section>

            <button type="button" class="forum-side-publish" @click="composerOpen = true">
              <Icon name="plus" size="sm" aria-hidden="true" />
              {{ t('resourceCenter.newPost') }}
            </button>

            <section class="forum-side-card">
              <div class="forum-side-card__heading">
                <span class="forum-side-card__icon"><Icon name="search" size="sm" /></span>
                <div>
                  <span class="forum-section-kicker">{{ t('resourceCenter.categoryEyebrow') }}</span>
                  <h2>{{ t('resourceCenter.categories') }}</h2>
                </div>
              </div>
              <div class="forum-tags">
                <button
                  v-for="category in categories"
                  :key="category.id"
                  type="button"
                  :class="{ 'is-active': selectedCategory === category.id }"
                  @click="selectCategory(category.id)"
                >
                  # {{ category.name }}
                </button>
              </div>
            </section>

            <section class="forum-side-card forum-side-card--stats">
              <div>
                <strong>{{ postsPage.total }}</strong>
                <span>{{ t('resourceCenter.statsPosts') }}</span>
              </div>
              <div>
                <strong>{{ posts.length }}</strong>
                <span>{{ t('resourceCenter.statsVisible') }}</span>
              </div>
              <div>
                <strong>{{ unreadCount }}</strong>
                <span>{{ t('resourceCenter.statsUnread') }}</span>
              </div>
            </section>

            <button type="button" class="forum-side-activity" @click="notificationOpen = !notificationOpen">
              <span>
                <Icon name="bell" size="sm" />
                {{ t('resourceCenter.notifications') }}
              </span>
              <span v-if="unreadCount" class="forum-unread-badge">{{ unreadCount }}</span>
              <Icon v-else name="chevronRight" size="sm" />
            </button>
          </aside>
        </div>
      </div>
    </main>

    <div v-if="composerOpen" class="forum-modal" role="presentation" @click.self="composerOpen = false">
      <form class="forum-composer" @submit.prevent="publishPost">
        <header class="forum-composer__header">
          <div>
            <span class="forum-section-kicker">{{ t('resourceCenter.composerEyebrow') }}</span>
            <h2>{{ t('resourceCenter.newPost') }}</h2>
            <p>{{ t('resourceCenter.composerDescription') }}</p>
          </div>
          <button type="button" :aria-label="t('resourceCenter.close')" @click="composerOpen = false">
            <Icon name="x" size="md" />
          </button>
        </header>

        <div class="forum-composer__body">
          <label>
            <span>{{ t('resourceCenter.postTitle') }}</span>
            <input v-model="postForm.title" required maxlength="120" class="forum-field" :placeholder="t('resourceCenter.postTitlePlaceholder')" />
          </label>
          <label>
            <span>{{ t('resourceCenter.categories') }}</span>
            <select v-model.number="postForm.category_id" required class="forum-field">
              <option :value="0" disabled>{{ t('resourceCenter.chooseCategory') }}</option>
              <option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option>
            </select>
          </label>
          <div>
            <span class="forum-composer__label">{{ t('resourceCenter.postContent') }}</span>
            <ResourceRichTextEditor v-model="postForm.content" :placeholder="t('resourceCenter.postContent')" :max-length="10000" />
          </div>
        </div>

        <footer class="forum-composer__footer">
          <span><Icon name="exclamationTriangle" size="sm" />{{ t('resourceCenter.composerSafetyHint') }}</span>
          <div>
            <button type="button" class="forum-composer__cancel" @click="composerOpen = false">{{ t('common.cancel') }}</button>
            <button type="submit" class="forum-composer__submit" :disabled="postSubmitting || !richTextHasText(postForm.content)">
              {{ postSubmitting ? t('common.submitting') : t('resourceCenter.publish') }}
            </button>
          </div>
        </footer>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import ResourceRichContent from '@/components/resource/ResourceRichContent.vue'
import ResourceRichTextEditor from '@/components/resource/ResourceRichTextEditor.vue'
import resourceAPI from '@/api/resourceCenter'
import adminResourceAPI from '@/api/admin/resourceCenter'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { ResourceCategory, ResourceNotification, ResourcePageInfo, ResourcePost } from '@/types'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const categories = ref<ResourceCategory[]>([])
const posts = ref<ResourcePost[]>([])
const postsPage = ref<ResourcePageInfo>({ page: 1, page_size: 10, total: 0, has_more: false })
const selectedCategory = ref(0)
const query = ref('')
const sortBy = ref<'latest' | 'popular'>('latest')
const loading = ref(true)
const notifications = ref<ResourceNotification[]>([])
const notificationOpen = ref(false)
const composerOpen = ref(false)
const postSubmitting = ref(false)
const postForm = reactive({ category_id: 0, title: '', content: '' })

const unreadCount = computed(() => notifications.value.filter(item => !item.read).length)

function avatarInitial(value: string): string {
  return value.trim().slice(0, 1).toUpperCase() || 'U'
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function errorMessage(error: unknown): string {
  const candidate = error as { message?: string; code?: string }
  if (candidate?.code === 'RESOURCE_CONTENT_BLOCKED') return t('resourceCenter.blockedContent')
  if (candidate?.code === 'RESOURCE_TITLE_TOO_LONG') return t('resourceCenter.titleTooLong')
  if (candidate?.code === 'RESOURCE_CONTENT_TOO_LONG') return t('resourceCenter.contentTooLong')
  return candidate?.message || t('resourceCenter.loadFailed')
}

function richTextHasText(value: string): boolean {
  const node = document.createElement('div')
  node.innerHTML = value
  return Boolean((node.textContent || '').replace(/\u00a0/g, ' ').trim())
}

async function loadPosts(page = 1): Promise<void> {
  loading.value = true
  try {
    const result = await resourceAPI.listPosts({
      category_id: selectedCategory.value || undefined,
      q: query.value.trim() || undefined,
      sort: sortBy.value,
      page,
      page_size: 10,
    })
    posts.value = result.items
    postsPage.value = result
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    loading.value = false
  }
}

async function loadNotifications(): Promise<void> {
  try {
    notifications.value = await resourceAPI.listNotifications()
  } catch {
    notifications.value = []
  }
}

async function load(): Promise<void> {
  try {
    const [categoryData] = await Promise.all([resourceAPI.listCategories(), loadPosts(), loadNotifications()])
    categories.value = categoryData
    if (!postForm.category_id && categoryData.length) postForm.category_id = categoryData[0].id
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

function searchPosts(): void {
  void loadPosts(1)
}

function selectCategory(id: number): void {
  selectedCategory.value = id
  void loadPosts(1)
}

function changePage(page: number): void {
  void loadPosts(page)
}

async function togglePostLike(post: ResourcePost): Promise<void> {
  try {
    const result = await resourceAPI.togglePostLike(post.id)
    post.liked = result.liked
    post.like_count += result.liked ? 1 : -1
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

function canDeletePost(post: ResourcePost): boolean {
  return authStore.isAdmin || authStore.user?.id === post.author.id
}

async function removePost(post: ResourcePost): Promise<void> {
  if (!window.confirm(t('admin.resourceCenter.confirmDeletePost'))) return
  try {
    if (authStore.isAdmin) await adminResourceAPI.deletePost(post.id)
    else await resourceAPI.deletePost(post.id)
    appStore.showSuccess(t('common.deleted'))
    await loadPosts(postsPage.value.page)
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function publishPost(): Promise<void> {
  if (!postForm.category_id || !postForm.title.trim() || !richTextHasText(postForm.content)) return
  postSubmitting.value = true
  try {
    const post = await resourceAPI.createPost({
      category_id: postForm.category_id,
      title: postForm.title.trim(),
      content: postForm.content,
    })
    appStore.showSuccess(t('resourceCenter.postPublished'))
    composerOpen.value = false
    postForm.title = ''
    postForm.content = ''
    await router.push('/resource-center/posts/' + post.id)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    postSubmitting.value = false
  }
}

async function markAllRead(): Promise<void> {
  await resourceAPI.markAllNotificationsRead()
  notifications.value = notifications.value.map(item => ({ ...item, read: true }))
}

async function openNotification(item: ResourceNotification): Promise<void> {
  if (!item.read) {
    await resourceAPI.markNotificationRead(item.id)
    item.read = true
  }
  notificationOpen.value = false
  await router.push('/resource-center/posts/' + item.post_id)
}

onMounted(() => {
  void load()
})
</script>

<style scoped>
.forum-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.forum-shell {
  width: min(1480px, calc(100% - 40px));
  margin-inline: auto;
  padding: 26px 0 48px;
}

.forum-hero {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(360px, .75fr);
  min-height: 300px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.forum-hero__copy {
  padding: 32px 36px;
}

.forum-hero__topline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.forum-kicker,
.forum-section-kicker {
  color: var(--color-primary);
  font-size: .7rem;
  font-weight: 750;
  letter-spacing: .08em;
  text-transform: uppercase;
}

.forum-hero__publish {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 7px;
  padding: 0 14px;
  border: 1px solid var(--color-primary);
  border-radius: 10px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 650;
}

.forum-hero h1 {
  max-width: 760px;
  margin: 22px 0 0;
  font-size: clamp(2rem, 3.3vw, 3.05rem);
  font-weight: 720;
  line-height: 1.13;
  letter-spacing: -.045em;
}

.forum-hero__copy > p {
  max-width: 720px;
  margin: 12px 0 0;
  color: var(--color-text-secondary);
  font-size: .9rem;
  line-height: 1.7;
}

.forum-hero__stats {
  display: flex;
  flex-wrap: wrap;
  gap: 28px;
  margin-top: 26px;
}

.forum-hero__stats div {
  display: grid;
  gap: 2px;
}

.forum-hero__stats strong {
  color: var(--color-primary);
  font-size: 1.05rem;
  font-weight: 720;
}

.forum-hero__stats span {
  color: var(--color-text-muted);
  font-size: .68rem;
}

.forum-hero__visual {
  position: relative;
  overflow: hidden;
  border-left: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}

.forum-hero__visual::before,
.forum-hero__visual::after {
  position: absolute;
  border: 1px solid var(--color-primary-border);
  border-radius: 50%;
  content: '';
}

.forum-hero__visual::before {
  width: 330px;
  height: 330px;
  top: -100px;
  right: -55px;
  opacity: .5;
}

.forum-hero__visual::after {
  width: 210px;
  height: 210px;
  left: 40px;
  bottom: -95px;
  border-color: color-mix(in srgb, var(--color-accent) 25%, var(--color-border));
}

.forum-core {
  position: absolute;
  z-index: 3;
  top: 49%;
  left: 55%;
  display: grid;
  width: 88px;
  height: 88px;
  place-items: center;
  border: 1px solid var(--color-primary-border);
  border-radius: 24px;
  background: var(--color-surface-raised);
  box-shadow: var(--shadow-lg);
  transform: translate(-50%, -50%) rotate(5deg);
}

.forum-core__mark {
  display: grid;
  width: 56px;
  height: 56px;
  place-items: center;
  border-radius: 15px;
  background: var(--color-accent);
  color: white;
  font-size: 1.5rem;
  font-weight: 800;
}

.forum-bubble {
  position: absolute;
  z-index: 2;
  display: grid;
  place-items: center;
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  background: var(--color-surface);
  color: var(--color-primary);
  box-shadow: var(--shadow-sm);
}

.forum-bubble--one {
  top: 62px;
  left: 80px;
  width: 66px;
  height: 48px;
}

.forum-bubble--two {
  top: 90px;
  right: 64px;
  width: 52px;
  height: 52px;
  color: var(--color-accent);
}

.forum-bubble--three {
  left: 112px;
  bottom: 60px;
  width: 52px;
  height: 52px;
}

.forum-visual-copy {
  position: absolute;
  right: 28px;
  bottom: 28px;
  display: grid;
  color: var(--color-primary);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: .7rem;
  line-height: 1.35;
}

.forum-disclaimer {
  display: flex;
  align-items: flex-start;
  gap: 9px;
  margin-top: 12px;
  padding: 10px 12px;
  border: 1px solid color-mix(in srgb, var(--color-warning) 24%, var(--color-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-warning) 6%, var(--color-surface));
  color: var(--color-text-secondary);
  font-size: .72rem;
  line-height: 1.5;
}

.forum-disclaimer svg {
  flex: 0 0 auto;
  margin-top: 1px;
  color: var(--color-warning);
}

.forum-notifications {
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.forum-notifications > header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.forum-notifications h2 {
  margin: 3px 0 0;
  font-size: .9rem;
  font-weight: 680;
}

.forum-notifications__actions {
  display: flex;
  align-items: center;
  gap: 7px;
}

.forum-notifications__actions button {
  min-height: 32px;
  padding: 0 9px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .7rem;
}

.forum-notifications__actions button:disabled {
  cursor: not-allowed;
  opacity: .45;
}

.forum-notifications__list {
  display: grid;
}

.forum-notification {
  display: grid;
  grid-template-columns: 9px minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border: 0;
  border-bottom: 1px solid var(--color-border-subtle);
  background: transparent;
  color: var(--color-text-primary);
  cursor: pointer;
  text-align: left;
}

.forum-notification:last-child {
  border-bottom: 0;
}

.forum-notification:hover {
  background: var(--color-surface-soft);
}

.forum-notification.is-read {
  opacity: .6;
}

.forum-notification__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-primary);
}

.forum-notification.is-read .forum-notification__dot {
  background: var(--color-text-disabled);
}

.forum-notification strong {
  display: block;
  font-size: .76rem;
  font-weight: 620;
}

.forum-notification time {
  display: block;
  margin-top: 3px;
  color: var(--color-text-muted);
  font-size: .65rem;
}

.forum-notifications__empty {
  display: flex;
  min-height: 110px;
  align-items: center;
  justify-content: center;
  gap: 8px;
  color: var(--color-text-muted);
  font-size: .76rem;
}

.forum-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 270px;
  gap: 14px;
  margin-top: 16px;
}

.forum-feed,
.forum-sidebar {
  min-width: 0;
}

.forum-controls {
  display: grid;
  gap: 10px;
}

.forum-categories {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.forum-category {
  display: inline-flex;
  min-height: 36px;
  align-items: center;
  gap: 6px;
  padding: 0 12px;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .76rem;
  font-weight: 600;
}

.forum-category span {
  min-width: 18px;
  padding: 2px 5px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .63rem;
  text-align: center;
}

.forum-category:hover {
  border-color: var(--color-primary-border);
  color: var(--color-primary);
}

.forum-category.is-active {
  border-color: var(--color-primary);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-search-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 150px auto;
  gap: 8px;
}

.forum-search {
  position: relative;
  min-width: 0;
}

.forum-search svg {
  position: absolute;
  z-index: 2;
  top: 50%;
  left: 12px;
  color: var(--color-text-muted);
  transform: translateY(-50%);
}

.forum-search input,
.forum-sort {
  width: 100%;
  min-height: 40px;
  border: 1px solid var(--color-border-strong);
  border-radius: 10px;
  outline: none;
  background: var(--color-surface);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .78rem;
}

.forum-search input {
  padding: 0 12px 0 36px;
}

.forum-sort {
  padding: 0 10px;
}

.forum-search input:focus,
.forum-sort:focus,
.forum-field:focus {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px var(--color-primary-ring);
}

.forum-search-button {
  display: inline-flex;
  min-width: 84px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  border: 1px solid var(--color-primary);
  border-radius: 10px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .76rem;
  font-weight: 650;
}

.forum-posts,
.forum-loading {
  display: grid;
  gap: 9px;
  margin-top: 12px;
}

.forum-pagination {
  margin-top: 20px;
}

.forum-post {
  min-width: 0;
  padding: 16px;
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
  transition:
    border-color var(--motion-base) var(--ease-standard),
    box-shadow var(--motion-base) var(--ease-standard),
    transform var(--motion-base) var(--ease-standard);
}

.forum-post:hover {
  border-color: var(--color-primary-border);
  box-shadow: var(--shadow-sm);
  transform: translateY(-1px);
}

.forum-post__main {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.forum-post__content {
  min-width: 0;
  flex: 1 1 auto;
  color: inherit;
  text-decoration: none;
}

.forum-post__topline {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
  color: var(--color-text-muted);
  font-size: .66rem;
}

.forum-post__category,
.forum-post__official {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  gap: 4px;
  padding: 0 7px;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-weight: 650;
}

.forum-post__official {
  background: color-mix(in srgb, var(--color-warning) 9%, var(--color-surface));
  color: var(--color-warning);
}

.forum-post h2 {
  margin: 9px 0 0;
  overflow-wrap: anywhere;
  color: var(--color-text-primary);
  font-size: .96rem;
  font-weight: 680;
  line-height: 1.4;
}

.forum-post__content:hover h2 {
  color: var(--color-primary);
}

.forum-post__excerpt {
  display: -webkit-box;
  max-height: 2.9rem;
  margin-top: 5px;
  overflow: hidden;
  color: var(--color-text-secondary);
  font-size: .75rem;
  line-height: 1.48;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.forum-post__actions {
  display: flex;
  flex: 0 0 auto;
  gap: 4px;
}

.forum-post__actions button {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
}

.forum-post__actions button:hover,
.forum-post__actions button.is-active {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-post__actions button.is-danger:hover {
  background: color-mix(in srgb, var(--color-danger) 8%, transparent);
  color: var(--color-danger);
}

.forum-post__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px;
  margin-top: 13px;
  padding-top: 11px;
  border-top: 1px solid var(--color-border-subtle);
  color: var(--color-text-muted);
  font-size: .66rem;
}

.forum-author,
.forum-post__metric {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}

.forum-author {
  color: var(--color-text-secondary);
  font-weight: 600;
}

.forum-author__avatar {
  display: grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-accent-soft);
  color: var(--color-accent);
  font-size: .65rem;
  font-weight: 700;
}

.forum-empty {
  display: grid;
  min-height: 380px;
  place-items: center;
  align-content: center;
  gap: 8px;
  margin-top: 12px;
  padding: 32px;
  border: 1px dashed var(--color-border-strong);
  border-radius: 16px;
  background: var(--color-surface);
  text-align: center;
}

.forum-empty__icon {
  display: grid;
  width: 58px;
  height: 58px;
  place-items: center;
  margin-bottom: 4px;
  border-radius: 18px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-empty h2 {
  margin: 0;
  font-size: .98rem;
  font-weight: 680;
}

.forum-empty p {
  margin: 0;
  color: var(--color-text-muted);
  font-size: .76rem;
}

.forum-empty button {
  display: inline-flex;
  min-height: 38px;
  align-items: center;
  gap: 7px;
  margin-top: 8px;
  padding: 0 14px;
  border: 1px solid var(--color-primary);
  border-radius: 9px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .76rem;
  font-weight: 650;
}

.forum-post--skeleton {
  display: grid;
  gap: 10px;
}

.forum-skeleton {
  display: block;
  border-radius: 7px;
  background: var(--color-surface-soft);
  animation: forum-pulse 1.5s ease-in-out infinite;
}

.forum-skeleton--tag { width: 80px; height: 20px; }
.forum-skeleton--title { width: 52%; height: 18px; }
.forum-skeleton--body { width: 90%; height: 36px; }
.forum-skeleton--meta { width: 36%; height: 18px; }

.forum-sidebar {
  display: grid;
  align-content: start;
  gap: 10px;
}

.forum-side-card,
.forum-side-activity {
  border: 1px solid var(--color-border);
  border-radius: 14px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.forum-side-card {
  padding: 14px;
}

.forum-side-card__heading {
  display: flex;
  align-items: flex-start;
  gap: 9px;
}

.forum-side-card__icon {
  display: grid;
  width: 32px;
  height: 32px;
  flex: 0 0 32px;
  place-items: center;
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-side-card h2 {
  margin: 3px 0 0;
  font-size: .84rem;
  font-weight: 670;
}

.forum-guide-list {
  display: grid;
  gap: 12px;
  margin: 14px 0 0;
  padding: 0;
  list-style: none;
}

.forum-guide-list li {
  display: grid;
  grid-template-columns: 30px minmax(0, 1fr);
  gap: 9px;
  align-items: start;
}

.forum-guide-list li > span {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  border-radius: 9px;
  background: var(--color-surface-soft);
  color: var(--color-primary);
}

.forum-guide-list strong {
  display: block;
  font-size: .73rem;
  font-weight: 650;
}

.forum-guide-list p {
  margin: 3px 0 0;
  color: var(--color-text-muted);
  font-size: .67rem;
  line-height: 1.5;
}

.forum-side-publish {
  display: flex;
  min-height: 42px;
  align-items: center;
  justify-content: center;
  gap: 7px;
  border: 1px solid var(--color-primary);
  border-radius: 11px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .78rem;
  font-weight: 650;
}

.forum-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 12px;
}

.forum-tags button {
  min-height: 28px;
  padding: 0 8px;
  border: 0;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .65rem;
}

.forum-tags button:hover,
.forum-tags button.is-active {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-side-card--stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.forum-side-card--stats div {
  display: grid;
  min-width: 0;
  justify-items: center;
  gap: 2px;
  padding: 4px;
}

.forum-side-card--stats strong {
  color: var(--color-primary);
  font-size: .88rem;
  font-weight: 700;
}

.forum-side-card--stats span {
  color: var(--color-text-muted);
  font-size: .6rem;
  text-align: center;
}

.forum-side-activity {
  display: flex;
  min-height: 42px;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 0 12px;
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .74rem;
  font-weight: 600;
}

.forum-side-activity > span:first-child {
  display: flex;
  align-items: center;
  gap: 7px;
}

.forum-unread-badge {
  display: grid;
  min-width: 22px;
  height: 22px;
  place-items: center;
  border-radius: 999px;
  background: var(--color-danger);
  color: white;
  font-size: .62rem;
  font-weight: 700;
}

.forum-modal {
  position: fixed;
  z-index: 70;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: rgba(15, 23, 42, .42);
  -webkit-backdrop-filter: blur(5px);
  backdrop-filter: blur(5px);
}

.forum-composer {
  width: min(720px, 100%);
  max-height: min(820px, calc(100dvh - 40px));
  overflow-y: auto;
  border: 1px solid var(--color-border);
  border-radius: 18px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xl);
}

.forum-composer__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 20px 22px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.forum-composer__header h2 {
  margin: 4px 0 0;
  font-size: 1.15rem;
  font-weight: 700;
}

.forum-composer__header p {
  margin: 5px 0 0;
  color: var(--color-text-muted);
  font-size: .72rem;
}

.forum-composer__header > button {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border: 0;
  border-radius: 9px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  cursor: pointer;
}

.forum-composer__body {
  display: grid;
  gap: 14px;
  padding: 20px 22px;
}

.forum-composer__body label {
  display: grid;
  gap: 7px;
  color: var(--color-text-secondary);
  font-size: .74rem;
  font-weight: 650;
}

.forum-composer__label {
  display: block;
  margin-bottom: 7px;
  color: var(--color-text-secondary);
  font-size: .74rem;
  font-weight: 650;
}

.forum-field {
  width: 100%;
  min-height: 42px;
  padding: 0 11px;
  border: 1px solid var(--color-border-strong);
  border-radius: 10px;
  outline: none;
  background: var(--color-bg-subtle);
  color: var(--color-text-primary);
  font: inherit;
  font-size: .78rem;
}

.forum-composer__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 22px 20px;
}

.forum-composer__footer > span {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  color: var(--color-text-muted);
  font-size: .66rem;
  line-height: 1.45;
}

.forum-composer__footer > div {
  display: flex;
  gap: 8px;
}

.forum-composer__cancel,
.forum-composer__submit {
  min-height: 38px;
  padding: 0 14px;
  border-radius: 9px;
  cursor: pointer;
  font: inherit;
  font-size: .75rem;
  font-weight: 650;
}

.forum-composer__cancel {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-text-secondary);
}

.forum-composer__submit {
  border: 1px solid var(--color-primary);
  background: var(--color-primary);
  color: var(--color-surface);
}

.forum-composer__submit:disabled {
  cursor: not-allowed;
  opacity: .5;
}

.forum-hero__publish:focus-visible,
.forum-category:focus-visible,
.forum-search-button:focus-visible,
.forum-side-publish:focus-visible,
.forum-side-activity:focus-visible,
.forum-tags button:focus-visible,
.forum-composer button:focus-visible,
.forum-notification:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

@keyframes forum-pulse {
  0%, 100% { opacity: .5; }
  50% { opacity: 1; }
}

@media (max-width: 1160px) {
  .forum-hero {
    grid-template-columns: minmax(0, 1fr) 320px;
  }

  .forum-layout {
    grid-template-columns: minmax(0, 1fr) 240px;
  }
}

@media (max-width: 900px) {
  .forum-shell {
    width: min(100% - 28px, 820px);
  }

  .forum-hero {
    grid-template-columns: 1fr;
  }

  .forum-hero__visual {
    min-height: 230px;
    border-top: 1px solid var(--color-border-subtle);
    border-left: 0;
  }

  .forum-layout {
    grid-template-columns: 1fr;
  }

  .forum-sidebar {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .forum-side-publish,
  .forum-side-activity {
    min-height: auto;
  }
}

@media (max-width: 640px) {
  .forum-shell {
    width: min(100% - 20px, 560px);
    padding-top: 18px;
  }

  .forum-hero {
    min-height: auto;
    border-radius: 14px;
  }

  .forum-hero__copy {
    padding: 22px 20px;
  }

  .forum-hero__topline {
    align-items: flex-start;
  }

  .forum-hero__publish {
    flex: 0 0 auto;
    min-height: 36px;
    padding-inline: 11px;
  }

  .forum-hero h1 {
    font-size: 1.9rem;
  }

  .forum-hero__stats {
    gap: 20px;
  }

  .forum-hero__visual {
    display: none;
  }

  .forum-search-row {
    grid-template-columns: minmax(0, 1fr) 115px;
  }

  .forum-search-button {
    grid-column: 1 / -1;
    min-height: 40px;
  }

  .forum-sidebar {
    grid-template-columns: 1fr;
  }

  .forum-post {
    padding: 14px;
  }

  .forum-pagination {
    margin-top: 14px;
  }

  .forum-post__footer {
    gap: 9px;
  }

  .forum-composer__footer {
    align-items: stretch;
    flex-direction: column;
  }

  .forum-composer__footer > div {
    justify-content: flex-end;
  }
}

@media (prefers-reduced-motion: reduce) {
  .forum-post,
  .forum-skeleton {
    transition: none;
    animation: none;
  }

  .forum-post:hover {
    transform: none;
  }
}
</style>
