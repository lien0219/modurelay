import type { Router } from 'vue-router'
import { mountHomeThreeBackground } from '@/utils/homeThreeBackground'

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
let activationFrame = 0
let cleanupThreeBackground: (() => void) | null = null

function normalizePath(path: string): string {
  const normalized = path.replace(/\/+$/, '')
  return normalized || '/'
}

function isHomePath(path: string): boolean {
  return HOME_PATHS.has(normalizePath(path))
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

function deactivateHomeMotion(): void {
  window.cancelAnimationFrame(scanFrame)
  window.cancelAnimationFrame(activationFrame)
  cleanupThreeBackground?.()
  cleanupThreeBackground = null
}

function activateHomeMotion(): void {
  deactivateHomeMotion()

  // Wait for the routed component and its background host to be committed.
  activationFrame = window.requestAnimationFrame(() => {
    activationFrame = window.requestAnimationFrame(() => {
      scanHomeMotionElements()
      cleanupThreeBackground = mountHomeThreeBackground()
    })
  })
}

export function installHomeMotion(router: Router): void {
  if (isHomePath(router.currentRoute.value.path)) activateHomeMotion()

  router.afterEach((to) => {
    if (isHomePath(to.path)) activateHomeMotion()
    else deactivateHomeMotion()
  })
}
