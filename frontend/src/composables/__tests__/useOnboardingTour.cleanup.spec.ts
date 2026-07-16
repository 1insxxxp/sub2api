import { describe, expect, it } from 'vitest'

import { cleanupDriverDomArtifacts, shouldAutoStartOnboardingTour } from '../useOnboardingTour'

describe('cleanupDriverDomArtifacts', () => {
  it('removes orphaned driver overlay nodes and global driver classes', () => {
    document.body.innerHTML = `
      <div class="driver-overlay"></div>
      <div class="driver-popover"></div>
      <div class="driver-popover-arrow"></div>
      <main class="driver-active-element driver-no-interaction"></main>
    `
    document.body.classList.add('driver-active', 'driver-fade', 'driver-simple')
    document.documentElement.classList.add('driver-active', 'driver-fade', 'driver-simple')

    cleanupDriverDomArtifacts()

    expect(document.querySelector('.driver-overlay')).toBeNull()
    expect(document.querySelector('.driver-popover')).toBeNull()
    expect(document.querySelector('.driver-popover-arrow')).toBeNull()
    expect(document.body.classList.contains('driver-active')).toBe(false)
    expect(document.documentElement.classList.contains('driver-active')).toBe(false)
    expect(document.querySelector('.driver-active-element')).toBeNull()
    expect(document.querySelector('.driver-no-interaction')).toBeNull()
  })
})

describe('shouldAutoStartOnboardingTour', () => {
  it('only auto-starts on the matching dashboard route', () => {
    expect(shouldAutoStartOnboardingTour('/admin/dashboard', true)).toBe(true)
    expect(shouldAutoStartOnboardingTour('/affiliate', true)).toBe(false)
    expect(shouldAutoStartOnboardingTour('/admin/promo-codes', true)).toBe(false)

    expect(shouldAutoStartOnboardingTour('/dashboard', false)).toBe(true)
    expect(shouldAutoStartOnboardingTour('/affiliate', false)).toBe(false)
  })
})
