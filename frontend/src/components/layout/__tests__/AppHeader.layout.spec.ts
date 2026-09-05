import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const directory = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(directory, '../AppHeader.vue'), 'utf8')
const layoutFixesSource = readFileSync(resolve(directory, '../../../styles/layout-fixes.css'), 'utf8')

describe('AppHeader account dropdown layout', () => {
  it('wraps long email addresses within the actual app header dropdown', () => {
    expect(headerSource).toContain('class="app-menu-meta user-menu-email text-xs"')
    expect(layoutFixesSource).toContain('.app-header .dropdown.glass-popover.w-56')
    expect(layoutFixesSource).toContain('.user-menu-email')
    expect(layoutFixesSource).toContain('min-width: 0;')
    expect(layoutFixesSource).toContain('overflow-wrap: anywhere;')
    expect(layoutFixesSource).not.toContain('header.glass')
  })
})
