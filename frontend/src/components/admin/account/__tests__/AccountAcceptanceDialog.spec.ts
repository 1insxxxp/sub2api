import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import AccountAcceptanceDialog from '../AccountAcceptanceDialog.vue'

const api = vi.hoisted(() => ({ targets: vi.fn(), preview: vi.fn(), retestPreview: vi.fn(), start: vi.fn(), list: vi.fn(), get: vi.fn(), cancel: vi.fn(), exportReport: vi.fn() }))
vi.mock('@/api/admin/channelAcceptance', () => ({ channelAcceptanceApi: api }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => params && Object.keys(params).length ? `${key} ${JSON.stringify(params)}` : key }) }))
const auth = reactive({ isAdmin: true })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
const price = { input_per_million: 2, output_per_million: 10, cache_per_million: 0.2, source: 'model_price' }
const target = { account_id: 7, name: 'Gemini supplier', platform: 'gemini', models: [{ model: 'gemini-test', upstream_model: 'gemini-test-native', protocols: ['gemini', 'chat'], price }] }
const defaultExtras = ['long_context', 'image', 'audio', 'tools']
const preview = { id: 'preview-1', can_start: true, issues: [], expires_at: '2099-01-01T00:00:00Z', generation_calls: 17, count_calls: 3, estimated_cost_usd: 0.3, samples: [], rule_version: 'channel-acceptance-v3' }
const run = { id: 'run-1', status: 'running', total: 20, completed: 1, samples: [], created_at: '2026-11-18T00:00:00Z', rule_version: 'channel-acceptance-v3', spent_cost_usd: 0.01, reserved_cost_usd: 0.2, estimated_cost_usd: 0.3 }
function render() {
  return mount(AccountAcceptanceDialog, { props: { show: true, accountIds: [7] }, global: { stubs: { teleport: true, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><footer data-testid="dialog-footer"><slot name="footer"/></footer></div>' } } } })
}
async function selectModel(wrapper: ReturnType<typeof render>) {
  await flushPromises()
  await wrapper.get('[data-testid="model-7-gemini-test"]').setValue(true)
}
async function startDetection(wrapper: ReturnType<typeof render>) {
  await flushPromises()
  await wrapper.get('[data-testid="select-all-models"]').trigger('click')
  await wrapper.get('[data-testid="start"]').trigger('click')
  await flushPromises()
}
enableAutoUnmount(afterEach)
beforeEach(() => {
  vi.resetAllMocks()
  auth.isAdmin = true
  api.targets.mockResolvedValue([target])
  api.retestPreview.mockResolvedValue({ ...preview, id: "retest-1" })
  api.preview.mockResolvedValue(preview)
  api.start.mockResolvedValue(run)
  api.get.mockResolvedValue(run)
  api.cancel.mockResolvedValue({ ...run, status: 'cancelled', completed: run.total })
})
afterEach(() => {
  expect(api.list).not.toHaveBeenCalled()
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

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
    await wrapper.get('[data-testid="start"]').trigger('click')
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'kiro/opus' }], extras: defaultExtras })
  })

  it.each(['vertex', 'unknown'] as const)('sends the optional %s declaration only on its account', async declaration => {
    api.targets.mockResolvedValue([{ ...target, models: [...target.models, { ...target.models[0], model: 'alias' }] }, { ...target, account_id: 9 }])
    const wrapper = render()
    await wrapper.setProps({ accountIds: [7, 9] })
    await flushPromises()
    const source = wrapper.get('[data-testid="declared-source-7"]')
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    await source.get('button').trigger('click')
    const options = source.findAll('[role="option"]')
    expect(options).toHaveLength(10)
    const choice = options.find(option => option.text().includes(`declaredSources.${declaration}`))
    expect(choice).toBeDefined()
    await choice!.trigger('click')
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenLastCalledWith({ targets: [
      { account_id: 7, model: 'gemini-test', declared_source: declaration }, { account_id: 7, model: 'alias', declared_source: declaration },
      { account_id: 9, model: 'gemini-test' }
    ], extras: defaultExtras })
    expect(api.start).toHaveBeenCalledWith(preview.id)
  })

  it('summarizes findings per account/model and starts the scoped retest from one click', async () => {
    vi.useFakeTimers()
    api.targets.mockResolvedValue([target, { ...target, account_id: 9 }])
    const measured = { input: 12, output: 0, source: 'upstream' }
    const sample = { category: 'input', protocol: 'gemini', stream: false, status: 'passed',
      target: { account_id: 7, model: 'gemini-test', declared_source: 'vertex' },
      result: { completed: true, upstream_usage_final: true, status: 'passed', raw_usage: measured,
        source: { kind: 'unverified', channel: 'unknown' }, findings: [
          { code: 'usage_inconsistent', level: 'warning', message: 'Input totals disagree' },
          { code: 'source_mismatch', level: 'warning', message: 'Source differs from declaration' }
        ] } }
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [
      { ...sample, id: 'a' }, { ...sample, id: 'b', category: 'tools' },
      { ...sample, id: 'c', target: { account_id: 9, model: 'gemini-test' }, result: { ...sample.result, status: 'passed', findings: [] } }
    ] })
    const wrapper = render()
    await wrapper.setProps({ accountIds: [7, 9] })
    await startDetection(wrapper)
    const rows = wrapper.findAll('[data-testid="model-overview-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('vertex')
    expect(rows[0].text().match(/Input totals disagree/g)).toHaveLength(1)
    expect(rows[0].text()).toContain('Source differs from declaration')
    expect(rows[1].text()).not.toContain('Input totals disagree')
    const retestRun = { ...run, id: 'retest-run' }
    api.start.mockResolvedValueOnce(retestRun)
    api.get.mockResolvedValueOnce(retestRun)
    api.retestPreview.mockResolvedValue({ ...preview, id: 'scoped-preview', generation_calls: 2, count_calls: 1, samples: [{ ...sample, category: 'input' }, { ...sample, category: 'input' }, { ...sample, category: 'count' }] })
    await wrapper.get('[data-testid="retest-anomalies"]').trigger('click')
    await flushPromises()
    expect(api.retestPreview.mock.calls).toEqual([[run.id]])
    expect(api.preview).toHaveBeenCalledOnce()
    expect(api.start.mock.calls).toEqual([[preview.id], ['scoped-preview']])
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe(String(retestRun.completed))
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[retestRun.id]])
  })

  it.each([
    ['stale configuration', true, 7, 'anomaly', 'warning'],
    ['outside account scope', false, 10, 'anomaly', 'warning'],
    ['only informational findings', false, 7, 'passed', 'info']
  ] as const)('disables retest for %s', async (_name, stale, accountId, status, level) => {
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', stale_config: stale, samples: [{ id: 's', category: 'input', target: { model: 'gemini-test', account_id: accountId }, status, result: { status, findings: [{ level, code: 'test', message: 'Finding' }] } }] })
    const wrapper = render()
    await startDetection(wrapper)
    const retest = wrapper.find('[data-testid="retest-anomalies"]')
    if (level === 'info') expect(retest.exists()).toBe(false)
    else expect(retest.attributes('disabled')).toBeDefined()
    expect(api.retestPreview).not.toHaveBeenCalled()
  })

  it.each([
    ['channel-acceptance-v2', 'completed', 20, false],
    ['channel-acceptance-v3', 'completed', 19, false],
    ['channel-acceptance-v3', 'running', 20, false],
    ['channel-acceptance-v3', 'failed', 20, false],
    ['channel-acceptance-v3', 'budget_exhausted', 19, false],
    ['channel-acceptance-v3', 'cancelled', 19, false],
    ['channel-acceptance-v3', 'budget_exhausted', 20, true],
    ['channel-acceptance-v3', 'cancelled', 20, true]
  ] as const)('checks retest eligibility for %s / %s / %i completed', async (version, status, completed, eligible) => {
    api.start.mockResolvedValueOnce({ ...run, rule_version: version, status, completed, samples: [
      { id: 'flagged', category: 'input', target: { account_id: 7, model: 'gemini-test' }, result: { status: 'error', findings: [] } }
    ] })
    const wrapper = render()
    await startDetection(wrapper)
    const active = status === 'running' || (['budget_exhausted', 'cancelled'].includes(status) && completed < run.total)
    const retest = wrapper.find('[data-testid="retest-anomalies"]')
    expect(wrapper.find('[data-testid="model-overview-row"]').exists()).toBe(!active)
    expect(retest.exists()).toBe(!active)
    if (!active) expect(retest.attributes('disabled') === undefined).toBe(eligible)
    if (version === 'channel-acceptance-v2') expect(wrapper.text()).toContain('admin.accounts.acceptance.retestRuleChanged')
    expect(api.retestPreview).not.toHaveBeenCalled()
  })

  it.each([['missing target', [7]], ['additional target', [7, 9, 10]]] as const)('rejects an unexpected retest scope: %s', async (_case, ids) => {
    const sample = (id: number) => ({ id: String(id), category: 'cache', target: { account_id: id, model: 'gemini-test' }, result: { status: 'anomaly', findings: [] } })
    api.targets.mockResolvedValue([target, { ...target, account_id: 9 }])
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [sample(7), sample(9)] })
    api.retestPreview.mockResolvedValue({ ...preview, samples: ids.map(sample) })
    const wrapper = render()
    await wrapper.setProps({ accountIds: [7, 9] })
    await startDetection(wrapper)
    await wrapper.get('[data-testid="retest-anomalies"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('retestScopeMismatch')
    expect(api.start.mock.calls).toEqual([[preview.id]])
    expect(api.preview).toHaveBeenCalledOnce()
  })

  it('disables retest when the server detects a stale scope after the report was loaded', async () => {
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [
      { id: 's', category: 'cache', target: { model: 'gemini-test', account_id: 7 }, result: { status: 'anomaly', findings: [] } }
    ] })
    api.retestPreview.mockRejectedValue({ reason: 'ACCEPTANCE_STALE', message: 'Configuration changed' })
    const wrapper = render()
    await startDetection(wrapper)
    await wrapper.get('[data-testid="retest-anomalies"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Configuration changed')
    expect(wrapper.get('[data-testid="retest-anomalies"]').attributes('disabled')).toBeDefined()
    expect(api.start.mock.calls).toEqual([[preview.id]])
  })

  it('discards an in-flight retest preview when the dialog closes', async () => {
    const flagged = { id: 'flagged', status: 'anomaly', result: { status: 'anomaly' }, category: 'input', target: { account_id: 7, model: 'gemini-test' } }
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [flagged] })
    let resolve!: (value: unknown) => void
    api.retestPreview.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    const wrapper = render()
    await startDetection(wrapper)
    await wrapper.get('[data-testid="retest-anomalies"]').trigger('click')
    expect(api.retestPreview).toHaveBeenCalledWith(run.id)
    await wrapper.setProps({ show: false })
    resolve({ ...preview, id: 'obsolete', samples: [flagged] })
    await flushPromises()
    expect(api.start.mock.calls).toEqual([[preview.id]])
  })

  it('waits for an eligible preview and starts it exactly once, even with repeated clicks', async () => {
    let resolvePreview!: (value: typeof preview) => void
    api.preview.mockImplementationOnce(() => new Promise(resolve => { resolvePreview = resolve }))
    const wrapper = render()
    await selectModel(wrapper)
    const start = wrapper.get('[data-testid="start"]')
    await start.trigger('click')
    await start.trigger('click')
    expect(api.preview).toHaveBeenCalledOnce()
    expect(api.start).not.toHaveBeenCalled()
    resolvePreview({ ...preview, id: 'approved-preview' })
    await flushPromises()
    expect(api.start).toHaveBeenCalledOnce()
    expect(api.start).toHaveBeenCalledWith('approved-preview')
  })

  it('uses automatic costs and account-only scope without supplier price or group controls', async () => {
    api.targets.mockResolvedValue([{ ...target, groups: [{ id: 10, name: 'Gemini group' }] }])
    const wrapper = render()
    await selectModel(wrapper)
    expect(wrapper.find('[data-testid="group-7"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="manual-price-7-gemini-test"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('gemini-test-native')
    await wrapper.get('button[aria-expanded]').trigger('click')
    expect(wrapper.text()).toContain('gemini-test-native')
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }], extras: defaultExtras })
    expect(api.start).toHaveBeenCalledWith(preview.id)
  })

  it('explains expired previews beside the action', async () => {
    const wrapper = render()
    await selectModel(wrapper)
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    api.preview.mockResolvedValue({ ...preview, expires_at: '2020-01-01T00:00:00Z' })
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(footer.text()).toContain('admin.accounts.acceptance.previewExpired')
    expect(footer.get('[data-testid="start"]').attributes('disabled')).toBeUndefined()
    expect(api.start).not.toHaveBeenCalled()
  })

  it('explains the server target limit before a preview request is sent', async () => {
    api.targets.mockResolvedValue([{ ...target, models: Array.from({ length: 26 }, (_, i) => ({ ...target.models[0], model: `gemini-${i}` })) }])
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    expect(footer.text()).toContain('admin.accounts.acceptance.tooManyModels')
    expect(footer.get('[data-testid="start"]').attributes('disabled')).toBeDefined()
    expect(api.preview).not.toHaveBeenCalled()
  })

  it('surfaces a preview failure and starts only the successful retry', async () => {
    api.preview.mockRejectedValueOnce(new Error('Preview unavailable'))
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toBe('Preview unavailable')
    expect(api.start).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledTimes(2)
    expect(api.start).toHaveBeenCalledOnce()
    expect(api.start).toHaveBeenCalledWith(preview.id)
  })

  it('requires a selected model and starts the default four extras with one click', async () => {
    const wrapper = render()
    await flushPromises()
    const start = wrapper.get('[data-testid="start"]')
    expect(start.attributes('disabled')).toBeDefined()
    await start.trigger('click')
    expect(api.preview).not.toHaveBeenCalled()
    expect(api.start).not.toHaveBeenCalled()
    await selectModel(wrapper)
    await start.trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }], extras: defaultExtras })
    expect(api.start).toHaveBeenCalledOnce()
    expect(api.start).toHaveBeenCalledWith(preview.id)
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe(String(run.completed))
    expect(api.get).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="history-tab"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="compare"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain(run.id)
    expect(wrapper.text()).not.toContain(run.rule_version)
  })

  it('identifies missing cost estimates without silently making them free or requesting manual prices', async () => {
    api.targets.mockResolvedValue([{ ...target, models: [{ ...target.models[0], price: null }] }])
    const wrapper = render()
    await selectModel(wrapper)
    expect(wrapper.get('[data-testid="start"]').attributes('disabled')).toBeDefined()
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
    expect(wrapper.get('[data-testid="start"]').attributes('disabled')).toBeUndefined()
  })

  it('pauses polling on close and resumes the current run when the same accounts reopen', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await startDetection(wrapper)
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[run.id]])
    await wrapper.setProps({ show: false })
    await vi.advanceTimersByTimeAsync(9000)
    expect(api.get).toHaveBeenCalledOnce()
    expect(api.cancel).not.toHaveBeenCalled()
    api.get.mockResolvedValue({ ...run, completed: 5 })
    await wrapper.setProps({ show: true })
    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls.length).toBeGreaterThan(1)
    for (const args of api.get.mock.calls) expect(args).toEqual([run.id])
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe('5')
    expect(api.start).toHaveBeenCalledOnce()
  })

  it('ignores the previous account’s pending poll after starting a run for another account', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await startDetection(wrapper)
    let finish!: (value: typeof run) => void
    api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[run.id]])

    const nextRun = { ...run, id: 'run-for-9', completed: 3 }
    api.targets.mockResolvedValueOnce([{ ...target, account_id: 9 }])
    api.start.mockResolvedValueOnce(nextRun)
    await wrapper.setProps({ accountIds: [9] })
    await startDetection(wrapper)
    expect(api.preview).toHaveBeenLastCalledWith({ targets: [{ account_id: 9, model: 'gemini-test' }], extras: defaultExtras })
    finish({ ...run, status: 'completed', completed: run.total })
    await flushPromises()
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe('3')
    api.get.mockResolvedValueOnce({ ...nextRun, completed: 4 })
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[run.id], [nextRun.id]])
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe('4')
  })

  it('never requests data for a sub administrator', async () => {
    auth.isAdmin = false
    const wrapper = render()
    await flushPromises()
    expect(api.targets).not.toHaveBeenCalled()
    expect(api.get).not.toHaveBeenCalled()
    expect(wrapper.find('[data-testid="start"]').exists()).toBe(false)
  })

  it('blocks a server-rejected preview and shows its issues', async () => {
    api.preview.mockResolvedValue({ ...preview, can_start: false, issues: ['account configuration changed'] })
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('account configuration changed')
    expect(api.start).not.toHaveBeenCalled()
  })

  it('renders nullable usage differently from zero and exports the server report', async () => {
    const usage = { input: 12, output: 0, thoughts: null, cache_read: null, total: 12, source: 'upstream' }
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', stale_config: true, samples: [{ id: 's1', ordinal: 1, category: 'protocol', protocol: 'gemini', stream: true, status: 'completed', target: { account_id: 7, account_name: target.name, model: 'gemini-test' }, result: { status: 'anomaly', raw_usage: usage, client_usage: usage, billing_usage: usage, source: { kind: 'unverified', host: 'relay.example', channel: 'unknown' }, findings: [{ code: 'missing', level: 'warning', message: 'usage missing' }], cost_usd: 0.01 } }] })
    api.exportReport.mockResolvedValue(new Blob(['{}'], { type: 'application/json' }))
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:report'), revokeObjectURL: vi.fn() }))
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const wrapper = render()
    await startDetection(wrapper)
    expect(wrapper.get('.acceptance-console-body').element.closest('details')).toBeNull()
    expect(wrapper.text()).toContain('usage missing')
    expect(wrapper.text()).toContain('admin.accounts.acceptance.stale')
    expect(wrapper.get('[data-testid="raw-output-s1"]').text()).toBe('0')
    expect(wrapper.get('[data-testid="raw-thoughts-s1"]').text()).toBe('—')
    await wrapper.get('[data-testid="export-run-1"]').trigger('click')
    await flushPromises()
    expect(api.exportReport).toHaveBeenCalledWith('run-1')
  })

  it('returns to configuration from the footer after polling reaches a terminal state', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await startDetection(wrapper)
    const footer = wrapper.get('[data-testid="dialog-footer"]')
    expect(footer.find('[data-testid="new-detection"]').exists()).toBe(false)
    api.get.mockResolvedValueOnce({ ...run, status: 'completed', completed: run.total })
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[run.id]])
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe(String(run.total))
    await footer.get('[data-testid="new-detection"]').trigger('click')
    expect(wrapper.find('[data-testid="acceptance-console"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-7-gemini-test"]').exists()).toBe(true)
    expect(footer.get('[data-testid="start"]').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(9000)
    expect(api.get).toHaveBeenCalledOnce()
    expect(api.start).toHaveBeenCalledOnce()
  })

  it('discards an in-flight preview after the selected models change', async () => {
    api.targets.mockResolvedValue([{ ...target, models: [...target.models, { ...target.models[0], model: 'other' }] }])
    let resolvePreview!: (value: typeof preview) => void
    api.preview.mockImplementationOnce(() => new Promise(resolve => { resolvePreview = resolve }))
    const wrapper = render()
    await selectModel(wrapper)
    await wrapper.get('[data-testid="start"]').trigger('click')
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }], extras: defaultExtras })
    await wrapper.get('[data-testid="model-7-other"]').setValue(true)
    await wrapper.get('[data-testid="model-7-gemini-test"]').setValue(false)
    resolvePreview({ ...preview, id: 'obsolete-preview' })
    await flushPromises()
    expect(api.start).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenLastCalledWith({ targets: [{ account_id: 7, model: 'other' }], extras: defaultExtras })
    expect(api.start).toHaveBeenCalledOnce()
    expect(api.start).toHaveBeenCalledWith(preview.id)
  })

  it('excludes interrupted or unconfirmed upstream usage tails from both cache ratios', async () => {
    const cacheSample = (id: string, input: number | null, cache_read: number | null, source = 'upstream', completed = true, final: boolean | null = true) => ({
      id, ordinal: 1, category: 'cache', protocol: 'gemini', stream: true, status: 'passed',
      target: { account_id: 7, account_name: target.name, model: 'gemini-test' },
      result: { status: 'passed', source: { kind: 'unverified' }, effective_parameters: { cache_min_input_tokens: 100 }, completed, ...(final == null ? {} : { upstream_usage_final: final }), raw_usage: { input, output: 3, thoughts: 0, cache_read, total: input, source }, findings: [] }
    })
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [cacheSample('cold', 100, 0), cacheSample('warm', 100, 80), cacheSample('missing', 100, null), cacheSample('estimate', 100, 90, 'estimated'), cacheSample('partial', 100, 90, 'upstream', false), cacheSample('unknown', 100, 90, 'unknown'), cacheSample('interrupted-tail', 100, 99, 'upstream', true, false), cacheSample('legacy-no-final-flag', 100, 99, 'upstream', true, null)] })
    const wrapper = render()
    await startDetection(wrapper)
    expect(wrapper.text()).toContain('40.0%')
    expect(wrapper.text()).toContain('1/2')
  })

  it('does not restart polling if unmounted during a refresh', async () => {
    vi.useFakeTimers()
    const wrapper = render()
    await startDetection(wrapper)
    let finish!: (value: unknown) => void
    api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[run.id]])
    wrapper.unmount()
    finish(run)
    await flushPromises()
    await vi.advanceTimersByTimeAsync(9000)
    expect(api.get).toHaveBeenCalledOnce()
    expect(vi.getTimerCount()).toBe(0)
  })

  it.each([
    { can_start: false, issues: ['Retest refused'] },
    { expires_at: '2020-01-01T00:00:00Z' }
  ])('refuses an ineligible automatic retest start with %o', async (restriction) => {
    const flagged = { id: 's', category: 'input', target: { account_id: 7, model: 'gemini-test' }, result: { status: 'anomaly', findings: [] } }
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [flagged] })
    api.retestPreview.mockResolvedValue({ ...preview, ...restriction, samples: [flagged] })
    const wrapper = render()
    await startDetection(wrapper)
    await wrapper.get('[data-testid="retest-anomalies"]').trigger('click')
    await flushPromises()
    expect(api.retestPreview).toHaveBeenCalledWith(run.id)
    expect(api.preview).toHaveBeenCalledOnce()
    expect(api.start.mock.calls).toEqual([[preview.id]])
  })

  it('clears the selection and submits every eligible account/model exactly once', async () => {
    api.targets.mockResolvedValue([target, { ...target, account_id: 9 }])
    const wrapper = render()
    await wrapper.setProps({ accountIds: [7, 9] })
    await selectModel(wrapper)
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    for (const id of [7, 9]) expect((wrapper.get(`[data-testid="model-${id}-gemini-test"]`).element as HTMLInputElement).checked).toBe(true)
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    for (const id of [7, 9]) expect((wrapper.get(`[data-testid="model-${id}-gemini-test"]`).element as HTMLInputElement).checked).toBe(false)
    expect(wrapper.get('[data-testid="start"]').attributes('disabled')).toBeDefined()
    expect(api.start).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="select-all-models"]').trigger('click')
    await wrapper.get('[data-testid="start"]').trigger('click')
    await flushPromises()
    expect(api.preview).toHaveBeenCalledWith({ targets: [{ account_id: 7, model: 'gemini-test' }, { account_id: 9, model: 'gemini-test' }], extras: defaultExtras })
    expect(api.start).toHaveBeenCalledWith(preview.id)
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
    const wrapper = render()
    await startDetection(wrapper)
    let finish!: (value: unknown) => void
    api.get.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(3000)
    await wrapper.get('[data-testid="cancel-run-1"]').trigger('click')
    await flushPromises()
    expect(api.cancel.mock.calls).toEqual([[run.id]])
    finish(run)
    await flushPromises()
    expect(wrapper.find('[data-testid="cancel-run-1"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('cancelled')
  })

  it('does not assess cache misses when the minimum is unknown or the prompt is too short', async () => {
    const cacheSample = (id: string, cached: number, minimum?: number) => ({
      id, category: 'cache', protocol: 'gemini', stream: true, status: 'passed', target: { account_id: 7, model: 'gemini-test' },
      result: { completed: true, upstream_usage_final: true, source: { kind: 'unverified' }, effective_parameters: { cache_min_input_tokens: minimum }, raw_usage: { input: 100, cache_read: cached, source: 'upstream_gemini_reported' }, findings: [] }
    })
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [cacheSample('unknown-minimum', 0), cacheSample('too-short', 0, 4096), cacheSample('positive-hit', 80)] })
    const wrapper = render()
    await startDetection(wrapper)
    expect(wrapper.text()).toContain('80.0%')
    expect(wrapper.text()).toContain('1/1')
  })

  it('labels aggregate completion output when the thought breakdown is unknown and prefers analyzed status', async () => {
    const aggregate = { input: 100, output: 42, output_includes_thoughts: true, thoughts: null, cache_read: 0, total: 142, source: 'upstream_chat_reported' }
    api.start.mockResolvedValueOnce({ ...run, completed: run.total, status: 'completed', samples: [{ id: 'aggregate', category: 'protocol', protocol: 'chat', stream: true, status: 'passed', target: { account_id: 7, model: 'gemini-test' }, result: { status: 'anomaly', source: { kind: 'unverified' }, raw_usage: aggregate, client_usage: aggregate, billing_usage: aggregate, findings: [] } }] })
    const wrapper = render()
    await startDetection(wrapper)
    expect(wrapper.get('[data-testid="raw-output-aggregate"]').text()).toBe('42')
    expect(wrapper.get('[data-testid="raw-thoughts-aggregate"]').text()).toBe('—')
    expect(wrapper.text()).toContain('admin.accounts.acceptance.aggregateOutputHint')
    expect(wrapper.get('.acceptance-console-row').text()).toContain('anomaly')
    expect(wrapper.get('.acceptance-console-row').text()).not.toContain('passed')
  })

  it('updates the visible terminal when the next sample starts', async () => {
    vi.useFakeTimers()
    const sample = (ordinal: number, status: string) => ({
      id: String(ordinal), ordinal, status, category: 'protocol', protocol: 'chat',
      stream: false, target: { account_id: 7, model: 'gemini-test' }
    })
    const initial = { ...run, completed: 0, total: 3, samples: [sample(0, 'running'), sample(1, 'pending'), sample(2, 'pending')] }
    api.start.mockResolvedValueOnce(initial)
    const wrapper = render()
    await startDetection(wrapper)
    const terminal = wrapper.get('[data-testid="acceptance-console"]')
    expect(terminal.findAll('.console-line-number').map(row => row.text())).toEqual(['1'])
    const body = terminal.get('.acceptance-console-body')
    const el = body.element as HTMLElement
    Object.defineProperties(el, { scrollHeight: { value: 900 }, clientHeight: { value: 200 } })
    el.scrollTop = 200
    await body.trigger('scroll')
    api.get.mockResolvedValue({ ...initial, completed: 1, samples: [sample(0, 'passed'), sample(1, 'pending'), sample(2, 'running')] })
    await vi.advanceTimersByTimeAsync(3000)
    expect(terminal.get('progress').attributes('value')).toBe('1')
    expect(terminal.findAll('.console-line-number').map(row => row.text())).toEqual(['1', '3'])
    expect(el.scrollTop).toBe(200)
    el.scrollTop = 700
    await body.trigger('scroll')
    api.get.mockResolvedValue({ ...initial, completed: 2, samples: [sample(0, 'passed'), sample(1, 'running'), sample(2, 'passed')] })
    await vi.advanceTimersByTimeAsync(3000)
    expect(terminal.findAll('.console-line-number').map(row => row.text())).toEqual(['1', '3', '2'])
    expect(el.scrollTop).toBe(900)
  })

  it('keeps budget-exhausted in-flight work cancellable and polling until terminal samples settle', async () => {
    vi.useFakeTimers()
    const exhausted = { ...run, status: 'budget_exhausted', completed: 19, total: 20, samples: [{ id: 'live', category: 'protocol', protocol: 'gemini', stream: true, status: 'running', target: { account_id: 7, model: 'gemini-test' } }] }
    api.start.mockResolvedValueOnce(exhausted)
    api.get.mockResolvedValueOnce(exhausted)
    const wrapper = render()
    await startDetection(wrapper)
    expect(wrapper.find('[data-testid="cancel-run-1"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="acceptance-console"] [data-state="running"]').text()).toContain('running')
    await vi.advanceTimersByTimeAsync(3000)
    expect(api.get.mock.calls).toEqual([[run.id]])
    api.cancel.mockResolvedValue({ ...exhausted, status: 'cancelled' })
    api.get.mockResolvedValue({ ...exhausted, status: 'cancelled', completed: 20, samples: [{ ...exhausted.samples[0], status: 'cancelled' }] })
    await wrapper.get('[data-testid="cancel-run-1"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="cancel-run-1"]').exists()).toBe(false)
    await vi.advanceTimersByTimeAsync(3000)
    expect(wrapper.get('[data-testid="acceptance-console"] progress').attributes('value')).toBe('20')
    await vi.advanceTimersByTimeAsync(6000)
    expect(api.get.mock.calls).toEqual([[run.id], [run.id]])
  })
})
