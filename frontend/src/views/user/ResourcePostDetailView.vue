<template>
  <AppLayout>
    <main class="forum-detail-page">
      <div class="forum-detail-shell">
        <RouterLink to="/resource-center" class="forum-detail-back">
          <Icon name="chevronLeft" size="sm" aria-hidden="true" />
          {{ t('resourceCenter.backToList') }}
        </RouterLink>

        <section class="forum-detail-disclaimer">
          <Icon name="exclamationTriangle" size="sm" aria-hidden="true" />
          <p>{{ t('resourceCenter.disclaimer') }}</p>
        </section>

        <div v-if="loading" class="forum-detail-loading" aria-live="polite">
          <span></span>
          <span></span>
          <span></span>
        </div>

        <template v-else-if="detail">
          <article class="forum-article">
            <header class="forum-article__header">
              <div class="forum-article__heading">
                <div class="forum-article__topline">
                  <span class="forum-article__category">{{ detail.post.category.name }}</span>
                  <span v-if="detail.post.author.role === 'admin'" class="forum-article__official">
                    <Icon name="badge" size="xs" />
                    {{ t('resourceCenter.official') }}
                  </span>
                </div>
                <h1>{{ detail.post.title }}</h1>
                <div class="forum-article__author">
                  <span class="forum-article__avatar">{{ avatarInitial(detail.post.author.username) }}</span>
                  <span>
                    <strong>{{ detail.post.author.username }}</strong>
                    <small>{{ detail.post.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}</small>
                  </span>
                  <time>{{ formatDate(detail.post.created_at) }}</time>
                </div>
              </div>

              <button
                v-if="canDeletePost && detail.post.status !== 'deleted'"
                type="button"
                class="forum-article__delete"
                :title="t('admin.resourceCenter.deletePost')"
                @click="removePost"
              >
                <Icon name="trash" size="sm" />
              </button>
            </header>

            <div class="forum-article__body">
              <ResourceRichContent :content="detail.post.content" />
            </div>

            <footer class="forum-article__footer">
              <button
                v-if="detail.post.status !== 'deleted'"
                type="button"
                :class="{ 'is-active': detail.post.liked }"
                @click="togglePostLike"
              >
                <Icon name="check" size="sm" />
                {{ detail.post.like_count }} {{ t('resourceCenter.likes') }}
              </button>
              <span><Icon name="chatBubble" size="sm" />{{ commentsPage.total }} {{ t('resourceCenter.comments') }}</span>
              <span>{{ detail.post.view_count }} {{ t('resourceCenter.views') }}</span>
            </footer>
          </article>

          <section class="forum-comments">
            <header class="forum-comments__header">
              <div>
                <span class="forum-section-kicker">{{ t('resourceCenter.discussionEyebrow') }}</span>
                <h2>{{ t('resourceCenter.comments') }} <span>{{ commentsPage.total }}</span></h2>
              </div>
              <button type="button" @click="commentsExpanded = !commentsExpanded">
                {{ commentsExpanded ? t('resourceCenter.collapseComments') : t('resourceCenter.expandComments') }}
              </button>
            </header>

            <template v-if="commentsExpanded">
              <div v-if="commentsLoading" class="forum-comments__loading">
                <span></span>
              </div>

              <div v-else-if="comments.length" class="forum-comments__list">
                <ResourceCommentItem
                  v-for="comment in comments"
                  :key="comment.id"
                  :comment="comment"
                  :can-delete="canDeleteComment(comment)"
                  @reply="startReply"
                  @like="toggleCommentLike"
                  @delete="removeComment"
                />
              </div>

              <section v-else class="forum-comments__empty">
                <span><Icon name="chatBubble" size="lg" /></span>
                <strong>{{ t('resourceCenter.noCommentsTitle') }}</strong>
                <p>{{ t('resourceCenter.noComments') }}</p>
              </section>

              <Pagination
                v-if="commentsPage.total"
                :page="commentsPage.page"
                :total="commentsPage.total"
                :page-size="commentsPage.page_size"
                :show-page-size-selector="false"
                @update:page="loadComments"
              />

              <footer v-if="detail.post.status !== 'deleted'" class="forum-comment-composer">
                <div v-if="replyingTo" class="forum-replying">
                  <span>{{ t('resourceCenter.replyTo', { name: replyingTo.author.username }) }}</span>
                  <button type="button" @click="replyingTo = null">{{ t('resourceCenter.cancelReply') }}</button>
                </div>
                <ResourceRichTextEditor v-model="commentContent" :placeholder="t('resourceCenter.writeComment')" :max-length="2000" />
                <div class="forum-comment-composer__footer">
                  <span><Icon name="exclamationTriangle" size="sm" />{{ t('resourceCenter.commentSafetyHint') }}</span>
                  <button
                    type="button"
                    :disabled="commentSubmitting || !richTextHasText(commentContent)"
                    @click="submitComment"
                  >
                    {{ commentSubmitting ? t('common.submitting') : t('resourceCenter.submitComment') }}
                  </button>
                </div>
              </footer>
            </template>
          </section>
        </template>
      </div>
    </main>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import ResourceCommentItem from '@/components/resource/ResourceCommentItem.vue'
import ResourceRichContent from '@/components/resource/ResourceRichContent.vue'
import ResourceRichTextEditor from '@/components/resource/ResourceRichTextEditor.vue'
import resourceAPI from '@/api/resourceCenter'
import adminResourceAPI from '@/api/admin/resourceCenter'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { ResourceComment, ResourcePageInfo, ResourcePostDetail } from '@/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const loading = ref(true)
const commentsLoading = ref(false)
const detail = ref<ResourcePostDetail | null>(null)
const comments = ref<ResourceComment[]>([])
const commentsPage = ref<ResourcePageInfo>({ page: 1, page_size: 10, total: 0, has_more: false })
const commentsExpanded = ref(true)
const replyingTo = ref<ResourceComment | null>(null)
const commentContent = ref('')
const commentSubmitting = ref(false)

const canDeletePost = computed(() => Boolean(detail.value && (authStore.isAdmin || authStore.user?.id === detail.value.post.author.id)))

function avatarInitial(value: string): string {
  return value.trim().slice(0, 1).toUpperCase() || 'U'
}

function postID(): number {
  const id = Number(route.params.id)
  return Number.isFinite(id) && id > 0 ? id : 0
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function errorMessage(error: unknown): string {
  const candidate = error as { message?: string; code?: string }
  if (candidate?.code === 'RESOURCE_CONTENT_BLOCKED') return t('resourceCenter.blockedContent')
  if (candidate?.code === 'RESOURCE_CONTENT_TOO_LONG') return t('resourceCenter.contentTooLong')
  return candidate?.message || t('resourceCenter.loadFailed')
}

function richTextHasText(value: string): boolean {
  const node = document.createElement('div')
  node.innerHTML = value
  return Boolean((node.textContent || '').replace(/\u00a0/g, ' ').trim())
}

async function loadDetail(): Promise<void> {
  const id = postID()
  if (!id) {
    await router.replace('/resource-center')
    return
  }

  loading.value = true
  try {
    const data = authStore.isAdmin ? await adminResourceAPI.getPost(id) : await resourceAPI.getPost(id)
    detail.value = data
    comments.value = data.comments
    commentsPage.value = { ...data.comments_page, page_size: 10 }
  } catch (error) {
    appStore.showError(errorMessage(error))
    await router.replace('/resource-center')
  } finally {
    loading.value = false
  }
}

async function loadComments(page: number): Promise<void> {
  if (!detail.value) return
  commentsLoading.value = true
  try {
    const result = authStore.isAdmin
      ? await adminResourceAPI.listComments(detail.value.post.id, { page, page_size: 10 })
      : await resourceAPI.listComments(detail.value.post.id, { page, page_size: 10 })
    comments.value = result.items
    commentsPage.value = result
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    commentsLoading.value = false
  }
}

async function togglePostLike(): Promise<void> {
  if (!detail.value) return
  try {
    const result = await resourceAPI.togglePostLike(detail.value.post.id)
    detail.value.post.liked = result.liked
    detail.value.post.like_count += result.liked ? 1 : -1
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function toggleCommentLike(comment: ResourceComment): Promise<void> {
  try {
    const result = await resourceAPI.toggleCommentLike(comment.id)
    comment.liked = result.liked
    comment.like_count += result.liked ? 1 : -1
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

function startReply(comment: ResourceComment): void {
  replyingTo.value = comment
}

async function submitComment(): Promise<void> {
  if (!detail.value || !richTextHasText(commentContent.value)) return
  commentSubmitting.value = true
  try {
    await resourceAPI.createComment(detail.value.post.id, {
      parent_id: replyingTo.value?.id,
      content: commentContent.value,
    })
    commentContent.value = ''
    replyingTo.value = null
    appStore.showSuccess(t('resourceCenter.commentPublished'))
    const lastPage = Math.max(1, Math.ceil((commentsPage.value.total + 1) / 10))
    await loadComments(lastPage)
  } catch (error) {
    appStore.showError(errorMessage(error))
  } finally {
    commentSubmitting.value = false
  }
}

function canDeleteComment(comment: ResourceComment): boolean {
  return authStore.isAdmin || authStore.user?.id === comment.author.id
}

async function removePost(): Promise<void> {
  if (!detail.value || !canDeletePost.value || !window.confirm(t('admin.resourceCenter.confirmDeletePost'))) return
  try {
    if (authStore.isAdmin) await adminResourceAPI.deletePost(detail.value.post.id)
    else await resourceAPI.deletePost(detail.value.post.id)
    appStore.showSuccess(t('common.deleted'))
    await router.push('/resource-center')
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

async function removeComment(comment: ResourceComment): Promise<void> {
  if (!canDeleteComment(comment) || !window.confirm(t('admin.resourceCenter.confirmDeleteComment'))) return
  try {
    if (authStore.isAdmin) await adminResourceAPI.deleteComment(comment.id)
    else await resourceAPI.deleteComment(comment.id)
    appStore.showSuccess(t('common.deleted'))
    await loadComments(commentsPage.value.page)
  } catch (error) {
    appStore.showError(errorMessage(error))
  }
}

onMounted(() => {
  void loadDetail()
})

watch(() => route.params.id, () => {
  void loadDetail()
})
</script>

<style scoped>
.forum-detail-page {
  min-height: calc(100dvh - 4rem);
  color: var(--color-text-primary);
}

.forum-detail-shell {
  width: min(1080px, calc(100% - 40px));
  margin-inline: auto;
  padding: 24px 0 48px;
}

.forum-detail-back {
  display: inline-flex;
  min-height: 34px;
  align-items: center;
  gap: 6px;
  color: var(--color-primary);
  font-size: .75rem;
  font-weight: 650;
  text-decoration: none;
}

.forum-detail-back:hover {
  color: var(--color-primary-hover);
}

.forum-detail-disclaimer {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 10px;
  padding: 9px 11px;
  border: 1px solid color-mix(in srgb, var(--color-warning) 22%, var(--color-border));
  border-radius: 10px;
  background: color-mix(in srgb, var(--color-warning) 5%, var(--color-surface));
  color: var(--color-text-muted);
  font-size: .68rem;
  line-height: 1.45;
}

.forum-detail-disclaimer svg {
  flex: 0 0 auto;
  color: var(--color-warning);
}

.forum-detail-loading {
  display: grid;
  gap: 12px;
  margin-top: 16px;
  padding: 32px;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
}

.forum-detail-loading span {
  height: 18px;
  border-radius: 8px;
  background: var(--color-surface-soft);
  animation: detail-pulse 1.5s ease-in-out infinite;
}

.forum-detail-loading span:first-child {
  width: 30%;
}

.forum-detail-loading span:nth-child(2) {
  width: 72%;
  height: 32px;
}

.forum-detail-loading span:last-child {
  width: 100%;
  height: 280px;
}

.forum-article {
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-sm);
}

.forum-article__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
  padding: 26px 28px 22px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.forum-article__heading {
  min-width: 0;
}

.forum-article__topline {
  display: flex;
  flex-wrap: wrap;
  gap: 7px;
}

.forum-article__category,
.forum-article__official {
  display: inline-flex;
  min-height: 24px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  border-radius: 999px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .66rem;
  font-weight: 650;
}

.forum-article__official {
  background: color-mix(in srgb, var(--color-warning) 9%, var(--color-surface));
  color: var(--color-warning);
}

.forum-article h1 {
  max-width: 850px;
  margin: 14px 0 0;
  overflow-wrap: anywhere;
  font-size: clamp(1.55rem, 2.8vw, 2.2rem);
  font-weight: 720;
  line-height: 1.28;
  letter-spacing: -.035em;
}

.forum-article__author {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 9px;
  margin-top: 15px;
}

.forum-article__avatar {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-accent-soft);
  color: var(--color-accent);
  font-size: .76rem;
  font-weight: 700;
}

.forum-article__author > span:nth-child(2) {
  display: grid;
  gap: 1px;
}

.forum-article__author strong {
  font-size: .75rem;
  font-weight: 650;
}

.forum-article__author small,
.forum-article__author time {
  color: var(--color-text-muted);
  font-size: .64rem;
}

.forum-article__author time {
  margin-left: 3px;
}

.forum-article__delete {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
}

.forum-article__delete:hover {
  background: color-mix(in srgb, var(--color-danger) 8%, transparent);
  color: var(--color-danger);
}

.forum-article__body {
  min-height: 300px;
  padding: 30px 28px 36px;
  color: var(--color-text-primary);
  font-size: .88rem;
  line-height: 1.8;
}

.forum-article__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  padding: 13px 28px;
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .7rem;
}

.forum-article__footer button,
.forum-article__footer span {
  display: inline-flex;
  min-height: 30px;
  align-items: center;
  gap: 6px;
}

.forum-article__footer button {
  padding: 0 9px;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
}

.forum-article__footer button:hover,
.forum-article__footer button.is-active {
  border-color: var(--color-primary-border);
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-comments {
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid var(--color-border);
  border-radius: 16px;
  background: var(--color-surface);
  box-shadow: var(--shadow-xs);
}

.forum-comments__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--color-border-subtle);
}

.forum-section-kicker {
  color: var(--color-primary);
  font-size: .66rem;
  font-weight: 750;
  letter-spacing: .08em;
  text-transform: uppercase;
}

.forum-comments__header h2 {
  margin: 4px 0 0;
  font-size: .92rem;
  font-weight: 680;
}

.forum-comments__header h2 span {
  color: var(--color-text-muted);
  font-weight: 600;
}

.forum-comments__header > button {
  min-height: 34px;
  padding: 0 10px;
  border: 1px solid var(--color-border);
  border-radius: 9px;
  background: var(--color-surface);
  color: var(--color-text-secondary);
  cursor: pointer;
  font: inherit;
  font-size: .7rem;
}

.forum-comments__list {
  display: grid;
  gap: 0;
  padding: 0 20px;
}

.forum-comments__loading {
  display: grid;
  min-height: 140px;
  place-items: center;
}

.forum-comments__loading span {
  width: 28px;
  height: 28px;
  border: 2px solid var(--color-primary);
  border-top-color: transparent;
  border-radius: 50%;
  animation: detail-spin .8s linear infinite;
}

.forum-comments__empty {
  display: grid;
  min-height: 220px;
  justify-items: center;
  align-content: center;
  gap: 6px;
  padding: 24px;
  text-align: center;
}

.forum-comments__empty > span {
  display: grid;
  width: 54px;
  height: 54px;
  place-items: center;
  margin-bottom: 4px;
  border-radius: 16px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.forum-comments__empty strong {
  font-size: .84rem;
  font-weight: 670;
}

.forum-comments__empty p {
  margin: 0;
  color: var(--color-text-muted);
  font-size: .72rem;
}

.forum-comment-composer {
  padding: 18px 20px 20px;
  border-top: 1px solid var(--color-border-subtle);
  background: var(--color-surface-soft);
}

.forum-replying {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
  padding: 8px 10px;
  border-radius: 9px;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: .7rem;
}

.forum-replying button {
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font: inherit;
  font-weight: 650;
}

.forum-comment-composer__footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  margin-top: 10px;
}

.forum-comment-composer__footer > span {
  display: flex;
  align-items: flex-start;
  gap: 6px;
  color: var(--color-text-muted);
  font-size: .65rem;
  line-height: 1.45;
}

.forum-comment-composer__footer > button {
  min-height: 38px;
  padding: 0 14px;
  border: 1px solid var(--color-primary);
  border-radius: 9px;
  background: var(--color-primary);
  color: var(--color-surface);
  cursor: pointer;
  font: inherit;
  font-size: .74rem;
  font-weight: 650;
}

.forum-comment-composer__footer > button:disabled {
  cursor: not-allowed;
  opacity: .5;
}

.forum-detail-back:focus-visible,
.forum-article button:focus-visible,
.forum-comments button:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

@keyframes detail-pulse {
  0%, 100% { opacity: .5; }
  50% { opacity: 1; }
}

@keyframes detail-spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 640px) {
  .forum-detail-shell {
    width: min(100% - 20px, 560px);
    padding-top: 16px;
  }

  .forum-article,
  .forum-comments {
    border-radius: 14px;
  }

  .forum-article__header {
    padding: 20px 18px 18px;
  }

  .forum-article__body {
    min-height: 240px;
    padding: 24px 18px 28px;
  }

  .forum-article__footer {
    padding-inline: 18px;
  }

  .forum-comments__header,
  .forum-comment-composer {
    padding-inline: 16px;
  }

  .forum-comments__list {
    padding-inline: 14px;
  }

  .forum-comment-composer__footer {
    align-items: stretch;
    flex-direction: column;
  }

  .forum-comment-composer__footer > button {
    width: 100%;
  }
}

@media (prefers-reduced-motion: reduce) {
  .forum-detail-loading span,
  .forum-comments__loading span {
    animation: none;
  }
}
</style>
