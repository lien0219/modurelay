import { beforeEach, describe, expect, it, vi } from 'vitest'

const appStore = vi.hoisted(() => ({
  cachedPublicSettings: undefined as undefined | { activity_center_enabled?: boolean },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

import { FeatureFlags, isFeatureFlagEnabled } from '../featureFlags'

describe('activity center feature flag', () => {
  beforeEach(() => {
    appStore.cachedPublicSettings = undefined
  })

  it('is opt-in and remains hidden until explicitly enabled', () => {
    expect(FeatureFlags.activityCenter.mode).toBe('opt-in')
    expect(isFeatureFlagEnabled(FeatureFlags.activityCenter)).toBe(false)
    appStore.cachedPublicSettings = { activity_center_enabled: true }
    expect(isFeatureFlagEnabled(FeatureFlags.activityCenter)).toBe(true)
    appStore.cachedPublicSettings = { activity_center_enabled: false }
    expect(isFeatureFlagEnabled(FeatureFlags.activityCenter)).toBe(false)
  })
})

