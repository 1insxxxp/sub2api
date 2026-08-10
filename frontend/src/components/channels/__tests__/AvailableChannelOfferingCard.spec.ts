import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AvailableChannelOfferingCard from '../AvailableChannelOfferingCard.vue'
import type { CatalogModelOffering, CatalogPriceCollection } from '../availableChannelCatalog'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => ({
  'availableChannels.catalog.officialPrice': '官方价', 'availableChannels.catalog.sitePrice': '本站价',
  'availableChannels.catalog.effectiveRate': '实际倍率', 'availableChannels.catalog.input': '输入',
  'availableChannels.catalog.groupRate': '分组倍率',
  'availableChannels.catalog.output': '输出', 'availableChannels.catalog.cacheWrite': '缓存写入',
  'availableChannels.catalog.cacheRead': '缓存读取', 'availableChannels.catalog.imageInput': '图片输入',
  'availableChannels.catalog.imageOutput': '图片输出', 'availableChannels.catalog.priceRequest': '按次',
  'availableChannels.catalog.priceImage': '图片', 'availableChannels.catalog.unpriced': '暂未定价',
  'availableChannels.catalog.perMillion': '/ 1M token', 'availableChannels.catalog.perRequest': '/ 次',
  'availableChannels.catalog.perImage': '/ 张', 'availableChannels.catalog.rangeBetween': `${params?.min}–${params?.max} token`,
  'availableChannels.catalog.rangeFrom': `${params?.min}+ token`,
}[key] ?? key) }) }))

const empty = (): CatalogPriceCollection => ({ input: null, output: null, cacheWrite: null, cacheRead: null, imageInput: null, imageOutput: null, request: null })
const offering = (): CatalogModelOffering => {
  const prices = { ...empty(), input: { official: 0.00001, site: 0.000012, peakSite: null }, output: { official: 0.00005, site: 0.00006, peakSite: null }, cacheRead: { official: 0.000001, site: 0.0000012, peakSite: null } }
  const model = { key: 'm', groupKey: 'g', name: 'claude-fable-5', platform: 'anthropic', billingMode: 'token' as const, hasPricing: true, normalRate: 7.5, effectiveRate: 1.239, defaultRate: 7.5, userRate: null, peakFactor: null, prices, intervals: [] }
  return { key: 'o', channelKey: 'c', channelName: 'Anthropic', groupKey: 'g', groupName: '余额 [claude max]', platform: 'anthropic', hasPricing: true, model, prices, intervals: [] }
}

describe('AvailableChannelOfferingCard', () => {
  it('renders one flat offering with compact metadata and price cells', () => {
    const wrapper = mount(AvailableChannelOfferingCard, { props: { offering: offering() } })
    expect(wrapper.get('[data-testid="flat-offering-card"]').classes()).not.toContain('xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(88px,0.5fr)_minmax(72px,0.4fr)]')
    expect(wrapper.text()).toContain('Anthropic')
    expect(wrapper.text()).toContain('余额 [claude max]')
    expect(wrapper.text()).toContain('7.50×')
    expect(wrapper.text()).toContain('分组倍率')
    expect(wrapper.text()).toContain('实际倍率')
    expect(wrapper.text()).toContain('1.23×')
    expect(wrapper.text()).not.toContain('1.239×')
    expect(wrapper.findAll('[data-testid="offering-price-cell"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('$10.00')
    expect(wrapper.text()).toContain('¥12.00')
    expect(wrapper.find('[data-testid="price-detail-toggle"]').exists()).toBe(false)
  })

  it('renders tier ranges inline and handles unpriced offerings', () => {
    const tiered = offering()
    tiered.prices = empty(); tiered.model.prices = tiered.prices
    tiered.intervals = [{ key: 'tier', minTokens: 0, maxTokens: 1000, tierLabel: null, prices: { ...empty(), input: { official: 0.000002, site: 0.000003, peakSite: null } } }]
    tiered.model.intervals = tiered.intervals
    const wrapper = mount(AvailableChannelOfferingCard, { props: { offering: tiered } })
    expect(wrapper.get('[data-testid="pricing-tier-flat"]').text()).toContain('0–1,000 token')
    expect(wrapper.text()).toContain('¥3.00')

    const none = offering(); none.prices = empty(); none.model.prices = none.prices; none.intervals = []; none.model.intervals = []; none.hasPricing = false
    const emptyWrapper = mount(AvailableChannelOfferingCard, { props: { offering: none } })
    expect(emptyWrapper.get('[data-testid="offering-unpriced"]').text()).toContain('暂未定价')
  })

  it('uses a responsive grid without nested model-price markup', async () => {
    const source = await import('../AvailableChannelOfferingCard.vue?raw').then(module => module.default)
    expect(source).toContain('grid-cols-2')
    expect(source).toContain('lg:grid-cols-4')
    expect(source).toContain('[overflow-wrap:anywhere]')
    expect(source).not.toContain('AvailableChannelModelPrice')
  })
})
