import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import LocaleSwitcher from '../LocaleSwitcher.vue'

const localeState = vi.hoisted(() => ({ value: 'zh' }))
const setLocaleMock = vi.hoisted(() => vi.fn(async (code: string) => {
  localeState.value = code
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: localeState,
      t: (key: string) => (key === 'admin.settings.emailTemplates.locale' ? '语言' : key),
    }),
  }
})

vi.mock('@/i18n', () => ({
  availableLocales: [
    { code: 'en', name: 'English', flag: '🇺🇸' },
    { code: 'zh', name: '中文', flag: '🇨🇳' },
  ],
  setLocale: setLocaleMock,
}))

function mountSwitcher() {
  return mount(LocaleSwitcher, {
    global: {
      stubs: {
        Icon: {
          props: ['name'],
          template: '<span class="icon-stub" :data-icon="name" />',
        },
      },
    },
  })
}

describe('LocaleSwitcher', () => {
  it('renders a simplified compact language menu', async () => {
    const wrapper = mountSwitcher()

    await wrapper.get('button[aria-haspopup="menu"]').trigger('click')

    const menu = wrapper.get('[role="menu"]')
    expect(menu.classes()).toContain('locale-menu-panel')
    expect(menu.classes()).toContain('w-48')
    expect(menu.classes()).not.toContain('brand-floating-panel')
    expect(menu.classes()).not.toContain('w-[14.5rem]')

    const header = wrapper.get('.locale-menu-header')
    expect(header.classes()).toContain('py-2')
    expect(header.text()).toContain('语言')
    expect(header.text()).toContain('中文')

    const items = wrapper.findAll('.locale-menu-item')
    expect(items).toHaveLength(2)
    for (const item of items) {
      expect(item.classes()).toContain('min-h-10')
      expect(item.classes()).toContain('rounded-lg')
      expect(item.classes()).not.toContain('rounded-2xl')
    }

    const activeItem = items.find((item) => item.attributes('aria-checked') === 'true')
    expect(activeItem?.classes().join(' ')).toContain('bg-blue-50')
    expect(activeItem?.classes().join(' ')).not.toContain('linear-gradient')
  })
})
