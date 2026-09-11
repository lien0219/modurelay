import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../TablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('TablePageLayout responsive table scrolling', () => {
  it('does not disable the table horizontal scroll container in mobile mode', () => {
    const tableWrapperBlocks = Array.from(
      componentSource.matchAll(/([^{}]*:deep\(\.table-wrapper\)[^{}]*)\{([^{}]*)\}/g)
    )

    expect(tableWrapperBlocks.length).toBeGreaterThan(0)

    const baseBlock = tableWrapperBlocks.find(([selector]) => !selector.includes('.mobile-mode'))
    const mobileBlocks = tableWrapperBlocks.filter(([selector]) => selector.includes('.mobile-mode'))

    expect(baseBlock?.[2]).toContain('overflow-x-auto')
    expect(mobileBlocks.every(([, , declarations]) => !declarations.includes('overflow-visible'))).toBe(
      true
    )
  })

  it('keeps the scrolling table and its header on stable rendering layers', () => {
    const wrapperBlock = componentSource.match(
      /\.table-scroll-container :deep\(\.table-wrapper\)\s*\{([^}]*)\}/
    )
    const headerBlock = componentSource.match(
      /\.table-scroll-container :deep\(thead\)\s*\{([^}]*)\}/
    )

    expect(wrapperBlock?.[1]).toContain('overscroll-behavior: contain')
    expect(wrapperBlock?.[1]).toContain('scrollbar-gutter: stable')
    expect(headerBlock?.[1]).toContain('background-color: var(--color-surface-raised)')
    expect(headerBlock?.[1]).not.toContain('backdrop-filter')
  })
})
