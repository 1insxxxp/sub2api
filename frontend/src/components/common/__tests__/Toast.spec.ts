import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useAppStore } from '@/stores/app'
import Toast from '../Toast.vue'

function mountToast() {
  return mount(Toast, {
    global: {
      stubs: {
        Icon: {
          props: ['name'],
          template: '<span class="icon-stub" :data-icon="name" />',
        },
        teleport: true,
        transitiongroup: false,
      },
    },
  })
}

describe('Toast', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('renders lightweight compact notification chrome', async () => {
    const store = useAppStore()
    store.showSuccess('签到规则已保存', 3000)

    const wrapper = mountToast()

    const toast = wrapper.get('.toast-panel')
    expect(toast.classes()).toContain('min-w-[280px]')
    expect(toast.classes()).toContain('rounded-xl')
    expect(toast.classes()).toContain('admin-toast-panel')
    expect(toast.classes()).not.toContain('brand-floating-panel')
    expect(toast.classes()).not.toContain('rounded-[28px]')

    const body = wrapper.get('.toast-body')
    expect(body.classes()).toContain('px-3')
    expect(body.classes()).toContain('py-2.5')

    const icon = wrapper.get('.toast-icon-shell')
    expect(icon.classes()).toContain('h-7')
    expect(icon.classes()).toContain('w-7')

    const close = wrapper.get('[aria-label="Close notification"]')
    expect(close.classes()).toContain('toast-close')
    expect(close.classes()).toContain('h-7')
    expect(close.classes()).not.toContain('brand-floating-close')

    const progress = wrapper.get('.toast-progress-track')
    expect(progress.classes()).toContain('h-0.5')
  })
})
