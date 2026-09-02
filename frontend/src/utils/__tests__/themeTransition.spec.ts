import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { toggleThemeWithTransition } from '@/utils/themeTransition'

function setReducedMotion(matches: boolean) {
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches }))
}

function setStartViewTransition(value?: (update: () => void) => unknown) {
  Object.defineProperty(document, 'startViewTransition', {
    configurable: true,
    value
  })
}

describe('toggleThemeWithTransition', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    document.documentElement.className = ''
    document.documentElement.removeAttribute('style')
    localStorage.clear()
    setReducedMotion(false)
    setStartViewTransition(undefined)
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
    setStartViewTransition(undefined)
  })

  it('applies the theme and clears the color-transition state after the motion window', () => {
    expect(toggleThemeWithTransition(false)).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(true)

    vi.advanceTimersByTime(260)

    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(false)
  })

  it('uses a short color fade without the ring when reduced motion is requested', () => {
    setReducedMotion(true)

    toggleThemeWithTransition(false, new MouseEvent('click', { clientX: 24, clientY: 32 }))

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(true)
    expect(document.documentElement.classList.contains('theme-transition-reduced')).toBe(true)
    expect(document.documentElement.classList.contains('theme-transition-has-origin')).toBe(false)

    vi.advanceTimersByTime(140)

    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(false)
  })

  it('never creates a root View Transition snapshot', () => {
    const startViewTransition = vi.fn()
    setStartViewTransition(startViewTransition)

    toggleThemeWithTransition(false, new MouseEvent('click', { clientX: 96, clientY: 72 }))

    expect(startViewTransition).not.toHaveBeenCalled()
  })

  it('anchors a local feedback ring at the interaction point', () => {
    toggleThemeWithTransition(false, new MouseEvent('click', { clientX: 96, clientY: 72 }))

    expect(document.documentElement.classList.contains('theme-transition-has-origin')).toBe(true)
    expect(document.documentElement.classList.contains('theme-transition-to-dark')).toBe(true)
    expect(document.documentElement.style.getPropertyValue('--theme-transition-x')).toBe('96px')
    expect(document.documentElement.style.getPropertyValue('--theme-transition-y')).toBe('72px')

    vi.advanceTimersByTime(260)

    expect(document.documentElement.classList.contains('theme-transition-has-origin')).toBe(false)
    expect(document.documentElement.style.getPropertyValue('--theme-transition-x')).toBe('')
  })

  it('restarts cleanup when a rapid toggle reverses the active color transition', () => {
    expect(toggleThemeWithTransition(false)).toBe(true)
    vi.advanceTimersByTime(130)
    expect(toggleThemeWithTransition(true)).toBe(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(true)
    expect(document.documentElement.classList.contains('theme-transition-to-light')).toBe(true)

    vi.advanceTimersByTime(259)

    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(true)

    vi.advanceTimersByTime(1)

    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })
})
