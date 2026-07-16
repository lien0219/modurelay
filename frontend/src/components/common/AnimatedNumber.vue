<template>
  <span :class="className" :title="title">{{ displayText }}</span>
</template>

<script setup lang="ts">
import { toRef } from 'vue'
import { useGsapCounter, type GsapCounterFormat } from '@/composables/useGsapCounter'

const props = withDefaults(
  defineProps<{
    value: number | null | undefined
    format?: GsapCounterFormat
    decimals?: number
    locale?: string
    prefix?: string
    suffix?: string
    fallback?: string
    duration?: number
    fromZeroOnFirst?: boolean
    className?: string
    title?: string
  }>(),
  {
    format: 'integer',
    decimals: 0,
    fallback: '—',
    fromZeroOnFirst: true
  }
)

const { displayText } = useGsapCounter(toRef(props, 'value'), props)
</script>
