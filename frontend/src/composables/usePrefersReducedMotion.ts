import { onMounted, onUnmounted, ref, type Ref } from 'vue'

/**
 * Reactive prefers-reduced-motion preference.
 */
export function usePrefersReducedMotion(): Ref<boolean> {
  const prefersReducedMotion = ref(false)
  let mediaQuery: MediaQueryList | null = null

  const update = () => {
    prefersReducedMotion.value = mediaQuery?.matches ?? false
  }

  onMounted(() => {
    if (typeof window === 'undefined' || !window.matchMedia) return
    mediaQuery = window.matchMedia('(prefers-reduced-motion: reduce)')
    update()
    mediaQuery.addEventListener('change', update)
  })

  onUnmounted(() => {
    mediaQuery?.removeEventListener('change', update)
    mediaQuery = null
  })

  return prefersReducedMotion
}
