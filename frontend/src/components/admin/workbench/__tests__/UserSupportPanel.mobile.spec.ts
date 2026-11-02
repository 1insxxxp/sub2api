import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh/userSupport'
import en from '@/i18n/locales/en/userSupport'
import UserSupportPanel from '../UserSupportPanel.vue'

const { lookup, search, grant } = vi.hoisted(() => ({ lookup: vi.fn(), search: vi.fn(), grant: vi.fn() }))
vi.mock('@/api/admin/userSupport', () => ({ lookup, search, grant }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ user: { id: 5, role: 'sub_admin' }, token: 'mobile-session' }) }))
vi.mock('@/composables/useUserSupportAccess', () => ({ useUserSupportAccess: () => ({ canAccess: ref(true), refresh: async () => true, revoke: vi.fn() }) }))

let view: VueWrapper | undefined
const originalWidth = window.innerWidth
afterEach(() => { view?.unmount(); view = undefined; vi.useRealTimers(); vi.clearAllMocks(); document.documentElement.classList.remove('dark'); Object.defineProperty(window, 'innerWidth', { configurable: true, value: originalWidth }) })

// jsdom checks touch/record semantics at H5 viewport states; browser geometry
// must be reviewed separately because jsdom does not perform visual layout.
describe('support H5 content and touch interaction', () => {
  it.each([
    { width: 320, dark: false, locale: 'zh' }, { width: 375, dark: true, locale: 'zh' },
    { width: 320, dark: false, locale: 'en' }, { width: 390, dark: true, locale: 'en' },
  ])('keeps all records and touch-only actions usable at $width px, $locale, dark=$dark', async ({ width, dark, locale }) => {
    vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: width })
    document.documentElement.classList.toggle('dark', dark)
    const email = `${'long.customer'.repeat(8)}@example.com`
    const original = 'Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.\nendpoint=* api_key=*\n<img src=x onerror="alert(1)">'
    search.mockResolvedValue({ items: [{ id: 7147, email }] })
    lookup.mockResolvedValue({
      user: { id: 7147, email, balance: 2 }, trial: { status: 'eligible', amount: 15, granted_at: '', legacy: false },
      usage: { status: 'ok', items: [{ id: 1, created_at: '2026-11-01T04:00:00Z', model: 'gemini-4-argon', group_name: 'Gemini <VIP>', input_tokens: 1234, output_tokens: 50, cache_read_tokens: 0, charged_amount: 0.2, duration_ms: 900, first_token_ms: 100, status: 'completed', stream: true }] },
      errors: { status: 'ok', items: [{ id: 2, created_at: '2026-11-01T04:00:00Z', model: 'gemini-4-argon', group_name: 'Gemini <VIP>', status_code: 502, error_kind: 'upstream', category: 'service_unavailable', original_message: original }] },
    })
    const i18n = createI18n({ legacy: false, locale, messages: { zh: { userSupport: zh }, en: { userSupport: en } } })
    view = mount(UserSupportPanel, { global: { plugins: [i18n], stubs: { Icon: true } } })
    await flushPromises()
    const input = view.get('[data-test="support-account"]')
    await input.setValue('long'); await vi.advanceTimersByTimeAsync(250)
    const option = view.get('[role="option"]')
    const pointer = new Event('pointerdown', { bubbles: true, cancelable: true })
    option.element.dispatchEvent(pointer)
    expect(pointer.defaultPrevented).toBe(true)
    await option.trigger('click'); await flushPromises()
    expect(lookup).toHaveBeenCalledWith('7147'); expect(grant).not.toHaveBeenCalled()
    expect(view.get('[data-test="support-grant"]').text()).toBe(locale === 'zh' ? '发送测试额度' : 'Send trial credit')
    const usage = view.get('[data-test="support-usage-row"]')
    expect(usage.get('h3').text()).toBe('gemini-4-argon'); expect(usage.text()).toContain('Gemini <VIP>')
    expect(usage.get('time').isVisible()).toBe(true); expect(usage.get('time').text()).toMatch(/\d{2}:\d{2}:\d{2}/)
    for (const text of locale === 'zh' ? ['流式', '已完成'] : ['Stream', 'Completed']) expect(usage.text()).toContain(text)
    for (const [field, zhLabel, enLabel, values] of [
      ['tokens', '输入 / 输出 Token', 'Input / output tokens', ['1,234', '50']], ['cache', '缓存', 'Cache', ['0']],
      ['charge', '实际扣费', 'Charged', ['0.20']], ['duration', '耗时', 'Duration', ['0.90s']],
      ['first-token', '首字', 'First token', ['0.10s']],
    ] as const) {
      const metric = usage.get(`[data-field="${field}"]`)
      expect(metric.isVisible()).toBe(true); expect(metric.get('dt').text()).toBe(locale === 'zh' ? zhLabel : enLabel)
      for (const value of values) expect(metric.get('dd').text()).toContain(value)
    }
    await view.get('[data-test="support-tab-errors"]').trigger('click')
    const error = view.get('[data-test="support-error-row"]')
    expect(error.get('[data-test="support-error-status"]').text()).toBe(locale === 'zh' ? 'HTTP 502｜服务端异常' : 'HTTP 502｜Server error')
    expect(error.get('[data-test="support-error-original"] .support-label').text()).toBe(locale === 'zh' ? '原始报错：' : 'Original error：')
    expect(error.get('[data-test="support-error-original"]').text()).toContain(original)
    expect(error.get('[data-test="support-error-chinese"] .support-label').text()).toBe(locale === 'zh' ? '中文说明：' : 'Chinese explanation：')
    // Unknown suffixes must not be silently discarded or fed to a model.
    expect(error.get('[data-test="support-error-chinese"]').text()).toContain(locale === 'zh' ? '暂未提供此报错的中文翻译，请查看原始报错。' : 'No Chinese translation is available for this error. Please refer to the original error.')
    expect(error.find('img').exists()).toBe(false)
    expect(view.get('[data-test="support-tab-errors"]').attributes('aria-selected')).toBe('true')
  })
  it.each([
    ['Gemini rejected the prompt (PROHIBITED_CONTENT). Review the text and attached media, then try again.', 'Gemini 拒绝了此次请求（PROHIBITED_CONTENT，内容限制）。请检查文本和附加媒体后重试。'],
    ['INVALID_ARGUMENT: max_tokens must exceed thinking.budget_tokens; endpoint=*; api_key=*; project=*', 'max_tokens 必须大于 thinking.budget_tokens（思考预算）；接口地址：*；API 密钥：*；项目：*'],
    ['RESOURCE_EXHAUSTED: Quota exceeded for project=*; retry after 30 seconds.', '项目 * 的配额已用尽；请在 30 秒后重试。'],
  ])('shows a separate Chinese explanation while retaining %s', async (original, chinese) => {
    lookup.mockResolvedValue({
      user: { id: 5262, email: 'error-preview@local.invalid', balance: 0 }, trial: { status: 'eligible', amount: 15, granted_at: '', legacy: false },
      usage: { status: 'ok', items: [] },
      errors: { status: 'ok', items: [{ id: 2, created_at: '2026-11-03T02:00:00Z', model: 'gemini-4-argon', group_name: '本地演示', status_code: 502, error_kind: 'upstream', category: 'service_unavailable', original_message: original }] },
    })
    const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh: { userSupport: zh } } })
    view = mount(UserSupportPanel, { global: { plugins: [i18n], stubs: { Icon: true } } })
    await flushPromises()
    await view.get('[data-test="support-account"]').setValue('5262')
    await view.get('form').trigger('submit'); await flushPromises()
    await view.get('[data-test="support-tab-errors"]').trigger('click')
    const error = view.get('[data-test="support-error-row"]')
    expect(error.get('[data-test="support-error-original"]').text()).toContain(original)
    expect(error.get('[data-test="support-error-chinese"]').text()).toBe(`中文说明：${chinese}`)
    expect(grant).not.toHaveBeenCalled()
  })

})
