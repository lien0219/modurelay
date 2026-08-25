<template>
  <AppLayout>
    <div class="mx-auto max-w-4xl space-y-5">
      <section class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/20"><div class="flex items-start gap-3 text-sm text-amber-900 dark:text-amber-200"><Icon name="exclamationTriangle" size="md" class="mt-0.5 flex-shrink-0" /><p class="leading-6">{{ t('resourceCenter.disclaimer') }}</p></div></section>
      <RouterLink to="/resource-center" class="inline-flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-primary-600 dark:text-gray-300"><Icon name="chevronLeft" size="sm" />{{ t('resourceCenter.backToList') }}</RouterLink>

      <div v-if="loading" class="flex justify-center py-20"><span class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" /></div>
      <template v-else-if="detail">
        <article class="rounded-lg border border-gray-200 bg-white p-6 shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <header class="flex items-start justify-between gap-4">
            <div class="min-w-0"><span class="text-xs font-medium text-primary-600">{{ detail.post.category.name }}</span><h1 class="mt-2 break-words text-2xl font-semibold text-gray-900 dark:text-white">{{ detail.post.title }}</h1><div class="mt-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400"><span>{{ detail.post.author.username }}</span><span class="rounded-full px-2 py-0.5" :class="detail.post.author.role === 'admin' ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-gray-100 dark:bg-dark-700'">{{ detail.post.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}</span><span>{{ formatDate(detail.post.created_at) }}</span></div></div>
            <button v-if="canDeletePost && detail.post.status !== 'deleted'" type="button" class="rounded-md p-2 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30" :title="t('admin.resourceCenter.deletePost')" @click="removePost"><Icon name="trash" size="sm" /></button>
          </header>
          <ResourceRichContent class="mt-6" :content="detail.post.content" />
          <div class="mt-6 flex flex-wrap items-center gap-5 border-t border-gray-100 pt-4 text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400"><button v-if="detail.post.status !== 'deleted'" type="button" class="inline-flex items-center gap-1.5 hover:text-primary-600" @click="togglePostLike"><Icon name="check" size="sm" :class="detail.post.liked ? 'text-primary-600' : ''" />{{ detail.post.like_count }} {{ t('resourceCenter.likes') }}</button><span>{{ detail.post.view_count }} {{ t('resourceCenter.views') }}</span><span>{{ commentsPage.total }} {{ t('resourceCenter.comments') }}</span></div>
        </article>

        <section class="overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800">
          <header class="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700"><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('resourceCenter.comments') }} ({{ commentsPage.total }})</h2><button type="button" class="btn btn-secondary !py-1.5" @click="commentsExpanded = !commentsExpanded">{{ commentsExpanded ? t('resourceCenter.collapseComments') : t('resourceCenter.expandComments') }}</button></header>
          <template v-if="commentsExpanded">
            <div v-if="commentsLoading" class="flex justify-center py-10"><span class="h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" /></div>
            <div v-else-if="comments.length" class="space-y-5 px-5 py-5"><ResourceCommentItem v-for="comment in comments" :key="comment.id" :comment="comment" :can-delete="canDeleteComment(comment)" @reply="startReply" @like="toggleCommentLike" @delete="removeComment" /></div>
            <p v-else class="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('resourceCenter.noComments') }}</p>
            <Pagination v-if="commentsPage.total" :page="commentsPage.page" :total="commentsPage.total" :page-size="commentsPage.page_size" :show-page-size-selector="false" @update:page="loadComments" />
            <footer v-if="detail.post.status !== 'deleted'" class="border-t border-gray-100 px-5 py-5 dark:border-dark-700"><div v-if="replyingTo" class="mb-2 flex items-center justify-between text-xs text-primary-600"><span>{{ t('resourceCenter.replyTo', { name: replyingTo.author.username }) }}</span><button type="button" class="text-gray-500 hover:text-gray-700" @click="replyingTo = null">{{ t('resourceCenter.cancelReply') }}</button></div><ResourceRichTextEditor v-model="commentContent" :placeholder="t('resourceCenter.writeComment')" :max-length="2000" /><div class="mt-3 flex justify-end"><button type="button" class="btn btn-primary" :disabled="commentSubmitting || !richTextHasText(commentContent)" @click="submitComment">{{ commentSubmitting ? t('common.submitting') : t('resourceCenter.submitComment') }}</button></div></footer>
          </template>
        </section>
      </template>
    </div>
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

function postID(): number { const id = Number(route.params.id); return Number.isFinite(id) && id > 0 ? id : 0 }
function formatDate(value: string): string { return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function errorMessage(error: unknown): string { const candidate = error as { message?: string; code?: string }; if (candidate?.code === 'RESOURCE_CONTENT_BLOCKED') return t('resourceCenter.blockedContent'); if (candidate?.code === 'RESOURCE_CONTENT_TOO_LONG') return t('resourceCenter.contentTooLong'); return candidate?.message || t('resourceCenter.loadFailed') }
function richTextHasText(value: string): boolean { const node = document.createElement('div'); node.innerHTML = value; return Boolean((node.textContent || '').replace(/\u00a0/g, ' ').trim()) }

async function loadDetail(): Promise<void> { const id = postID(); if (!id) { await router.replace('/resource-center'); return } loading.value = true; try { const data = authStore.isAdmin ? await adminResourceAPI.getPost(id) : await resourceAPI.getPost(id); detail.value = data; comments.value = data.comments; commentsPage.value = { ...data.comments_page, page_size: 10 } } catch (error) { appStore.showError(errorMessage(error)); await router.replace('/resource-center') } finally { loading.value = false } }
async function loadComments(page: number): Promise<void> { if (!detail.value) return; commentsLoading.value = true; try { const result = authStore.isAdmin ? await adminResourceAPI.listComments(detail.value.post.id, { page, page_size: 10 }) : await resourceAPI.listComments(detail.value.post.id, { page, page_size: 10 }); comments.value = result.items; commentsPage.value = result } catch (error) { appStore.showError(errorMessage(error)) } finally { commentsLoading.value = false } }
async function togglePostLike(): Promise<void> { if (!detail.value) return; try { const result = await resourceAPI.togglePostLike(detail.value.post.id); detail.value.post.liked = result.liked; detail.value.post.like_count += result.liked ? 1 : -1 } catch (error) { appStore.showError(errorMessage(error)) } }
async function toggleCommentLike(comment: ResourceComment): Promise<void> { try { const result = await resourceAPI.toggleCommentLike(comment.id); comment.liked = result.liked; comment.like_count += result.liked ? 1 : -1 } catch (error) { appStore.showError(errorMessage(error)) } }
function startReply(comment: ResourceComment): void { replyingTo.value = comment }
async function submitComment(): Promise<void> { if (!detail.value || !richTextHasText(commentContent.value)) return; commentSubmitting.value = true; try { await resourceAPI.createComment(detail.value.post.id, { parent_id: replyingTo.value?.id, content: commentContent.value }); commentContent.value = ''; replyingTo.value = null; appStore.showSuccess(t('resourceCenter.commentPublished')); const lastPage = Math.max(1, Math.ceil((commentsPage.value.total + 1) / 10)); await loadComments(lastPage) } catch (error) { appStore.showError(errorMessage(error)) } finally { commentSubmitting.value = false } }
function canDeleteComment(comment: ResourceComment): boolean { return authStore.isAdmin || authStore.user?.id === comment.author.id }
async function removePost(): Promise<void> { if (!detail.value || !canDeletePost.value || !window.confirm(t('admin.resourceCenter.confirmDeletePost'))) return; try { if (authStore.isAdmin) await adminResourceAPI.deletePost(detail.value.post.id); else await resourceAPI.deletePost(detail.value.post.id); appStore.showSuccess(t('common.deleted')); await router.push('/resource-center') } catch (error) { appStore.showError(errorMessage(error)) } }
async function removeComment(comment: ResourceComment): Promise<void> { if (!canDeleteComment(comment) || !window.confirm(t('admin.resourceCenter.confirmDeleteComment'))) return; try { if (authStore.isAdmin) await adminResourceAPI.deleteComment(comment.id); else await resourceAPI.deleteComment(comment.id); appStore.showSuccess(t('common.deleted')); await loadComments(commentsPage.value.page) } catch (error) { appStore.showError(errorMessage(error)) } }

onMounted(() => { void loadDetail() })
watch(() => route.params.id, () => { void loadDetail() })
</script>
