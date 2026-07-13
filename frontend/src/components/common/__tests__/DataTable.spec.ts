import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DataTable from '../DataTable.vue'
import type { Column } from '../types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const stubDesktopMatchMedia = () => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: true,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

const stubMobileMatchMedia = () => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
}

describe('DataTable', () => {
  beforeEach(() => {
    stubDesktopMatchMedia()
    localStorage.clear()
  })

  it('renders the branded admin desktop table shell for empty backend tables', async () => {
    const columns: Column[] = [
      { key: 'name', label: 'Name' },
      { key: 'status', label: 'Status' },
    ]

    const wrapper = mount(DataTable, {
      props: {
        columns,
        data: [],
        loading: false,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.get('.table-wrapper').classes()).toContain('admin-data-table')
    expect(wrapper.get('thead').classes()).toContain('admin-data-table-head')
    expect(wrapper.get('tbody').classes()).toContain('admin-data-table-body')
    expect(wrapper.get('td').classes()).toContain('admin-empty-cell')
    expect(wrapper.find('.admin-empty-state').exists()).toBe(true)
  })

  it('renders paired sort arrows and highlights the active direction', async () => {
    const wrapper = mount(DataTable, {
      props: {
        columns: [
          { key: 'name', label: 'Name', sortable: true },
          { key: 'created_at', label: 'Created', sortable: true },
        ],
        data: [
          { id: 1, name: 'Beta', created_at: '2026-01-02T00:00:00Z' },
          { id: 2, name: 'Alpha', created_at: '2026-01-01T00:00:00Z' },
        ],
        defaultSortKey: 'name',
        defaultSortOrder: 'asc',
      },
    })

    await wrapper.vm.$nextTick()

    const nameHeader = wrapper.findAll('th')[0]
    expect(nameHeader.attributes('aria-sort')).toBe('ascending')
    expect(nameHeader.findAll('svg')).toHaveLength(2)
    expect(nameHeader.findAll('svg')[0].classes()).toContain('text-primary-600')
    expect(nameHeader.findAll('svg')[1].classes()).toContain('text-gray-300')

    await nameHeader.trigger('click')
    await wrapper.vm.$nextTick()

    expect(nameHeader.attributes('aria-sort')).toBe('descending')
    expect(nameHeader.findAll('svg')[0].classes()).toContain('text-gray-300')
    expect(nameHeader.findAll('svg')[1].classes()).toContain('text-primary-600')
  })

  it('hides mobileHidden columns from cards without removing desktop columns', () => {
    const columns: Column[] = [
      { key: 'name', label: 'Name' },
      { key: 'details', label: 'Details', mobileHidden: true },
    ]
    const data = [{ id: 1, name: 'Alice', details: 'Desktop detail' }]

    stubMobileMatchMedia()
    const mobileWrapper = mount(DataTable, { props: { columns, data } })

    expect(mobileWrapper.text()).toContain('Name')
    expect(mobileWrapper.text()).not.toContain('Details')
    expect(mobileWrapper.text()).not.toContain('Desktop detail')

    stubDesktopMatchMedia()
    const desktopWrapper = mount(DataTable, { props: { columns, data } })

    expect(desktopWrapper.findAll('th').map((header) => header.text())).toEqual(['Name', 'Details'])
  })
})
