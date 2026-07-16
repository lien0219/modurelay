import {
  nextTick,
  onActivated,
  onMounted,
  onUnmounted,
  ref,
  watch,
  type Ref
} from 'vue'
import gsap from 'gsap'

export interface UseDashboardRevealOptions {
  /** Unique key so remount / refresh does not replay full entrance */
  scopeKey: string
  /** Selector within container. Default [data-reveal] */
  selector?: string
  /** Per-item duration (seconds). Default 0.34 */
  duration?: number
  /** Stagger between items (seconds). Default 0.05 */
  stagger?: number
  /** Y offset in px. Default 8 */
  y?: number
}

/** Module-level: entrance plays at most once per scope (survives remount / refresh) */
const playedScopes = new Set<string>()

function prefersReducedMotion(): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

/**
 * First-paint staggered reveal for 4–6 dashboard modules.
 * Plays once per scopeKey; skips on keep-alive activate and data refresh remounts.
 */
export function useDashboardReveal(
  containerRef: Ref<HTMLElement | null | undefined>,
  options: UseDashboardRevealOptions
) {
  const selector = options.selector ?? '[data-reveal]'
  const duration = options.duration ?? 0.34
  const stagger = options.stagger ?? 0.05
  const y = options.y ?? 8

  const alreadyPlayed = playedScopes.has(options.scopeKey)
  /** When true, CSS keeps modules invisible until GSAP takes over (avoids flash) */
  const isPending = ref(!alreadyPlayed && !prefersReducedMotion())

  let ctx: gsap.Context | null = null
  let playedInInstance = false

  const cleanup = () => {
    ctx?.revert()
    ctx = null
  }

  const showFinal = (root: HTMLElement) => {
    const targets = root.querySelectorAll<HTMLElement>(selector)
    gsap.set(targets, { autoAlpha: 1, y: 0, clearProps: 'transform' })
    isPending.value = false
    playedScopes.add(options.scopeKey)
    playedInInstance = true
  }

  const revealNow = (root: HTMLElement) => {
    const targets = Array.from(root.querySelectorAll<HTMLElement>(selector))
    if (targets.length === 0) {
      isPending.value = false
      return
    }

    if (prefersReducedMotion() || playedScopes.has(options.scopeKey)) {
      showFinal(root)
      return
    }

    cleanup()
    // Mark before tween so remount during animation does not double-play
    playedScopes.add(options.scopeKey)
    playedInInstance = true

    ctx = gsap.context(() => {
      gsap.fromTo(
        targets,
        { autoAlpha: 0, y },
        {
          autoAlpha: 1,
          y: 0,
          duration,
          stagger,
          ease: 'power2.out',
          clearProps: 'transform',
          onStart: () => {
            isPending.value = false
          },
          onComplete: () => {
            isPending.value = false
          }
        }
      )
    }, root)
  }

  const tryReveal = async () => {
    await nextTick()
    const root = containerRef.value
    if (!root || playedInInstance) return
    revealNow(root)
  }

  onMounted(() => {
    void tryReveal()
  })

  onActivated(() => {
    const root = containerRef.value
    if (!root) return
    if (playedScopes.has(options.scopeKey) || playedInInstance) {
      showFinal(root)
      return
    }
    void tryReveal()
  })

  watch(containerRef, (el) => {
    if (el && !playedInInstance) {
      void tryReveal()
    }
  })

  onUnmounted(() => {
    cleanup()
  })

  return {
    isPending,
    hasPlayed: () => playedScopes.has(options.scopeKey),
    /** Test helper */
    __resetScopeForTests: () => {
      playedScopes.delete(options.scopeKey)
      playedInInstance = false
      isPending.value = !prefersReducedMotion()
    }
  }
}
