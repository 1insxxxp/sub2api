import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { adminAppearanceKey } from '@/composables/useAdminAppearance'

import BaseDialog from '../BaseDialog.vue'

enableAutoUnmount(afterEach)

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('BaseDialog', () => {
  it('inherits admin appearance reactively while respecting an explicit appearance', async () => {
    const adminAppearance = ref(false)
    const wrapper = mount(BaseDialog, {
      props: { show: true, title: 'Settings' },
      global: { provide: { [adminAppearanceKey as symbol]: adminAppearance }, stubs: { teleport: true } },
    })
    expect(wrapper.get('[role="dialog"]').classes()).not.toContain('modal-neutral')
    adminAppearance.value = true
    await nextTick()
    expect(wrapper.get('[role="dialog"]').classes()).toContain('modal-neutral')
    await wrapper.setProps({ appearance: 'default' })
    expect(wrapper.get('[role="dialog"]').classes()).not.toContain('modal-neutral')
    await wrapper.setProps({ appearance: undefined })
    adminAppearance.value = false
    await nextTick()
    expect(wrapper.get('[role="dialog"]').classes()).not.toContain('modal-neutral')
  })

  afterEach(() => {
    document.body.innerHTML = ''
    document.body.classList.remove('modal-open')
  })

  it('renders the branded admin dialog shell and footer slot', () => {
    const wrapper = mount(BaseDialog, {
      props: {
        show: true,
        title: 'Channel Settings',
      },
      slots: {
        default: '<div data-test="dialog-body-content">body</div>',
        footer: '<button data-test="dialog-footer-action">save</button>',
      },
      global: {
        stubs: {
          Icon: true,
          teleport: true,
          transition: false,
        },
      },
    })

    expect(wrapper.get('[role="dialog"]').exists()).toBe(true)
    expect(wrapper.get('.modal-overlay').classes()).toContain('brand-overlay')
    expect(wrapper.get('.modal-content').classes()).toContain('brand-floating-panel')
    expect(wrapper.get('.modal-content').classes()).toContain('admin-surface')
    expect(wrapper.get('.modal-header').classes()).toContain('brand-floating-header')
    expect(wrapper.get('.modal-footer').classes()).toContain('admin-dialog-footer')
    expect(wrapper.get('[data-test="dialog-body-content"]').text()).toBe('body')
    expect(wrapper.get('[data-test="dialog-footer-action"]').text()).toBe('save')
  })

  it('keeps scrolling locked until the final visible dialog closes', async () => {
    const history = mount(BaseDialog, { props: { show: true, title: 'History', zIndex: 60 } })
    const balance = mount(BaseDialog, { props: { show: true, title: 'Deposit', zIndex: 70 } })
    const hidden = mount(BaseDialog, { props: { show: false, title: 'Other' } })
    await nextTick()
    expect(document.body.classList.contains('modal-open')).toBe(true)
    hidden.unmount()
    expect(document.body.classList.contains('modal-open')).toBe(true)
    await balance.setProps({ show: false })
    expect(document.body.classList.contains('modal-open')).toBe(true)
    await history.setProps({ show: false })
    expect(document.body.classList.contains('modal-open')).toBe(false)
  })

  it('only closes the topmost dialog on Escape regardless of mounting order', async () => {
    const balance = mount(BaseDialog, { props: { show: false, title: 'Deposit', zIndex: 70 } })
    const history = mount(BaseDialog, { props: { show: true, title: 'History', zIndex: 60 } })
    await balance.setProps({ show: true })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(balance.emitted('close')).toHaveLength(1)
    expect(history.emitted('close')).toBeUndefined()
    await balance.setProps({ show: false })
    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(history.emitted('close')).toHaveLength(1)
  })

  it('resets body scroll position when reopened', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: false, title: 'Details' },
      slots: { default: '<div style="height: 2000px">content</div>' },
      global: { stubs: { Icon: true } }
    })

    await wrapper.setProps({ show: true })
    await nextTick()
    const body = document.body.querySelector<HTMLElement>('.modal-body')
    expect(body).not.toBeNull()
    body!.scrollTop = 480

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    expect(document.body.querySelector<HTMLElement>('.modal-body')?.scrollTop).toBe(0)
    wrapper.unmount()
  })
})
