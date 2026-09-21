export interface OpsTimeRangeSnapshot {
  timeRange: string
  customStartTime: string | null
  customEndTime: string | null
}

export const OPS_TIME_RANGE_STORAGE_KEY = 'admin-ops-dashboard-time-range-v1'

function isValidDate(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0 && Number.isFinite(Date.parse(value))
}

function normalizeSnapshot(value: unknown, allowedRanges: ReadonlySet<string>): OpsTimeRangeSnapshot | null {
  if (!value || typeof value !== 'object') return null
  const input = value as Record<string, unknown>
  const timeRange = typeof input.timeRange === 'string' ? input.timeRange : ''
  if (!allowedRanges.has(timeRange)) return null

  if (timeRange !== 'custom') {
    return { timeRange, customStartTime: null, customEndTime: null }
  }

  const customStartTime = input.customStartTime
  const customEndTime = input.customEndTime
  if (!isValidDate(customStartTime) || !isValidDate(customEndTime)) return null
  if (Date.parse(customStartTime) >= Date.parse(customEndTime)) return null

  return {
    timeRange,
    customStartTime,
    customEndTime
  }
}

export function loadOpsTimeRangeSnapshot(
  storage: Pick<Storage, 'getItem'> | null | undefined,
  allowedRanges: ReadonlySet<string>
): OpsTimeRangeSnapshot | null {
  if (!storage) return null
  try {
    const raw = storage.getItem(OPS_TIME_RANGE_STORAGE_KEY)
    if (!raw) return null
    return normalizeSnapshot(JSON.parse(raw), allowedRanges)
  } catch {
    return null
  }
}

export function saveOpsTimeRangeSnapshot(
  storage: Pick<Storage, 'setItem'> | null | undefined,
  snapshot: OpsTimeRangeSnapshot
): void {
  if (!storage) return
  const normalized = snapshot.timeRange === 'custom'
    ? snapshot
    : { ...snapshot, customStartTime: null, customEndTime: null }
  try {
    storage.setItem(OPS_TIME_RANGE_STORAGE_KEY, JSON.stringify(normalized))
  } catch {
    // Storage can be unavailable in private browsing or after quota exhaustion.
  }
}
