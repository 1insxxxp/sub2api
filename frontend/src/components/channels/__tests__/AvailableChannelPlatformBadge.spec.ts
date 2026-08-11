import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import AvailableChannelPlatformBadge from '../AvailableChannelPlatformBadge.vue'

describe('AvailableChannelPlatformBadge', () => {
  it('delegates platform colors and icons to the API-key group badge', () => {
    const wrapper = mount(AvailableChannelPlatformBadge, {
      props: { platform: 'anthropic' },
      global: {
        stubs: {
          GroupBadge: {
            props: ['name', 'platform', 'showRate'],
            template: '<span data-group-badge :data-name="name" :data-platform="platform" :data-show-rate="String(showRate)" />',
          },
        },
      },
    })

    const badge = wrapper.get('[data-testid="available-channel-platform-badge"]')
    expect(badge.attributes('data-group-badge')).toBeDefined()
    expect(badge.attributes('data-name')).toBe('anthropic')
    expect(badge.attributes('data-platform')).toBe('anthropic')
    expect(badge.attributes('data-show-rate')).toBe('false')
  })
})
