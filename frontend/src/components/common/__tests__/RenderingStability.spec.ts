import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const sourceRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../../..')
const readSource = (path: string) => readFileSync(resolve(sourceRoot, path), 'utf8')

const styleSource = readSource('style.css')

describe('shared rendering stability', () => {
  it('keeps large scrollable dialog surfaces opaque and bounded', () => {
    const appHeaderBlock = styleSource.match(/\.app-header\s*\{[\s\S]*?\n {2}\}/)
    const modalBlock = styleSource.match(/\.modal-content\s*\{[\s\S]*?\n {2}\}/)
    const bodyBlock = styleSource.match(/\.modal-body\s*\{[\s\S]*?\n {2}\}/)
    const scrimBlock = styleSource.match(/\.viewport-scrim\s*\{[\s\S]*?\n {2}\}/)
    const popoverBlock = styleSource.match(/\.scrollable-popover\s*\{[\s\S]*?\n {2}\}/)

    expect(appHeaderBlock?.[0]).toContain('background-color: var(--color-surface)')
    expect(appHeaderBlock?.[0]).toContain('backdrop-filter: none')
    expect(modalBlock?.[0]).toContain('background-color: var(--color-surface-raised)')
    expect(modalBlock?.[0]).toContain('backdrop-filter: none')
    expect(bodyBlock?.[0]).toContain('overscroll-behavior: contain')
    expect(bodyBlock?.[0]).toContain('scrollbar-gutter: stable')
    expect(scrimBlock?.[0]).toContain('backdrop-filter: none')
    expect(popoverBlock?.[0]).toContain('backdrop-filter: none')
  })

  it.each([
    ['announcement popup', 'components/common/AnnouncementPopup.vue'],
    ['announcement list', 'components/common/AnnouncementBell.vue'],
    ['login agreement', 'components/auth/LoginAgreementPrompt.vue'],
    ['payment overlays', 'views/user/PaymentView.vue']
  ])('uses an unfiltered viewport scrim for %s', (_name, path) => {
    const source = readSource(path)

    expect(source).toContain('viewport-scrim')
    expect(source).not.toMatch(/fixed inset-0[^"\n]*backdrop-blur/)
  })

  it.each([
    ['select options', 'components/common/Select.vue'],
    ['resource notifications', 'components/common/ResourceNotificationBell.vue'],
    ['user column selector', 'views/admin/UsersView.vue']
  ])('uses an opaque surface for the scrollable %s popover', (_name, path) => {
    expect(readSource(path)).toContain('scrollable-popover')
  })

  it('keeps transform-driven and sticky content surfaces free of live blur', () => {
    const stableSources = [
      readSource('views/AILearningView.vue'),
      readSource('views/KeyUsageView.vue'),
      readSource('components/user/monitor/MonitorCard.vue'),
      readSource('components/user/dashboard/UserDashboardCharts.vue'),
      readSource('views/user/ChannelStatusV2View.vue'),
      readSource('features/prompt-audit/PromptAuditView.vue'),
      readSource('views/admin/SettingsView.vue')
    ]

    for (const source of stableSources) {
      expect(source).not.toContain('backdrop-blur')
      expect(source).not.toContain('backdrop-filter')
    }
  })
})
