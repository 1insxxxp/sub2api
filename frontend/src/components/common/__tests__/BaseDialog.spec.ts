import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import BaseDialog from '../BaseDialog.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('BaseDialog', () => {
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
