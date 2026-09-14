<template>
  <AppLayout>
    <section class="mx-auto max-w-7xl space-y-8">
      <header class="space-y-2">
        <p class="text-xs font-semibold uppercase tracking-[0.18em] text-primary-600 dark:text-primary-400">{{ t('planCatalog.eyebrow') }}</p>
        <h1 class="text-3xl font-semibold tracking-tight text-gray-900 dark:text-white">{{ t('planCatalog.title') }}</h1>
        <p class="max-w-2xl text-sm text-gray-500 dark:text-gray-400">{{ t('planCatalog.description') }}</p>
      </header>
      <div v-if="loading" class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="i in 3" :key="i" class="h-96 animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800" />
      </div>
      <div v-else-if="error" class="card p-8 text-center text-sm text-red-600 dark:text-red-400">{{ error }}</div>
      <div v-else-if="!items.length" class="card p-12 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('planCatalog.empty') }}</div>
      <div v-else class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <article v-for="item in items" :key="item.id" :class="['relative flex flex-col rounded-2xl border bg-white p-6 shadow-sm dark:bg-dark-800', accentClass(item.accent), item.is_featured ? 'ring-2 ring-primary-500/50' : 'border-gray-200 dark:border-dark-700']">
          <span v-if="item.badge" class="absolute right-4 top-4 rounded-full bg-primary-50 px-2.5 py-1 text-xs font-medium text-primary-700 dark:bg-primary-900/30 dark:text-primary-300">{{ item.badge }}</span>
          <h2 class="pr-16 text-xl font-semibold text-gray-900 dark:text-white">{{ item.name }}</h2>
          <p class="mt-1 min-h-10 text-sm text-gray-500 dark:text-gray-400">{{ item.subtitle }}</p>
          <div class="mt-6 flex items-end gap-2"><span class="text-4xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ currency(item.currency) }}{{ item.price }}</span><span class="pb-1 text-xs text-gray-500">{{ period(item.billing_period) }}</span></div>
          <span v-if="item.original_price" class="mt-1 text-sm text-gray-400 line-through">{{ currency(item.currency) }}{{ item.original_price }}</span>
          <p v-if="item.description" class="mt-4 text-sm leading-6 text-gray-600 dark:text-gray-300">{{ item.description }}</p>
          <ul class="mt-6 flex-1 space-y-3"><li v-for="benefit in item.benefits" :key="benefit" class="flex gap-2 text-sm text-gray-700 dark:text-gray-200"><span class="text-primary-600">✓</span><span>{{ benefit }}</span></li></ul>
          <a :href="item.payment_url" target="_blank" rel="noopener noreferrer" class="btn btn-primary mt-6 w-full justify-center">{{ t('planCatalog.cta') }}</a>
        </article>
      </div>
    </section>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { planCatalogAPI } from '@/api/planCatalog'
import type { PlanCatalogAccent, PlanCatalogCurrency, PlanCatalogItem, PlanCatalogPeriod } from '@/types/planCatalog'
const { t } = useI18n(); const items = ref<PlanCatalogItem[]>([]); const loading = ref(true); const error = ref('')
const currency = (value: PlanCatalogCurrency) => ({ CNY: '¥', USD: '$', EUR: '€', HKD: 'HK$' }[value])
const period = (value: PlanCatalogPeriod) => t(`planCatalog.period.${value}`)
const accentClass = (value: PlanCatalogAccent) => ({ indigo: 'border-primary-200 dark:border-primary-800', emerald: 'border-emerald-200 dark:border-emerald-800', amber: 'border-amber-200 dark:border-amber-800', rose: 'border-rose-200 dark:border-rose-800', slate: 'border-slate-300 dark:border-slate-700' }[value])
onMounted(async () => { try { items.value = (await planCatalogAPI.list()).data ?? [] } catch { error.value = t('planCatalog.loadError') } finally { loading.value = false } })
</script>
