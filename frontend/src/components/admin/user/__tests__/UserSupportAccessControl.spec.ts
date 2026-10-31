import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UserSupportAccessControl from '../UserSupportAccessControl.vue'

const { getUserAccess, setUserAccess } = vi.hoisted(() => ({ getUserAccess: vi.fn(), setUserAccess: vi.fn() }))
vi.mock('@/api/admin/userSupport', () => ({ getUserAccess, setUserAccess }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

beforeEach(() => { vi.clearAllMocks(); getUserAccess.mockResolvedValue({ enabled: false }); setUserAccess.mockResolvedValue({ enabled: true }) })
describe('separate user support authorization', () => {
  it('loads existing access and only writes an explicit permission save', async () => {
    const view = mount(UserSupportAccessControl, { props: { userId: 91 } })
    await flushPromises()
    expect(getUserAccess).toHaveBeenCalledWith(91)
    expect(setUserAccess).not.toHaveBeenCalled()
    await view.get('input[type="checkbox"]').setValue(true)
    expect(setUserAccess).not.toHaveBeenCalled()
    await view.get('[data-test="support-access-save"]').trigger('click')
    await flushPromises()
    expect(setUserAccess).toHaveBeenCalledWith(91, true)
    expect(view.text()).toContain('userSupport.permission.saved')
  })
  it('fails closed and never renders a permission endpoint error', async () => {
    getUserAccess.mockRejectedValue({ message: 'https://upstream.invalid/sk-secret' })
    const view = mount(UserSupportAccessControl, { props: { userId: 91 } })
    await flushPromises()
    expect(view.get('input').attributes('disabled')).toBeDefined()
    expect(view.text()).toContain('userSupport.permission.loadFailed')
    expect(view.html()).not.toContain('upstream.invalid')
  })
})
