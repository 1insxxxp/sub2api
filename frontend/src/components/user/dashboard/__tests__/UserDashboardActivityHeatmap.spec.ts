import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

import UserDashboardActivityHeatmap from '../UserDashboardActivityHeatmap.vue'
import type { ActivityHeatmapDay } from '../activityHeatmap'

const day = (date: string, success: number, failed = 0): ActivityHeatmapDay => ({
  date,
  success_requests: success,
  failed_requests: failed,
  input_tokens: success * 10,
  output_tokens: success * 5,
  total_tokens: success * 15,
  billed_cost: success * 0.01,
  model_count: success ? 1 : 0,
})

const mountPopover = () => mount(UserDashboardActivityHeatmap, {
  props: {
    startDate: '2026-09-20',
    endDate: '2026-09-21',
    days: [day('2026-09-20', 2)],
    loading: false,
    error: false,
  },
  global: {
    stubs: {
      LoadingSpinner: true,
      RouterLink: { template: '<a href="/usage" class="activity-heatmap-link"><slot /></a>' },
    },
  },
})

describe('UserDashboardActivityHeatmap', () => {
  it('closes the popover when the pointer leaves the date and its details', async () => {
    const wrapper = mountPopover()
    await wrapper.get('.activity-heatmap-cell').trigger('mouseenter')
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(true)

    await wrapper.get('.activity-heatmap-cell-wrap').trigger('mouseleave', { relatedTarget: null })
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps details open while moving into the record link, then closes on leaving', async () => {
    const wrapper = mountPopover()
    const region = wrapper.get('.activity-heatmap-cell-wrap')
    await wrapper.get('.activity-heatmap-cell').trigger('mouseenter')
    await region.trigger('mouseleave', { relatedTarget: wrapper.get('.activity-heatmap-link').element })
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(true)

    await region.trigger('mouseleave', { relatedTarget: wrapper.get('.activity-heatmap-metric').element })
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps details open for keyboard focus inside and closes when focus leaves', async () => {
    const wrapper = mountPopover()
    const region = wrapper.get('.activity-heatmap-cell-wrap')
    await wrapper.get('.activity-heatmap-cell').trigger('focus')
    await region.trigger('focusout', { relatedTarget: wrapper.get('.activity-heatmap-link').element })
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(true)

    await region.trigger('focusout', { relatedTarget: null })
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(false)
    wrapper.unmount()
  })

  it('dismisses details with Escape', async () => {
    const wrapper = mountPopover()
    const cell = wrapper.get('.activity-heatmap-cell')
    await cell.trigger('focus')
    await cell.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not immediately close on the click following hover and focus', async () => {
    const wrapper = mountPopover()
    const cell = wrapper.get('.activity-heatmap-cell')
    await cell.trigger('mouseenter')
    await cell.trigger('focus')
    await cell.trigger('click')
    expect(wrapper.find('.activity-heatmap-popover').exists()).toBe(true)
    wrapper.unmount()
  })

  it('switches metrics and keeps failed-only days at the empty intensity level', async () => {
    const wrapper = mount(UserDashboardActivityHeatmap, {
      props: {
        startDate: '2026-09-20',
        endDate: '2026-09-22',
        days: [day('2026-09-20', 4), day('2026-09-21', 0, 3)],
        loading: false,
        error: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          RouterLink: { template: '<a class="activity-heatmap-link"><slot /></a>' },
        },
      },
    })

    const cells = wrapper.findAll('.activity-heatmap-cell')
    expect(cells).toHaveLength(3)
    expect(cells[0].classes()).toContain('activity-heatmap-level-4')
    expect(cells[1].classes()).toContain('activity-heatmap-level-0')

    await wrapper.findAll('.activity-heatmap-metric')[1].trigger('click')
    expect(wrapper.findAll('.activity-heatmap-metric')[1].attributes('aria-pressed')).toBe('true')
    expect(wrapper.find('.activity-heatmap-cell').classes()).toContain('activity-heatmap-level-4')
  })

  it('shows the complete daily breakdown and record link on focus', async () => {
    const wrapper = mount(UserDashboardActivityHeatmap, {
      props: {
        startDate: '2026-09-20',
        endDate: '2026-09-20',
        days: [day('2026-09-20', 2, 1)],
        loading: false,
        error: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          RouterLink: { template: '<a class="activity-heatmap-link"><slot /></a>' },
        },
      },
    })

    await wrapper.find('.activity-heatmap-cell').trigger('focus')
    expect(wrapper.text()).toContain('dashboard.activityHeatmap.inputTokens')
    expect(wrapper.text()).toContain('dashboard.activityHeatmap.outputTokens')
    expect(wrapper.text()).toContain('dashboard.activityHeatmap.failedRequests')
    expect(wrapper.find('a.activity-heatmap-link').exists()).toBe(true)
  })

  it('hides the record link when the day has no retained usage records', async () => {
    const wrapper = mount(UserDashboardActivityHeatmap, {
      props: {
        startDate: '2026-09-20',
        endDate: '2026-09-21',
        days: [day('2026-09-20', 2)],
        loading: false,
        error: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          RouterLink: { template: '<a class="activity-heatmap-link"><slot /></a>' },
        },
      },
    })

    await wrapper.find('.activity-heatmap-cell[aria-label^="2026-09-21"]').trigger('focus')
    expect(wrapper.find('a.activity-heatmap-link').exists()).toBe(false)
  })

  it('keeps the calendar compact and places edge popovers inside the grid', async () => {
    const wrapper = mount(UserDashboardActivityHeatmap, {
      props: {
        startDate: '2026-09-20',
        endDate: '2026-09-30',
        days: [],
        loading: false,
        error: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          RouterLink: { template: '<a class="activity-heatmap-link"><slot /></a>' },
        },
      },
    })

    expect(wrapper.find('.activity-heatmap-track').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('dashboard.activityHeatmap.title')

    await wrapper.find('.activity-heatmap-cell[aria-label^="2026-09-23"]').trigger('focus')
    expect(wrapper.find('.activity-heatmap-popover').classes()).toContain('activity-heatmap-popover-above-start')

    await wrapper.find('.activity-heatmap-cell[aria-label^="2026-09-30"]').trigger('focus')
    expect(wrapper.find('.activity-heatmap-popover').classes()).toContain('activity-heatmap-popover-above-end')
  })

  it('does not stack two month labels in the first week column', () => {
    const wrapper = mount(UserDashboardActivityHeatmap, {
      props: {
        startDate: '2025-09-28',
        endDate: '2026-09-27',
        days: [],
        loading: false,
        error: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          RouterLink: { template: '<a class="activity-heatmap-link"><slot /></a>' },
        },
      },
    })

    expect(wrapper.findAll('.activity-heatmap-month')).toHaveLength(12)
  })
})
