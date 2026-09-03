<template>
  <Teleport to="body">
    <div
      v-if="displayedAnnouncement"
      data-testid="announcement-popup-overlay"
      class="fixed inset-0 z-[120] flex items-start justify-center overflow-y-auto bg-black/60 p-4 pt-[8vh] backdrop-blur-sm"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="`announcement-popup-title-${displayedAnnouncement.id ?? 'preview'}`"
    >
      <div
        data-testid="announcement-popup-panel"
        class="announcement-popup-panel w-full max-w-[680px] overflow-hidden rounded-2xl bg-white shadow-lg ring-1 ring-black/5 dark:bg-dark-800 dark:ring-white/10"
        @click.stop
      >
        <!-- Header -->
        <div class="relative overflow-hidden border-b border-amber-100/80 bg-amber-50/80 px-8 py-6 dark:border-dark-700/50 dark:bg-amber-900/20">
          <div class="relative z-10">
            <!-- Icon and badge -->
            <div class="mb-3 flex items-center gap-2">
              <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-amber-600 text-white shadow-sm">
                <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
                </svg>
              </div>
              <span class="inline-flex items-center gap-1.5 rounded-lg bg-amber-600 px-2.5 py-1 text-xs font-medium text-white shadow-sm">
                <span class="relative flex h-2 w-2">
                  <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-white opacity-75"></span>
                  <span class="relative inline-flex h-2 w-2 rounded-full bg-white"></span>
                </span>
                {{ t('announcements.unread') }}
              </span>
            </div>

            <!-- Title -->
            <h2 :id="`announcement-popup-title-${displayedAnnouncement.id ?? 'preview'}`" class="mb-2 text-2xl font-bold leading-tight text-gray-900 dark:text-white">
              {{ displayedAnnouncement.title }}
            </h2>

            <!-- Time -->
            <div class="flex items-center gap-1.5 text-sm text-gray-600 dark:text-gray-400">
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <time>{{ formatRelativeWithDateTime(displayedAnnouncement.created_at) }}</time>
            </div>
          </div>
        </div>

        <!-- Body -->
        <div class="max-h-[50vh] overflow-y-auto bg-white px-8 py-8 dark:bg-dark-800">
          <div class="relative">
            <div class="absolute bottom-0 left-0 top-0 w-1 rounded-full bg-amber-600"></div>
            <div class="pl-6">
              <div
                class="markdown-body prose prose-sm max-w-none dark:prose-invert"
                v-html="renderedContent"
              ></div>
            </div>
          </div>
        </div>

        <!-- Footer -->
        <div class="border-t border-gray-100 bg-gray-50/50 px-8 py-5 dark:border-dark-700 dark:bg-dark-900/30">
          <div class="flex items-center justify-end">
            <button
              @click="handleDismiss"
              data-testid="announcement-popup-dismiss"
              class="rounded-lg bg-amber-600 px-6 py-2.5 text-sm font-medium text-white shadow-sm transition-colors hover:bg-amber-700"
            >
              <span class="flex items-center gap-2">
                <svg v-if="preview" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
                </svg>
                <svg v-else class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
                </svg>
                {{ preview ? t('common.close') : t('announcements.markRead') }}
              </span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'
import type { Announcement, UserAnnouncement } from '@/types'
import '@/styles/announcement-markdown.css'

type PreviewAnnouncement = Pick<Announcement | UserAnnouncement, 'title' | 'content' | 'created_at'> & {
  id?: number
}

const props = withDefaults(defineProps<{
  announcement?: PreviewAnnouncement | null
  preview?: boolean
}>(), {
  announcement: null,
  preview: false,
})

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const displayedAnnouncement = computed(() => (
  props.preview ? props.announcement : announcementStore.currentPopup
))

marked.setOptions({
  breaks: true,
  gfm: true,
})

const renderedContent = computed(() => {
  const content = displayedAnnouncement.value?.content
  if (!content) return ''
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

function unlockBodyScroll() {
  document.body.style.removeProperty('overflow')
}

function handleDismiss() {
  if (props.preview) {
    emit('close')
    return
  }
  announcementStore.dismissPopup()
}

watch(
  displayedAnnouncement,
  (popup) => {
    if (popup) {
      document.body.style.overflow = 'hidden'
    } else {
      unlockBodyScroll()
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  unlockBodyScroll()
})
</script>

<style scoped>
.announcement-popup-panel {
  opacity: 1 !important;
  visibility: visible !important;
  filter: none !important;
  animation: announcement-popup-enter 220ms cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes announcement-popup-enter {
  from { transform: translateY(-10px) scale(0.97); }
  to { transform: translateY(0) scale(1); }
}

@media (prefers-reduced-motion: reduce) {
  .announcement-popup-panel {
    animation: none;
  }
}

/* Scrollbar Styling */
.overflow-y-auto::-webkit-scrollbar {
  width: 8px;
}

.overflow-y-auto::-webkit-scrollbar-track {
  background: transparent;
}

.overflow-y-auto::-webkit-scrollbar-thumb {
  background: #94a3b8;
  border-radius: 4px;
}

.dark .overflow-y-auto::-webkit-scrollbar-thumb {
  background: #4b5563;
}
</style>
