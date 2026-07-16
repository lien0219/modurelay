<template>
  <motion.div
    class="empty-state"
    :initial="motionInitial"
    :animate="motionAnimate"
    :transition="motionTransition"
  >
    <!-- Icon -->
    <div
      class="mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-100 dark:bg-dark-800"
    >
      <slot name="icon">
        <component v-if="icon" :is="icon" class="empty-state-icon mb-0 h-8 w-8" aria-hidden="true" />
        <svg
          v-else
          class="empty-state-icon mb-0 h-8 w-8"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          stroke-width="1.5"
          aria-hidden="true"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"
          />
        </svg>
      </slot>
    </div>

    <!-- Title -->
    <h3 class="empty-state-title">
      {{ displayTitle }}
    </h3>

    <!-- Description -->
    <p v-if="description || message" class="empty-state-description">
      {{ description || message }}
    </p>

    <!-- Action -->
    <div v-if="actionText || $slots.action" class="mt-6">
      <slot name="action">
        <component
          :is="actionTo ? 'RouterLink' : 'button'"
          v-if="actionText"
          :to="actionTo"
          @click="!actionTo && $emit('action')"
          class="btn btn-primary"
        >
          <Icon v-if="actionIcon" name="plus" size="md" class="mr-2" />
          {{ actionText }}
        </component>
      </slot>
    </div>
  </motion.div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Component } from 'vue'
import { motion } from 'motion-v'
import Icon from '@/components/icons/Icon.vue'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'

const { t } = useI18n()
const prefersReducedMotion = usePrefersReducedMotion()

interface Props {
  icon?: Component | string
  title?: string
  description?: string
  actionText?: string
  actionTo?: string | object
  actionIcon?: boolean
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  description: '',
  actionIcon: true
})

const displayTitle = computed(() => props.title || t('common.noData'))

const motionInitial = computed(() =>
  prefersReducedMotion.value ? { opacity: 1 } : { opacity: 0 }
)
const motionAnimate = computed(() => ({ opacity: 1 }))
const motionTransition = computed(() => ({
  duration: prefersReducedMotion.value ? 0.01 : 0.18,
  ease: 'easeOut'
}))

defineEmits(['action'])
</script>
