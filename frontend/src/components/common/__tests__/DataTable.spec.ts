import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

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

const columns: Column[] = [
  { key: 'name', label: 'Name' },
  { key: 'status', label: 'Status' },
]

describe('DataTable', () => {
  it('renders the branded admin desktop table shell for empty backend tables', async () => {
    vi.stubGlobal('matchMedia', vi.fn().mockImplementation((query: string) => ({
      matches: query === '(min-width: 768px)',
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })))

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
  })
})
