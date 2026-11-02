import { describe, expect, it } from 'vitest'
import { translateSupportErrorToChinese } from '../supportErrorTranslation'

describe('local Chinese support error explanations', () => {
  it.each([
    ['Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.', 'Gemini 拒绝了此次请求（PROHIBITED_CONTENT，内容限制）。请检查文本和附加媒体后重试。'],
    ['Gemini rejected prompt: PROHIBITED_CONTENT', 'Gemini 拒绝了此次请求（PROHIBITED_CONTENT，内容限制）。'],
    ['INVALID_ARGUMENT: max_tokens must exceed thinking.budget_tokens; endpoint=*; api_key=*; project=*', 'max_tokens 必须大于 thinking.budget_tokens（思考预算）；接口地址：*；API 密钥：*；项目：*'],
    ['RESOURCE_EXHAUSTED: Quota exceeded for project=*; retry after 30 seconds.', '项目 * 的配额已用尽；请在 30 秒后重试。'],
    ['Too many requests; retry after 2 seconds', '请求过于频繁；请在 2 秒后重试。'],
    ['rate_limit_error', '请求过于频繁。'],
    ['RESOURCE_EXHAUSTED', '请求资源或配额暂不可用。'],
    ['INVALID_ARGUMENT', '请求参数无效。'],
    ['Invalid API key: *', 'API 密钥无效：*'],
    ['PERMISSION_DENIED', '请求权限不足。'],
    ['DEADLINE_EXCEEDED: Request timed out', '请求超时。'],
    ['Request timed out', '请求超时。'],
    ['Service unavailable', '服务暂不可用。'],
    ['dial tcp *: connection refused', '无法建立连接：连接被拒绝。'],
  ])('explains %s without changing its meaning', (original, expected) => {
    expect(translateSupportErrorToChinese(original)).toBe(expected)
  })

  it.each([
    '', ' ', '*', 'HTTP 500', 'constructor', '自定义中文报错', 'PROHIBITED_CONTENT_SUFFIX',
    'Unknown provider-specific diagnostic',
    'PERMISSION_DENIED: Service unavailable',
    'PROHIBITED_CONTENT: Request timed out',
    'INVALID_ARGUMENT: Service unavailable',
    'Final failure; prompt: PROHIBITED_CONTENT',
    'INVALID_ARGUMENT: unfamiliar constraint that has no reviewed translation',
    'Service unavailable; a second unrelated failure occurred',
    'RESOURCE_EXHAUSTED: Quota exceeded for project=private-name',
    'Request timed out <img src=x onerror=alert(1)>',
  ])('does not fabricate a translation for %s', (original) => {
    expect(translateSupportErrorToChinese(original)).toBeNull()
  })
})
