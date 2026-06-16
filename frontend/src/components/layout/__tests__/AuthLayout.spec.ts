import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AuthLayout from '@/components/layout/AuthLayout.vue'

const appStore = {
  siteName: 'PassionAPI',
  siteLogo: '',
  cachedPublicSettings: {
    site_subtitle: 'Unified AI API gateway',
  },
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn(),
}

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
}))

describe('AuthLayout', () => {
  it('renders the blue-cyan branded auth shell with slots', () => {
    const wrapper = mount(AuthLayout, {
      slots: {
        default: '<form data-test="auth-form">Form content</form>',
        footer: '<a href="/login">Footer link</a>',
      },
    })

    expect(wrapper.find('.auth-shell').exists()).toBe(true)
    expect(wrapper.find('.auth-grid-bg').exists()).toBe(true)
    expect(wrapper.find('.auth-logo-frame').exists()).toBe(true)
    expect(wrapper.find('.auth-card').exists()).toBe(true)
    expect(wrapper.find('[data-test="auth-form"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('Footer link')
    expect(wrapper.text()).toContain('PassionAPI')
  })
})
