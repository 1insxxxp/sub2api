import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserSupportAccessControl from '../UserSupportAccessControl.vue'

const { getUserWorkbenchAccess, setUserWorkbenchAccess } = vi.hoisted(() => ({ getUserWorkbenchAccess: vi.fn(), setUserWorkbenchAccess: vi.fn() }))
vi.mock('@/api/admin/userSupport', () => ({ getUserWorkbenchAccess, setUserWorkbenchAccess }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

beforeEach(() => { vi.clearAllMocks(); getUserWorkbenchAccess.mockResolvedValue({ commission: false, affiliate_leaderboard: false, user_support: false }); setUserWorkbenchAccess.mockResolvedValue({ commission: false, affiliate_leaderboard: false, user_support: true }) })
describe('separate admin workbench authorization', () => {
  it('loads existing access and only writes an explicit permission save', async () => {
    const view = mount(UserSupportAccessControl, { props: { userId: 91 } })
    await flushPromises()
    expect(getUserWorkbenchAccess).toHaveBeenCalledWith(91)
    expect(view.findAll('input[type="checkbox"]')).toHaveLength(3)
    expect(setUserWorkbenchAccess).not.toHaveBeenCalled()
    await view.get('[data-test="workbench-access-user_support"]').setValue(true)
    expect(setUserWorkbenchAccess).not.toHaveBeenCalled()
    await view.get('[data-test="support-access-save"]').trigger('click')
    await flushPromises()
    expect(setUserWorkbenchAccess).toHaveBeenCalledWith(91, { commission: false, affiliate_leaderboard: false, user_support: true })
    expect(view.text()).toContain('userSupport.permission.saved')
  })
  it('fails closed and never renders a permission endpoint error', async () => {
    getUserWorkbenchAccess.mockRejectedValue({ message: 'https://upstream.invalid/sk-secret' })
    const view = mount(UserSupportAccessControl, { props: { userId: 91 } })
    await flushPromises()
    expect(view.get('input').attributes('disabled')).toBeDefined()
    expect(view.text()).toContain('userSupport.permission.loadFailed')
    expect(view.html()).not.toContain('upstream.invalid')
  })
})
