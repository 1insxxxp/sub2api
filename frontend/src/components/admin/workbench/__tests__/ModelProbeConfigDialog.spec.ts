import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import ModelProbeConfigDialog from '../ModelProbeConfigDialog.vue'

const { listConfigGroups, saveConfigGroups, state } = vi.hoisted(() => ({
  listConfigGroups: vi.fn(), saveConfigGroups: vi.fn(),
  state: { id: 1, role: 'admin', token: 'session-one' },
}))
vi.mock('@/api/admin/modelProbes', () => ({ listConfigGroups, saveConfigGroups }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => { const s = reactive(state); return { get user() { return { id: s.id, role: s.role } }, get token() { return s.token } } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))
const views: VueWrapper[] = []
const DialogStub = { props: ['show', 'closeOnEscape', 'showCloseButton'], emits: ['close'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' }
function create() {
  const view = mount(ModelProbeConfigDialog, { global: { stubs: { BaseDialog: DialogStub, Icon: true } } })
  views.push(view); return view
}
function deferred<T>() { let resolve!: (value: T) => void; let reject!: (error: unknown) => void; const promise = new Promise<T>((a, b) => { resolve = a; reject = b }); return { promise, resolve, reject } }
const catalog = () => ({ items: [
  { id: 1, name: 'Enabled Gemini', platform: 'gemini', enabled: true },
  { id: 2, name: 'Other OpenAI', platform: 'openai', enabled: false },
  { id: 3, name: 'Another Gemini', platform: 'gemini', enabled: false },
] })
beforeEach(() => {
  vi.resetAllMocks(); Object.assign(reactive(state), { id: 1, role: 'admin', token: 'session-one' })
  listConfigGroups.mockResolvedValue(catalog())
  saveConfigGroups.mockImplementation(async (groups: { group_id: number; enabled: boolean }[]) => ({ items: groups }))
})
afterEach(() => { views.splice(0).forEach(view => view.unmount()) })

describe('model probe group configuration', () => {
  it('does not request privileged configuration for a secondary administrator', async () => {
    state.role = 'sub_admin'; const view = create(); await flushPromises()
    expect(listConfigGroups).not.toHaveBeenCalled(); expect(saveConfigGroups).not.toHaveBeenCalled()
    expect(view.emitted('close')).toHaveLength(1)
  })
  it('loads group switches without saving and disables save until something changes', async () => {
    const view = create(); await flushPromises()
    expect(listConfigGroups).toHaveBeenCalledTimes(1)
    const inputs = view.findAll<HTMLInputElement>('[data-test="probe-config-group"]')
    expect(inputs.map(input => [input.attributes('data-group-id'), input.element.checked])).toEqual([['1', true], ['2', false], ['3', false]])
    expect(view.find('[data-test="probe-config-model"]').exists()).toBe(false)
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(saveConfigGroups).not.toHaveBeenCalled()
  })
  it('searches groups, selects only visible results and explicitly saves changed groups only', async () => {
    const view = create(); await flushPromises()
    await view.get('[data-test="probe-config-search"]').setValue('openai')
    await view.get('[data-test="probe-config-select-visible"]').trigger('click')
    expect(saveConfigGroups).not.toHaveBeenCalled()
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(saveConfigGroups.mock.calls[0][0]).toEqual([{ group_id: 2, enabled: true }])
    expect(saveConfigGroups.mock.calls[0][1]).toBeInstanceOf(AbortSignal)
    expect(view.emitted('saved')).toEqual([[]])
  })
  it('sends only previously enabled groups when clearing the selection', async () => {
    const view = create(); await flushPromises()
    await view.get('[data-test="probe-config-clear"]').trigger('click')
    expect(view.text()).toContain('modelProbes.config.emptyHint')
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(saveConfigGroups.mock.calls[0][0]).toEqual([{ group_id: 1, enabled: false }])
  })
  it('removes a reverted edit from the delta instead of overwriting other administrators', async () => {
    const view = create(); await flushPromises()
    const input = view.get('[data-group-id="2"]')
    await input.setValue(true); await input.setValue(false)
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(saveConfigGroups).not.toHaveBeenCalled()
  })
  it('isolates configuration denial without revoking shared support access', async () => {
    listConfigGroups.mockRejectedValue({ status: 403, message: 'https://upstream-secret.invalid' })
    const view = create(); await flushPromises()
    expect(view.text()).toContain('modelProbes.config.errors.forbidden')
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(view.emitted('access-denied')).toBeUndefined(); expect(view.text()).not.toContain('upstream-secret')
    expect(saveConfigGroups).not.toHaveBeenCalled()
  })
  it('cannot save after a failed load and can retry discovery safely', async () => {
    listConfigGroups.mockRejectedValueOnce({ status: 503, message: 'private' })
    const view = create(); await flushPromises()
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    await view.get('[data-test="probe-config-reload"]').trigger('click'); await flushPromises()
    expect(view.findAll('[data-test="probe-config-group"]')).toHaveLength(3)
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(saveConfigGroups).not.toHaveBeenCalled()
  })
  it.each([
    { reason: 'legacy snapshot without enabled', data: { items: [{ id: 1, name: 'Legacy', platform: 'gemini' }] } },
    { reason: 'duplicate group IDs', data: { items: [...catalog().items, { id: 1, name: 'Duplicate', platform: 'gemini', enabled: false }] } },
    { reason: 'non-array items', data: { items: null } },
    { reason: 'non-positive ID', data: { items: [{ ...catalog().items[0], id: 0 }] } },
    { reason: 'unsafe integer ID', data: { items: [{ ...catalog().items[0], id: Number.MAX_SAFE_INTEGER + 1 }] } },
    { reason: 'non-boolean enabled', data: { items: [{ ...catalog().items[0], enabled: 'true' }] } },
    { reason: 'non-string name', data: { items: [{ ...catalog().items[0], name: null }] } },
    { reason: 'non-string platform', data: { items: [{ ...catalog().items[0], platform: 1 }] } },
  ])('rejects the entire configuration snapshot for $reason', async ({ data }) => {
    listConfigGroups.mockResolvedValueOnce(data)
    const view = create(); await flushPromises()
    expect(view.text()).toContain('modelProbes.config.errors.groups')
    expect(view.findAll('[data-test="probe-config-group"]')).toHaveLength(0)
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(saveConfigGroups).not.toHaveBeenCalled()
  })
  it('locks editing and closing during save and ignores an expired session response', async () => {
    const pending = deferred<{ items: { group_id: number; enabled: boolean }[] }>(); saveConfigGroups.mockReturnValueOnce(pending.promise)
    const view = create(); await flushPromises(); await view.get('[data-group-id="2"]').setValue(true)
    await view.get('[data-test="probe-config-save"]').trigger('click')
    expect(view.get('[data-test="probe-config-search"]').attributes('disabled')).toBeDefined()
    expect(view.get('[data-group-id="2"]').attributes('disabled')).toBeDefined()
    expect(view.get('[data-test="probe-config-cancel"]').attributes('disabled')).toBeDefined()
    expect(view.getComponent(DialogStub).props('closeOnEscape')).toBe(false)
    view.getComponent(DialogStub).vm.$emit('close'); await flushPromises()
    expect(view.emitted('close')).toBeUndefined()
    const signal = saveConfigGroups.mock.calls[0][1] as AbortSignal
    reactive(state).token = 'session-two'; await flushPromises()
    expect(signal.aborted).toBe(true); expect(view.emitted('close')).toHaveLength(1)
    pending.resolve({ items: [{ group_id: 2, enabled: true }] }); await flushPromises()
    expect(view.emitted('saved')).toBeUndefined()
  })
  it('keeps edits after an atomic batch failure without reporting success', async () => {
    saveConfigGroups.mockRejectedValueOnce({ status: 503, message: 'private details' })
    const view = create(); await flushPromises(); await view.get('[data-group-id="2"]').setValue(true)
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(view.emitted('saved')).toBeUndefined(); expect(view.text()).toContain('modelProbes.config.errors.save')
    expect(view.text()).not.toContain('private details'); expect(view.get<HTMLInputElement>('[data-group-id="2"]').element.checked).toBe(true)
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeUndefined()
  })
  it.each([
    [{ group_id: 2, enabled: false }],
    [{ group_id: 3, enabled: true }],
    [{ group_id: 2, enabled: true }, { group_id: 2, enabled: true }],
    [],
  ])('rejects mismatched or duplicate confirmation entries %j', async (...items) => {
    saveConfigGroups.mockResolvedValueOnce({ items })
    const view = create(); await flushPromises(); await view.get('[data-group-id="2"]').setValue(true)
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(view.emitted('saved')).toBeUndefined(); expect(view.text()).toContain('modelProbes.config.errors.save')
  })
  it('accepts a matching confirmation regardless of entry ordering', async () => {
    saveConfigGroups.mockResolvedValueOnce({ items: [{ group_id: 3, enabled: true }, { group_id: 2, enabled: true }] })
    const view = create(); await flushPromises(); await view.get('[data-test="probe-config-select-visible"]').trigger('click')
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(view.emitted('saved')).toEqual([[]])
  })
  it('blocks save when more than 500 group switches changed', async () => {
    listConfigGroups.mockResolvedValueOnce({ items: Array.from({ length: 501 }, (_, i) => ({ id: i + 1, name: `Group ${i}`, platform: 'gemini', enabled: false })) })
    const view = create(); await flushPromises(); await view.get('[data-test="probe-config-select-visible"]').trigger('click')
    expect(view.text()).toContain('modelProbes.config.errors.limit')
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined(); expect(saveConfigGroups).not.toHaveBeenCalled()
    await view.get('[data-group-id="501"]').setValue(false)
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeUndefined()
  })
  it('aborts discovery and ignores late data when the editor closes', async () => {
    const pending = deferred<ReturnType<typeof catalog>>(); listConfigGroups.mockReturnValueOnce(pending.promise)
    const view = create(); await flushPromises(); const signal = listConfigGroups.mock.calls[0][0] as AbortSignal
    view.unmount(); views.pop(); expect(signal.aborted).toBe(true)
    pending.resolve(catalog()); await flushPromises(); expect(view.emitted('saved')).toBeUndefined()
  })
})
