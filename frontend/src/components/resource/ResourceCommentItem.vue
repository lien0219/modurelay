<template>
  <article class="resource-comment" :class="{ 'resource-comment--reply': comment.parent_id }">
    <div class="resource-comment__avatar">{{ comment.author.username.slice(0, 1).toUpperCase() }}</div>
    <div class="resource-comment__body">
      <header class="resource-comment__header">
        <div class="resource-comment__identity">
          <strong>{{ comment.author.username }}</strong>
          <span :class="{ 'is-official': comment.author.role === 'admin' }">
            <Icon :name="comment.author.role === 'admin' ? 'badge' : 'user'" size="xs" />
            {{ comment.author.role === 'admin' ? t('resourceCenter.official') : t('resourceCenter.member') }}
          </span>
          <time>{{ formatDate(comment.created_at) }}</time>
        </div>
        <button
          v-if="canDelete && comment.status !== 'deleted'"
          type="button"
          class="resource-comment__delete"
          :title="t('admin.resourceCenter.deleteComment')"
          @click="$emit('delete', comment)"
        >
          <Icon name="trash" size="xs" />
        </button>
      </header>

      <ResourceRichContent class="resource-comment__content" :content="comment.content" />
      <p v-if="comment.status === 'deleted'" class="resource-comment__deleted">{{ t('resourceCenter.deleted') }}</p>

      <footer class="resource-comment__actions">
        <button
          v-if="comment.status !== 'deleted'"
          type="button"
          :class="{ 'is-active': comment.liked }"
          @click="$emit('like', comment)"
        >
          <Icon name="check" size="xs" />
          {{ comment.like_count }}
        </button>
        <button v-if="comment.status !== 'deleted'" type="button" @click="$emit('reply', comment)">
          <Icon name="chatBubble" size="xs" />
          {{ t('resourceCenter.reply') }}
        </button>
      </footer>
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

<style scoped>
.resource-comment {
  display: grid;
  grid-template-columns: 34px minmax(0, 1fr);
  gap: 10px;
  padding: 16px 0;
  border-bottom: 1px solid var(--color-border-subtle);
}

.resource-comment:last-child {
  border-bottom: 0;
}

.resource-comment--reply {
  margin-left: 42px;
  padding-left: 12px;
  border-left: 2px solid var(--color-primary-border);
}

.resource-comment__avatar {
  display: grid;
  width: 34px;
  height: 34px;
  place-items: center;
  border-radius: 50%;
  background: var(--color-accent-soft);
  color: var(--color-accent);
  font-size: .72rem;
  font-weight: 700;
}

.resource-comment__body {
  min-width: 0;
}

.resource-comment__header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.resource-comment__identity {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
}

.resource-comment__identity strong {
  color: var(--color-text-primary);
  font-size: .75rem;
  font-weight: 670;
}

.resource-comment__identity > span {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  gap: 4px;
  padding: 0 7px;
  border-radius: 999px;
  background: var(--color-surface-soft);
  color: var(--color-text-muted);
  font-size: .62rem;
  font-weight: 600;
}

.resource-comment__identity > span.is-official {
  background: color-mix(in srgb, var(--color-warning) 9%, var(--color-surface));
  color: var(--color-warning);
}

.resource-comment__identity time {
  color: var(--color-text-disabled);
  font-size: .62rem;
}

.resource-comment__delete {
  display: grid;
  width: 28px;
  height: 28px;
  flex: 0 0 28px;
  place-items: center;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
}

.resource-comment__delete:hover {
  background: color-mix(in srgb, var(--color-danger) 8%, transparent);
  color: var(--color-danger);
}

.resource-comment__content {
  margin-top: 7px;
  color: var(--color-text-secondary);
  font-size: .78rem;
  line-height: 1.65;
}

.resource-comment__deleted {
  margin: 6px 0 0;
  color: var(--color-danger);
  font-size: .66rem;
}

.resource-comment__actions {
  display: flex;
  align-items: center;
  gap: 5px;
  margin-top: 8px;
}

.resource-comment__actions button {
  display: inline-flex;
  min-height: 28px;
  align-items: center;
  gap: 5px;
  padding: 0 8px;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  font: inherit;
  font-size: .66rem;
}

.resource-comment__actions button:hover,
.resource-comment__actions button.is-active {
  background: var(--color-primary-soft);
  color: var(--color-primary);
}

.resource-comment button:focus-visible {
  outline: 2px solid var(--color-primary-ring);
  outline-offset: 2px;
}

@media (max-width: 560px) {
  .resource-comment--reply {
    margin-left: 18px;
    padding-left: 9px;
  }
}
</style>
