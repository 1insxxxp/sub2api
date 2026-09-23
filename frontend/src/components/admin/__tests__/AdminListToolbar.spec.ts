import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AdminListToolbar from '../AdminListToolbar.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('AdminListToolbar', () => {
  beforeEach(() => {
    vi.stubGlobal('matchMedia', vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(), removeEventListener: vi.fn(),
      addListener: vi.fn(), removeListener: vi.fn()
    })))
  })

  it('keeps search and actions visible while mobile filters are collapsed', async () => {
    const wrapper = mount(AdminListToolbar, {
      attachTo: document.body,
      props: { activeFilters: 2 },
      slots: {
        search: '<input aria-label="Search" />',
        actions: '<button>Create</button>',
        filters: '<input aria-label="Status" value="active" />',
        secondary: '<button>Bulk edit</button>'
      }
    })
    await wrapper.vm.$nextTick()
    const toggle = wrapper.get('[data-test="admin-filter-toggle"]')
    expect(toggle.attributes('aria-label')).toBe('common.filter')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(toggle.text()).toContain('2')
    expect(wrapper.get('[data-test="admin-filter-panel"]').isVisible()).toBe(false)
    expect(wrapper.get('input[aria-label="Search"]').isVisible()).toBe(true)
    expect(wrapper.text()).toContain('Create')
    expect(wrapper.text()).toContain('Bulk edit')
    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(wrapper.get('[data-test="admin-filter-panel"]').isVisible()).toBe(true)
    await wrapper.get('[data-test="admin-filter-panel"]').trigger('keydown', { key: 'Escape' })
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(wrapper.get<HTMLInputElement>('input[aria-label="Status"]').element.value).toBe('active')
    wrapper.unmount()
  })

  it('does not create a filter control for pages without filters', () => {
    const wrapper = mount(AdminListToolbar, { slots: { search: '<input />' } })
    expect(wrapper.find('[data-test="admin-filter-toggle"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="admin-filter-panel"]').exists()).toBe(false)
  })

  it('keeps desktop filters expanded and assigns independent control ids', async () => {
    vi.mocked(window.matchMedia).mockReturnValue({
      matches: false,
      addEventListener: vi.fn(), removeEventListener: vi.fn()
    } as unknown as MediaQueryList)
    const first = mount(AdminListToolbar, { slots: { filters: '<input />' } })
    const second = mount(AdminListToolbar, { slots: { filters: '<input />' } })
    await first.vm.$nextTick()
    expect(first.get('[data-test="admin-filter-panel"]').isVisible()).toBe(true)
    expect(first.get('[data-test="admin-filter-panel"]').attributes('id'))
      .not.toBe(second.get('[data-test="admin-filter-panel"]').attributes('id'))
  })
})
