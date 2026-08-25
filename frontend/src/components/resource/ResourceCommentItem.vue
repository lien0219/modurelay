<template>
  <article class="border-l border-gray-200 pl-4 dark:border-dark-600" :class="comment.parent_id ? 'ml-5 sm:ml-10' : ''">
    <div class="flex items-start gap-3">
      <div class="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-full bg-primary-50 text-sm font-semibold text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">
        {{ comment.author.username.slice(0, 1).toUpperCase() }}
      </div>
      <div class="min-w-0 flex-1">
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span class="font-semibold text-gray-900 dark:text-white">{{ comment.author.username }}</span>
          <span
            class="inline-flex items-center rounded-full px-2 py-0.5 font-medium"
            :class="comment.author.role === 'admin' ? 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-300' : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
          >
            <Icon :name="comment.author.role === 'admin' ? 'badge' : 'user'" size="xs" class="mr-1" />
            {{ comment.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}
          </span>
          <time class="text-gray-400 dark:text-gray-500">{{ formatDate(comment.created_at) }}</time>
        </div>
        <ResourceRichContent class="mt-2" :content="comment.content" />
        <p v-if="comment.status === 'deleted'" class="mt-2 text-xs text-red-500">{{ t('resourceCenter.deleted') }}</p>
        <div class="mt-2 flex items-center gap-3 text-xs text-gray-500 dark:text-gray-400">
          <button v-if="comment.status !== 'deleted'" type="button" class="inline-flex items-center gap-1 hover:text-primary-600" @click="$emit('like', comment)">
            <Icon name="check" size="xs" :class="comment.liked ? 'text-primary-600' : ''" />
            {{ comment.like_count }}
          </button>
          <button v-if="comment.status !== 'deleted'" type="button" class="inline-flex items-center gap-1 hover:text-primary-600" @click="$emit('reply', comment)">
            <Icon name="chatBubble" size="xs" /> {{ t('resourceCenter.reply') }}
          </button>
          <button v-if="canDelete && comment.status !== 'deleted'" type="button" class="ml-auto inline-flex items-center gap-1 text-red-500 hover:text-red-700" :title="t('admin.resourceCenter.deleteComment')" @click="$emit('delete', comment)"><Icon name="trash" size="xs" /></button>
        </div>
      </div>
    </div>
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ResourceRichContent from './ResourceRichContent.vue'
import type { ResourceComment } from '@/types'

defineOptions({ name: 'ResourceCommentItem' })
withDefaults(defineProps<{ comment: ResourceComment; canDelete?: boolean }>(), { canDelete: false })
defineEmits<{ reply: [comment: ResourceComment]; like: [comment: ResourceComment]; delete: [comment: ResourceComment] }>()
const { t } = useI18n()

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}
</script>
