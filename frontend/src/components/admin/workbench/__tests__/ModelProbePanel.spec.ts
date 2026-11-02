import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { computed, reactive, ref } from 'vue'
import ModelProbePanel from '../ModelProbePanel.vue'

const { listGroups, listModels, runProbe, refresh, revoke, state } = vi.hoisted(() => ({
  listGroups: vi.fn(), listModels: vi.fn(), runProbe: vi.fn(), refresh: vi.fn(), revoke: vi.fn(),
  state: { id: 7, role: 'sub_admin', token: 'session-one', allowed: true },
}))
vi.mock('@/api/admin/modelProbes', () => ({ listGroups, listModels, runProbe }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => { const s = reactive(state); return { get user() { return { id: s.id, role: s.role } }, get token() { return s.token } } } }))
vi.mock('@/composables/useUserSupportAccess', () => ({ useUserSupportAccess: () => ({ canAccess: computed(() => reactive(state).allowed), refresh, revoke }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, locale: ref('zh') }) }))
const views: VueWrapper[] = []
const SelectStub = { props: ['modelValue', 'options', 'searchable'], emits: ['update:modelValue'], template: '<select :value="modelValue ?? \'\'" @change="$emit(\'update:modelValue\', Number($event.target.value))"><option value="">Choose</option><option v-for="item in options" :key="item.value" :value="item.value">{{ item.label }}</option></select>' }
const ConfigStub = { emits: ['close', 'saved'], template: '<div data-test="config-editor"><button data-test="config-save" @click="$emit(\'saved\')">Save</button></div>' }
function create() { const v = mount(ModelProbePanel, { global: { stubs: { Select: SelectStub, Icon: true, ModelProbeConfigDialog: ConfigStub } } }); views.push(v); return v }
function deferred<T>() { let resolve!: (v: T) => void; let reject!: (e: unknown) => void; const promise = new Promise<T>((a, b) => { resolve = a; reject = b }); return { promise, resolve, reject } }
function success(model = 'model-1') { return { group_id: 1, model, status: 'success', duration_ms: 240, started_at: '2026-11-02T00:00:00Z' } }
async function select(v: VueWrapper, id = 1) { await flushPromises(); await v.get('[data-test="probe-group"]').setValue(String(id)); await flushPromises() }
beforeEach(() => {
  vi.resetAllMocks(); Object.assign(reactive(state), { id: 7, role: 'sub_admin', token: 'session-one', allowed: true })
  refresh.mockResolvedValue(true); listGroups.mockResolvedValue({ items: [{ id: 1, name: 'Public Gemini', platform: 'gemini' }, { id: 2, name: 'Other group', platform: 'openai' }] })
  listModels.mockImplementation(async (id: number) => ({ group: { id, name: 'Group', platform: 'gemini' }, items: Array.from({ length: 5 }, (_, i) => ({ id: `model-${i + 1}` })) }))
  runProbe.mockImplementation(async (_id: number, model: string) => success(model))
})
afterEach(() => { views.splice(0).forEach(v => v.unmount()) })

describe('group model probes', () => {
  it('offers configuration only to the primary administrator', async () => {
    const sub = create(); await flushPromises(); expect(sub.find('[data-test="probe-configure"]').exists()).toBe(false)
    sub.unmount(); views.pop(); state.role = 'admin'
    const admin = create(); await select(admin)
    await admin.get('[data-test="probe-configure"]').trigger('click')
    expect(admin.getComponent(ConfigStub).attributes('initial-group-id')).toBeUndefined()
    reactive(state).token = 'new-session'; await flushPromises()
    expect(admin.find('[data-test="config-editor"]').exists()).toBe(false)
  })
  it('stops a batch and refreshes eligible groups after configuration changes', async () => {
    state.role = 'admin'
    const pending = deferred<ReturnType<typeof success>>(); runProbe.mockReturnValue(pending.promise)
    const v = create(); await select(v); await v.get('[data-test="probe-all"]').trigger('click'); await flushPromises()
    const signals = runProbe.mock.calls.map(args => args[2] as AbortSignal)
    await v.get('[data-test="probe-configure"]').trigger('click')
    listGroups.mockResolvedValueOnce({ items: [] })
    await v.get('[data-test="config-save"]').trigger('click'); await flushPromises()
    expect(signals.every(signal => signal.aborted)).toBe(true)
    expect(listGroups).toHaveBeenCalledTimes(2)
    expect(v.findAll('[data-test="probe-row"]')).toHaveLength(0)
    expect(revoke).not.toHaveBeenCalled()
    pending.resolve(success()); await flushPromises()
    expect(v.findAll('[data-probe-status="success"]')).toHaveLength(0)
  })
  it('reloads configured models when the selected group is still eligible after saving', async () => {
    state.role = 'admin'
    const v = create(); await select(v)
    await v.get('[data-test="probe-configure"]').trigger('click')
    listModels.mockResolvedValueOnce({ group: { id: 1, name: 'Group', platform: 'gemini' }, items: [{ id: 'newly-configured-model' }] })
    await v.get('[data-test="config-save"]').trigger('click'); await flushPromises()
    expect(listGroups).toHaveBeenCalledTimes(2); expect(listModels).toHaveBeenCalledTimes(2)
    expect(v.findAll('[data-test="probe-row"]')).toHaveLength(1)
    expect(v.text()).toContain('newly-configured-model'); expect(runProbe).not.toHaveBeenCalled()
  })
  it('checks support permission and offers searchable groups before any probe', async () => {
    const v = create(); await flushPromises(); expect(refresh).toHaveBeenCalled(); expect(listGroups).toHaveBeenCalledTimes(1)
    expect(v.getComponent(SelectStub).props('searchable')).toBe(true); expect(runProbe).not.toHaveBeenCalled(); expect(listModels).not.toHaveBeenCalled()
    await select(v); expect(listModels.mock.calls[0][0]).toBe(1); expect(v.findAll('[data-test="probe-row"]')).toHaveLength(5)
  })
  it('rejects ordinary users and denied support capability without loading groups', async () => {
    state.role = 'user'; const deniedRole = create(); await flushPromises(); expect(listGroups).not.toHaveBeenCalled(); expect(deniedRole.text()).toContain('modelProbes.errors.forbidden')
    deniedRole.unmount(); views.pop(); state.role = 'sub_admin'; refresh.mockResolvedValue(false); create(); await flushPromises(); expect(listGroups).not.toHaveBeenCalled()
  })
  it('notifies the workbench when its fresh access check is denied', async () => {
    refresh.mockResolvedValue(false)
    const v = create(); await flushPromises()
    expect(v.emitted('access-denied')).toHaveLength(1)
    expect(listGroups).not.toHaveBeenCalled()
  })
  it('runs only the selected group/public model and shows safe status plus latency', async () => {
    const v = create(); await select(v); await v.get('[data-test="probe-single-model-1"]').trigger('click'); await flushPromises()
    expect(runProbe).toHaveBeenCalledTimes(1); expect(runProbe.mock.calls[0].slice(0, 2)).toEqual([1, 'model-1']); expect(runProbe.mock.calls[0][2]).toBeInstanceOf(AbortSignal)
    expect(v.get('[data-test="probe-row"]').text()).toContain('modelProbes.status.success'); expect(v.text()).toContain('240 ms')
  })
  it('bounds a batch to three concurrent probes and fills freed slots', async () => {
    const pending = Array.from({ length: 5 }, () => deferred<ReturnType<typeof success>>()); runProbe.mockImplementation((_g, model) => pending[Number(model.split('-')[1]) - 1].promise)
    const v = create(); await select(v); await v.get('[data-test="probe-all"]').trigger('click'); await flushPromises(); expect(runProbe).toHaveBeenCalledTimes(3)
    pending[0].resolve(success()); await flushPromises(); expect(runProbe).toHaveBeenCalledTimes(4)
    pending[1].resolve(success('model-2')); await flushPromises(); expect(runProbe).toHaveBeenCalledTimes(5)
    pending.slice(2).forEach((p, i) => p.resolve(success(`model-${i + 3}`))); await flushPromises(); expect(v.findAll('[data-probe-status="success"]')).toHaveLength(5)
  })
  it('stops active and queued work and ignores late results', async () => {
    const pending = deferred<ReturnType<typeof success>>(); runProbe.mockReturnValue(pending.promise)
    const v = create(); await select(v); await v.get('[data-test="probe-all"]').trigger('click'); await flushPromises()
    const signals = runProbe.mock.calls.map(args => args[2] as AbortSignal); await v.get('[data-test="probe-stop"]').trigger('click'); expect(signals.every(s => s.aborted)).toBe(true)
    expect(v.findAll('[data-probe-status="cancelled"]')).toHaveLength(5); pending.resolve(success()); await flushPromises(); expect(runProbe).toHaveBeenCalledTimes(3); expect(v.findAll('[data-probe-status="success"]')).toHaveLength(0)
  })
  it('aborts and rejects results after a group or session changes', async () => {
    const pending = deferred<ReturnType<typeof success>>(); runProbe.mockReturnValue(pending.promise)
    const v = create(); await select(v); await v.get('[data-test="probe-single-model-1"]').trigger('click'); const signal = runProbe.mock.calls[0][2] as AbortSignal
    await select(v, 2); expect(signal.aborted).toBe(true); pending.resolve(success()); await flushPromises(); expect(v.findAll('[data-probe-status="success"]')).toHaveLength(0)
    reactive(state).token = 'new-session'; await flushPromises(); expect(listGroups).toHaveBeenCalledTimes(2); expect(v.findAll('[data-test="probe-row"]')).toHaveLength(0)
  })
  it('revokes permission on a batch 403, cancels the queue, and never echoes raw errors', async () => {
    runProbe.mockRejectedValue({ status: 403, message: 'https://secret-upstream.invalid token-secret' })
    const v = create(); await select(v); await v.get('[data-test="probe-all"]').trigger('click'); await flushPromises()
    expect(revoke).toHaveBeenCalled(); expect(v.emitted('access-denied')).toHaveLength(1); expect(runProbe.mock.calls.length).toBeLessThanOrEqual(3); expect(v.text()).not.toContain('secret-upstream')
  })
  it('retries failed models and aborts in-flight work on unmount', async () => {
    runProbe.mockResolvedValueOnce({ ...success(), status: 'failed', message: 'safe reason' }); const v = create(); await select(v)
    await v.get('[data-test="probe-single-model-1"]').trigger('click'); await flushPromises(); expect(v.get('[data-probe-status="failed"]').text()).toContain('modelProbes.reasons.failed')
    const pending = deferred<ReturnType<typeof success>>(); runProbe.mockReturnValue(pending.promise); await v.get('[data-test="probe-retry-failed"]').trigger('click'); await flushPromises()
    const signal = runProbe.mock.calls[1][2] as AbortSignal; v.unmount(); views.pop(); expect(signal.aborted).toBe(true)
  })
  it('reloads the selected group catalog after a model-list failure', async () => {
    listModels.mockRejectedValueOnce({ status: 503, message: 'private upstream error' })
    const v = create(); await select(v)
    expect(v.text()).toContain('modelProbes.errors.models')
    await v.get('[role="alert"] button').trigger('click'); await flushPromises()
    expect(listModels).toHaveBeenCalledTimes(2)
    expect(v.findAll('[data-test="probe-row"]')).toHaveLength(5)
    expect(v.text()).not.toContain('private upstream')
  })
  it('fails closed for mismatched response identities and unexpected statuses', async () => {
    runProbe.mockResolvedValueOnce({ ...success(), model: 'different-model', status: 'success', message: 'https://private.invalid' })
    const v = create(); await select(v); await v.get('[data-test="probe-single-model-1"]').trigger('click'); await flushPromises()
    expect(v.find('[data-probe-status="failed"]').exists()).toBe(true)
    expect(v.text()).not.toContain('private.invalid')
  })
  it('does not let an older catalog replace a newly selected group', async () => {
    const oldCatalog = deferred<{ group: { id: number; name: string; platform: string }; items: { id: string }[] }>()
    listModels.mockReturnValueOnce(oldCatalog.promise)
    const v = create(); await select(v); const signal = listModels.mock.calls[0][1] as AbortSignal
    await select(v, 2); expect(signal.aborted).toBe(true)
    oldCatalog.resolve({ group: { id: 1, name: 'Old', platform: 'gemini' }, items: [{ id: 'stale-model' }] }); await flushPromises()
    expect(v.text()).not.toContain('stale-model')
    expect(v.findAll('[data-test="probe-row"]')).toHaveLength(5)
  })

})
