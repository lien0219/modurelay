<template>
  <div class="card p-4">
    <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">
      {{ t('payment.admin.dailyRevenue') }}
    </h3>
    <div class="h-64">
      <div v-if="loading" class="flex h-full items-center justify-center">
        <LoadingSpinner size="md" />
      </div>
      <Line v-else-if="chartData" :data="chartData" :options="chartOptions" />
      <div
        v-else
        class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('payment.admin.noData') }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { DailyPaymentStats } from '@/types/payment'
import { getChartJsAnimation } from '@/utils/chartAnimation'
import {
  getChartSeriesStyle,
  useChartPalette,
  useChartThemeColors,
  withChartAlpha
} from '@/utils/chartColors'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, Filler)

const { t } = useI18n()

const props = defineProps<{
  data: DailyPaymentStats[]
  loading?: boolean
}>()

const chartPalette = useChartPalette()
const chartTheme = useChartThemeColors()

const chartData = computed(() => {
  if (!props.data || props.data.length === 0) return null
  const currencies = [...new Set(props.data.flatMap(day => Object.keys(day.amount)))].sort()
  return {
    labels: props.data.map(d => d.date),
    datasets: [
      ...currencies.map((currency, index) => {
        const borderColor = chartPalette.value[index % chartPalette.value.length]
        return {
          label: `${currency} ${t('payment.admin.revenue')}`,
          data: props.data.map(day => day.amount[currency] || 0),
          borderColor,
          backgroundColor: withChartAlpha(borderColor, 0.12),
          fill: true,
          tension: 0.3,
          ...getChartSeriesStyle(index),
          borderWidth: 2.5,
          pointRadius: 3,
          pointHoverRadius: 5,
        }
      }),
      {
        label: t('payment.admin.orderCount'),
        data: props.data.map(d => d.count),
        borderColor: chartPalette.value[currencies.length % chartPalette.value.length],
        backgroundColor: withChartAlpha(
          chartPalette.value[currencies.length % chartPalette.value.length],
          0.12
        ),
        fill: false,
        tension: 0.3,
        ...getChartSeriesStyle(currencies.length),
        borderWidth: 2.5,
        pointRadius: 3,
        pointHoverRadius: 5,
        yAxisID: 'y1',
      }
    ]
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: getChartJsAnimation(),
  interaction: { mode: 'index' as const, intersect: false },
  scales: {
    y: {
      type: 'linear' as const,
      display: true,
      position: 'left' as const,
      grid: { color: chartTheme.value.grid },
      ticks: { color: chartTheme.value.text },
      title: { display: true, text: t('payment.admin.revenue'), color: chartTheme.value.text },
    },
    y1: {
      type: 'linear' as const,
      display: true,
      position: 'right' as const,
      ticks: { color: chartTheme.value.text },
      title: { display: true, text: t('payment.admin.orderCount'), color: chartTheme.value.text },
      grid: { drawOnChartArea: false },
    }
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: { color: chartTheme.value.text, usePointStyle: true }
    },
    tooltip: {
      backgroundColor: chartTheme.value.surfaceRaised,
      titleColor: chartTheme.value.text,
      bodyColor: chartTheme.value.text,
      borderColor: chartTheme.value.grid,
      borderWidth: 1
    }
  }
}))
</script>
