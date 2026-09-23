import { beforeEach, describe, expect, it, vi } from 'vitest'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: undefined as undefined | { tool_center_enabled?: boolean },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

import { FeatureFlags, isFeatureFlagEnabled } from '../featureFlags'

describe('tool center feature flag', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = undefined
  })

  it('defaults to enabled until the backend explicitly disables it', () => {
    expect(FeatureFlags.toolCenter.mode).toBe('opt-out')
    expect(isFeatureFlagEnabled(FeatureFlags.toolCenter)).toBe(true)

    appStore.cachedPublicSettings = { tool_center_enabled: true }
    expect(isFeatureFlagEnabled(FeatureFlags.toolCenter)).toBe(true)

    appStore.cachedPublicSettings = { tool_center_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.toolCenter)).toBe(false)
  })
})
