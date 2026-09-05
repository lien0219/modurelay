import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(componentSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(componentSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar activity center navigation', () => {
  it('uses the shared opt-in feature flag for the user route', () => {
    expect(componentSource).toContain('const flagActivityCenter = makeSidebarFlag(FeatureFlags.activityCenter)')
    expect(componentSource).toContain("{ path: '/activities', label: t('nav.activityCenter'), icon: CalendarIcon, featureFlag: flagActivityCenter }")
  })

  it('uses a distinct activity icon while keeping the gift icon for redeem', () => {
    expect(componentSource).toContain("{ path: '/redeem', label: t('nav.redeem'), icon: GiftIcon, hideInSimpleMode: true }")
    expect(componentSource).toContain("{ path: '/admin/activities', label: t('nav.activityManagement'), icon: CalendarIcon }")
  })
})

describe('AppSidebar distribution placeholders', () => {
  it('adds distinct user and administrator distribution entries', () => {
    expect(componentSource).toContain("{ path: '/distribution', label: t('nav.distribution'), icon: DistributionIcon }")
    expect(componentSource).toContain("{ path: '/admin/distribution', label: t('nav.distributionManagement'), icon: DistributionIcon }")
  })
})

describe('AppSidebar footer controls', () => {
  it('uses the same compact button treatment for theme and sidebar actions', () => {
    expect(componentSource.match(/class="sidebar-footer-action sidebar-link/g)).toHaveLength(2)
    expect(componentSource).toContain('.sidebar-footer-action {')
    expect(componentSource).toContain('min-height: 44px;')
    expect(componentSource).not.toContain('theme-switch-track')
  })

  it('keeps collapsed icon actions accessible by name', () => {
    expect(componentSource).toContain(`:aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"`)
    expect(componentSource).toContain(`:aria-label="sidebarCollapsed ? t('nav.expand') : t('nav.collapse')"`)
  })
})
