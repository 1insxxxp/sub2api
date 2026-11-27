import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import PublicGatewayErrorRules from '../PublicGatewayErrorRules.vue'
import ErrorPassthroughRulesModal from '../ErrorPassthroughRulesModal.vue'
import type { PublicGatewayErrorSettings } from '@/api/admin/publicGatewayErrors'

const mocks = vi.hoisted(() => ({
  getSettings: vi.fn(), updateSettings: vi.fn(), preview: vi.fn(),
  list: vi.fn(), showSuccess: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/admin/publicGatewayErrors', () => ({ publicGatewayErrorsAPI: mocks }))
vi.mock('@/api/admin', () => ({ adminAPI: { errorPassthrough: { list: mocks.list } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
afterEach(() => vi.restoreAllMocks())

function settings(): PublicGatewayErrorSettings {
  return {
    rules: [
      { id: 'capacity', name: 'Capacity', enabled: true, priority: 10,
        upstream_statuses: [429, 503], pattern: '(?i)capacity',
        status: 503, code: 'busy', message: 'Please retry later.' },
      { id: 'context', name: 'Context', enabled: true, priority: 10,
        upstream_statuses: [400], pattern: '',
        status: 400, code: 'context_limit', message: 'Shorten your request.' }
    ],
    fallback: { status: 502, code: 'upstream_error', message: 'Please retry.' },
    bad_request_fallback: { status: 400, code: 'bad_request', message: 'Check your request.' }
  }
}

const stubs = {
  BaseDialog: {
    props: ['show'],
    template: '<div v-if="show"><slot /><slot name="footer" /></div>'
  },
  ConfirmDialog: {
    props: ['show'], emits: ['confirm', 'cancel'],
    template: '<div v-if="show" data-testid="confirm-discard"><button @click="$emit(\'confirm\')">Discard</button><button @click="$emit(\'cancel\')">Keep editing</button></div>'
  },
  Icon: true
}

beforeEach(() => {
  vi.resetAllMocks()
  mocks.getSettings.mockResolvedValue(settings())
  mocks.updateSettings.mockImplementation(async (value: PublicGatewayErrorSettings) => JSON.parse(JSON.stringify(value)))
  mocks.preview.mockResolvedValue({
    status: 503, code: 'busy', message: 'Please retry later.', rule_id: 'capacity',
    rule_name: 'Capacity', payload: { error: { code: 'busy', message: 'Please retry later.' } }
  })
  mocks.list.mockResolvedValue([])
})

async function openRules() {
  const wrapper = mount(PublicGatewayErrorRules, { global: { stubs } })
  await flushPromises()
  return wrapper
}

describe('public gateway error rule drafts', () => {
  it('toggles locally and saves the full settings with stable IDs and array order', async () => {
    const wrapper = await openRules()
    await wrapper.get('[data-rule-id="capacity"] [role="switch"]').trigger('click')
    expect(mocks.updateSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-rule-id="capacity"] [role="switch"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.emitted('dirty-change')?.at(-1)).toEqual([true])

    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await flushPromises()
    const expected = settings()
    expected.rules[0].enabled = false
    expect(mocks.updateSettings.mock.calls).toEqual([[expected]])
    expect(wrapper.emitted('dirty-change')?.at(-1)).toEqual([false])
    expect(wrapper.get('[data-testid="gateway-save"]').attributes('disabled')).toBeDefined()
  })

  it('edits, adds and deletes locally until Save all', async () => {
    vi.spyOn(crypto, 'randomUUID').mockReturnValue('12345678-1234-1234-1234-123456789012')
    const wrapper = await openRules()
    await wrapper.get('[data-testid="edit-rule-capacity"]').trigger('click')
    await wrapper.get('[data-testid="gateway-rule-form"] [name="name"]').setValue('Capacity changed')
    await wrapper.get('[data-testid="gateway-rule-form"] [name="pattern"]').setValue('(?i)busy|capacity')
    await wrapper.get('[data-testid="gateway-rule-form"]').trigger('submit')
    expect(wrapper.get('[data-rule-id="capacity"]').text()).toContain('Capacity changed')

    await wrapper.get('[data-testid="gateway-add"]').trigger('click')
    const editor = wrapper.get('[data-testid="gateway-rule-form"]')
    await editor.get('[name="name"]').setValue('New rule')
    await editor.get('[name="upstream_statuses"]').setValue('500, 502')
    await editor.get('[name="code"]').setValue('temporary')
    await editor.get('[name="message"]').setValue('Try again soon.')
    await editor.trigger('submit')
    const addedId = wrapper.findAll('[data-rule-id]')[2].attributes('data-rule-id')
    expect(addedId).toBeTruthy()
    expect(addedId).not.toBe('capacity')
    expect(addedId).toMatch(/^[a-z][a-z0-9_-]{0,63}$/)

    await wrapper.get('[data-testid="delete-rule-context"]').trigger('click')
    expect(wrapper.find('[data-rule-id="context"]').exists()).toBe(false)
    expect(mocks.updateSettings).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await flushPromises()
    expect(mocks.updateSettings.mock.calls[0][0].rules.map((rule: { id: string }) => rule.id))
      .toEqual(['capacity', addedId])
  })

  it('keeps a failed save dirty and adopts the successful server response on retry', async () => {
    mocks.updateSettings.mockRejectedValueOnce({ message: 'Invalid RE2 pattern' })
    const wrapper = await openRules()
    await wrapper.get('[data-rule-id="capacity"] [role="switch"]').trigger('click')
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="gateway-save-error"]').text()).toContain('Invalid RE2 pattern')
    expect(wrapper.emitted('dirty-change')?.at(-1)).toEqual([true])
    const saved = settings()
    saved.rules[0].name = 'Saved by server'
    saved.rules[0].enabled = false
    mocks.updateSettings.mockResolvedValueOnce(saved)
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-rule-id="capacity"]').text()).toContain('Saved by server')
    expect(wrapper.emitted('dirty-change')?.at(-1)).toEqual([false])
  })

  it('disables mutations and duplicate saves while a save is pending', async () => {
    let finish!: (value: PublicGatewayErrorSettings) => void
    mocks.updateSettings.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const wrapper = await openRules()
    await wrapper.get('[data-rule-id="capacity"] [role="switch"]').trigger('click')
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    expect(mocks.updateSettings).toHaveBeenCalledTimes(1)
    expect(wrapper.get('fieldset').attributes('disabled')).toBeDefined()
    const saved = settings()
    saved.rules[0].enabled = false
    finish(saved)
    await flushPromises()
    expect(wrapper.get('fieldset').attributes('disabled')).toBeUndefined()
  })

  it('retries a failed load without offering empty settings for saving', async () => {
    mocks.getSettings.mockRejectedValueOnce(new Error('offline'))
    const wrapper = await openRules()
    expect(wrapper.get('[data-testid="gateway-load-error"]').text()).toContain('offline')
    expect(wrapper.find('[data-testid="gateway-save"]').exists()).toBe(false)
    await wrapper.get('[data-testid="gateway-retry"]').trigger('click')
    await flushPromises()
    expect(mocks.getSettings).toHaveBeenCalledTimes(2)
    expect(wrapper.findAll('[data-rule-id]')).toHaveLength(2)
  })

  it.each(['0', '399', '600', '400.5', '400oops'])('rejects an invalid source status %s', async (status) => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="edit-rule-capacity"]').trigger('click')
    await wrapper.get('[data-testid="gateway-rule-form"] [name="upstream_statuses"]').setValue(status)
    await wrapper.get('[data-testid="gateway-rule-form"]').trigger('submit')
    expect(wrapper.get('[data-testid="gateway-rule-error"]').text()).toContain('admin.publicGatewayErrors.validation.sourceStatuses')
    expect(wrapper.emitted('dirty-change')?.at(-1)).toEqual([false])
  })

  it('requires a match condition and validates editable fallback outputs before saving', async () => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="edit-rule-capacity"]').trigger('click')
    const editor = wrapper.get('[data-testid="gateway-rule-form"]')
    await editor.get('[name="upstream_statuses"]').setValue('')
    await editor.get('[name="pattern"]').setValue('')
    await editor.trigger('submit')
    expect(wrapper.get('[data-testid="gateway-rule-error"]').text()).toContain('admin.publicGatewayErrors.validation.conditions')
    await wrapper.get('[data-testid="gateway-rule-cancel"]').trigger('click')
    await wrapper.get('[data-testid="fallback"] [name="status"]').setValue(200)
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    expect(mocks.updateSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="gateway-save-error"]').text()).toContain('admin.publicGatewayErrors.validation.output')
  })

  it('limits the rules list to 100 entries', async () => {
    const many = settings()
    many.rules = Array.from({ length: 100 }, (_, index) => ({ ...many.rules[0], id: `rule-${index}` }))
    mocks.getSettings.mockResolvedValue(many)
    const wrapper = await openRules()
    expect(wrapper.get('[data-testid="gateway-add"]').attributes('disabled')).toBeDefined()
  })

  it.each(['UPSTREAM', 'bad-code', '9error', 'a'.repeat(65)])('rejects an invalid output code %s before save or preview', async (code) => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="fallback"] [name="code"]').setValue(code)
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    expect(mocks.updateSettings).not.toHaveBeenCalled()
    expect(mocks.preview).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="gateway-save-error"]').text()).toContain('admin.publicGatewayErrors.validation.code')
  })

  it('validates rule codes before applying them to the draft', async () => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="edit-rule-capacity"]').trigger('click')
    const editor = wrapper.get('[data-testid="gateway-rule-form"]')
    await editor.get('[name="code"]').setValue('bad-code')
    await editor.trigger('submit')
    expect(wrapper.get('[data-testid="gateway-rule-error"]').text()).toContain('admin.publicGatewayErrors.validation.code')
    expect(wrapper.emitted('dirty-change')?.at(-1)).toEqual([false])

    const validCode = `a${'1_'.repeat(31)}z`
    await editor.get('[name="code"]').setValue(validCode)
    await editor.trigger('submit')
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await flushPromises()
    expect(mocks.updateSettings.mock.calls[0][0].rules[0].code).toBe(validCode)
  })

  it('validates the bad-request fallback code before save or preview', async () => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="bad_request_fallback"] [name="code"]').setValue('BadRequest')
    await wrapper.get('[data-testid="gateway-save"]').trigger('click')
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    expect(mocks.updateSettings).not.toHaveBeenCalled()
    expect(mocks.preview).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="gateway-preview-error"]').text()).toContain('admin.publicGatewayErrors.validation.code')
  })
})

describe('public gateway error preview', () => {
  it('accepts status 0 for unknown streams and explains pattern matching', async () => {
    const wrapper = await openRules()
    expect(wrapper.get('[data-testid="gateway-preview-status"]').attributes('min')).toBe('0')
    expect(wrapper.text()).toContain('admin.publicGatewayErrors.preview.unknownStatusHint')
    await wrapper.get('[data-testid="gateway-preview-status"]').setValue(0)
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    await flushPromises()
    expect(mocks.preview.mock.calls[0][0].status).toBe(0)
  })

  it.each(['http', 'sse', 'responses', 'gemini'])('sends unsaved rules and both fallbacks for %s', async (format) => {
    const wrapper = await openRules()
    await wrapper.get('[data-rule-id="capacity"] [role="switch"]').trigger('click')
    await wrapper.get('[data-testid="bad_request_fallback"] [name="message"]').setValue('Review the input.')
    await wrapper.get('[data-testid="fallback"] [name="code"]').setValue('retry_later')
    await wrapper.get('[data-testid="gateway-preview-status"]').setValue(429)
    await wrapper.get('[data-testid="gateway-preview-diagnostic"]').setValue('CAPACITY exhausted')
    await wrapper.get('[data-testid="gateway-preview-format"]').setValue(format)
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    await flushPromises()
    const expected = settings()
    expected.rules[0].enabled = false
    expected.bad_request_fallback.message = 'Review the input.'
    expected.fallback.code = 'retry_later'
    expect(mocks.preview.mock.calls).toEqual([[{
      settings: expected, status: 429, diagnostic: 'CAPACITY exhausted', format
    }]])
    expect(mocks.updateSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="gateway-preview-result"]').text()).toContain('Capacity')
    expect(JSON.parse(wrapper.get('pre').text())).toEqual({ error: { code: 'busy', message: 'Please retry later.' } })
    if (format === 'sse' || format === 'responses') {
      expect(wrapper.text()).toContain('admin.publicGatewayErrors.preview.streamHint')
    }
  })

  it('clears stale preview results and ignores a response for an older draft', async () => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.find('pre').exists()).toBe(true)
    await wrapper.get('[data-testid="gateway-preview-diagnostic"]').setValue('Updated diagnostic')
    expect(wrapper.find('pre').exists()).toBe(false)
    let finish!: (value: unknown) => void
    mocks.preview.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    await wrapper.get('[data-rule-id="capacity"] [role="switch"]').trigger('click')
    finish({ status: 503, code: 'old', message: 'old', payload: {} })
    await flushPromises()
    expect(wrapper.find('pre').exists()).toBe(false)
  })

  it('shows preview failures and validates the preview source status', async () => {
    const wrapper = await openRules()
    await wrapper.get('[data-testid="gateway-preview-status"]').setValue(200)
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    expect(mocks.preview).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="gateway-preview-error"]').text()).toContain('admin.publicGatewayErrors.validation.previewStatus')
    await wrapper.get('[data-testid="gateway-preview-status"]').setValue(502)
    mocks.preview.mockRejectedValueOnce({ message: 'Invalid RE2 expression' })
    await wrapper.get('[data-testid="gateway-preview-form"]').trigger('submit')
    await flushPromises()
    expect(wrapper.get('[data-testid="gateway-preview-error"]').text()).toContain('Invalid RE2 expression')
  })
})

describe('error handling tabs', () => {
  it('opens client mapping first, preserves its draft across tabs and confirms discarding on close', async () => {
    const wrapper = mount(ErrorPassthroughRulesModal, { props: { show: true }, global: { stubs } })
    await flushPromises()
    expect(wrapper.get('[data-testid="public-errors-tab"]').attributes('aria-selected')).toBe('true')
    await wrapper.get('[data-rule-id="capacity"] [role="switch"]').trigger('click')
    await wrapper.get('[data-testid="upstream-errors-tab"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="public-errors-tab"]').trigger('click')
    expect(mocks.getSettings).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-rule-id="capacity"] [role="switch"]').attributes('aria-checked')).toBe('false')
    await wrapper.get('[data-testid="error-rules-close"]').trigger('click')
    expect(wrapper.emitted('close')).toBeUndefined()
    await wrapper.get('[data-testid="confirm-discard"] button:last-child').trigger('click')
    expect(wrapper.emitted('close')).toBeUndefined()
    await wrapper.get('[data-testid="error-rules-close"]').trigger('click')
    await wrapper.get('[data-testid="confirm-discard"] button:first-child').trigger('click')
    expect(wrapper.emitted('close')).toHaveLength(1)
  })
})
