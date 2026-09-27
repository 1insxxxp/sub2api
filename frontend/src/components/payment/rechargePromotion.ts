import type { BalanceRechargePromotionSettings } from '@/api/admin/settings'
import type { BalanceRechargeTier } from '@/types/payment'

function creditedAmountForTier(tier: BalanceRechargeTier): number {
  return Math.round(tier.amount * tier.multiplier * 100) / 100
}

export function pruneRechargePromotionPriceTiers(
  promotion: BalanceRechargePromotionSettings,
  tiers: BalanceRechargeTier[] = [],
): BalanceRechargePromotionSettings {
  const validCredits = new Set(tiers.map(creditedAmountForTier))
  return {
    ...promotion,
    price_tiers: (promotion.price_tiers || []).filter((priceTier) => (
      validCredits.has(Math.round(Number(priceTier.credited_amount) * 100) / 100)
    )),
  }
}

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

export function validateRechargePromotion(
  value: BalanceRechargePromotionSettings,
  tiers: BalanceRechargeTier[] = [],
): 'priceTiers' | 'range' | undefined {
  if (!value.enabled) return undefined
  const priceTiers = Array.isArray(value.price_tiers) ? value.price_tiers : []
  if (priceTiers.length > 0) {
    if (tiers.length === 0) return 'priceTiers'
    const seenCredits = new Set<number>()
    const seenPrices = new Set<number>()
    for (const priceTier of priceTiers) {
      const credit = Number(priceTier.credited_amount)
      const price = Number(priceTier.price)
      if (![credit, price].every((item) => Number.isFinite(item) && item > 0 && Math.abs(item - Math.round(item * 100) / 100) < 0.0000001)) return 'priceTiers'
      if (seenCredits.has(credit) || seenPrices.has(price)) return 'priceTiers'
      seenCredits.add(credit)
      seenPrices.add(price)
      const matches = tiers.filter((tier) => Math.abs(creditedAmountForTier(tier) - credit) < 0.0000001)
      if (matches.length !== 1) return 'priceTiers'
      if (tiers.some((tier) => Math.abs(tier.amount - price) < 0.0000001 && Math.abs(creditedAmountForTier(tier) - credit) >= 0.0000001)) return 'priceTiers'
    }
  }
  if (priceTiers.length === 0) return 'priceTiers'
  if (!value.start_at || !value.end_at) return 'range'
  const start = Date.parse(value.start_at)
  const end = Date.parse(value.end_at)
  if (Number.isNaN(start) || Number.isNaN(end) || end <= start) return 'range'
  return undefined
}
