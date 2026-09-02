<script setup lang="ts">
import { RouterView, useRouter, useRoute } from 'vue-router'
import { onMounted, onBeforeUnmount, watch } from 'vue'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
import AdminComplianceDialog from '@/components/admin/AdminComplianceDialog.vue'
import { resolveRouteDocumentTitle } from '@/router/title'
import { brand } from '@/config/brand'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'
import { useAppStore, useAuthStore, useSubscriptionStore, useAnnouncementStore, useAdminComplianceStore, useAdminSettingsStore, useOnboardingStore } from '@/stores'
import { getSetupStatus } from '@/api/setup'
import { updateFavicon } from '@/utils/branding'
import { disposeOnboardingTour, removeOnboardingArtifacts } from '@/utils/onboardingCleanup'

const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()
const announcementStore = useAnnouncementStore()
const adminComplianceStore = useAdminComplianceStore()
const adminSettingsStore = useAdminSettingsStore()
const onboardingStore = useOnboardingStore()
const HOME_ROUTE_PATH = '/home'
const ONBOARDING_DISABLED_PATHS = new Set([HOME_ROUTE_PATH, '/ai-learning', '/activities'])
let homeOverlayObserver: MutationObserver | null = null
let homeOverlayCleanupFrame = 0

function isHomeRoute(path = router.currentRoute.value.path) {
  return path === HOME_ROUTE_PATH
}

function disposePublicOnboardingOverlay() {
  disposeOnboardingTour(onboardingStore)
}

function cleanupHomeOverlayState() {
  if (!isHomeRoute()) return

  // The public home has no modal state. Clear the body lock left behind by a
  // tour or dialog before it can affect the next render.
  document.body.classList.remove('modal-open')
  document.body.style.removeProperty('overflow')
  removeOnboardingArtifacts()
}

function scheduleHomeOverlayCleanup() {
  if (!isHomeRoute()) return
  if (homeOverlayCleanupFrame) window.cancelAnimationFrame(homeOverlayCleanupFrame)

  homeOverlayCleanupFrame = window.requestAnimationFrame(() => {
    homeOverlayCleanupFrame = window.requestAnimationFrame(() => {
      homeOverlayCleanupFrame = 0
      cleanupHomeOverlayState()
    })
  })
}

function stopHomeOverlayGuard() {
  homeOverlayObserver?.disconnect()
  homeOverlayObserver = null
}

function startHomeOverlayGuard() {
  stopHomeOverlayGuard()
  if (!isHomeRoute() || !document.body) return

  homeOverlayObserver = new MutationObserver(() => {
    cleanupHomeOverlayState()
  })
  homeOverlayObserver.observe(document.body, { childList: true, subtree: true })
  cleanupHomeOverlayState()
}

function syncHomeRouteGuard(path: string) {
  const isHome = path === HOME_ROUTE_PATH
  document.body.classList.toggle('modurelay-home-route', isHome)

  if (isHome) {
    startHomeOverlayGuard()
    scheduleHomeOverlayCleanup()
  } else {
    stopHomeOverlayGuard()
  }
}

function updateDocumentTitle() {
  const customMenuItems = [
    ...(appStore.cachedPublicSettings?.custom_menu_items ?? []),
    ...(authStore.isAdmin ? adminSettingsStore.customMenuItems : []),
  ]
  document.title = resolveRouteDocumentTitle(route, appStore.siteName, customMenuItems)
}

// Watch for site settings changes and update favicon/title
watch(
  () => appStore.siteLogo || brand.favicon,
  (newLogo) => {
    if (newLogo) {
      updateFavicon(newLogo)
    }
  },
  { immediate: true }
)

watch(
  () => route.path,
  (path) => syncHomeRouteGuard(path),
  { immediate: true }
)

watch(
  [
    () => route.fullPath,
    () => route.meta.title,
    () => route.meta.titleKey,
    () => appStore.siteName,
    () => appStore.cachedPublicSettings?.custom_menu_items,
    () => authStore.isAdmin,
    () => adminSettingsStore.customMenuItems,
  ],
  updateDocumentTitle,
  { deep: true }
)

// Watch for authentication state and manage subscription data + announcements
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
}

function onAdminComplianceRequired(event: Event) {
  const detail = (event as CustomEvent<Record<string, string>>).detail || {}
  adminComplianceStore.requireAcknowledgement(detail)
}

watch(
  () => authStore.isAuthenticated,
  (isAuthenticated, oldValue) => {
    if (isAuthenticated) {
      if (authStore.isAdmin) {
        adminComplianceStore.fetchStatus().catch((error) => {
          console.error('Failed to fetch admin compliance status:', error)
        })
      }

      // User logged in: preload subscriptions and start polling
      subscriptionStore.fetchActiveSubscriptions().catch((error) => {
        console.error('Failed to preload subscriptions:', error)
      })
      subscriptionStore.startPolling()

      // Announcements: new login vs page refresh restore
      if (oldValue === false) {
        // New login: delay 3s then force fetch
        setTimeout(() => announcementStore.fetchAnnouncements(true), 3000)
      } else {
        // Page refresh restore (oldValue was undefined)
        announcementStore.fetchAnnouncements()
      }

      // Register visibility change listener
      document.addEventListener('visibilitychange', onVisibilityChange)
    } else {
      // User logged out: clear data and stop polling
      subscriptionStore.clear()
      announcementStore.reset()
      adminComplianceStore.reset()
      document.removeEventListener('visibilitychange', onVisibilityChange)
      document.body.classList.remove('modal-open')
      document.body.style.removeProperty('overflow')
    }
  },
  { immediate: true }
)

// Route change trigger (throttled by store)
const removeOnboardingBeforeEach = router.beforeEach((to) => {
  if (ONBOARDING_DISABLED_PATHS.has(to.path)) {
    disposePublicOnboardingOverlay()
  }

  if (to.path === HOME_ROUTE_PATH) {
    syncHomeRouteGuard(to.path)
    scheduleHomeOverlayCleanup()
  }
})

router.afterEach((to) => {
  if (to.meta.requiresAuth === false || ONBOARDING_DISABLED_PATHS.has(to.path)) {
    disposePublicOnboardingOverlay()
  }

  syncHomeRouteGuard(to.path)
  if (to.path === HOME_ROUTE_PATH) scheduleHomeOverlayCleanup()

  if (authStore.isAuthenticated) {
    announcementStore.fetchAnnouncements()
  }
})

onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  window.removeEventListener('admin-compliance-required', onAdminComplianceRequired)
  removeOnboardingBeforeEach()
  stopHomeOverlayGuard()
  if (homeOverlayCleanupFrame) window.cancelAnimationFrame(homeOverlayCleanupFrame)
  document.body.classList.remove('modurelay-home-route')
})

onMounted(async () => {
  window.addEventListener('admin-compliance-required', onAdminComplianceRequired)
  syncHomeRouteGuard(route.path)

  // Check if setup is needed
  try {
    const status = await getSetupStatus()
    if (status.needs_setup && route.path !== '/setup') {
      router.replace('/setup')
      return
    }
  } catch {
    // If setup endpoint fails, assume normal mode and continue
  }

  // Load public settings into appStore (will be cached for other components)
  await appStore.fetchPublicSettings()

  // Re-resolve document title now that site settings are available
  updateDocumentTitle()
})
</script>

<template>
  <NavigationProgress />
  <RouterView />
  <Toast />
  <AnnouncementPopup v-if="authStore.isAuthenticated" />
  <AdminComplianceDialog v-if="authStore.isAuthenticated && authStore.isAdmin" />
</template>
