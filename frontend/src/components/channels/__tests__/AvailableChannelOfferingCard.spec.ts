import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AvailableChannelOfferingCard from '../AvailableChannelOfferingCard.vue'
import type { CatalogModelOffering, CatalogPriceCollection } from '../availableChannelCatalog'

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, params?: Record<string, unknown>) => ({
  'availableChannels.catalog.officialPrice': '官方价', 'availableChannels.catalog.sitePrice': '本站价',
  'availableChannels.catalog.effectiveRate': '实际倍率', 'availableChannels.catalog.input': '输入',
  'availableChannels.catalog.groupRate': '分组倍率',
  'availableChannels.catalog.output': '输出', 'availableChannels.catalog.cacheWrite': '缓存写入',
  'availableChannels.catalog.cacheRead': '缓存读取', 'availableChannels.catalog.imageInput': '图片输入',
  'availableChannels.catalog.imageOutput': '图片输出', 'availableChannels.catalog.priceRequest': '按次',
  'availableChannels.catalog.priceImage': '图片', 'availableChannels.catalog.unpriced': '暂未定价',
  'availableChannels.catalog.peakPrice': '高峰价格',
  'availableChannels.catalog.perMillion': '/ 1M token', 'availableChannels.catalog.perRequest': '/ 次',
  'availableChannels.catalog.perImage': '/ 张', 'availableChannels.catalog.rangeBetween': `${params?.min}–${params?.max} token`,
  'availableChannels.catalog.rangeFrom': `${params?.min}+ token`,
  }[key] ?? key) }),
}))

const empty = (): CatalogPriceCollection => ({ input: null, output: null, cacheWrite: null, cacheRead: null, imageInput: null, imageOutput: null, request: null })
const price = (
  official: number | null,
  site: number | null,
  siteMax: number | null = site,
  peakSite: number | null = null,
  peakSiteMax: number | null = peakSite,
) => ({ official, officialCny: official, site, siteMax, peakSite, peakSiteMax })
const offering = (): CatalogModelOffering => {
  const prices = { ...empty(), input: price(0.00001, 0.000012), output: price(0.00005, 0.00006), cacheRead: price(0.000001, 0.0000012) }
  const model = { key: 'm', groupKey: 'g', name: 'claude-fable-5', platform: 'anthropic', billingMode: 'token' as const, hasPricing: true, normalRate: 7.5, effectiveRate: 1.239, effectiveRateMax: 1.239, defaultRate: 7.5, userRate: null, peakFactor: null, prices, intervals: [] }
  return { key: 'o', channelKey: 'c', channelName: 'Anthropic', groupKey: 'g', groupName: '余额 [claude max]', platform: 'anthropic', hasPricing: true, model, prices, intervals: [] }
}
const platformBadgeStub = {
  AvailableChannelPlatformBadge: { props: ['platform'], template: '<span data-platform-badge :data-platform="platform">{{ platform }}</span>' },
}

describe('AvailableChannelOfferingCard', () => {
  it('renders one flat offering with compact metadata and price cells', () => {
    const wrapper = mount(AvailableChannelOfferingCard, { props: { offering: offering() }, global: { stubs: platformBadgeStub } })
    const root = wrapper.get('[data-testid="flat-offering-card"]')
    expect(root.classes()).toContain('offering-row')
    expect(root.classes()).not.toContain('shadow-sm')
    expect(root.classes()).not.toContain('rounded-2xl')
    expect(wrapper.text()).toContain('Anthropic')
    expect(wrapper.get('h4').text()).toBe('Anthropic')
    expect(wrapper.text()).toContain('余额 [claude max]')
    expect(wrapper.get('[data-platform-badge]').attributes('data-platform')).toBe('anthropic')
    expect(wrapper.text()).toContain('7.50×')
    expect(wrapper.text()).toContain('分组倍率')
    expect(wrapper.text()).toContain('实际倍率')
    expect(wrapper.text()).toContain('1.23×')
    expect(wrapper.text()).not.toContain('1.239×')
    expect(wrapper.findAll('[data-testid="offering-price-cell"]')).toHaveLength(3)
    expect(wrapper.get('[data-testid="offering-price-table"]').classes()).toContain('divide-y')
    expect(wrapper.get('[data-testid="offering-price-heading"]').text()).toContain('官方价')
    expect(wrapper.get('[data-testid="offering-price-heading"]').text()).toContain('本站价')
    expect(wrapper.text()).toContain('$10.00')
    expect(wrapper.text()).toContain('¥12.00')
    expect(wrapper.find('[data-testid="price-detail-toggle"]').exists()).toBe(false)
  })

  it('renders tier ranges inline and handles unpriced offerings', () => {
    const tiered = offering()
    tiered.prices = empty(); tiered.model.prices = tiered.prices
    tiered.intervals = [{ key: 'tier', minTokens: 0, maxTokens: 1000, tierLabel: null, prices: { ...empty(), input: price(0.000002, 0.000003, 0.00000375) } }]
    tiered.model.intervals = tiered.intervals
    const wrapper = mount(AvailableChannelOfferingCard, { props: { offering: tiered }, global: { stubs: platformBadgeStub } })
    expect(wrapper.get('[data-testid="pricing-tier-flat"]').text()).toContain('0–1,000 token')
    expect(wrapper.text()).toContain('¥3.00–¥3.75')

    const none = offering(); none.prices = empty(); none.model.prices = none.prices; none.intervals = []; none.model.intervals = []; none.hasPricing = false
    const emptyWrapper = mount(AvailableChannelOfferingCard, { props: { offering: none }, global: { stubs: platformBadgeStub } })
    expect(emptyWrapper.get('[data-testid="offering-unpriced"]').text()).toContain('暂未定价')
  })

  it('keeps the group rate singular and floors the actual-rate range to two decimals', () => {
    const ranged = offering()
    ranged.model.effectiveRate = 0.129
    ranged.model.effectiveRateMax = 0.159
    ranged.prices.input = price(0.000005, 0.0000006, 0.00000075, 0.0000009, 0.000001125)
    ranged.model.prices = ranged.prices

    const wrapper = mount(AvailableChannelOfferingCard, { props: { offering: ranged }, global: { stubs: platformBadgeStub } })
    expect(wrapper.text()).toContain('7.50×')
    expect(wrapper.text()).toContain('0.12×–0.15×')
    expect(wrapper.text()).toContain('¥0.60–¥0.75')
    expect(wrapper.text()).toContain('高峰价格 ¥0.90–¥1.125')
  })

  it('uses a responsive grid without nested model-price markup', async () => {
    const source = await import('../AvailableChannelOfferingCard.vue?raw').then(module => module.default)
    expect(source).toContain('sm:grid-cols-[minmax(0,1fr)_auto]')
    expect(source).toContain('grid-cols-[minmax(0,1fr)_auto_auto]')
    expect(source).toContain('[overflow-wrap:anywhere]')
    expect(source).not.toContain('AvailableChannelModelPrice')
  })
})
