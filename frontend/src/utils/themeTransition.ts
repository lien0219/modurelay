type ThemeViewTransition = {
  ready: Promise<void>
}

type ThemeTransitionDocument = Document & {
  startViewTransition?: (update: () => void) => ThemeViewTransition
}

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function transitionRadius(x: number, y: number): number {
  return Math.hypot(Math.max(x, window.innerWidth - x), Math.max(y, window.innerHeight - y))
}

/** Apply the existing theme state and reveal the new theme from the click point. */
export function toggleThemeWithTransition(currentDark: boolean, event?: MouseEvent): boolean {
  const nextDark = !currentDark
  const root = document.documentElement
  const applyTheme = () => {
    root.classList.toggle('dark', nextDark)
    localStorage.setItem('theme', nextDark ? 'dark' : 'light')
  }
  const transitionDocument = document as ThemeTransitionDocument
  const startViewTransition = transitionDocument.startViewTransition

  if (!startViewTransition || prefersReducedMotion()) {
    applyTheme()
    return nextDark
  }

  const x = event && Number.isFinite(event.clientX) ? event.clientX : window.innerWidth / 2
  const y = event && Number.isFinite(event.clientY) ? event.clientY : window.innerHeight / 2
  const transition = startViewTransition.call(document, applyTheme)

  transition.ready.then(() => {
    if (typeof root.animate !== 'function') return
    root.animate(
      {
        clipPath: [`circle(0px at ${x}px ${y}px)`, `circle(${transitionRadius(x, y)}px at ${x}px ${y}px)`]
      },
      {
        duration: 620,
        easing: 'cubic-bezier(0.65, 0, 0.35, 1)',
        pseudoElement: '::view-transition-new(root)'
      } as KeyframeAnimationOptions
    )
  }).catch(() => {
    // The theme class has already been applied; an unsupported animation is safe to skip.
  })

  return nextDark
}
