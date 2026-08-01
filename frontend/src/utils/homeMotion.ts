import type { Router } from 'vue-router'

const HOME_PATHS = new Set(['/', '/home'])
const MOTION_SELECTOR = [
  '.service-card',
  '.advantage-card',
  '.process-card',
  '.advertising-panel',
  '.contact-panel'
].join(',')

const enhancedElements = new WeakSet<HTMLElement>()
let scanFrame = 0

function normalizePath(path: string): string {
  const normalized = path.replace(/\/+$/, '')
  return normalized || '/'
}

function supportsPointerMotion(): boolean {
  return !window.matchMedia('(prefers-reduced-motion: reduce)').matches
    && window.matchMedia('(hover: hover) and (pointer: fine)').matches
}

function resetElement(element: HTMLElement): void {
  element.classList.remove('is-pointer-active')
  element.style.setProperty('--motion-rotate-x', '0deg')
  element.style.setProperty('--motion-rotate-y', '0deg')
  element.style.setProperty('--motion-shine-x', '50%')
  element.style.setProperty('--motion-shine-y', '50%')
}

function enhanceElement(element: HTMLElement): void {
  if (enhancedElements.has(element)) return
  enhancedElements.add(element)
  element.classList.add('home-motion-card')

  element.addEventListener('pointermove', (event) => {
    if (!supportsPointerMotion() || event.pointerType === 'touch') return

    const rect = element.getBoundingClientRect()
    const x = (event.clientX - rect.left) / rect.width - 0.5
    const y = (event.clientY - rect.top) / rect.height - 0.5

    element.classList.add('is-pointer-active')
    element.style.setProperty('--motion-rotate-x', `${y * -5}deg`)
    element.style.setProperty('--motion-rotate-y', `${x * 7}deg`)
    element.style.setProperty('--motion-shine-x', `${(x + 0.5) * 100}%`)
    element.style.setProperty('--motion-shine-y', `${(y + 0.5) * 100}%`)
  })

  element.addEventListener('pointerleave', () => resetElement(element))
  element.addEventListener('blur', () => resetElement(element), true)
}

function scanHomeMotionElements(): void {
  window.cancelAnimationFrame(scanFrame)
  scanFrame = window.requestAnimationFrame(() => {
    document.querySelectorAll<HTMLElement>(MOTION_SELECTOR).forEach(enhanceElement)
  })
}

export function installHomeMotion(router: Router): void {
  if (HOME_PATHS.has(normalizePath(router.currentRoute.value.path))) {
    scanHomeMotionElements()
  }

  router.afterEach((to) => {
    if (!HOME_PATHS.has(normalizePath(to.path))) return
    scanHomeMotionElements()
  })
}
