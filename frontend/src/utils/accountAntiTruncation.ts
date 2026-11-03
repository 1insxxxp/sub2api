export interface AccountAntiTruncationRule {
  model: string
  enabled: boolean
  max_rounds?: number
  max_output_tokens?: number
  max_duration_seconds?: number
}

export interface AccountAntiTruncationPolicy {
  version: 1
  rules: AccountAntiTruncationRule[]
}

export function accountAntiTruncationFromExtra(extra: unknown): AccountAntiTruncationPolicy | null {
  if (!extra || typeof extra !== 'object' || !('anti_truncation' in extra)) return null
  const raw = extra.anti_truncation
  if (!raw || typeof raw !== 'object' || !('version' in raw) || raw.version !== 1 || !('rules' in raw) || !Array.isArray(raw.rules)) {
    // Existing malformed policy is fail-closed on the server. Keep ownership
    // when displayed rather than silently falling back to legacy group rules.
    return { version: 1, rules: [] }
  }
  if (Object.keys(raw).some(key => !['version', 'rules'].includes(key)) || raw.rules.some(rule =>
    !rule || typeof rule !== 'object' || Array.isArray(rule) ||
    !('model' in rule) || typeof rule.model !== 'string' ||
    !('enabled' in rule) || typeof rule.enabled !== 'boolean' ||
    Object.keys(rule).some(key => !['model', 'enabled', 'max_rounds', 'max_output_tokens', 'max_duration_seconds'].includes(key))
  )) return { version: 1, rules: [] }
  const policy = JSON.parse(JSON.stringify(raw)) as AccountAntiTruncationPolicy
  return validateAccountAntiTruncation(policy) === null ? policy : { version: 1, rules: [] }
}

export function mergeAccountAntiTruncationExtra<T extends Record<string, unknown> | undefined>(extra: T, policy: AccountAntiTruncationPolicy | null): T | Record<string, unknown> {
  if (policy === null) return extra
  return { ...(extra || {}), anti_truncation: JSON.parse(JSON.stringify(policy)) }
}

export function validateAccountAntiTruncation(policy: AccountAntiTruncationPolicy | null): string | null {
  if (policy === null) return null
  const seen = new Set<string>()
  for (const rule of policy.rules) {
    if (!rule.model.trim()) return 'modelRequired'
    if ([...rule.model].length > 200 || /[\s\p{Cc}]/u.test(rule.model)) return 'modelInvalid'
    if (seen.has(rule.model)) return 'modelDuplicate'
    seen.add(rule.model)
    for (const [field, max] of [['max_rounds', 3], ['max_output_tokens', 32768], ['max_duration_seconds', 300]] as const) {
      const value = rule[field]
      if (value !== undefined && (!Number.isInteger(value) || value < 0 || value > max)) return 'limitsInvalid'
    }
  }
  return null
}
