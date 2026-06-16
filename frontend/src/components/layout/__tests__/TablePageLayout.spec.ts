import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'

import TablePageLayout from '../TablePageLayout.vue'

describe('TablePageLayout', () => {
  it('renders admin workspace shells around actions, filters, table, and pagination slots', () => {
    const wrapper = mount(TablePageLayout, {
      slots: {
        actions: '<div data-test="actions-slot">actions</div>',
        filters: '<div data-test="filters-slot">filters</div>',
        table: '<div data-test="table-slot">table</div>',
        pagination: '<div data-test="pagination-slot">pagination</div>',
      },
    })

    const fixedSections = wrapper.findAll('.layout-section-fixed')

    expect(wrapper.get('.table-page-layout').classes()).toContain('admin-workspace')
    expect(fixedSections[0].classes()).toContain('admin-toolbar-surface')
    expect(fixedSections[1].classes()).toContain('admin-toolbar-surface')
    expect(wrapper.get('.table-scroll-container').classes()).toContain('admin-table-stage')
    expect(fixedSections[2].classes()).toContain('admin-pagination-surface')
    expect(wrapper.get('[data-test="table-slot"]').text()).toBe('table')
  })
})
