import type { BalanceRechargePromotion, BalanceRechargeTier } from '@/types/payment'

export function validRechargeTiers(tiers: BalanceRechargeTier[]): boolean {
  return tiers.length <= 50 && tiers.every((tier, index) => {
    if (![tier.amount, tier.multiplier].every(Number.isFinite)) return false
    if (tier.amount <= 0 || tier.multiplier <= 0) return false
    const previous = tiers[index - 1]
    return !previous || tier.amount > previous.amount
  })
}

export function validRechargePromotion(promotion: BalanceRechargePromotion | undefined | null, now = Date.now()): boolean {
  if (!promotion?.active || !Number.isFinite(promotion.multiplier) || (promotion.multiplier ?? 0) <= 0) return false
  if (promotion.start_at) {
    const startAt = Date.parse(promotion.start_at)
    if (!Number.isFinite(startAt) || startAt > now) return false
  }
  if (!promotion.end_at) return true
  const endAt = Date.parse(promotion.end_at)
  return Number.isFinite(endAt) && endAt > now
}

export function resolveRechargeMultiplier(
  amount: number,
  tiers: BalanceRechargeTier[] | undefined,
  fallback: number,
  promotion?: BalanceRechargePromotion | null,
  now = Date.now(),
): number {
  if (validRechargePromotion(promotion, now)) return promotion!.multiplier as number
  const fixed = Number.isFinite(fallback) && fallback > 0 ? fallback : 1
  if (!tiers || !validRechargeTiers(tiers)) return fixed
  return tiers.find(tier => Math.abs(amount - tier.amount) < 0.0000001)?.multiplier ?? fixed
}
