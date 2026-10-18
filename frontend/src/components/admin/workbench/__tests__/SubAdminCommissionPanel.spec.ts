import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SubAdminCommissionPanel from '../SubAdminCommissionPanel.vue'
const { auth } = vi.hoisted(() => ({ auth: { user: { role: 'admin' } } }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
const wrappers: Array<ReturnType<typeof mount>> = []
const create = () => {
  const wrapper = mount(SubAdminCommissionPanel, { global: { stubs: {
    SubAdminCommissionManagement: { template: '<button data-test="change-settings" @click="$emit(\'changed\')">Change</button>' },
    SubAdminCommissionCalendar: { props: ['refreshKey'], template: '<button data-test="calendar" @click="$emit(\'select-day\', \'2026-10-18\', { date: \'2026-10-18\', actual_cost: 12, commission_amount: 1 })">{{ refreshKey }}</button>' },
    SubAdminCommissionDayDrawer: { props: ['date', 'day'], template: '<div data-test="selected-date">{{ date }}</div>' },
  } } })
  wrappers.push(wrapper)
  return wrapper
}
afterEach(() => { wrappers.splice(0).forEach((wrapper) => wrapper.unmount()); auth.user.role = 'admin' })
describe('commission panel refresh', () => {
  it('closes the old day and refreshes aggregates after management changes', async () => {
    const wrapper = create()
    await wrapper.get('[data-test="calendar"]').trigger('click')
    expect(wrapper.get('[data-test="selected-date"]').text()).toBe('2026-10-18')
    await wrapper.get('[data-test="change-settings"]').trigger('click')
    expect(wrapper.get('[data-test="selected-date"]').text()).toBe('')
    expect(wrapper.get('[data-test="calendar"]').text()).toBe('1')
  })
  it('does not mount blacklist management for secondary admins', () => {
    auth.user.role = 'sub_admin'
    const wrapper = create()
    expect(wrapper.find('[data-test="change-settings"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="calendar"]').exists()).toBe(true)
  })
})
