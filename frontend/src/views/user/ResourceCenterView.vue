<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-5">
      <section class="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 dark:border-amber-900/60 dark:bg-amber-950/20">
        <div class="flex items-start gap-3 text-sm text-amber-900 dark:text-amber-200"><Icon name="exclamationTriangle" size="md" class="mt-0.5 flex-shrink-0" /><p class="leading-6">{{ t('resourceCenter.disclaimer') }}</p></div>
      </section>

      <header class="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div><h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('resourceCenter.title') }}</h1><p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('resourceCenter.description') }}</p></div>
        <div class="flex items-center gap-2">
          <button type="button" class="btn btn-secondary inline-flex items-center gap-2" @click="notificationOpen = !notificationOpen"><Icon name="bell" size="sm" /><span>{{ t('resourceCenter.notifications') }}</span><span v-if="unreadCount" class="rounded-full bg-red-500 px-1.5 py-0.5 text-[11px] font-semibold text-white">{{ unreadCount }}</span></button>
          <button type="button" class="btn btn-primary inline-flex items-center gap-2" @click="composerOpen = true"><Icon name="plus" size="sm" />{{ t('resourceCenter.newPost') }}</button>
        </div>
      </header>

      <section v-if="notificationOpen" class="rounded-lg border border-gray-200 bg-white p-4 shadow-sm dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center justify-between gap-3"><h2 class="font-semibold text-gray-900 dark:text-white">{{ t('resourceCenter.notifications') }}</h2><button type="button" class="text-xs font-medium text-primary-600 hover:text-primary-700 disabled:opacity-50" :disabled="!unreadCount" @click="markAllRead">{{ t('resourceCenter.markAllRead') }}</button></div>
        <div v-if="notifications.length" class="mt-3 divide-y divide-gray-100 dark:divide-dark-700">
          <button v-for="item in notifications" :key="item.id" type="button" class="flex w-full items-start gap-3 py-3 text-left" :class="item.read ? 'opacity-60' : ''" @click="openNotification(item)"><span class="mt-1 h-2 w-2 flex-shrink-0 rounded-full" :class="item.read ? 'bg-gray-300' : 'bg-primary-500'" /><span class="min-w-0 flex-1 text-sm text-gray-700 dark:text-gray-200">{{ item.kind === 'reply' ? t('resourceCenter.notificationReply', { actor: item.actor.username }) : t('resourceCenter.notificationComment', { actor: item.actor.username }) }}<time class="mt-1 block text-xs text-gray-400">{{ formatDate(item.created_at) }}</time></span></button>
        </div>
        <p v-else class="py-5 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('resourceCenter.noNotifications') }}</p>
      </section>

      <div class="grid gap-5 lg:grid-cols-[220px_minmax(0,1fr)]">
        <aside class="rounded-lg border border-gray-200 bg-white p-3 dark:border-dark-700 dark:bg-dark-800">
          <div class="mb-2 px-2 text-xs font-semibold uppercase text-gray-400">{{ t('resourceCenter.categories') }}</div>
          <button type="button" class="mb-1 flex w-full items-center justify-between rounded-md px-3 py-2 text-left text-sm font-medium" :class="selectedCategory === 0 ? activeCategoryClass : inactiveCategoryClass" @click="selectCategory(0)"><span>{{ t('resourceCenter.allCategories') }}</span><span class="text-xs text-gray-400">{{ postsPage.total }}</span></button>
          <button v-for="category in categories" :key="category.id" type="button" class="mb-1 w-full rounded-md px-3 py-2 text-left text-sm" :class="selectedCategory === category.id ? activeCategoryClass : inactiveCategoryClass" @click="selectCategory(category.id)">{{ category.name }}</button>
        </aside>

        <main class="min-w-0 space-y-4">
          <div class="flex flex-col gap-2 sm:flex-row">
            <label class="relative min-w-0 flex-1"><Icon name="search" size="sm" class="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" /><input v-model="query" type="search" class="form-input w-full pl-9" :placeholder="t('resourceCenter.searchPlaceholder')" @keyup.enter="searchPosts" /></label>
            <select v-model="sortBy" class="form-input sm:w-40"><option value="latest">{{ t('resourceCenter.latest') }}</option><option value="popular">{{ t('resourceCenter.popular') }}</option></select>
            <button type="button" class="btn btn-primary inline-flex items-center justify-center gap-2" @click="searchPosts"><Icon name="search" size="sm" />{{ t('resourceCenter.search') }}</button>
          </div>
          <div v-if="loading" class="flex justify-center py-16"><span class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent" /></div>
          <div v-else-if="!posts.length" class="rounded-lg border border-dashed border-gray-300 p-12 text-center text-sm text-gray-500 dark:border-dark-600 dark:text-gray-400">{{ t('resourceCenter.noPosts') }}</div>
          <div v-else class="space-y-3">
            <article v-for="post in posts" :key="post.id" class="rounded-lg border border-gray-200 bg-white p-5 transition hover:border-primary-300 hover:shadow-sm dark:border-dark-700 dark:bg-dark-800 dark:hover:border-primary-700">
              <div class="flex items-start justify-between gap-3">
                <RouterLink :to="`/resource-center/posts/${post.id}`" class="min-w-0 flex-1"><div class="mb-2 flex flex-wrap items-center gap-2 text-xs text-gray-500 dark:text-gray-400"><span class="rounded bg-primary-50 px-2 py-0.5 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ post.category.name }}</span><span>{{ formatDate(post.created_at) }}</span></div><h2 class="truncate text-lg font-semibold text-gray-900 hover:text-primary-600 dark:text-white">{{ post.title }}</h2></RouterLink>
                <div class="flex flex-shrink-0 gap-1"><button type="button" class="rounded-md p-1.5 text-gray-400 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" :aria-label="post.liked ? t('resourceCenter.liked') : t('resourceCenter.like')" @click="togglePostLike(post)"><Icon name="check" size="sm" :class="post.liked ? 'text-primary-600' : ''" /></button><button v-if="canDeletePost(post)" type="button" class="rounded-md p-1.5 text-gray-400 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30" :title="t('admin.resourceCenter.deletePost')" @click="removePost(post)"><Icon name="trash" size="sm" /></button></div>
              </div>
              <RouterLink :to="`/resource-center/posts/${post.id}`" class="block"><ResourceRichContent class="mt-2 line-clamp-2" :content="post.content" /></RouterLink>
              <div class="mt-4 flex flex-wrap items-center gap-4 text-xs text-gray-500 dark:text-gray-400"><span class="inline-flex items-center gap-1"><Icon name="user" size="xs" />{{ post.author.username }}<span class="rounded-full px-1.5 py-0.5" :class="post.author.role === 'admin' ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-gray-100 dark:bg-dark-700'">{{ post.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}</span></span><span>{{ post.view_count }} {{ t('resourceCenter.views') }}</span><span>{{ post.comment_count }} {{ t('resourceCenter.comments') }}</span><span>{{ post.like_count }} {{ t('resourceCenter.likes') }}</span></div>
            </article>
          </div>
          <Pagination v-if="postsPage.total" :page="postsPage.page" :total="postsPage.total" :page-size="postsPage.page_size" :show-page-size-selector="false" @update:page="changePage" />
        </main>
      </div>
    </div>

    <div v-if="composerOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="composerOpen = false">
      <form class="w-full max-w-2xl space-y-4 rounded-lg bg-white p-6 shadow-xl dark:bg-dark-800" @submit.prevent="publishPost"><div class="flex items-center justify-between"><h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('resourceCenter.newPost') }}</h2><button type="button" class="rounded-md p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-dark-700" :aria-label="t('resourceCenter.close')" @click="composerOpen = false"><Icon name="x" size="md" /></button></div><input v-model="postForm.title" required maxlength="120" class="form-input w-full" :placeholder="t('resourceCenter.postTitle')" /><select v-model.number="postForm.category_id" required class="form-input w-full"><option :value="0" disabled>{{ t('resourceCenter.chooseCategory') }}</option><option v-for="category in categories" :key="category.id" :value="category.id">{{ category.name }}</option></select><ResourceRichTextEditor v-model="postForm.content" :placeholder="t('resourceCenter.postContent')" :max-length="10000" /><div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="composerOpen = false">{{ t('common.cancel') }}</button><button type="submit" class="btn btn-primary" :disabled="postSubmitting || !richTextHasText(postForm.content)">{{ postSubmitting ? t('common.submitting') : t('resourceCenter.publish') }}</button></div></form>
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
const activeCategoryClass = 'bg-primary-50 font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300'
const inactiveCategoryClass = 'text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-dark-700'

function formatDate(value: string): string { return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
function errorMessage(error: unknown): string { const candidate = error as { message?: string; code?: string }; if (candidate?.code === 'RESOURCE_CONTENT_BLOCKED') return t('resourceCenter.blockedContent'); if (candidate?.code === 'RESOURCE_TITLE_TOO_LONG') return t('resourceCenter.titleTooLong'); if (candidate?.code === 'RESOURCE_CONTENT_TOO_LONG') return t('resourceCenter.contentTooLong'); return candidate?.message || t('resourceCenter.loadFailed') }
function richTextHasText(value: string): boolean { const node = document.createElement('div'); node.innerHTML = value; return Boolean((node.textContent || '').replace(/\u00a0/g, ' ').trim()) }

async function loadPosts(page = 1): Promise<void> { loading.value = true; try { const result = await resourceAPI.listPosts({ category_id: selectedCategory.value || undefined, q: query.value.trim() || undefined, sort: sortBy.value, page, page_size: 10 }); posts.value = result.items; postsPage.value = result } catch (error) { appStore.showError(errorMessage(error)) } finally { loading.value = false } }
async function loadNotifications(): Promise<void> { try { notifications.value = await resourceAPI.listNotifications() } catch { notifications.value = [] } }
async function load(): Promise<void> { try { const [categoryData] = await Promise.all([resourceAPI.listCategories(), loadPosts(), loadNotifications()]); categories.value = categoryData; if (!postForm.category_id && categoryData.length) postForm.category_id = categoryData[0].id } catch (error) { appStore.showError(errorMessage(error)) } }
function searchPosts(): void { void loadPosts(1) }
function selectCategory(id: number): void { selectedCategory.value = id; void loadPosts(1) }
function changePage(page: number): void { void loadPosts(page) }
async function togglePostLike(post: ResourcePost): Promise<void> { try { const result = await resourceAPI.togglePostLike(post.id); post.liked = result.liked; post.like_count += result.liked ? 1 : -1 } catch (error) { appStore.showError(errorMessage(error)) } }
function canDeletePost(post: ResourcePost): boolean { return authStore.isAdmin || authStore.user?.id === post.author.id }
async function removePost(post: ResourcePost): Promise<void> { if (!window.confirm(t('admin.resourceCenter.confirmDeletePost'))) return; try { if (authStore.isAdmin) await adminResourceAPI.deletePost(post.id); else await resourceAPI.deletePost(post.id); appStore.showSuccess(t('common.deleted')); await loadPosts(postsPage.value.page) } catch (error) { appStore.showError(errorMessage(error)) } }
async function publishPost(): Promise<void> { if (!postForm.category_id || !postForm.title.trim() || !richTextHasText(postForm.content)) return; postSubmitting.value = true; try { const post = await resourceAPI.createPost({ category_id: postForm.category_id, title: postForm.title.trim(), content: postForm.content }); appStore.showSuccess(t('resourceCenter.postPublished')); composerOpen.value = false; postForm.title = ''; postForm.content = ''; await router.push(`/resource-center/posts/${post.id}`) } catch (error) { appStore.showError(errorMessage(error)) } finally { postSubmitting.value = false } }
async function markAllRead(): Promise<void> { await resourceAPI.markAllNotificationsRead(); notifications.value = notifications.value.map(item => ({ ...item, read: true })) }
async function openNotification(item: ResourceNotification): Promise<void> { if (!item.read) { await resourceAPI.markNotificationRead(item.id); item.read = true } notificationOpen.value = false; await router.push(`/resource-center/posts/${item.post_id}`) }

onMounted(() => { void load() })
</script>
