import type { Router } from 'vue-router'

const HOME_ROUTE_CLASS = 'modurelay-home-route'
const HOME_PATHS = new Set(['/', '/home'])

function normalizePath(path: string): string {
  const normalized = path.replace(/\/+$/, '')
  return normalized || '/'
}

function isHomePath(path: string): boolean {
  return HOME_PATHS.has(normalizePath(path))
}

function forceScrollTop(): void {
  if (window.location.hash) return

  window.scrollTo({ top: 0, left: 0, behavior: 'auto' })
  document.documentElement.scrollTop = 0
  document.body.scrollTop = 0
}

function resetHomeScrollPosition(): void {
  if ('scrollRestoration' in window.history) {
    window.history.scrollRestoration = 'manual'
  }

  // Native browser restoration may run after the first paint. Reset across
  // the immediate frame boundary so a hard refresh never reopens near footer.
  forceScrollTop()
  window.requestAnimationFrame(() => {
    forceScrollTop()
    window.requestAnimationFrame(forceScrollTop)
  })
  window.setTimeout(forceScrollTop, 80)
}

function syncHomeRouteState(path: string): void {
  const isHomeRoute = isHomePath(path)
  document.body.classList.toggle(HOME_ROUTE_CLASS, isHomeRoute)

  if (!isHomeRoute) return

  // Public homepage must never inherit a stale modal scroll lock.
  document.body.classList.remove('modal-open')
  document.body.style.removeProperty('overflow')
}

export function installHomeRouteGuard(router: Router): void {
  const initialPath = router.currentRoute.value.path
  syncHomeRouteState(initialPath)
  if (isHomePath(initialPath)) resetHomeScrollPosition()

  router.afterEach((to, from) => {
    window.requestAnimationFrame(() => {
      syncHomeRouteState(to.path)
      if (isHomePath(to.path) && !isHomePath(from.path)) {
        resetHomeScrollPosition()
      }
    })
  })
}
