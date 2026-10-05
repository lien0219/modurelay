<template>
  <section class="workspace-daily-spend">
    <div class="workspace-daily-spend__heading">
      <h3>{{ t('workspace.dailySpend') }}</h3>
      <p v-if="timezone">{{ t('workspace.timezone') }}: {{ timezone }}</p>
    </div>
    <template v-if="rows.length">
      <div class="workspace-daily-spend__chart">
        <Line :data="chartData" :options="chartOptions" role="img" :aria-label="t('workspace.dailySpend')" />
      </div>
      <div class="workspace-daily-spend__table-wrap">
        <table>
          <caption class="sr-only">{{ t('workspace.dailySpend') }}</caption>
          <thead><tr><th scope="col">{{ t('workspace.date') }}</th><th scope="col">{{ t('workspace.requests') }}</th><th scope="col">{{ t('workspace.spend') }}</th></tr></thead>
          <tbody>
            <tr v-for="row in rows" :key="row.date">
              <th scope="row"><time :datetime="row.date">{{ row.date }}</time></th>
              <td>{{ Number.isFinite(row.requests) ? row.requests.toLocaleString() : '—' }}</td>
              <td>{{ money(row.spend) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
    <p v-else class="workspace-daily-spend__empty">{{ t('workspace.noUsage') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend, type ChartOptions } from 'chart.js'
import { Line } from 'vue-chartjs'
import type { WorkspaceDailySpendPoint } from '@/api/workspace'
import { getChartJsAnimation } from '@/utils/chartAnimation'
import { getChartSeriesStyle, useChartPalette, useChartThemeColors } from '@/utils/chartColors'
import { formatCurrency } from '@/utils/format'

ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Tooltip, Legend)

const props = defineProps<{ rows: WorkspaceDailySpendPoint[]; timezone?: string }>()
const { t } = useI18n()
const palette = useChartPalette()
const colors = useChartThemeColors()

function money(value: number) { return Number.isFinite(value) ? formatCurrency(value) : '—' }

// Dates and amounts are server aggregates in the requested timezone.
const chartData = computed(() => ({
  labels: props.rows.map(row => row.date),
  datasets: [{
    label: t('workspace.spend'),
    data: props.rows.map(row => Number.isFinite(row.spend) ? row.spend : null),
    borderColor: palette.value[0],
    backgroundColor: palette.value[0],
    borderWidth: 2,
    pointRadius: 2,
    pointHoverRadius: 4,
    tension: 0,
    ...getChartSeriesStyle(0),
  }],
}))

const chartOptions = computed<ChartOptions<'line'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: getChartJsAnimation(),
  interaction: { intersect: false, mode: 'index' },
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: { label: context => `${t('workspace.spend')}: ${money(context.parsed.y ?? Number.NaN)}` },
    },
  },
  scales: {
    x: { grid: { display: false }, ticks: { color: colors.value.text, maxTicksLimit: 10 } },
    y: { beginAtZero: true, grid: { color: colors.value.grid }, ticks: { color: colors.value.text, callback: value => money(Number(value)) } },
  },
}))
</script>

<style scoped>
.workspace-daily-spend { min-width: 0; margin-top: 20px; }
.workspace-daily-spend__heading { display: flex; align-items: baseline; flex-wrap: wrap; justify-content: space-between; gap: 8px; }
.workspace-daily-spend h3 { margin: 0; color: var(--color-text-primary); font-size: 16px; }
.workspace-daily-spend__heading p, .workspace-daily-spend__empty { margin: 0; color: var(--color-text-muted); font-size: 12px; }
.workspace-daily-spend__chart { position: relative; height: 220px; margin: 14px 0; }
.workspace-daily-spend__table-wrap { max-height: 260px; overflow: auto; }
.workspace-daily-spend table { width: 100%; min-width: 300px; border-collapse: collapse; }
.workspace-daily-spend th, .workspace-daily-spend td { border-bottom: 1px solid var(--color-border-subtle); padding: 12px 8px; color: var(--color-text-secondary); font-size: 13px; font-variant-numeric: tabular-nums; text-align: right; }
.workspace-daily-spend thead th { color: var(--color-text-muted); font-size: 12px; }
.workspace-daily-spend th:first-child { text-align: left; }
.workspace-daily-spend tbody th { font-weight: 400; }
.workspace-daily-spend td { white-space: nowrap; }
.workspace-daily-spend__empty { margin-top: 14px; }
</style>
