import { describe, expect, it } from 'vitest'
import {
  buildActivityHeatmapCells,
  getActivityHeatmapLevel,
  type ActivityHeatmapDay,
} from '../activityHeatmap'

const day = (date: string, requests: number): ActivityHeatmapDay => ({
  date,
  success_requests: requests,
  failed_requests: 0,
  input_tokens: 0,
  output_tokens: 0,
  total_tokens: 0,
  billed_cost: 0,
  model_count: 0,
})

describe('activity heatmap helpers', () => {
  it('fills every date in the requested range, including dates with no activity', () => {
    const cells = buildActivityHeatmapCells('2024-02-27', '2024-03-02', [day('2024-02-29', 4)])

    expect(cells.map((cell) => cell.date)).toEqual([
      '2024-02-27',
      '2024-02-28',
      '2024-02-29',
      '2024-03-01',
      '2024-03-02',
    ])
    expect(cells.find((cell) => cell.date === '2024-02-28')?.success_requests).toBe(0)
    expect(cells.find((cell) => cell.date === '2024-02-29')?.success_requests).toBe(4)
  })

  it('uses the Sunday-to-Saturday week position for calendar layout', () => {
    const cells = buildActivityHeatmapCells('2024-03-03', '2024-03-09', [])

    expect(cells.map((cell) => cell.weekday)).toEqual([0, 1, 2, 3, 4, 5, 6])
    expect(cells.map((cell) => cell.weekIndex)).toEqual([0, 0, 0, 0, 0, 0, 0])
  })

  it('calculates relative levels from non-zero values and keeps zero at level zero', () => {
    expect(getActivityHeatmapLevel(0, [1, 2, 3, 4])).toBe(0)
    expect(getActivityHeatmapLevel(1, [1, 2, 3, 4])).toBe(1)
    expect(getActivityHeatmapLevel(2, [1, 2, 3, 4])).toBe(2)
    expect(getActivityHeatmapLevel(3, [1, 2, 3, 4])).toBe(3)
    expect(getActivityHeatmapLevel(4, [1, 2, 3, 4])).toBe(4)
  })
})
