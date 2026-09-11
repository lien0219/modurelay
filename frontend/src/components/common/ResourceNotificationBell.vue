<template>
  <div ref="root" class="relative">
    <button type="button" class="relative flex h-9 w-9 items-center justify-center rounded-lg text-gray-600 transition hover:bg-gray-100 dark:text-gray-400 dark:hover:bg-dark-800" :class="{ 'text-primary-600 dark:text-primary-400': unreadCount > 0 }" :aria-label="t('resourceCenter.notifications')" @click="toggle">
      <Icon name="chatBubble" size="md" />
      <span v-if="unreadCount" class="absolute right-1 top-1 flex h-2 w-2"><span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-red-500 opacity-75"></span><span class="relative inline-flex h-2 w-2 rounded-full bg-red-500"></span></span>
    </button>
    <div v-if="open" class="glass-popover scrollable-popover absolute right-0 top-full z-50 mt-2 w-80 overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
      <div class="flex items-center justify-between border-b border-gray-100 px-4 py-3 dark:border-dark-700"><span class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('resourceCenter.notifications') }}</span><button type="button" class="text-xs text-primary-600 disabled:opacity-50" :disabled="!unreadCount" @click="markAllRead">{{ t('resourceCenter.markAllRead') }}</button></div>
      <div v-if="items.length" class="max-h-72 overflow-y-auto"><button v-for="item in items" :key="item.id" type="button" class="flex w-full items-start gap-2 border-b border-gray-100 px-4 py-3 text-left text-xs hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700" :class="item.read ? 'opacity-60' : ''" @click="openItem(item)"><span class="mt-1 h-1.5 w-1.5 flex-shrink-0 rounded-full" :class="item.read ? 'bg-gray-300' : 'bg-primary-500'"></span><span class="min-w-0 flex-1 text-gray-700 dark:text-gray-200">{{ item.kind === 'reply' ? t('resourceCenter.notificationReply', { actor: item.actor.username }) : t('resourceCenter.notificationComment', { actor: item.actor.username }) }}<time class="mt-1 block text-[11px] text-gray-400">{{ formatDate(item.created_at) }}</time></span></button></div><p v-else class="px-4 py-8 text-center text-xs text-gray-500">{{ t('resourceCenter.noNotifications') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import resourceAPI from '@/api/resourceCenter'
import type { ResourceNotification } from '@/types'

const { t } = useI18n()
const router = useRouter()
const root = ref<HTMLElement | null>(null)
const open = ref(false)
const items = ref<ResourceNotification[]>([])
const unreadCount = computed(() => items.value.filter(item => !item.read).length)

function formatDate(value: string): string { return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) }
async function load(): Promise<void> { try { items.value = await resourceAPI.listNotifications() } catch { items.value = [] } }
async function toggle(): Promise<void> { open.value = !open.value; if (open.value) await load() }
async function markAllRead(): Promise<void> { await resourceAPI.markAllNotificationsRead(); items.value = items.value.map(item => ({ ...item, read: true })) }
async function openItem(item: ResourceNotification): Promise<void> { if (!item.read) { await resourceAPI.markNotificationRead(item.id); item.read = true } open.value = false; await router.push(`/resource-center/posts/${item.post_id}`) }
function handleOutside(event: MouseEvent): void { if (root.value && !root.value.contains(event.target as Node)) open.value = false }
onMounted(() => { document.addEventListener('click', handleOutside); void load() })
onBeforeUnmount(() => document.removeEventListener('click', handleOutside))
</script>
