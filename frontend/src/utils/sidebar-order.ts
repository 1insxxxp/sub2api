export interface SidebarOrderItem {
  path: string
}

/**
 * Stable identifiers for the built-in user navigation entries. Keep this list
 * independent from feature visibility: the sidebar applies the configured
 * order after it has filtered entries that are unavailable on the current
 * site.
 */
export const DEFAULT_USER_SIDEBAR_ORDER = [
  '/dashboard',
  '/model-plaza',
  '/keys',
  '/images',
  '/batch-image',
  '/usage',
  '/available-channels',
  '/monitor',
  '/model-status',
  '/subscriptions',
  '/purchase',
  '/orders',
  '/redeem',
  '/balance-transfer',
  '/lottery',
  '/affiliate',
  '/profile',
] as const

/**
 * Apply an administrator-provided order while keeping the current navigation
 * list authoritative. Unknown and duplicate identifiers are ignored, and any
 * current item omitted from the setting is appended in its default order.
 */
export function orderSidebarItems<T extends SidebarOrderItem>(
  items: readonly T[],
  configuredOrder?: readonly string[] | null,
): T[] {
  if (!configuredOrder?.length) return [...items]

  const byPath = new Map(items.map((item) => [item.path, item]))
  const ordered: T[] = []
  const seen = new Set<string>()

  for (const path of configuredOrder) {
    if (seen.has(path)) continue
    const item = byPath.get(path)
    if (!item) continue
    seen.add(path)
    ordered.push(item)
  }

  for (const item of items) {
    if (seen.has(item.path)) continue
    seen.add(item.path)
    ordered.push(item)
  }

  return ordered
}

/**
 * Return the complete orderable catalog used by the administrator editor.
 * Custom menu paths are appended to the built-in catalog, while the generic
 * ordering helper keeps configured entries first and removes stale IDs.
 */
export function resolveUserSidebarOrder(
  configuredOrder?: readonly string[] | null,
  customPaths: readonly string[] = [],
): string[] {
  const paths = [
    ...DEFAULT_USER_SIDEBAR_ORDER,
    ...customPaths.filter((path) => typeof path === 'string' && path.trim()),
  ]
  const items = paths.map((path) => ({ path }))
  return orderSidebarItems(items, configuredOrder).map((item) => item.path)
}
