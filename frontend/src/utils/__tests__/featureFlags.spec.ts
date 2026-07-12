import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useAppStore } from '@/stores/app'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

describe('image studio feature flag', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('is disabled until public settings explicitly enable it', () => {
    expect(isFeatureFlagEnabled(FeatureFlags.imageStudio)).toBe(false)
  })

  it('is enabled when public settings enable image studio', () => {
    const appStore = useAppStore()
    appStore.cachedPublicSettings = { image_studio_enabled: true } as any

    expect(isFeatureFlagEnabled(FeatureFlags.imageStudio)).toBe(true)
  })
})
