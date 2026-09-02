const THEME_TRANSITION_DURATION_MS = 260
const REDUCED_THEME_TRANSITION_DURATION_MS = 140
const THEME_TRANSITION_STATE_CLASSES = [
  'theme-transition-running',
  'theme-transition-reduced',
  'theme-transition-has-origin',
  'theme-transition-to-dark',
  'theme-transition-to-light'
] as const
let cleanupTimer: number | null = null

function prefersReducedMotion(): boolean {
  return (
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-reduced-motion: reduce)').matches
  )
}

function clearThemeTransition(root: HTMLElement): void {
  if (cleanupTimer !== null) window.clearTimeout(cleanupTimer)
  cleanupTimer = null
  root.classList.remove(...THEME_TRANSITION_STATE_CLASSES)
  root.style.removeProperty('--theme-transition-x')
  root.style.removeProperty('--theme-transition-y')
}

function setThemeTransitionOrigin(root: HTMLElement, event?: MouseEvent): void {
  if (!event) return

  const target = event.currentTarget
  const rect = target instanceof Element ? target.getBoundingClientRect() : null
  const x = rect && rect.width > 0 ? rect.left + rect.width / 2 : event.clientX
  const y = rect && rect.height > 0 ? rect.top + rect.height / 2 : event.clientY
  if (!Number.isFinite(x) || !Number.isFinite(y)) return

  root.style.setProperty('--theme-transition-x', `${x}px`)
  root.style.setProperty('--theme-transition-y', `${y}px`)
  root.classList.add('theme-transition-has-origin')
}

/** Apply the existing theme state with interruptible color interpolation and no page snapshots. */
export function toggleThemeWithTransition(currentDark: boolean, event?: MouseEvent): boolean {
  const nextDark = !currentDark
  const root = document.documentElement
  const applyTheme = () => {
    root.classList.toggle('dark', nextDark)
    localStorage.setItem('theme', nextDark ? 'dark' : 'light')
  }

  if (cleanupTimer !== null) window.clearTimeout(cleanupTimer)
  root.classList.remove(
    'theme-transition-reduced',
    'theme-transition-to-dark',
    'theme-transition-to-light',
    'theme-transition-has-origin',
  )

  const reducedMotion = prefersReducedMotion()
  if (reducedMotion) {
    root.classList.add('theme-transition-reduced')
  } else {
    setThemeTransitionOrigin(root, event)
    root.classList.add(nextDark ? 'theme-transition-to-dark' : 'theme-transition-to-light')
  }

  root.classList.add('theme-transition-running')
  document.body.getBoundingClientRect()
  applyTheme()
  cleanupTimer = window.setTimeout(
    () => clearThemeTransition(root),
    reducedMotion ? REDUCED_THEME_TRANSITION_DURATION_MS : THEME_TRANSITION_DURATION_MS,
  )
  return nextDark
}
