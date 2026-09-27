export interface ActivityHeatmapDay {
  date: string
  success_requests: number
  failed_requests: number
  input_tokens: number
  output_tokens: number
  total_tokens: number
  billed_cost: number
  model_count: number
}

export interface ActivityHeatmapCell extends ActivityHeatmapDay {
  weekday: number
  weekIndex: number
}

const dateFromInput = (date: string) => {
  const [year, month, day] = date.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day))
}

const dateToInput = (date: Date) => date.toISOString().slice(0, 10)

export function buildActivityHeatmapCells(
  startDate: string,
  endDate: string,
  days: ActivityHeatmapDay[],
): ActivityHeatmapCell[] {
  const start = dateFromInput(startDate)
  const end = dateFromInput(endDate)
  const values = new Map(days.map((day) => [day.date, day]))
  const cells: ActivityHeatmapCell[] = []
  const startWeekday = start.getUTCDay()

  for (const cursor = new Date(start); cursor <= end; cursor.setUTCDate(cursor.getUTCDate() + 1)) {
    const date = dateToInput(cursor)
    const weekday = cursor.getUTCDay()
    const day = values.get(date) ?? {
      date,
      success_requests: 0,
      failed_requests: 0,
      input_tokens: 0,
      output_tokens: 0,
      total_tokens: 0,
      billed_cost: 0,
      model_count: 0,
    }

    cells.push({
      ...day,
      weekday,
      weekIndex: Math.floor((cells.length + startWeekday) / 7),
    })
  }

  return cells
}

export function getActivityHeatmapLevel(value: number, nonZeroValues: number[]): number {
  if (value <= 0 || nonZeroValues.length === 0) return 0

  const rank = nonZeroValues.filter((item) => item <= value).length / nonZeroValues.length
  return Math.max(1, Math.min(4, Math.ceil(rank * 4)))
}
