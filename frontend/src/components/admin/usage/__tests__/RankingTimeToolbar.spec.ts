import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import RankingTimeToolbar from '../RankingTimeToolbar.vue'

const DateRangePickerStub = defineComponent({ emits: ['change'], template: '<button data-test="invalid-range" @click="$emit(\'change\', { startDate: \'2026-09-08\', endDate: \'2026-09-01\', preset: null })">invalid</button>' })

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, vars?: Record<string, string>) => vars ? `${key}:${vars.timezone}` : key }) }))

describe('RankingTimeToolbar', () => {
  it('emits today and other inclusive presets', async () => {
    const wrapper = mount(RankingTimeToolbar, { props: { startDate: '2026-09-14', endDate: '2026-09-14' }, global: { stubs: { DateRangePicker: DateRangePickerStub } } })
    await wrapper.get('[data-test="ranking-range-today"]').trigger('click')
    expect(wrapper.emitted('change')?.[0]).toEqual([{ startDate: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), endDate: expect.stringMatching(/^\d{4}-\d{2}-\d{2}$/), preset: 'today' }])
  })
  it('ignores reversed custom ranges', async () => {
    const wrapper = mount(RankingTimeToolbar, { props: { startDate: '2026-09-01', endDate: '2026-09-07' }, global: { stubs: { DateRangePicker: DateRangePickerStub } } })
    await wrapper.get('[data-test=invalid-range]').trigger('click')
    expect(wrapper.emitted('change')).toBeUndefined()
  })
  it('shows exact shared range and timezone', () => {
    const wrapper = mount(RankingTimeToolbar, { props: { startDate: '2026-09-01', endDate: '2026-09-15' }, global: { stubs: { DateRangePicker: DateRangePickerStub } } })
    expect(wrapper.get('[data-test="ranking-range-value"]').text()).toContain('2026-09-01 — 2026-09-15')
    expect(wrapper.text()).toContain('admin.usage.timeRange.timezone:')
  })
})
