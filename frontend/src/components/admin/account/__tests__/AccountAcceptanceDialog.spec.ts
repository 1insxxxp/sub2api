import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import AccountAcceptanceDialog from '../AccountAcceptanceDialog.vue'

const api = vi.hoisted(() => ({ targets: vi.fn(), preview: vi.fn(), start: vi.fn(), list: vi.fn(), get: vi.fn(), cancel: vi.fn(), exportReport: vi.fn() }))
vi.mock('@/api/admin/channelAcceptance', () => ({ channelAcceptanceApi: api }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params && Object.keys(params).length ? `${key} ${JSON.stringify(params)}` : key }) }))
const auth = reactive({ isAdmin: true })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
const price = { input_per_million: 2, output_per_million: 10, cache_per_million: 0.2, source: 'model_price' }
const target = { account_id: 7, name: 'Gemini supplier', platform: 'gemini', models: [{ model: 'gemini-test', upstream_model: 'gemini-test-native', protocols: ['gemini', 'chat'], price }] }
const preview = { id: 'preview-1', can_start: true, issues: [], expires_at: '2099-01-01T00:00:00Z', generation_calls: 17, count_calls: 3, estimated_cost_usd: 0.3, samples: [], rule_version: 'v1' }
const run = { id: 'run-1', status: 'running', total: 20, completed: 1, samples: [], created_at: '2026-11-18T00:00:00Z', rule_version: 'v1', spent_cost_usd: 0.01, reserved_cost_usd: 0.2, estimated_cost_usd: 0.3 }
function render() {
  return mount(AccountAcceptanceDialog, { props: { show: true, accountIds: [7] }, global: { stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><footer data-testid="dialog-footer"><slot name="footer"/></footer></div>' } } } })
}
async function selectModel(wrapper: ReturnType<typeof render>) {
  await flushPromises()
  await wrapper.get('[data-testid="model-7-gemini-test"]').setValue(true)
}
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  auth.isAdmin = true
  api.targets.mockResolvedValue([target])
  api.preview.mockResolvedValue(preview)
  api.list.mockResolvedValue([])
  api.start.mockResolvedValue(run)
  api.get.mockResolvedValue(run)
  api.cancel.mockResolvedValue({ ...run, status: 'cancelled' })
})
afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('AccountAcceptanceDialog', () => {
  it('selects all supported Claude aliases while showing the reason for unavailable models', async () => {
    const models = [
      { model: 'kiro/opus', upstream_model: 'claude-opus', protocols: ['messages', 'chat'], price },
      { model: 'image-only', upstream_model: 'image-only', protocols: [], unavailable_reason: '图片专用接口' },
    ]
    api.targets.mockResolvedValue([{ account_id: 7, name: 'Claude supplier', platform: 'anthropic', models }])
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('[data-testid="model-7-image-only"]').attributes('disabled')).toBeDefined()
    expect(wrapper.text()).toContain('图片专用接口')
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    await wrapper.get('[data-testid="preview"]').trigger('click')
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'kiro/opus' }], extras: [] })
  })

  it('keeps preview, reviewed costs and explicit start together in the dialog footer', async () => {
    const wrapper = render()
    await selectModel(wrapper)
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    expect(footer.find('[data-testid="preview"]').exists()).toBe(true)
    await footer.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(footer.text()).toContain('$0.300000')
    expect(footer.text()).toContain('17')
    expect(footer.text()).toContain('3')
    expect(api.start).not.toHaveBeenCalled()
    await footer.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.start).toHaveBeenCalledOnce()
    expect(footer.find('[data-testid="preview"]').exists()).toBe(false)
  })

  it('uses automatic costs and account-only scope without supplier price or group controls', async () => {
    api.targets.mockResolvedValue([{ ...target, groups: [{ id: 10, name: 'Gemini group' }] }])
    const wrapper = render()
    await selectModel(wrapper)
    expect(wrapper.find('[data-testid="group-7"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="manual-price-7-gemini-test"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.acceptance.scopeHint')
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }], extras: [] })
    expect(api.start).not.toHaveBeenCalled()
  })

  it('explains expired previews beside the action', async () => {
    const wrapper = render()
    await selectModel(wrapper)
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    api.preview.mockResolvedValue({ ...preview, expires_at: '2020-01-01T00:00:00Z' })
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(footer.text()).toContain('admin.accounts.acceptance.previewExpired')
    expect(footer.get('[data-testid="preview"]').attributes('disabled')).toBeUndefined()
    expect(footer.get('[data-testid="start"]').attributes('disabled')).toBeDefined()
    expect(api.start).not.toHaveBeenCalled()
  })

  it('explains the server target limit before a preview request is sent', async () => {
    api.targets.mockResolvedValue([{ ...target, models: Array.from({ length: 26 }, (_, i) => ({ ...target.models[0], model: `gemini-${i}` })) }])
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    expect(footer.text()).toContain('admin.accounts.acceptance.tooManyModels')
    expect(footer.get('[data-testid="preview"]').attributes('disabled')).toBeDefined()
    expect(api.preview).not.toHaveBeenCalled()
  })

  it('shows a failed preview next to the action and allows retry without starting a run', async () => {
    api.preview.mockRejectedValueOnce(new Error('Preview unavailable'))
    const wrapper = render()
    await selectModel(wrapper)
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    await footer.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(footer.get('[role="alert"]').text()).toBe('Preview unavailable')
    expect(footer.find('[data-testid="start"]').exists()).toBe(false)
    await footer.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(footer.find('[role="alert"]').exists()).toBe(false)
    expect(footer.get('[data-testid="start"]').attributes('disabled')).toBeUndefined()
    expect(api.start).not.toHaveBeenCalled()
  })

  it('requires selecting a model and previewing before explicit start', async () => {
    const wrapper = render()
    await flushPromises()
    expect(api.start).not.toHaveBeenCalled()
    expect(wrapper.get('[data-testid="preview"]').attributes('disabled')).toBeDefined()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }], extras: [] })
    expect(api.start).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('17')
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.start).toHaveBeenCalledWith('preview-1')
    expect(wrapper.text()).toContain('run-1')
  })

  it('selects and clears all optional checks and invalidates the preview', async () => {
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    const selectAll = wrapper.get('[data-testid="select-all-extras"]')
    await selectAll.trigger('click')
    expect(wrapper.find('[data-testid="start"]').exists()).toBe(false)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenLastCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }], extras: ['long_context', 'image', 'audio', 'tools'] })
    await selectAll.trigger('click')
    expect(wrapper.find('[data-testid="start"]').exists()).toBe(false)
    expect(wrapper.findAll('input[data-testid^="extra-"]:checked')).toHaveLength(0)
  })

  it('identifies missing cost estimates without silently making them free or requesting manual prices', async () => {
    api.targets.mockResolvedValue([{ ...target, models: [{ ...target.models[0], price: null }] }])
    const wrapper = render()
    await selectModel(wrapper)
    expect(wrapper.get('[data-testid="preview"]').attributes('disabled')).toBeDefined()
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    expect(footer.text()).toContain('admin.accounts.acceptance.missingEstimate')
    expect(footer.text()).toContain('gemini-test')
    expect(wrapper.find('[data-testid="input-price-7-gemini-test"]').exists()).toBe(false)
    expect(api.preview).not.toHaveBeenCalled()
    expect(api.start).not.toHaveBeenCalled()
  })

  it('accepts explicit zero catalog prices without requiring a manual override', async () => {
    api.targets.mockResolvedValue([{ ...target, models: [{ ...target.models[0], price: { input_per_million: 0, output_per_million: 0, cache_per_million: 0, source: 'catalog' } }] }])
    const wrapper = render()
    await selectModel(wrapper)
    expect(wrapper.get('[data-testid="preview"]').attributes('disabled')).toBeUndefined()
  })

  it('restores active reports from history after close without cancelling the server task', async () => {
    vi.useFakeTimers()
    api.list.mockResolvedValue([run])
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    await wrapper.setProps({ show: false })
    const requests = api.get.mock.calls.length
    await vi.advanceTimersByTimeAsync(6000)
    expect(api.get).toHaveBeenCalledTimes(requests)
    expect(api.cancel).not.toHaveBeenCalled()
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(api.list).toHaveBeenCalledTimes(2)
    await wrapper.get('[data-testid="cancel-run-1"]').trigger('click')
    await flushPromises()
    expect(api.cancel).toHaveBeenCalledWith('run-1')
  })

  it('never requests data for a sub administrator', async () => {
    auth.isAdmin = false
    const wrapper = render()
    await flushPromises()
    expect(api.targets).not.toHaveBeenCalled()
    expect(api.list).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="preview"]').exists()).toBe(false)
  })

  it('blocks a server-rejected or expired preview and shows its issues', async () => {
    api.preview.mockResolvedValue({ ...preview, can_start: false, issues: ['account configuration changed'] })
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('account configuration changed')
    expect(wrapper.get('[data-testid="start"]').attributes('disabled')).toBeDefined()
  })

  it('renders nullable usage differently from zero and exports the server report', async () => {
    const usage = { input: 12, output: 0, thoughts: null, cache_read: null, total: 12, source: 'upstream' }
    api.list.mockResolvedValue([run])
    api.get.mockResolvedValue({ ...run, status: 'completed', stale_config: true, samples: [{ id: 's1', ordinal: 1, category: 'protocol', protocol: 'gemini', stream: true, status: 'completed', target: { account_id: 7, account_name: target.name, model: 'gemini-test' }, result: { status: 'anomaly', raw_usage: usage, client_usage: usage, billing_usage: usage, source: { kind: 'unverified', host: 'relay.example', channel: 'unknown' }, findings: [{ code: 'missing', level: 'warning', message: 'usage missing' }], cost_usd: 0.01 } }] })
    api.exportReport.mockResolvedValue(new Blob(['{}'], { type: 'application/json' }))
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:report'), revokeObjectURL: vi.fn() }))
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('usage missing')
    expect(wrapper.text()).toContain('admin.accounts.acceptance.stale')
    expect(wrapper.get('[data-testid="raw-output-s1"]').text()).toBe('0')
    expect(wrapper.get('[data-testid="raw-thoughts-s1"]').text()).toBe('—')
    await wrapper.get('[data-testid="export-run-1"]').trigger('click')
    await flushPromises()
    expect(api.exportReport).toHaveBeenCalledWith('run-1')
    vi.restoreAllMocks()
  })
  it('compares only matched model, category and protocol samples', async () => {
    const sample = { id: 'left', ordinal: 1, category: 'protocol', protocol: 'gemini', stream: true, status: 'completed', target: { account_id: 7, account_name: target.name, model: 'gemini-test' } }
    api.list.mockResolvedValue([run, { ...run, id: 'run-2', status: 'completed' }])
    api.get.mockImplementation(async (id: string) => ({ ...run, id, status: 'completed', samples: id === 'run-1' ? [sample] : [{ ...sample, id: 'right' }, { ...sample, id: 'different', target: { ...sample.target, model: 'other-model' } }] }))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="compare-run-1"]').setValue(true)
    await wrapper.get('[data-testid="compare-run-2"]').setValue(true)
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="comparison-row"]')).toHaveLength(1)
    expect(wrapper.get('[data-testid="comparison"]').text()).toContain('gemini-test')
    expect(wrapper.get('[data-testid="comparison"]').text()).not.toContain('other-model')
  })

  it('ignores an obsolete preview response after selection changes', async () => {
    let resolvePreview!: (value: unknown) => void
    api.preview.mockImplementation(() => new Promise(resolve => { resolvePreview = resolve }))
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await wrapper.get('[data-testid="extra-tools"]').setValue(true)
    resolvePreview(preview)
    await flushPromises()
    expect(wrapper.find('[data-testid="start"]').exists()).toBe(false)
  })

  it('counts only completed upstream cache samples and excludes estimated or absent fields', async () => {
    const cacheSample = (id: string, input: number | null, cache_read: number | null, source = 'upstream', completed = true) => ({
      id, ordinal: 1, category: 'cache', protocol: 'gemini', stream: true, status: 'passed',
      target: { account_id: 7, account_name: target.name, model: 'gemini-test' },
      result: { status: 'passed', source: { kind: 'unverified' }, effective_parameters: { cache_min_input_tokens: 100 }, completed, raw_usage: { input, output: 3, thoughts: 0, cache_read, total: input, source }, findings: [] }
    })
    api.list.mockResolvedValue([run])
    api.get.mockResolvedValue({ ...run, status: 'completed', samples: [cacheSample('cold', 100, 0), cacheSample('warm', 100, 80), cacheSample('missing', 100, null), cacheSample('estimate', 100, 90, 'estimated'), cacheSample('partial', 100, 90, 'upstream', false), cacheSample('unknown', 100, 90, 'unknown')] })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('40.0%')
    expect(wrapper.text()).toContain('1/2')
  })

  it('does not restart polling if unmounted during a refresh', async () => {
    vi.useFakeTimers()
    api.list.mockResolvedValue([run])
    const wrapper = render()
    await flushPromises()
    let finish!: (value: unknown) => void
    api.list.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(3000)
    wrapper.unmount()
    finish([run])
    await flushPromises()
    const calls = api.list.mock.calls.length
    await vi.advanceTimersByTimeAsync(9000)
    expect(api.list).toHaveBeenCalledTimes(calls)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('refuses an expired preview without creating a run', async () => {
    api.preview.mockResolvedValue({ ...preview, expires_at: '2020-01-01T00:00:00Z' })
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="start"]').attributes('disabled')).toBeDefined()
    expect(api.start).not.toHaveBeenCalled()
  })

  it('selects every account/model exactly once and clears the selection and preview', async () => {
    api.targets.mockResolvedValue([target, { ...target, account_id: 9 }])
    const wrapper = render()
    await wrapper.setProps({ accountIds: [7, 9] })
    await selectModel(wrapper)
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    for (const id of [7, 9]) expect((wrapper.get(`[data-testid="model-${id}-gemini-test"]`).element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-testid="preview"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }, { account_id: 9, model: 'gemini-test' }], extras: [] })
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    expect(wrapper.find('[data-testid="start"]').exists()).toBe(false)
    for (const id of [7, 9]) expect((wrapper.get(`[data-testid="model-${id}-gemini-test"]`).element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.get('[data-testid="preview"]').attributes('disabled')).toBeDefined()
    expect(api.start).not.toHaveBeenCalled()
  })

  it('selects and clears one account without changing other accounts', async () => {
    api.targets.mockResolvedValue([target, { ...target, account_id: 9, models: [target.models[0], { ...target.models[0], model: 'gemini-other' }] }])
    const wrapper = render()
    await wrapper.setProps({ accountIds: [7, 9] })
    await selectModel(wrapper)
    await wrapper.get('[data-testid="select-account-models-9"]').trigger('click')
    expect((wrapper.get('[data-testid="model-7-gemini-test"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-testid="model-9-gemini-test"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-testid="model-9-gemini-other"]').element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-testid="select-account-models-9"]').trigger('click')
    expect((wrapper.get('[data-testid="model-7-gemini-test"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-testid="model-9-gemini-test"]').element as HTMLInputElement).checked).toBe(false)
    expect((wrapper.get('[data-testid="model-9-gemini-other"]').element as HTMLInputElement).checked).toBe(false)
  })

  it('does not overwrite a cancellation with an older polling response', async () => {
    vi.useFakeTimers()
    api.list.mockResolvedValue([run])
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    let finish!: (value: unknown) => void
    api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(3000)
    await wrapper.get('[data-testid="cancel-run-1"]').trigger('click')
    await flushPromises()
    finish(run)
    await flushPromises()
    expect(wrapper.find('[data-testid="cancel-run-1"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('cancelled')
  })

  it('does not assess cache misses when the minimum is unknown or the prompt is too short', async () => {
    const cacheSample = (id: string, cached: number, minimum?: number) => ({
      id, category: 'cache', protocol: 'gemini', stream: true, status: 'passed', target: { account_id: 7, model: 'gemini-test' },
      result: { completed: true, source: { kind: 'unverified' }, effective_parameters: { cache_min_input_tokens: minimum }, raw_usage: { input: 100, cache_read: cached, source: 'upstream_gemini_reported' }, findings: [] }
    })
    api.list.mockResolvedValue([run])
    api.get.mockResolvedValue({ ...run, status: 'completed', samples: [cacheSample('unknown-minimum', 0), cacheSample('too-short', 0, 4096), cacheSample('positive-hit', 80)] })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('80.0%')
    expect(wrapper.text()).toContain('1/1')
  })

  it('labels aggregate completion output when the thought breakdown is unknown and prefers analyzed status', async () => {
    const aggregate = { input: 100, output: 42, output_includes_thoughts: true, thoughts: null, cache_read: 0, total: 142, source: 'upstream_chat_reported' }
    api.list.mockResolvedValue([run])
    api.get.mockResolvedValue({ ...run, status: 'completed', samples: [{ id: 'aggregate', category: 'protocol', protocol: 'chat', stream: true, status: 'passed', target: { account_id: 7, model: 'gemini-test' }, result: { status: 'anomaly', source: { kind: 'unverified' }, raw_usage: aggregate, client_usage: aggregate, billing_usage: aggregate, findings: [] } }] })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="raw-output-aggregate"]').text()).toBe('42')
    expect(wrapper.get('[data-testid="raw-thoughts-aggregate"]').text()).toBe('—')
    expect(wrapper.text()).toContain('admin.accounts.acceptance.aggregateOutputHint')
    expect(wrapper.find('details > summary').text()).toContain('anomaly')
    expect(wrapper.find('details > summary').text()).not.toContain('passed')
  })

  it('keeps budget-exhausted in-flight work cancellable and polling until terminal samples settle', async () => {
    vi.useFakeTimers()
    const exhausted = { ...run, status: 'budget_exhausted', completed: 19, total: 20, samples: [{ id: 'live', category: 'protocol', protocol: 'gemini', stream: true, status: 'running', target: { account_id: 7, model: 'gemini-test' } }] }
    api.list.mockResolvedValue([{ ...exhausted, samples: undefined }])
    api.get.mockResolvedValue(exhausted)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="report-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="cancel-run-1"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="acceptance-console"] [data-state="running"]').text()).toContain('running')
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.list).toHaveBeenCalledTimes(2)
    api.cancel.mockResolvedValue({ ...exhausted, status: 'cancelled' })
    api.list.mockResolvedValue([{ ...exhausted, status: 'cancelled', completed: 20, samples: undefined }])
    api.get.mockResolvedValue({ ...exhausted, status: 'cancelled', completed: 20, samples: [{ ...exhausted.samples[0], status: 'cancelled' }] })
    await wrapper.get('[data-testid="cancel-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="cancel-run-1"]').exists()).toBe(false)
    await vi.advanceTimersByTimeAsync(3000)
    const polls = api.list.mock.calls.length
    await vi.advanceTimersByTimeAsync(6000)
    expect(api.list).toHaveBeenCalledTimes(polls)
  })

  it('pairs different channels by mapped model and sample shape while labeling their identity and config', async () => {
    const sample = { id: 'left', ordinal: 1, category: 'protocol', protocol: 'gemini', stream: true, input_size: 100, status: 'passed', target: { account_id: 7, account_name: 'Supplier A', model: 'a/gemini-test', upstream_model: 'gemini-test', group_id: 10, fingerprint: 'config-a' } }
    api.list.mockResolvedValue([run, { ...run, id: 'run-2', status: 'completed' }])
    api.get.mockImplementation(async (id: string) => ({ ...run, id, rule_version: id === 'run-1' ? 'v1' : 'v2', status: 'completed', samples: id === 'run-1' ? [sample] : [{ ...sample, id: 'right', target: { ...sample.target, account_id: 9, account_name: 'Supplier B', model: 'b/gemini-test', group_id: 20, fingerprint: 'config-b' } }, { ...sample, id: 'wrong-protocol', protocol: 'chat' }, { ...sample, id: 'wrong-size', input_size: 4096 }] }))
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="history-tab"]').trigger('click')
    await wrapper.get('[data-testid="compare-run-1"]').setValue(true)
    await wrapper.get('[data-testid="compare-run-2"]').setValue(true)
    await wrapper.get('[data-testid="compare"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="comparison-row"]')).toHaveLength(1)
    const row = wrapper.get('[data-testid="comparison-row"]').text()
    for (const expected of ['Supplier A', 'Supplier B', '#7', '#9', 'config-a', 'config-b', 'a/gemini-test', 'b/gemini-test']) expect(row).toContain(expected)
    expect(wrapper.get('[data-testid="comparison"]').text()).toContain('v2')
  })

})
