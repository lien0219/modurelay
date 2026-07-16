<template>
  <AppLayout>
    <div class="relative min-h-[12rem] space-y-6">
      <AnimatePresence mode="wait">
        <motion.div
          v-if="loading && !stats"
          key="dashboard-loading"
          class="flex items-center justify-center py-12"
          :initial="fadeInitial"
          :animate="fadeAnimate"
          :exit="fadeExit"
          :transition="fadeTransition"
        >
          <LoadingSpinner />
        </motion.div>

        <motion.div
          v-else-if="stats"
          :key="'dashboard-content'"
          class="space-y-6"
          :initial="contentInitial"
          :animate="fadeAnimate"
          :exit="fadeExit"
          :transition="fadeTransition"
        >
          <div
            ref="contentRoot"
            class="space-y-6"
            :class="{ 'dashboard-reveal-pending': revealPending }"
          >
            <UserDashboardStats
              :stats="stats"
              :balance="user?.balance || 0"
              :is-simple="authStore.isSimpleMode"
              :platform-quotas="platformQuotas"
            />
            <div data-reveal>
              <UserDashboardCharts
                v-model:startDate="startDate"
                v-model:endDate="endDate"
                v-model:granularity="granularity"
                :loading="loadingCharts"
                :trend="trendData"
                :models="modelStats"
                @dateRangeChange="loadCharts"
                @granularityChange="loadCharts"
                @refresh="refreshAll"
              />
            </div>
            <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
              <div data-reveal class="lg:col-span-2">
                <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
              </div>
              <div data-reveal class="lg:col-span-1">
                <UserDashboardQuickActions />
              </div>
            </div>
          </div>
        </motion.div>
      </AnimatePresence>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { AnimatePresence, motion } from 'motion-v'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import { usePrefersReducedMotion } from '@/composables/usePrefersReducedMotion'
import { useDashboardReveal } from '@/composables/useDashboardReveal'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'

const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref('day')

const contentRoot = ref<HTMLElement | null>(null)
const prefersReducedMotion = usePrefersReducedMotion()
const { isPending: revealPending } = useDashboardReveal(contentRoot, {
  scopeKey: 'user-dashboard'
})

const fadeInitial = computed(() =>
  prefersReducedMotion.value ? { opacity: 1 } : { opacity: 0 }
)
/** Content enter is owned by GSAP reveal — avoid double opacity fade */
const contentInitial = { opacity: 1 }
const fadeAnimate = computed(() => ({ opacity: 1 }))
const fadeExit = computed(() => ({ opacity: 0 }))
const fadeTransition = computed(() => ({
  duration: prefersReducedMotion.value ? 0.01 : 0.18
}))

const loadStats = async () => {
  // Keep existing content visible on refresh to avoid full re-entrance flash
  if (!stats.value) loading.value = true
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}
const loadCharts = async () => {
  loadingCharts.value = true
  try {
    const res = await Promise.all([
      usageAPI.getDashboardTrend({
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value as any
      }),
      usageAPI.getDashboardModels({ start_date: startDate.value, end_date: endDate.value })
    ])
    trendData.value = res[0].trend || []
    modelStats.value = res[1].models || []
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    loadingCharts.value = false
  }
}
const loadRecent = async () => {
  loadingUsage.value = true
  try {
    const res = await usageAPI.getByDateRange(startDate.value, endDate.value)
    recentUsage.value = res.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}
const loadPlatformQuotas = async () => {
  try {
    const data = await getMyPlatformQuotas()
    platformQuotas.value = data.platform_quotas ?? []
  } catch (error) {
    console.warn('Failed to load platform quotas:', error)
    platformQuotas.value = []
  }
}
const refreshAll = () => {
  loadStats()
  loadCharts()
  loadRecent()
  loadPlatformQuotas()
}

onMounted(() => {
  refreshAll()
})
</script>
