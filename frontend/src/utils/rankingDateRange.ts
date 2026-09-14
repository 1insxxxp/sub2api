export type RankingDatePreset = 'today' | 'yesterday' | '7days' | '30days'

export interface DateRangeValue { start: string; end: string }

export const formatCalendarDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/** Calendar ranges are evaluated in the browser's local timezone. */
export const getRankingDateRange = (
  preset: RankingDatePreset,
  now: Date = new Date(),
): DateRangeValue => {
  const end = new Date(now)
  end.setHours(0, 0, 0, 0)
  const start = new Date(end)
  if (preset === 'yesterday') start.setDate(start.getDate() - 1)
  if (preset === '7days') start.setDate(start.getDate() - 6)
  if (preset === '30days') start.setDate(start.getDate() - 29)
  return { start: formatCalendarDate(start), end: formatCalendarDate(preset === 'yesterday' ? start : end) }
}
