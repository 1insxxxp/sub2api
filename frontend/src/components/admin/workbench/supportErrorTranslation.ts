// This helper only consumes the masked public original_message. It never calls
// a translation API or copies arbitrary error fragments into its output.
const codeExplanations: Record<string, string> = {
  PROHIBITED_CONTENT: '请求因内容限制被拒绝（PROHIBITED_CONTENT）。',
  SAFETY: '请求被内容安全检查拦截。',
  BLOCKED_REASON_SAFETY: '请求被内容安全检查拦截。',
  CONTENT_FILTERED: '请求被内容安全检查拦截。',
  CONTENT_POLICY_VIOLATION: '请求违反内容安全策略。',
  RESOURCE_EXHAUSTED: '请求资源或配额暂不可用。',
  QUOTA_EXCEEDED: '可用配额已用尽。',
  INSUFFICIENT_QUOTA: '可用额度不足。',
  RATE_LIMIT_EXCEEDED: '请求过于频繁。',
  RATE_LIMIT_ERROR: '请求过于频繁。',
  TOO_MANY_REQUESTS: '请求过于频繁。',
  UNAUTHENTICATED: '请求认证失败。',
  INVALID_API_KEY: 'API 密钥无效。',
  AUTHENTICATION_ERROR: '请求认证失败。',
  PERMISSION_DENIED: '请求权限不足。',
  INVALID_ARGUMENT: '请求参数无效。',
  INVALID_REQUEST_ERROR: '请求参数无效。',
  CONTEXT_LENGTH_EXCEEDED: '请求超出模型允许的上下文长度。',
  DEADLINE_EXCEEDED: '请求超时。',
  REQUEST_TIMEOUT: '请求超时。',
  TIMEOUT_ERROR: '请求超时。',
  UNAVAILABLE: '服务暂不可用。',
  SERVICE_UNAVAILABLE: '服务暂不可用。',
  OVERLOADED_ERROR: '服务暂时过载。',
}

const phraseExplanations = new Map([
  ['too many requests', '请求过于频繁。'],
  ['request timed out', '请求超时。'],
  ['the request timed out. please try again later', '请求超时，请稍后重试。'],
  ['deadline exceeded', '请求超时。'],
  ['service unavailable', '服务暂不可用。'],
  ['the service is temporarily unavailable. please try again later', '服务暂不可用，请稍后重试。'],
  ['request failed', '请求失败。'],
  ['invalid request parameters. review the request and try again', '请求参数无效，请检查请求后重试。'],
  ['request authentication failed. please contact support', '请求认证失败，请联系支持人员。'],
  ['request permission was denied. please contact support', '请求权限不足，请联系支持人员。'],
])

function translateClause(value: string): string | null {
  const clause = value.replace(/\.$/, '')
  if (Object.prototype.hasOwnProperty.call(codeExplanations, clause.toUpperCase())) return codeExplanations[clause.toUpperCase()]
  const phrase = phraseExplanations.get(clause.toLowerCase())
  if (phrase) return phrase

  const rejection = /^Gemini rejected (?:the )?(?:prompt|request)(?: \(PROHIBITED_CONTENT\)|:\s*PROHIBITED_CONTENT)(\. Review the text and attached media, then try again)?$/i.exec(clause)
  if (rejection) return 'Gemini 拒绝了此次请求（PROHIBITED_CONTENT，内容限制）。' + (rejection[1] ? '请检查文本和附加媒体后重试。' : '')
  if (/^max_tokens must (?:exceed|be greater than) thinking\.budget_tokens$/i.test(clause)) return 'max_tokens 必须大于 thinking.budget_tokens（思考预算）。'
  if (/^max_output_tokens must be greater than thinking_budget$/i.test(clause)) return 'max_output_tokens 必须大于 thinking_budget（思考预算）。'
  if (/^quota exceeded for project=\*$/i.test(clause)) return '项目 * 的配额已用尽。'
  if (/^quota exceeded$/i.test(clause)) return '可用配额已用尽。'
  if (/^invalid api key:\s*\*$/i.test(clause)) return 'API 密钥无效：*'
  if (/^(?:dial tcp \*:\s*)?connection refused$/i.test(clause)) return '无法建立连接：连接被拒绝。'

  const retry = /^retry after ([0-9]{1,6}(?:\.[0-9]{1,3})?) seconds?$/i.exec(clause)
  if (retry) return `请在 ${retry[1]} 秒后重试。`
  const maskedField = /^(endpoint|api[_-]?key|project)\s*=\s*\*$/i.exec(clause)
  if (maskedField) {
    const label = maskedField[1].toLowerCase()
    return `${label === 'endpoint' ? '接口地址' : label === 'project' ? '项目' : 'API 密钥'}：*`
  }
  return null
}

/** Returns null when the complete reason has no reviewed local translation. */
export function translateSupportErrorToChinese(message: string): string | null {
  const original = message.trim()
  if (!original || original.length > 2000) return null
  // A symbolic envelope may prefix a more specific reason. Never let that
  // generic code replace unknown diagnostic prose following it.
  const envelope = /^([A-Z_]+):\s+/i.exec(original)
  let reason = original
  if (envelope && Object.prototype.hasOwnProperty.call(codeExplanations, envelope[1].toUpperCase())) {
    const code = envelope[1].toUpperCase()
    const detail = original.slice(envelope[0].length)
    const firstClause = detail.split(/[;\n]/)[0].trim().replace(/\.$/, '')
    const specific = translateClause(firstClause)
    const parameterDetail = ['INVALID_ARGUMENT', 'INVALID_REQUEST_ERROR'].includes(code)
      && /^(?:max_tokens must (?:exceed|be greater than) thinking\.budget_tokens|max_output_tokens must be greater than thinking_budget)$/i.test(firstClause)
    const quotaDetail = ['RESOURCE_EXHAUSTED', 'QUOTA_EXCEEDED'].includes(code)
      && /^quota exceeded(?: for project=\*)?$/i.test(firstClause)
    // Only omit a general code when the reviewed detail expresses that same
    // cause. Contradictory envelopes must not silently lose an error reason.
    if (!specific || (specific !== codeExplanations[code] && !parameterDetail && !quotaDetail)) return null
    reason = detail
  }
  const clauses = reason.split(/[;\n]+/).map(value => value.trim()).filter(Boolean)
  const translated: string[] = []
  for (const clause of clauses) {
    const explanation = translateClause(clause)
    if (!explanation) return null
    translated.push(explanation)
  }
  if (!translated.length) return null
  return translated.map((value, index) => index < translated.length - 1 ? value.replace(/。$/, '') : value).join('；')
}
