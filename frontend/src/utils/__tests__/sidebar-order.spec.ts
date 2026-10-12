import { describe, expect, it } from 'vitest'

import {
  DEFAULT_USER_SIDEBAR_ORDER,
  orderSidebarItems,
  resolveUserSidebarOrder,
} from '@/utils/sidebar-order'

describe('orderSidebarItems', () => {
  const items = [
    { path: '/dashboard', label: 'Dashboard' },
    { path: '/keys', label: 'API keys' },
    { path: '/model-plaza', label: 'Model Plaza' },
    { path: '/usage', label: 'Usage' },
  ]

  it('puts configured entries first and appends omitted entries in default order', () => {
    expect(orderSidebarItems(items, ['/model-plaza', '/keys'])).toEqual([
      items[2],
      items[1],
      items[0],
      items[3],
    ])
  })

  it('ignores unknown and duplicate identifiers without duplicating items', () => {
    expect(orderSidebarItems(items, ['/missing', '/keys', '/keys', '/dashboard'])).toEqual([
      items[1],
      items[0],
      items[2],
      items[3],
    ])
  })

  it('returns the original order when no configuration is present', () => {
    expect(orderSidebarItems(items, [])).toEqual(items)
    expect(orderSidebarItems(items, null)).toEqual(items)
  })
})

describe('resolveUserSidebarOrder', () => {
  it('keeps Model Plaza as the second default entry', () => {
    expect(DEFAULT_USER_SIDEBAR_ORDER.slice(0, 2)).toEqual([
      '/dashboard',
      '/model-plaza',
    ])
  })

  it('keeps the configured order and appends current custom entries', () => {
    expect(resolveUserSidebarOrder(['/keys'], ['/custom/help'])).toEqual([
      '/keys',
      ...DEFAULT_USER_SIDEBAR_ORDER.filter((path) => path !== '/keys'),
      '/custom/help',
    ])
  })
})
