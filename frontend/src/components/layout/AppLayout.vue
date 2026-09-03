<template>
  <div class="app-shell min-h-screen">

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="app-main-column relative min-h-screen transition-[margin] duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader />

      <!-- Main Content -->
      <main class="app-main p-4 md:p-6 lg:p-8">
        <div class="app-main-content">
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import '@/styles/layout-fixes.css'
import { computed, onMounted, watch } from 'vue'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import { extractSemanticVersion } from '@/utils/version'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'

const props = withDefaults(defineProps<{
  enableOnboarding?: boolean
}>(), {
  enableOnboarding: true
})

const appStore = useAppStore()
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

watch(
  () => appStore.siteVersion,
  (version) => {
    const normalizedVersion = extractSemanticVersion(version)
    if (normalizedVersion && normalizedVersion !== version) {
      appStore.siteVersion = normalizedVersion
    }
  },
  { immediate: true }
)

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: props.enableOnboarding,
  enabled: props.enableOnboarding
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(props.enableOnboarding ? replayTour : null)
})

defineExpose({ replayTour })
</script>
