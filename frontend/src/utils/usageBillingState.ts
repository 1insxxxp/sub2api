import type { UsageLog } from '@/types'

type BillingState = Pick<UsageLog, 'billing_status'>
type Translate = (key: string) => string

export function isUsageBillingPending(log: BillingState | null | undefined): boolean {
  return log?.billing_status === 'pending_review'
}

export function usageBillingStatusLabel(log: BillingState, t: Translate): string {
  return t(isUsageBillingPending(log) ? 'usage.billingPendingReview' : 'usage.billingSettled')
}

// A pending charge is not a measured zero. Keep that distinction in every export.
export function formatUsageBillingCost(value: number | null | undefined, log: BillingState, t: Translate, digits = 6): string {
  return isUsageBillingPending(log) ? t('usage.billingPendingReview') : (value ?? 0).toFixed(digits)
}
