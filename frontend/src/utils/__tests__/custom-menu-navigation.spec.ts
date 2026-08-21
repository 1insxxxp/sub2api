import { describe, expect, it } from 'vitest'

import { resolveCustomMenuNavigation } from '@/utils/custom-menu-navigation'

describe('resolveCustomMenuNavigation', () => {
  it('keeps the original URL for a new-tab menu', () => {
    expect(resolveCustomMenuNavigation({
      id: 'card',
      url: 'https://card.example.com/categories/cards?source=menu',
      open_mode: 'new_tab',
    })).toEqual({
      path: '/custom/card',
      externalUrl: 'https://card.example.com/categories/cards?source=menu',
    })
  })

  it('keeps legacy menus embedded', () => {
    expect(resolveCustomMenuNavigation({
      id: 'help',
      url: 'https://help.example.com',
    })).toEqual({ path: '/custom/help' })
  })

  it('never treats markdown content as an external link', () => {
    expect(resolveCustomMenuNavigation({
      id: 'docs',
      url: 'md:docs',
      open_mode: 'new_tab',
    })).toEqual({ path: '/custom/docs' })
  })

  it('rejects non-http URLs from the external branch', () => {
    expect(resolveCustomMenuNavigation({
      id: 'unsafe',
      url: 'javascript:alert(1)',
      open_mode: 'new_tab',
    })).toEqual({ path: '/custom/unsafe' })
  })
})
