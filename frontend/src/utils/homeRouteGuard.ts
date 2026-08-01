import type { Router } from 'vue-router'

const HOME_ROUTE_CLASS = 'modurelay-home-route'
const HOME_PATHS = new Set(['/', '/home'])

function normalizePath(path: string): string {
  const normalized = path.replace(/\/+$/, '')
  return normalized || '/'
}

function syncHomeRouteState(path: string): void {
  const isHomeRoute = HOME_PATHS.has(normalizePath(path))
  document.body.classList.toggle(HOME_ROUTE_CLASS, isHomeRoute)

  if (!isHomeRoute) return

  // Public homepage must never inherit a stale modal scroll lock.
  document.body.classList.remove('modal-open')
  document.body.style.removeProperty('overflow')
}

export function installHomeRouteGuard(router: Router): void {
  syncHomeRouteState(router.currentRoute.value.path)

  router.afterEach((to) => {
    window.requestAnimationFrame(() => {
      syncHomeRouteState(to.path)
    })
  })
}
