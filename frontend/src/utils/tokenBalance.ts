export const TOKENS_PER_USD_ESTIMATE = 1_000_000

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function trimTrailingZero(value: string): string {
  return value.replace(/\.0$/, '')
}

export function estimateAvailableTokens(
  balance: number | null | undefined,
  tokensPerUsd: number = TOKENS_PER_USD_ESTIMATE
): number | null {
  if (!isFiniteNumber(balance) || !isFiniteNumber(tokensPerUsd) || tokensPerUsd <= 0) {
    return null
  }

  return Math.max(0, Math.floor(balance * tokensPerUsd))
}

export function formatAvailableTokens(tokens: number | null | undefined): string {
  if (!isFiniteNumber(tokens)) {
    return '-'
  }

  const safeTokens = Math.max(0, Math.floor(tokens))
  if (safeTokens >= 1_000_000_000) {
    return `${trimTrailingZero((safeTokens / 1_000_000_000).toFixed(2))}B`
  }
  if (safeTokens >= 1_000_000) {
    return `${trimTrailingZero((safeTokens / 1_000_000).toFixed(1))}M`
  }
  if (safeTokens >= 1_000) {
    return `${trimTrailingZero((safeTokens / 1_000).toFixed(1))}K`
  }
  return safeTokens.toString()
}
