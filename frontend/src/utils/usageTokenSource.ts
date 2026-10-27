import type { UsageTokenSource } from '@/types'

type Translate = (key: string) => string

type UsageSources = {
  input_tokens_source?: UsageTokenSource
  output_tokens_source?: UsageTokenSource
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

export function formatUsageTokenCount(value: number | undefined, source: UsageTokenSource | undefined, t: Translate): string {
  return source === 'unreported' ? t('usage.tokensUnreported') : (value ?? 0).toLocaleString()
}

// Empty numeric cells prevent spreadsheets from treating an absent measurement as zero.
// Legacy values stay intact and carry a separate unknown-source column.
export function exportUsageTokenCount(value: number, source: UsageTokenSource | undefined): number | '' {
  return source === 'unreported' ? '' : value
}

export function hasUnreportedUsage(log: UsageSources | null): boolean {
  return log?.input_tokens_source === 'unreported' || log?.output_tokens_source === 'unreported'
}
