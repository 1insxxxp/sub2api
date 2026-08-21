import type { CustomMenuItem } from '@/types'

export interface CustomMenuNavigation {
  path: string
  externalUrl?: string
}

export function resolveCustomMenuNavigation(
  item: Pick<CustomMenuItem, 'id' | 'url' | 'open_mode'>,
): CustomMenuNavigation {
  const url = item.url.trim()
  if (item.open_mode === 'new_tab' && /^https?:\/\//i.test(url)) {
    return { path: `/custom/${item.id}`, externalUrl: url }
  }
  return { path: `/custom/${item.id}` }
}
