import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { toggleThemeWithTransition } from '@/utils/themeTransition'

const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

function setStartViewTransition(value?: (update: () => void) => unknown) {
  Object.defineProperty(document, 'startViewTransition', {
    configurable: true,
    value
  })
}

describe('toggleThemeWithTransition', () => {
  beforeEach(() => {
    document.documentElement.className = ''
    document.documentElement.removeAttribute('style')
    localStorage.clear()
    setStartViewTransition(undefined)
  })

  afterEach(() => {
    setStartViewTransition(undefined)
  })

  it('applies and persists the theme immediately without transition state', () => {
    expect(toggleThemeWithTransition(false)).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(false)
    expect(document.documentElement.classList.contains('theme-transition-reduced')).toBe(false)
  })

  it('never creates a root View Transition snapshot', () => {
    const startViewTransition = vi.fn()
    setStartViewTransition(startViewTransition)

    toggleThemeWithTransition(false, new MouseEvent('click', { clientX: 96, clientY: 72 }))

    expect(startViewTransition).not.toHaveBeenCalled()
  })

  it('does not create overlay or directional animation state', () => {
    toggleThemeWithTransition(false, new MouseEvent('click', { clientX: 96, clientY: 72 }))

    expect(document.documentElement.classList.contains('theme-transition-has-origin')).toBe(false)
    expect(document.documentElement.classList.contains('theme-transition-to-dark')).toBe(false)
    expect(document.documentElement.classList.contains('theme-transition-to-light')).toBe(false)
    expect(document.documentElement.style.getPropertyValue('--theme-transition-x')).toBe('')
    expect(document.documentElement.style.getPropertyValue('--theme-transition-y')).toBe('')
  })

  it('does not dim the application or render a transition overlay', () => {
    expect(styleSource).not.toContain('theme-transition-running')
    expect(styleSource).not.toContain('theme-content-settle')
    expect(styleSource).not.toContain('theme-toggle-feedback')
    expect(styleSource).not.toContain('theme-transition-has-origin::after')
  })

  it('keeps rapid theme changes synchronous and consistent', () => {
    expect(toggleThemeWithTransition(false)).toBe(true)
    expect(toggleThemeWithTransition(true)).toBe(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.classList.contains('theme-transition-running')).toBe(false)
    expect(localStorage.getItem('theme')).toBe('light')
  })
})
