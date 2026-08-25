<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <header class="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('admin.resourceCenter.title') }}</h1>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.resourceCenter.description') }}</p>
        </div>
        <div class="flex gap-2">
          <RouterLink to="/resource-center" class="btn btn-primary inline-flex items-center gap-2">
            <Icon name="chatBubble" size="sm" />{{ t('admin.resourceCenter.openForum') }}
          </RouterLink>
          <button type="button" class="btn btn-secondary inline-flex items-center gap-2" @click="refreshAll">
            <Icon name="refresh" size="sm" />{{ t('admin.resourceCenter.refresh') }}
          </button>
        </div>
      </header>

      <form class="space-y-5 rounded-lg border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-800" @submit.prevent="saveConfig">
        <div class="grid gap-4 md:grid-cols-2">
          <label class="flex items-start gap-3 rounded-md border border-gray-100 p-4 dark:border-dark-700">
            <input v-model="config.enabled" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600" />
            <span><span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.resourceCenter.enabled') }}</span><span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.resourceCenter.enabledHint') }}</span></span>
          </label>
          <label class="flex items-start gap-3 rounded-md border border-gray-100 p-4 dark:border-dark-700">
            <input v-model="config.forbid_urls" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600" />
            <span><span class="block text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.resourceCenter.forbidUrls') }}</span><span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.resourceCenter.forbidUrlsHint') }}</span></span>
          </label>
        </div>
        <label class="block">
          <span class="text-sm font-medium text-gray-900 dark:text-white">{{ t('admin.resourceCenter.bannedWords') }}</span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">{{ t('admin.resourceCenter.bannedWordsHint') }}</span>
          <textarea v-model="bannedWordsText" rows="5" class="form-input mt-3 w-full resize-y" />
        </label>
        <div class="flex justify-end"><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? t('common.saving') : t('admin.resourceCenter.save') }}</button></div>
      </form>

      <ModerationSection
        :title="t('admin.resourceCenter.posts')"
        :loading="postsLoading"
        :empty="!posts.length"
        :page="postsPage"
        :selected-count="selectedPosts.size"
        @search="searchPosts"
        @page="changePostsPage"
        @batch-delete="batchRemovePosts"
      >
        <template #filters>
          <label class="min-w-0 flex-1 text-xs text-gray-500 dark:text-gray-400"><span class="mb-1 block">{{ t('admin.resourceCenter.keyword') }}</span><input v-model="postFilters.query" type="search" class="form-input w-full" :placeholder="t('resourceCenter.searchPlaceholder')" @keyup.enter="searchPosts" /></label>
          <label class="text-xs text-gray-500 dark:text-gray-400"><span class="mb-1 block">{{ t('admin.resourceCenter.startDate') }}</span><input v-model="postFilters.start" type="date" class="form-input sm:w-40" /></label>
          <label class="text-xs text-gray-500 dark:text-gray-400"><span class="mb-1 block">{{ t('admin.resourceCenter.endDate') }}</span><input v-model="postFilters.end" type="date" class="form-input sm:w-40" /></label>
        </template>
        <template #select-all>
          <input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :checked="allPostsSelected" :aria-label="t('admin.resourceCenter.selectPage')" @change="toggleAllPosts" />
        </template>
        <article v-for="post in posts" :key="post.id" class="flex items-start gap-3 px-5 py-4">
          <input v-if="post.status !== 'deleted'" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600" :checked="selectedPosts.has(post.id)" @change="toggleSelection(selectedPosts, post.id)" />
          <span v-else class="w-4" />
          <div class="min-w-0 flex-1">
            <RouterLink :to="`/resource-center/posts/${post.id}`" class="font-semibold text-gray-900 hover:text-primary-600 dark:text-white">{{ post.title }}</RouterLink>
            <div class="mt-1 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ post.category.name }}</span><span>{{ post.author.username }}</span><span>{{ post.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}</span><span>{{ formatDate(post.created_at) }}</span>
              <span v-if="post.status === 'deleted'" class="rounded-full bg-red-100 px-2 py-0.5 text-red-700 dark:bg-red-950/40 dark:text-red-300">{{ t('admin.resourceCenter.deleted') }}</span>
            </div>
          </div>
          <button v-if="post.status !== 'deleted'" type="button" class="rounded-md p-2 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30" :title="t('admin.resourceCenter.deletePost')" @click="removePost(post)"><Icon name="trash" size="sm" /></button>
        </article>
      </ModerationSection>

      <ModerationSection
        :title="t('admin.resourceCenter.comments')"
        :loading="commentsLoading"
        :empty="!comments.length"
        :page="commentsPage"
        :selected-count="selectedComments.size"
        @search="searchComments"
        @page="changeCommentsPage"
        @batch-delete="batchRemoveComments"
      >
        <template #filters>
          <label class="min-w-0 flex-1 text-xs text-gray-500 dark:text-gray-400"><span class="mb-1 block">{{ t('admin.resourceCenter.keyword') }}</span><input v-model="commentFilters.query" type="search" class="form-input w-full" :placeholder="t('resourceCenter.searchPlaceholder')" @keyup.enter="searchComments" /></label>
          <label class="text-xs text-gray-500 dark:text-gray-400"><span class="mb-1 block">{{ t('admin.resourceCenter.startDate') }}</span><input v-model="commentFilters.start" type="date" class="form-input sm:w-40" /></label>
          <label class="text-xs text-gray-500 dark:text-gray-400"><span class="mb-1 block">{{ t('admin.resourceCenter.endDate') }}</span><input v-model="commentFilters.end" type="date" class="form-input sm:w-40" /></label>
        </template>
        <template #select-all>
          <input type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" :checked="allCommentsSelected" :aria-label="t('admin.resourceCenter.selectPage')" @change="toggleAllComments" />
        </template>
        <article v-for="comment in comments" :key="comment.id" class="flex items-start gap-3 px-5 py-4">
          <input v-if="comment.status !== 'deleted'" type="checkbox" class="mt-1 h-4 w-4 rounded border-gray-300 text-primary-600" :checked="selectedComments.has(comment.id)" @change="toggleSelection(selectedComments, comment.id)" />
          <span v-else class="w-4" />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span class="font-medium text-gray-700 dark:text-gray-200">{{ comment.author.username }}</span><span>{{ comment.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}</span><span>{{ formatDate(comment.created_at) }}</span>
              <span v-if="comment.status === 'deleted'" class="text-red-500">{{ t('admin.resourceCenter.deleted') }}</span>
            </div>
            <RouterLink :to="`/resource-center/posts/${comment.post_id}`" class="mt-1 block truncate text-xs text-primary-600 hover:underline">{{ comment.post_title || `#${comment.post_id}` }}</RouterLink>
            <ResourceRichContent class="mt-1" :content="comment.content" />
          </div>
          <button v-if="comment.status !== 'deleted'" type="button" class="rounded-md p-2 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30" :title="t('admin.resourceCenter.deleteComment')" @click="removeComment(comment)"><Icon name="trash" size="sm" /></button>
        </article>
      </ModerationSection>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, onMounted, reactive, ref, type PropType } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import ResourceRichContent from '@/components/resource/ResourceRichContent.vue'
import adminResourceAPI from '@/api/admin/resourceCenter'
import { useAppStore } from '@/stores/app'
import type { ResourceCenterConfig, ResourceComment, ResourcePageInfo, ResourcePost } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const emptyPage = (): ResourcePageInfo => ({ page: 1, page_size: 10, total: 0, has_more: false })

const ModerationSection = defineComponent({
  props: { title: { type: String, required: true }, loading: Boolean, empty: Boolean, page: { type: Object as PropType<ResourcePageInfo>, required: true }, selectedCount: { type: Number, required: true } },
  emits: ['search', 'page', 'batch-delete'],
  setup(props, { slots, emit }) {
    return () => h('section', { class: 'overflow-hidden rounded-lg border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-800' }, [
      h('div', { class: 'space-y-3 border-b border-gray-100 px-5 py-4 dark:border-dark-700' }, [
        h('div', { class: 'flex items-center justify-between gap-3' }, [h('h2', { class: 'font-semibold text-gray-900 dark:text-white' }, props.title), props.selectedCount ? h('button', { type: 'button', class: 'btn btn-secondary !py-1.5 text-red-600', onClick: () => emit('batch-delete') }, `${t('admin.resourceCenter.batchDelete')} (${props.selectedCount})`) : null]),
        h('div', { class: 'flex flex-col gap-2 sm:flex-row' }, [slots.filters?.(), h('button', { type: 'button', class: 'btn btn-primary inline-flex items-center justify-center gap-2 sm:w-auto sm:self-end', onClick: () => emit('search') }, [h(Icon, { name: 'search', size: 'sm' }), t('admin.resourceCenter.search')])]),
      ]),
      h('div', { class: 'flex h-10 items-center gap-3 border-b border-gray-100 px-5 dark:border-dark-700' }, [slots['select-all']?.(), h('span', { class: 'text-xs text-gray-500 dark:text-gray-400' }, t('admin.resourceCenter.selectPage'))]),
      props.loading ? h('div', { class: 'flex justify-center py-12' }, [h('span', { class: 'h-7 w-7 animate-spin rounded-full border-2 border-primary-500 border-t-transparent' })]) : props.empty ? h('div', { class: 'p-10 text-center text-sm text-gray-500 dark:text-gray-400' }, t('admin.resourceCenter.noResults')) : h('div', { class: 'divide-y divide-gray-100 dark:divide-dark-700' }, slots.default?.()),
      props.page.total ? h(Pagination, { page: props.page.page, total: props.page.total, pageSize: props.page.page_size, showPageSizeSelector: false, 'onUpdate:page': (page: number) => emit('page', page) }) : null,
    ])
  },
})

const config = reactive<ResourceCenterConfig>({ enabled: true, forbid_urls: false, banned_words: [] })
const bannedWordsText = ref('')
const saving = ref(false)
const posts = ref<ResourcePost[]>([])
const postsPage = ref<ResourcePageInfo>(emptyPage())
const postsLoading = ref(true)
const comments = ref<ResourceComment[]>([])
const commentsPage = ref<ResourcePageInfo>(emptyPage())
const commentsLoading = ref(true)
const postFilters = reactive({ query: '', start: '', end: '' })
const commentFilters = reactive({ query: '', start: '', end: '' })
const selectedPosts = reactive(new Set<number>())
const selectedComments = reactive(new Set<number>())
const allPostsSelected = computed(() => posts.value.some(item => item.status !== 'deleted') && posts.value.filter(item => item.status !== 'deleted').every(item => selectedPosts.has(item.id)))
const allCommentsSelected = computed(() => comments.value.some(item => item.status !== 'deleted') && comments.value.filter(item => item.status !== 'deleted').every(item => selectedComments.has(item.id)))

function formatDate(value: string): string { return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function apiError(error: unknown): string { return String((error as { message?: string })?.message || t('common.unknownError')) }
function dateParams(start: string, end: string): { start_at?: string; end_at?: string } {
  const params: { start_at?: string; end_at?: string } = {}
  if (start) params.start_at = new Date(`${start}T00:00:00`).toISOString()
  if (end) { const value = new Date(`${end}T00:00:00`); value.setDate(value.getDate() + 1); params.end_at = value.toISOString() }
  return params
}
function toggleSelection(target: Set<number>, id: number): void { target.has(id) ? target.delete(id) : target.add(id) }
function toggleAllPosts(): void { allPostsSelected.value ? selectedPosts.clear() : posts.value.filter(item => item.status !== 'deleted').forEach(item => selectedPosts.add(item.id)) }
function toggleAllComments(): void { allCommentsSelected.value ? selectedComments.clear() : comments.value.filter(item => item.status !== 'deleted').forEach(item => selectedComments.add(item.id)) }

async function loadConfig(): Promise<void> { try { const data = await adminResourceAPI.getConfig(); Object.assign(config, data); bannedWordsText.value = data.banned_words.join('\n') } catch (error) { appStore.showError(apiError(error)) } }
async function loadPosts(page = postsPage.value.page): Promise<void> { postsLoading.value = true; selectedPosts.clear(); try { const result = await adminResourceAPI.listPosts({ q: postFilters.query.trim() || undefined, sort: 'latest', ...dateParams(postFilters.start, postFilters.end), page, page_size: 10 }); posts.value = result.items; postsPage.value = result } catch (error) { appStore.showError(apiError(error)) } finally { postsLoading.value = false } }
async function loadComments(page = commentsPage.value.page): Promise<void> { commentsLoading.value = true; selectedComments.clear(); try { const result = await adminResourceAPI.listAllComments({ q: commentFilters.query.trim() || undefined, ...dateParams(commentFilters.start, commentFilters.end), page, page_size: 10 }); comments.value = result.items; commentsPage.value = result } catch (error) { appStore.showError(apiError(error)) } finally { commentsLoading.value = false } }
function searchPosts(): void { void loadPosts(1) }
function searchComments(): void { void loadComments(1) }
function changePostsPage(page: number): void { void loadPosts(page) }
function changeCommentsPage(page: number): void { void loadComments(page) }
async function refreshAll(): Promise<void> { await Promise.all([loadConfig(), loadPosts(), loadComments()]) }
async function saveConfig(): Promise<void> { saving.value = true; try { const data = await adminResourceAPI.updateConfig({ enabled: config.enabled, forbid_urls: config.forbid_urls, banned_words: bannedWordsText.value.split(/\r?\n/).map(item => item.trim()).filter(Boolean) }); Object.assign(config, data); bannedWordsText.value = data.banned_words.join('\n'); appStore.showSuccess(t('admin.resourceCenter.saved')) } catch (error) { appStore.showError(apiError(error)) } finally { saving.value = false } }
async function removePost(post: ResourcePost): Promise<void> { if (!window.confirm(t('admin.resourceCenter.confirmDeletePost'))) return; try { await adminResourceAPI.deletePost(post.id); appStore.showSuccess(t('common.deleted')); await loadPosts() } catch (error) { appStore.showError(apiError(error)) } }
async function removeComment(comment: ResourceComment): Promise<void> { if (!window.confirm(t('admin.resourceCenter.confirmDeleteComment'))) return; try { await adminResourceAPI.deleteComment(comment.id); appStore.showSuccess(t('common.deleted')); await loadComments() } catch (error) { appStore.showError(apiError(error)) } }
async function batchRemovePosts(): Promise<void> { const ids = [...selectedPosts]; if (!ids.length || !window.confirm(t('admin.resourceCenter.confirmBatchDelete', { count: ids.length }))) return; try { const result = await adminResourceAPI.batchDeletePosts(ids); appStore.showSuccess(t('admin.resourceCenter.batchDeleted', { count: result.deleted })); await loadPosts() } catch (error) { appStore.showError(apiError(error)) } }
async function batchRemoveComments(): Promise<void> { const ids = [...selectedComments]; if (!ids.length || !window.confirm(t('admin.resourceCenter.confirmBatchDelete', { count: ids.length }))) return; try { const result = await adminResourceAPI.batchDeleteComments(ids); appStore.showSuccess(t('admin.resourceCenter.batchDeleted', { count: result.deleted })); await loadComments() } catch (error) { appStore.showError(apiError(error)) } }

onMounted(() => { void refreshAll() })
</script>
