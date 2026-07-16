/**
 * Chart.js animation config that respects prefers-reduced-motion.
 * Prefer Chart.js native animation over GSAP for canvas charts.
 */
export function getChartJsAnimation(): false | { duration: number; easing: 'easeOutQuart' } {
  if (typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) {
    return false
  }
  return {
    duration: 520,
    easing: 'easeOutQuart'
  }
}
