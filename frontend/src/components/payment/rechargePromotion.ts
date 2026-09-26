import type { BalanceRechargePromotionSettings } from '@/api/admin/settings'

export function localDateTimeToRFC3339(value: string): string | undefined {
  if (!value) return undefined
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(value)
  if (!match) return undefined
  const [, year, month, day, hour, minute] = match
  const date = new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute), 0, 0)
  if (Number.isNaN(date.getTime())) return undefined
  const pad = (part: number) => String(part).padStart(2, '0')
  const offset = -date.getTimezoneOffset()
  const sign = offset >= 0 ? '+' : '-'
  const offsetMinutes = Math.abs(offset)
  return `${year}-${month}-${day}T${hour}:${minute}:00${sign}${pad(Math.floor(offsetMinutes / 60))}:${pad(offsetMinutes % 60)}`
}

export function rfc3339ToLocalDateTime(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (part: number) => String(part).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function validateRechargePromotion(value: BalanceRechargePromotionSettings): 'multiplier' | 'range' | undefined {
  if (!value.enabled) return undefined
  if (!Number.isFinite(Number(value.multiplier)) || Number(value.multiplier) <= 0) return 'multiplier'
  if (!value.start_at || !value.end_at) return 'range'
  const start = Date.parse(value.start_at)
  const end = Date.parse(value.end_at)
  if (Number.isNaN(start) || Number.isNaN(end) || end <= start) return 'range'
  return undefined
}
