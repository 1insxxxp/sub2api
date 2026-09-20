import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import Pagination from '../Pagination.vue'
import { nextTick, ref } from 'vue'
import { adminAppearanceKey } from '@/composables/useAdminAppearance'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

function mountPagination() {
  return mount(Pagination, {
    props: { total: 200, page: 1, pageSize: 20, showJump: true, showPageSizeSelector: false },
    global: { stubs: { Icon: true } }
  })
}

describe('pagination jump input', () => {
  it('uses compact pagination in admin pages without overriding an explicit variant', async () => {
    const adminAppearance = ref(false)
    const wrapper = mount(Pagination, {
      props: { total: 200, page: 1, pageSize: 20, showPageSizeSelector: false },
      global: { provide: { [adminAppearanceKey as symbol]: adminAppearance }, stubs: { Icon: true } },
    })
    expect(wrapper.classes()).not.toContain('pagination-compact')
    adminAppearance.value = true
    await nextTick()
    expect(wrapper.classes()).toContain('pagination-compact')
    await wrapper.get('nav button:last-child').trigger('click')
    expect(wrapper.emitted('update:page')).toEqual([[2]])
    await wrapper.setProps({ variant: 'default' })
    expect(wrapper.classes()).not.toContain('pagination-compact')
    await wrapper.setProps({ variant: undefined })
    adminAppearance.value = false
    await nextTick()
    expect(wrapper.classes()).not.toContain('pagination-compact')
  })

  it('keeps the compact mobile controls usable at 320px', async () => {
    const adminAppearance = ref(true)
    const wrapper = mount(Pagination, {
      props: { total: 200, page: 1, pageSize: 20, showPageSizeSelector: false },
      global: { provide: { [adminAppearanceKey as symbol]: adminAppearance }, stubs: { Icon: true } },
    })

    const controls = wrapper.get('.pagination-mobile-controls')
    expect(controls.classes()).toContain('pagination-mobile-controls')
    expect(controls.findAll('button')).toHaveLength(2)
    expect(controls.get('.pagination-current').attributes('aria-label')).toBe('pagination.pageOf')
    expect(controls.findAll('button').every((button) => button.classes().includes('pagination-touch-target'))).toBe(true)
  })

  it.each(['click', 'enter'])('jumps to the entered numeric page using %s', async (action) => {
    const wrapper = mountPagination()
    const input = wrapper.get('input[type="number"]')
    await input.setValue('3')
    if (action === 'click') await wrapper.get('.btn').trigger('click')
    else await input.trigger('keyup', { key: 'Enter' })
    expect(wrapper.emitted('update:page')).toEqual([[3]])
    expect((input.element as HTMLInputElement).value).toBe('')
  })

  it('clamps an entered page to the last page', async () => {
    const wrapper = mountPagination()
    await wrapper.get('input').setValue('99')
    await wrapper.get('.btn').trigger('click')
    expect(wrapper.emitted('update:page')).toEqual([[10]])
  })

  it('ignores an empty jump input', async () => {
    const wrapper = mountPagination()
    await wrapper.get('.btn').trigger('click')
    expect(wrapper.emitted('update:page')).toBeUndefined()
  })
})
