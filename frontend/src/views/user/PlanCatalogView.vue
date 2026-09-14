<template>
  <AppLayout>
    <section class="mx-auto max-w-7xl space-y-6">
      <div v-if="loading" class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <div v-for="i in 3" :key="i" class="h-[440px] animate-pulse rounded-2xl bg-gray-100 dark:bg-dark-800" />
      </div>

      <div v-else-if="error" class="card p-8 text-center text-sm text-red-600 dark:text-red-400">
        {{ error }}
      </div>

      <div v-else-if="!items.length" class="card p-12 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('planCatalog.empty') }}
      </div>

      <div v-else class="grid items-stretch gap-5 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="item in items"
          :key="item.id"
          :class="[
            'group relative flex h-full min-h-[440px] flex-col overflow-hidden rounded-2xl border bg-white shadow-sm transition-ui',
            'hover:-translate-y-0.5 hover:shadow-xl dark:bg-dark-800',
            cardClasses(item.accent).border,
            item.is_featured ? 'ring-2 ring-primary-500/50' : '',
          ]"
        >
          <div :class="['h-1.5 shrink-0', cardClasses(item.accent).bar]" />

          <div class="flex flex-1 flex-col p-4">
            <div class="mb-3 flex items-start justify-between gap-3">
              <div class="min-w-0 flex-1">
                <span
                  v-if="item.badge"
                  :class="[
                    'mb-1.5 inline-flex max-w-full items-center rounded-full px-2 py-0.5 text-[11px] font-medium',
                    cardClasses(item.accent).badge,
                  ]"
                >
                  {{ item.badge }}
                </span>
                <h2
                  :title="item.name"
                  class="min-w-0 break-words text-base font-bold leading-6 text-gray-900 line-clamp-2 dark:text-white"
                >
                  {{ item.name }}
                </h2>
                <p v-if="item.subtitle" class="mt-0.5 text-xs leading-relaxed text-gray-500 line-clamp-2 dark:text-dark-400">
                  {{ item.subtitle }}
                </p>
              </div>

              <div class="shrink-0 text-right">
                <div class="flex items-baseline justify-end gap-1">
                  <span class="text-xs text-gray-400 dark:text-dark-500">{{ currency(item.currency) }}</span>
                  <span :class="['text-2xl font-extrabold tracking-tight', cardClasses(item.accent).text]">{{ item.price }}</span>
                  <span class="text-xs font-medium text-gray-400 dark:text-dark-500">{{ item.currency }}</span>
                </div>
                <div class="mt-0.5 flex items-center justify-end gap-1">
                  <span :class="['inline-flex shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium', cardClasses(item.accent).badge]">
                    {{ period(item.billing_period) }}
                  </span>
                </div>
                <div v-if="item.original_price" class="mt-0.5 flex items-center justify-end gap-1.5">
                  <span class="text-xs text-gray-400 line-through dark:text-dark-500">{{ currency(item.currency) }}{{ item.original_price }}</span>
                  <span
                    v-if="discountText(item)"
                    :class="['rounded px-1 py-0.5 text-[10px] font-semibold', cardClasses(item.accent).discount]"
                  >
                    {{ discountText(item) }}
                  </span>
                </div>
              </div>
            </div>

            <div v-if="item.description" :class="['mb-3 rounded-lg px-3 py-2 text-xs leading-relaxed text-gray-600 dark:text-gray-300', cardClasses(item.accent).soft]">
              {{ item.description }}
            </div>

            <ul class="mb-3 flex-1 space-y-1.5">
              <li v-for="benefit in item.benefits" :key="benefit" class="flex items-start gap-1.5">
                <svg :class="['mt-0.5 h-3.5 w-3.5 shrink-0', cardClasses(item.accent).icon]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
                </svg>
                <span class="text-xs text-gray-600 dark:text-gray-300">{{ benefit }}</span>
              </li>
              <li v-if="!item.benefits.length" class="text-xs text-gray-400 dark:text-dark-500">-</li>
            </ul>

            <a
              :href="item.payment_url"
              target="_blank"
              rel="noopener noreferrer"
              :class="['mt-auto flex w-full items-center justify-center rounded-xl py-2.5 text-sm font-semibold transition-ui active:scale-[0.98]', cardClasses(item.accent).button]"
            >
              {{ t('planCatalog.cta') }}
            </a>
          </div>
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
import { currencySymbol } from '@/components/payment/currency'
import type { PlanCatalogAccent, PlanCatalogCurrency, PlanCatalogItem, PlanCatalogPeriod } from '@/types/planCatalog'

type CardClasses = {
  bar: string
  border: string
  badge: string
  text: string
  icon: string
  button: string
  discount: string
  soft: string
}

const CARD_CLASSES: Record<PlanCatalogAccent, CardClasses> = {
  indigo: {
    bar: 'bg-gradient-to-r from-indigo-400 to-indigo-500',
    border: 'border-indigo-500/25 dark:border-indigo-500/30',
    badge: 'bg-indigo-500/10 text-indigo-600 dark:bg-indigo-500/10 dark:text-indigo-300',
    text: 'text-indigo-600 dark:text-indigo-400',
    icon: 'text-indigo-500 dark:text-indigo-400',
    button: 'bg-indigo-500 text-white hover:bg-indigo-600 active:bg-indigo-700 dark:bg-indigo-500/80 dark:hover:bg-indigo-500',
    discount: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300',
    soft: 'bg-indigo-50/80 dark:bg-indigo-900/15',
  },
  emerald: {
    bar: 'bg-gradient-to-r from-emerald-400 to-emerald-500',
    border: 'border-emerald-500/25 dark:border-emerald-500/30',
    badge: 'bg-emerald-500/10 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-300',
    text: 'text-emerald-600 dark:text-emerald-400',
    icon: 'text-emerald-500 dark:text-emerald-400',
    button: 'bg-emerald-600 text-white hover:bg-emerald-700 active:bg-emerald-800 dark:bg-emerald-600/80 dark:hover:bg-emerald-600',
    discount: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300',
    soft: 'bg-emerald-50/80 dark:bg-emerald-900/15',
  },
  amber: {
    bar: 'bg-gradient-to-r from-amber-400 to-amber-500',
    border: 'border-amber-500/25 dark:border-amber-500/30',
    badge: 'bg-amber-500/10 text-amber-700 dark:bg-amber-500/10 dark:text-amber-300',
    text: 'text-amber-600 dark:text-amber-400',
    icon: 'text-amber-500 dark:text-amber-400',
    button: 'bg-amber-500 text-white hover:bg-amber-600 active:bg-amber-700 dark:bg-amber-500/80 dark:hover:bg-amber-500',
    discount: 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300',
    soft: 'bg-amber-50/80 dark:bg-amber-900/15',
  },
  rose: {
    bar: 'bg-gradient-to-r from-rose-400 to-rose-500',
    border: 'border-rose-500/25 dark:border-rose-500/30',
    badge: 'bg-rose-500/10 text-rose-600 dark:bg-rose-500/10 dark:text-rose-300',
    text: 'text-rose-600 dark:text-rose-400',
    icon: 'text-rose-500 dark:text-rose-400',
    button: 'bg-rose-500 text-white hover:bg-rose-600 active:bg-rose-700 dark:bg-rose-500/80 dark:hover:bg-rose-500',
    discount: 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300',
    soft: 'bg-rose-50/80 dark:bg-rose-900/15',
  },
  slate: {
    bar: 'bg-gradient-to-r from-slate-500 to-slate-700',
    border: 'border-slate-300 dark:border-slate-700',
    badge: 'bg-slate-500/10 text-slate-600 dark:bg-slate-500/10 dark:text-slate-300',
    text: 'text-slate-700 dark:text-slate-200',
    icon: 'text-slate-500 dark:text-slate-300',
    button: 'bg-slate-700 text-white hover:bg-slate-800 active:bg-slate-900 dark:bg-slate-600 dark:hover:bg-slate-500',
    discount: 'bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-200',
    soft: 'bg-slate-50/80 dark:bg-slate-900/25',
  },
}

const { t } = useI18n()
const items = ref<PlanCatalogItem[]>([])
const loading = ref(true)
const error = ref('')

const currency = (value: PlanCatalogCurrency) => currencySymbol(value)
const period = (value: PlanCatalogPeriod) => t(`planCatalog.period.${value}`)
const cardClasses = (value: PlanCatalogAccent) => CARD_CLASSES[value] || CARD_CLASSES.indigo

function discountText(item: PlanCatalogItem): string {
  const price = Number(item.price)
  const original = Number(item.original_price)
  if (!Number.isFinite(price) || !Number.isFinite(original) || original <= 0 || price >= original) return ''
  return `-${Math.round((1 - price / original) * 100)}%`
}

onMounted(async () => {
  try {
    items.value = (await planCatalogAPI.list()).data ?? []
  } catch {
    error.value = t('planCatalog.loadError')
  } finally {
    loading.value = false
  }
})
</script>
