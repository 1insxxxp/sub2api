import { formatTokenCount } from './format'

export const TOKENS_PER_USD_ESTIMATE = 1_000_000

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
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
  return formatTokenCount(safeTokens)
}
