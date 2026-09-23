import { syncThemeMode } from '@/composables/useThemeMode'

export interface ThemeTransitionRequest {
  nextDark: boolean
  apply: () => void
}

type ThemeTransitionRunner = (request: ThemeTransitionRequest) => Promise<void>

let activeRunner: ThemeTransitionRunner | null = null
let transitionInFlight = false

function applyTheme(nextDark: boolean) {
  const root = document.documentElement
  root.classList.toggle('dark', nextDark)
  syncThemeMode(nextDark)
  localStorage.setItem('theme', nextDark ? 'dark' : 'light')
}

export function registerThemeTransitionRunner(runner: ThemeTransitionRunner) {
  activeRunner = runner
  return () => {
    if (activeRunner === runner) activeRunner = null
  }
}

/**
 * Toggle the theme and, when the shared workspace transition is mounted, let
 * it close the glass doors before applying the new theme and opening them.
 * The returned value stays synchronous so existing controls keep their
 * current state semantics.
 */
export function toggleThemeWithTransition(currentDark: boolean, _event?: MouseEvent): boolean {
  if (transitionInFlight) return currentDark

  const nextDark = !currentDark
  if (!activeRunner) {
    applyTheme(nextDark)
    return nextDark
  }

  transitionInFlight = true
  let applied = false
  const request: ThemeTransitionRequest = {
    nextDark,
    apply: () => {
      if (applied) return
      applied = true
      applyTheme(nextDark)
    },
  }

  void activeRunner(request)
    .catch(() => {
      // Never leave the control in a stale state if an animation fails.
      request.apply()
    })
    .finally(() => {
      transitionInFlight = false
    })

  return nextDark
}
