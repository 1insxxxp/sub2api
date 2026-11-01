import type { UsageLog, UsageTokenSource } from '@/types'
import { isUsageBillingPending } from '@/utils/usageBillingState'

type Translate = (key: string) => string

type UsageSources = {
  input_tokens?: number
  output_tokens?: number
  input_tokens_source?: UsageTokenSource
  output_tokens_source?: UsageTokenSource
  input_tokens_known?: boolean
  output_tokens_known?: boolean
}

export function usageTokenSourceLabel(source: UsageTokenSource | undefined, t: Translate): string {
  switch (source) {
    case 'upstream': return t('usage.tokensSourceUpstream')
    case 'unreported': return t('usage.tokensUnreported')
    case 'estimated': return t('usage.tokensEstimated')
    default: return t('usage.tokensSourceUnknown')
  }
}

export function usageTokenSourceHint(source: UsageTokenSource | undefined, field: 'input' | 'output', t: Translate): string {
  switch (source) {
    case 'unreported': return t(`usage.${field}TokensUnreportedHint`)
    case 'estimated': return t(`usage.${field}TokensEstimatedHint`)
    case 'upstream': return t('usage.tokensSourceUpstream')
    default: return t('usage.tokensSourceUnknownHint')
  }
}

export function isUsageTokenCountKnown(value: number | undefined, source: UsageTokenSource | undefined, known?: boolean): boolean {
  return known !== false && source !== 'unreported' && !(source === 'estimated' && (value ?? 0) <= 0)
}

export function formatUsageTokenCount(value: number | undefined, source: UsageTokenSource | undefined, _t: Translate, known?: boolean): string {
  return isUsageTokenCountKnown(value, source, known) ? (value ?? 0).toLocaleString() : '—'
}

// Empty numeric cells prevent spreadsheets from treating an absent measurement as zero.
// Legacy values stay intact and carry a separate unknown-source column.
export function exportUsageTokenCount(value: number, source: UsageTokenSource | undefined, known?: boolean): number | '' {
  return isUsageTokenCountKnown(value, source, known) ? value : ''
}

// Settlement quantities are independent of whether every upstream measurement
// was available. Preserve the persisted count, including a charged partial count.
export function exportBilledUsageTokenCount(value: number | undefined, log: Pick<UsageLog, 'billing_status'>): number | '' {
  return !isUsageBillingPending(log) && typeof value === 'number' && Number.isInteger(value) && value >= 0 ? value : ''
}

export function formatBilledUsageTokenCount(value: number | undefined, log: Pick<UsageLog, 'billing_status'>, t: Translate): string {
  if (isUsageBillingPending(log)) return t('usage.billingPendingReview')
  const count = exportBilledUsageTokenCount(value, log)
  return count === '' ? t('usage.billedTokensUnavailable') : count.toLocaleString()
}

export function hasUnreportedUsage(log: UsageSources | null): boolean {
  return !!log && (!isUsageTokenCountKnown(log.input_tokens, log.input_tokens_source, log.input_tokens_known)
    || !isUsageTokenCountKnown(log.output_tokens, log.output_tokens_source, log.output_tokens_known))
}
