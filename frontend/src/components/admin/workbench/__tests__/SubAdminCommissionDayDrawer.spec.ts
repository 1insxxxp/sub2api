import { afterEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SubAdminCommissionDayDrawer from '../SubAdminCommissionDayDrawer.vue'
const { getWorkbenchDayGroups, getWorkbenchDayGroupLogs } = vi.hoisted(() => ({ getWorkbenchDayGroups: vi.fn(), getWorkbenchDayGroupLogs: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { subAdminCommission: { getWorkbenchDayGroups, getWorkbenchDayGroupLogs } } }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const group = (id: number) => ({ group_id: id, group_name: `Group ${id}`, requests: 2, total_tokens: 1500, actual_cost: 12, commission_amount: 1.44 })
const wrappers: Array<ReturnType<typeof mount>> = []
const create = () => {
  const wrapper = mount(SubAdminCommissionDayDrawer, { props: { date: '2026-10-18', day: null }, global: { stubs: { Icon: true, Transition: true } } })
  wrappers.push(wrapper)
  return wrapper
}
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((done, fail) => { resolve = done; reject = fail })
  return { promise, resolve, reject }
}
afterEach(() => { wrappers.splice(0).forEach((wrapper) => wrapper.unmount()); vi.clearAllMocks() })
describe('commission day summary', () => {
  it('keeps group totals without request-level controls or logs', async () => {
    getWorkbenchDayGroups.mockResolvedValueOnce([group(3)])
    const wrapper = create()
    await flushPromises()
    expect(wrapper.get('[data-test="commission-day-group-3"]').text()).toContain('1500 tokens')
    expect(wrapper.find('[data-test="commission-day-group-3-toggle"]').exists()).toBe(false)
    expect(getWorkbenchDayGroupLogs).not.toHaveBeenCalled()
  })
  it('ignores an older date response after opening a new date', async () => {
    const older = deferred<ReturnType<typeof group>[]>()
    getWorkbenchDayGroups.mockReturnValueOnce(older.promise).mockResolvedValueOnce([group(4)])
    const wrapper = create()
    await wrapper.setProps({ date: '2026-10-19' })
    await flushPromises()
    older.resolve([group(3)])
    await flushPromises()
    expect(wrapper.find('[data-test="commission-day-group-3"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="commission-day-group-4"]').exists()).toBe(true)
  })
  it('does not let an old error replace the latest date totals', async () => {
    const older = deferred<ReturnType<typeof group>[]>()
    getWorkbenchDayGroups.mockReturnValueOnce(older.promise).mockResolvedValueOnce([group(4)])
    const wrapper = create()
    await wrapper.setProps({ date: '2026-10-19' })
    await flushPromises()
    older.reject(new Error('old error'))
    await flushPromises()
    expect(wrapper.find('[data-test="commission-day-group-4"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('old error')
  })
})
