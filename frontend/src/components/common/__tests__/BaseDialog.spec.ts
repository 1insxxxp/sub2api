import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import BaseDialog from '../BaseDialog.vue'

describe('BaseDialog', () => {
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
    expect(wrapper.get('.modal-content').classes()).toContain('brand-floating-panel')
    expect(wrapper.get('.modal-header').classes()).toContain('brand-floating-header')
    expect(wrapper.get('.modal-footer').classes()).toContain('admin-dialog-footer')
    expect(wrapper.get('[data-test="dialog-body-content"]').text()).toBe('body')
    expect(wrapper.get('[data-test="dialog-footer-action"]').text()).toBe('save')
  })
})
