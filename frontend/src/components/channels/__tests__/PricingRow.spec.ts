import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PricingRow from '../PricingRow.vue'

describe('PricingRow', () => {
  it('labels official and site display prices when a converted price is present', () => {
    const wrapper = mount(PricingRow, {
      props: {
        label: '输入',
        value: 0.000005,
        unit: '/ 1M token',
        scale: 1_000_000,
        officialLabel: '官方价',
        convertedValue: 0.0000004,
        convertedUnit: '/ 1M token',
        convertedScale: 1_000_000,
        convertedLabel: '本站价',
        convertedCurrencySymbol: '¥',
      },
    })

    expect(wrapper.text()).toContain('官方价')
    expect(wrapper.text()).toContain('$5 / 1M token')
    expect(wrapper.text()).toContain('本站价')
    expect(wrapper.text()).toContain('¥0.4 / 1M token')
  })
})
