import type { BalanceRechargeTier } from '@/types/payment'

export function validRechargeTiers(tiers: BalanceRechargeTier[]): boolean {
  return tiers.length <= 50 && tiers.every((tier, index) => {
    if (![tier.amount, tier.multiplier].every(Number.isFinite)) return false
    if (tier.amount <= 0 || tier.multiplier <= 0) return false
    const previous = tiers[index - 1]
    return !previous || tier.amount > previous.amount
  })
}

export function resolveRechargeMultiplier(amount: number, tiers: BalanceRechargeTier[] | undefined, fallback: number): number {
  const fixed = Number.isFinite(fallback) && fallback > 0 ? fallback : 1
  if (!tiers || !validRechargeTiers(tiers)) return fixed
  return tiers.find(tier => Math.abs(amount - tier.amount) < 0.0000001)?.multiplier ?? fixed
}
