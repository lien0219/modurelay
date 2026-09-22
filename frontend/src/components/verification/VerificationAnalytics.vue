<template>
  <section class="space-y-4" :aria-label="t(admin ? 'verificationRecords.analytics.adminTitle' : 'verificationRecords.analytics.userTitle')">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t(admin ? 'verificationRecords.analytics.adminTitle' : 'verificationRecords.analytics.userTitle') }}</h2>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t(admin ? 'verificationRecords.analytics.adminDescription' : 'verificationRecords.analytics.userDescription') }}</p>
      </div>
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('verificationRecords.analytics.terminalHint') }}</span>
    </div>

    <div class="grid gap-4" :class="rateSections.length === 1 ? 'xl:grid-cols-1' : rateSections.length === 2 ? 'xl:grid-cols-2' : 'xl:grid-cols-3'">
      <article v-for="section in rateSections" :key="section.key" class="card min-w-0 p-5">
        <div class="mb-4 flex items-center gap-2">
          <Icon :name="section.icon" size="sm" class="text-primary-600 dark:text-primary-300" aria-hidden="true" />
          <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ section.title }}</h3>
        </div>
        <div v-if="section.items.length" class="h-52">
          <Bar :data="successRateData(section.key, section.items)" :options="rateOptions" />
        </div>
        <p v-else class="flex h-52 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('verificationRecords.analytics.empty') }}</p>
        <div v-if="section.items.length" class="mt-4 max-h-40 overflow-auto border-t border-gray-100 pt-2 dark:border-dark-700">
          <div v-for="item in section.items" :key="item.key" class="flex items-center justify-between gap-3 py-1.5 text-xs">
            <span class="min-w-0 truncate text-gray-600 dark:text-gray-300">{{ breakdownLabel(section.key, item.key) }}</span>
            <span class="shrink-0 tabular-nums text-gray-900 dark:text-white">{{ percent(item.success_rate) }} · {{ item.success }}/{{ item.total }}</span>
          </div>
        </div>
      </article>
    </div>

    <article v-if="admin && analytics.financial.length" class="card min-w-0 p-5">
      <div class="mb-4 flex items-center gap-2">
        <Icon name="chartBar" size="sm" class="text-primary-600 dark:text-primary-300" aria-hidden="true" />
        <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('verificationRecords.analytics.profitTitle') }}</h3>
      </div>
      <div class="grid gap-5 lg:grid-cols-[minmax(0,1.5fr)_minmax(320px,.8fr)] lg:items-center">
        <div class="h-64"><Bar :data="financialData" :options="financialOptions" /></div>
        <div class="overflow-x-auto">
          <table class="w-full min-w-[420px] text-sm">
            <thead class="text-xs text-gray-500 dark:text-gray-400"><tr><th class="py-2 text-left">{{ t('verificationRecords.analytics.currency') }}</th><th class="py-2 text-right">{{ t('verificationRecords.summary.grossAmount') }}</th><th class="py-2 text-right">{{ t('verificationRecords.summary.netRevenue') }}</th><th class="py-2 text-right">{{ t('verificationRecords.summary.netProviderCost') }}</th><th class="py-2 text-right">{{ t('verificationRecords.summary.netProfit') }}</th></tr></thead>
            <tbody><tr v-for="item in analytics.financial" :key="item.currency" class="border-t border-gray-100 dark:border-dark-700"><td class="py-2 font-medium">{{ item.currency }}<span v-if="item.estimated" class="ml-1 text-[10px] text-amber-600 dark:text-amber-300">{{ t('verificationRecords.estimated') }}</span></td><td class="py-2 text-right tabular-nums">{{ money(item.sale_amount, item.currency) }}</td><td class="py-2 text-right tabular-nums">{{ money(item.net_revenue, item.currency) }}</td><td class="py-2 text-right tabular-nums">{{ money(item.provider_cost, item.currency) }}</td><td class="py-2 text-right tabular-nums" :class="item.estimated_profit < 0 ? 'text-red-600 dark:text-red-300' : 'text-green-700 dark:text-green-300'">{{ money(item.estimated_profit, item.currency) }}</td></tr></tbody>
          </table>
        </div>
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar } from 'vue-chartjs'
import { BarElement, CategoryScale, Chart as ChartJS, Legend, LinearScale, Tooltip } from 'chart.js'
import Icon from '@/components/icons/Icon.vue'
import type { VerificationRecordAnalytics, VerificationRecordBreakdown } from '@/api/verificationRecords'
import { formatCurrency } from '@/utils/format'
import { getChartJsAnimation } from '@/utils/chartAnimation'
import { useChartThemeColors, withChartAlpha } from '@/utils/chartColors'

ChartJS.register(CategoryScale, LinearScale, BarElement, Tooltip, Legend)

const props = defineProps<{ analytics: VerificationRecordAnalytics; admin: boolean; verificationType?: string; platformLabels?: Record<string, string>; countryLabels?: Record<string, string> }>()
const { t } = useI18n()
const theme = useChartThemeColors()

const rateSections = computed(() => {
  const platform = { key: 'platform', title: t('verificationRecords.analytics.byPlatform'), icon: 'grid' as const, items: props.analytics.by_platform.slice(0, 10) }
  if (props.verificationType === 'email') return [platform]
  const country = { key: 'country', title: t('verificationRecords.analytics.byCountry'), icon: 'globe' as const, items: props.analytics.by_country.slice(0, 10) }
  if (props.verificationType === 'sms') return [platform, country]
  return [{ key: 'type', title: t('verificationRecords.analytics.byType'), icon: 'phone' as const, items: props.analytics.by_type.slice(0, 10) }, platform, country]
})

function breakdownLabel(dimension: string, key: string): string {
  if (dimension === 'type') return t(`verificationRecords.types.${key}`)
  if (dimension === 'platform') return props.platformLabels?.[key] || key
  return props.countryLabels?.[key] || key
}

function successRateData(dimension: string, items: VerificationRecordBreakdown[]) {
  return {
    labels: items.map(item => breakdownLabel(dimension, item.key)),
    datasets: [{
      label: t('verificationRecords.analytics.successRate'),
      data: items.map(item => Number((item.success_rate * 100).toFixed(2))),
      backgroundColor: withChartAlpha(theme.value.primary, 0.72),
      borderColor: theme.value.primary,
      borderWidth: 1,
      borderRadius: 4,
    }],
  }
}

const rateOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: getChartJsAnimation(),
  indexAxis: 'y' as const,
  scales: {
    x: { beginAtZero: true, max: 100, ticks: { color: theme.value.text, callback: (value: string | number) => `${value}%` }, grid: { color: theme.value.grid } },
    y: { ticks: { color: theme.value.text }, grid: { display: false } },
  },
  plugins: { legend: { display: false }, tooltip: { backgroundColor: theme.value.surfaceRaised, titleColor: theme.value.text, bodyColor: theme.value.text, borderColor: theme.value.grid, borderWidth: 1 } },
}))

const financialData = computed(() => ({
  labels: props.analytics.financial.map(item => item.currency),
  datasets: [
    { label: t('verificationRecords.summary.grossAmount'), data: props.analytics.financial.map(item => item.sale_amount), backgroundColor: withChartAlpha(theme.value.primary, 0.72), borderColor: theme.value.primary, borderWidth: 1, borderRadius: 4 },
    { label: t('verificationRecords.summary.netRevenue'), data: props.analytics.financial.map(item => item.net_revenue), backgroundColor: withChartAlpha(theme.value.secondary, 0.72), borderColor: theme.value.secondary, borderWidth: 1, borderRadius: 4 },
    { label: t('verificationRecords.summary.netProviderCost'), data: props.analytics.financial.map(item => item.provider_cost), backgroundColor: withChartAlpha(theme.value.primary, 0.48), borderColor: theme.value.primary, borderWidth: 1, borderRadius: 4 },
    { label: t('verificationRecords.summary.netProfit'), data: props.analytics.financial.map(item => item.estimated_profit), backgroundColor: withChartAlpha(theme.value.success, 0.72), borderColor: theme.value.success, borderWidth: 1, borderRadius: 4 },
  ],
}))

const financialOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: getChartJsAnimation(),
  scales: { x: { ticks: { color: theme.value.text }, grid: { display: false } }, y: { ticks: { color: theme.value.text }, grid: { color: theme.value.grid } } },
  plugins: { legend: { labels: { color: theme.value.text, usePointStyle: true } }, tooltip: { backgroundColor: theme.value.surfaceRaised, titleColor: theme.value.text, bodyColor: theme.value.text, borderColor: theme.value.grid, borderWidth: 1 } },
}))

const percent = (value: number) => `${(value * 100).toFixed(1)}%`
const money = (value: number, currency: string) => formatCurrency(value, currency)
</script>
