import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { reactive, ref } from 'vue'
import ModelProbeConfigDialog from '../ModelProbeConfigDialog.vue'

const { listConfigGroups, getGroupConfig, saveGroupConfig, state } = vi.hoisted(() => ({
  listConfigGroups: vi.fn(), getGroupConfig: vi.fn(), saveGroupConfig: vi.fn(),
  state: { id: 1, role: 'admin', token: 'session-one' },
}))
vi.mock('@/api/admin/modelProbes', () => ({ listConfigGroups, getGroupConfig, saveGroupConfig }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => { const s = reactive(state); return { get user() { return { id: s.id, role: s.role } }, get token() { return s.token } } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))
const views: VueWrapper[] = []
const SelectStub = { props: ['modelValue', 'options', 'disabled', 'searchable'], emits: ['update:modelValue'], template: '<select :disabled="disabled" :value="modelValue ?? \'\'" @change="$emit(\'update:modelValue\', Number($event.target.value))"><option value="">Choose</option><option v-for="item in options" :key="item.value" :value="item.value">{{ item.label }}</option></select>' }
const DialogStub = { props: ['show'], emits: ['close'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' }
function create(initialGroupId: number | null = 1) {
  const view = mount(ModelProbeConfigDialog, { props: { initialGroupId }, global: { stubs: { BaseDialog: DialogStub, Select: SelectStub, Icon: true } } })
  views.push(view); return view
}
function deferred<T>() { let resolve!: (value: T) => void; let reject!: (error: unknown) => void; const promise = new Promise<T>((a, b) => { resolve = a; reject = b }); return { promise, resolve, reject } }
const detail = (id = 1) => ({ group: { id, name: 'Group', platform: 'gemini' }, models: ['Exact-Model'], available_models: [{ id: 'Exact-Model' }, { id: 'exact-model' }, { id: 'Other-Model' }] })
beforeEach(() => {
  vi.resetAllMocks(); Object.assign(reactive(state), { id: 1, role: 'admin', token: 'session-one' })
  listConfigGroups.mockResolvedValue({ items: [{ id: 1, name: 'One', platform: 'gemini' }, { id: 2, name: 'Two', platform: 'openai' }] })
  getGroupConfig.mockImplementation(async (id: number) => detail(id))
  saveGroupConfig.mockImplementation(async (id: number, models: string[]) => ({ group_id: id, models }))
})
afterEach(() => { views.splice(0).forEach(view => view.unmount()) })

describe('model probe configuration', () => {
  it('does not request privileged configuration for a secondary administrator', async () => {
    state.role = 'sub_admin'; const view = create(); await flushPromises()
    expect(listConfigGroups).not.toHaveBeenCalled(); expect(getGroupConfig).not.toHaveBeenCalled()
    expect(view.emitted('close')).toHaveLength(1)
  })
  it('loads public candidates without saving and preserves exact case-sensitive IDs', async () => {
    const view = create(); await flushPromises()
    expect(listConfigGroups).toHaveBeenCalledTimes(1); expect(getGroupConfig.mock.calls[0][0]).toBe(1)
    expect(view.getComponent(SelectStub).props('searchable')).toBe(true)
    const inputs = view.findAll<HTMLInputElement>('[data-test="probe-config-model"]')
    expect(inputs.map(input => [input.attributes('aria-label'), input.element.checked])).toEqual([['Exact-Model', true], ['exact-model', false], ['Other-Model', false]])
    expect(saveGroupConfig).not.toHaveBeenCalled()
  })
  it('selects only the current search results and saves explicitly', async () => {
    const view = create(); await flushPromises()
    await view.get('[data-test="probe-config-search"]').setValue('other')
    await view.get('[data-test="probe-config-select-visible"]').trigger('click')
    expect(saveGroupConfig).not.toHaveBeenCalled()
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(saveGroupConfig.mock.calls[0].slice(0, 2)).toEqual([1, ['Exact-Model', 'Other-Model']])
    expect(view.emitted('saved')).toEqual([[1]])
  })
  it('can explicitly clear all models and close group probing', async () => {
    const view = create(); await flushPromises()
    await view.get('[data-test="probe-config-clear"]').trigger('click')
    expect(view.text()).toContain('modelProbes.config.emptyHint')
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(saveGroupConfig.mock.calls[0].slice(0, 2)).toEqual([1, []])
  })
  it('isolates configuration denial without revoking shared support access', async () => {
    getGroupConfig.mockRejectedValue({ status: 403, message: 'https://upstream-secret.invalid' })
    const view = create(); await flushPromises()
    expect(view.text()).toContain('modelProbes.config.errors.forbidden')
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(view.emitted('access-denied')).toBeUndefined(); expect(view.text()).not.toContain('upstream-secret')
    expect(saveGroupConfig).not.toHaveBeenCalled()
  })
  it('prevents a failed catalog load from overwriting existing settings', async () => {
    getGroupConfig.mockRejectedValueOnce({ status: 503, message: 'private' })
    const view = create(); await flushPromises()
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    await view.get('[data-test="probe-config-reload"]').trigger('click'); await flushPromises()
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeUndefined()
    expect(saveGroupConfig).not.toHaveBeenCalled()
  })
  it('ignores an older group response after changing groups', async () => {
    const pending = deferred<ReturnType<typeof detail>>(); getGroupConfig.mockReturnValueOnce(pending.promise)
    const view = create(); await flushPromises()
    const signal = getGroupConfig.mock.calls[0][1] as AbortSignal
    await view.get('[data-test="probe-config-group"]').setValue('2'); await flushPromises()
    expect(signal.aborted).toBe(true)
    pending.resolve({ ...detail(), available_models: [{ id: 'stale-model' }] }); await flushPromises()
    expect(view.text()).not.toContain('stale-model')
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(saveGroupConfig.mock.calls[0][0]).toBe(2)
  })
  it('locks the group during save and ignores results from an expired session', async () => {
    const pending = deferred<{ group_id: number; models: string[] }>(); saveGroupConfig.mockReturnValueOnce(pending.promise)
    const view = create(); await flushPromises(); await view.get('[data-test="probe-config-save"]').trigger('click')
    expect(view.get('[data-test="probe-config-group"]').attributes('disabled')).toBeDefined()
    const signal = saveGroupConfig.mock.calls[0][2] as AbortSignal
    reactive(state).token = 'session-two'; await flushPromises()
    expect(signal.aborted).toBe(true); expect(view.emitted('close')).toHaveLength(1)
    pending.resolve({ group_id: 1, models: ['Exact-Model'] }); await flushPromises()
    expect(view.emitted('saved')).toBeUndefined()
  })
  it('limits a bulk selection to at most 200 exact model IDs', async () => {
    getGroupConfig.mockResolvedValueOnce({ ...detail(), models: [], available_models: Array.from({ length: 201 }, (_, i) => ({ id: `model-${i}` })) })
    const view = create(); await flushPromises(); await view.get('[data-test="probe-config-select-visible"]').trigger('click')
    expect(view.text()).toContain('modelProbes.config.errors.limit')
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    expect(view.findAll<HTMLInputElement>('[data-test="probe-config-model"]').filter(input => input.element.checked)).toHaveLength(0)
    expect(saveGroupConfig).not.toHaveBeenCalled()
  })
  it('keeps unavailable configured models visible so they can be removed', async () => {
    getGroupConfig.mockResolvedValueOnce({ ...detail(), models: ['Removed-Model', 'Exact-Model'] })
    const view = create(); await flushPromises()
    expect(view.text()).toContain('Removed-Model'); expect(view.text()).toContain('modelProbes.config.unavailable')
    expect(view.get('[data-test="probe-config-save"]').attributes('disabled')).toBeDefined()
    const removed = view.findAll<HTMLInputElement>('[data-test="probe-config-model"]').find(input => input.attributes('aria-label') === 'Removed-Model')!
    await removed.setValue(false)
    expect(removed.attributes('disabled')).toBeDefined()
    removed.element.click(); await flushPromises()
    expect(removed.element.checked).toBe(false)
    await view.get('[data-test="probe-config-save"]').trigger('click'); await flushPromises()
    expect(saveGroupConfig.mock.calls[0].slice(0, 2)).toEqual([1, ['Exact-Model']])
  })
  it('aborts outstanding detail requests when the editor closes', async () => {
    const pending = deferred<ReturnType<typeof detail>>(); getGroupConfig.mockReturnValueOnce(pending.promise)
    const view = create(); await flushPromises(); const signal = getGroupConfig.mock.calls[0][1] as AbortSignal
    view.unmount(); views.pop(); expect(signal.aborted).toBe(true)
    pending.resolve(detail()); await flushPromises(); expect(view.emitted('saved')).toBeUndefined()
  })
})
