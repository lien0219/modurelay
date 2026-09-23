import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const readView = (name: string) =>
  readFileSync(resolve(process.cwd(), `src/views/admin/${name}.vue`), 'utf8')

describe('table menu overflow protection', () => {
  it('renders user table menus as viewport-level overlays', () => {
    const source = readView('UsersView')

    expect(source).toContain('<Teleport to="body">')
    expect(source).toContain('class="user-group-menu glass-popover scrollable-popover fixed')
    expect(source).toContain('class="usage-sort-menu glass-popover scrollable-popover fixed')
    expect(source).toContain(':style="floatingPanelStyle(expandedGroupMenuPosition)"')
    expect(source).toContain(':style="floatingPanelStyle(usageSortMenuPosition)"')
    expect(source).not.toContain('class="absolute right-0 top-full z-50 mt-1 min-w-[120px]')
  })

  it('renders the proxy copy menu outside the table scroll container', () => {
    const source = readView('ProxiesView')

    expect(source).toContain('<Teleport to="body">')
    expect(source).toContain('class="proxy-copy-menu glass-popover scrollable-popover fixed')
    expect(source).toContain(':style="floatingPanelStyle(copyMenuPosition)"')
    expect(source).toContain('@keydown.shift.f10.prevent.stop="toggleCopyMenu(row.id, $event)"')
    expect(source).not.toContain('class="absolute left-0 top-full z-50 mt-1 w-auto min-w-[180px]')
  })
})
