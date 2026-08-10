export function formatAvailableChannelRate(value: number | null | undefined): string {
  if (value == null || !Number.isFinite(value) || value < 0) return '-'

  const scaled = value * 100
  if (!Number.isFinite(scaled)) return '-'

  const boundaryCompensation = Number.EPSILON * Math.max(1, Math.abs(scaled))
  const truncated = Math.floor(scaled + boundaryCompensation) / 100
  return truncated.toFixed(2)
}
