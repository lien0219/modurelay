import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { brand } from '@/config/brand'
import { tokenMarketRoutes } from '@/features/token-market'
import { useAppStore } from '@/stores/app'
import { updateFavicon } from '@/utils/branding'
import { isIOSDevice } from '@/utils/device'
import { installHomeRouteGuard } from '@/utils/homeRouteGuard'
import { syncThemeMode } from '@/composables/useThemeMode'
import '@fontsource-variable/noto-sans-sc/wght.css'
import 'flag-icons/css/flag-icons.min.css'
import './style.css'
import './styles/home-route-guard.css'
import './styles/home.css'

// Standalone feature routes: Token Market owns its visual shell and remains
// outside AppLayout/sidebar. The whole route family lives with the feature so
// product, merchant, wallet and commerce pages can evolve together.
for (const route of tokenMarketRoutes) router.addRoute(route)

function initIOSViewportZoomFix() {
  const viewport = document.querySelector('meta[name="viewport"]')
  if (!isIOSDevice() || !viewport) return
  const content = viewport.getAttribute('content') || ''
  if (/maximum-scale/i.test(content)) return
  viewport.setAttribute('content', `${content}, maximum-scale=1.0`)
}

function initThemeClass() {
  const savedTheme = localStorage.getItem('theme')
  const shouldUseDark = savedTheme === 'dark' || (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', shouldUseDark)
  syncThemeMode(shouldUseDark)
}

async function bootstrap() {
  initThemeClass()
  initIOSViewportZoomFix()
  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  const appStore = useAppStore()
  appStore.initFromInjectedConfig()
  if (appStore.siteName && appStore.siteName !== brand.name) document.title = `${appStore.siteName} - ${brand.fullName}`
  updateFavicon(appStore.siteLogo)

  await initI18n()
  app.use(router)
  app.use(i18n)
  await router.isReady()
  installHomeRouteGuard(router)
  app.mount('#app')
}

bootstrap()
