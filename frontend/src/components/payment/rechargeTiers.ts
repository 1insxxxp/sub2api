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
  if (!promotion?.active) return false
  const hasMultiplier = Number.isFinite(promotion.multiplier) && (promotion.multiplier ?? 0) > 0
  const hasPriceTiers = Array.isArray(promotion.price_tiers) && promotion.price_tiers.length > 0 && promotion.price_tiers.every((tier) => (
    Number.isFinite(tier.credited_amount) && tier.credited_amount > 0 && Number.isFinite(tier.price) && tier.price > 0
  ))
  if (!hasMultiplier && !hasPriceTiers) return false
  if (promotion.start_at) {
    const startAt = Date.parse(promotion.start_at)
    if (!Number.isFinite(startAt) || startAt > now) return false
  }
  if (!promotion.end_at) return true
  const endAt = Date.parse(promotion.end_at)
  return Number.isFinite(endAt) && endAt > now
}

export function resolveRechargePaymentAmount(
  tier: BalanceRechargeTier,
  promotion: BalanceRechargePromotion | undefined | null,
  now = Date.now(),
): number {
  if (!validRechargePromotion(promotion, now)) return tier.amount
  const credited = Math.round(tier.amount * tier.multiplier * 100) / 100
  const priceTier = promotion?.price_tiers?.find((item) => Math.abs(item.credited_amount - credited) < 0.0000001)
  return priceTier?.price ?? tier.amount
}

export function resolveRechargeMultiplier(
  amount: number,
  tiers: BalanceRechargeTier[] | undefined,
  fallback: number,
  promotion?: BalanceRechargePromotion | null,
  now = Date.now(),
): number {
  if (validRechargePromotion(promotion, now)) {
    const priceTier = promotion!.price_tiers?.find((tier) => Math.abs(tier.price - amount) < 0.0000001)
    if (priceTier) return priceTier.credited_amount / amount
    if (Number.isFinite(promotion!.multiplier) && (promotion!.multiplier ?? 0) > 0) return promotion!.multiplier as number
  }
  const fixed = Number.isFinite(fallback) && fallback > 0 ? fallback : 1
  if (!tiers || !validRechargeTiers(tiers)) return fixed
  return tiers.find(tier => Math.abs(amount - tier.amount) < 0.0000001)?.multiplier ?? fixed
}
