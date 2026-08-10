import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import enDashboard from '@/i18n/locales/en/dashboard'
import zhDashboard from '@/i18n/locales/zh/dashboard'
import AvailableChannelModelList from '../AvailableChannelModelList.vue'
import type { CatalogModelListEntry, CatalogModelOffering, CatalogPriceCollection } from '../availableChannelCatalog'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, number>) => ({
  'availableChannels.catalog.modelColumn': '模型', 'availableChannels.catalog.officialPrice': '官方价',
  'availableChannels.catalog.sitePrice': '本站价', 'availableChannels.catalog.unpriced': '暂未定价',
  'availableChannels.catalog.moreOfferings': `还有 ${params?.count ?? 0} 个方案`,
  'availableChannels.catalog.channelsCount': `${params?.count ?? 0} 个渠道`,
  'availableChannels.catalog.groupsCount': `${params?.count ?? 0} 个分组`,
  'availableChannels.catalog.showOfferings': '展开方案', 'availableChannels.catalog.hideOfferings': '收起方案',
  'availableChannels.catalog.offeringsColumn': '方案', 'availableChannels.catalog.representativePrice': '代表价',
  'availableChannels.catalog.perMillion': '/ 1M token', 'availableChannels.catalog.perRequest': '/ 次',
  'availableChannels.catalog.perImage': '/ 张', 'availableChannels.catalog.tieredSummary': '阶梯价 · 展开查看',
  'availableChannels.catalog.noModels': '该渠道暂无可用模型',
  'availableChannels.catalog.savePercent': `省 ${params?.count ?? 0}%`,
  'availableChannels.catalog.priceInput': '输入',
  'availableChannels.catalog.input': '输入', 'availableChannels.catalog.output': '输出',
  'availableChannels.catalog.cacheWrite': '缓存写入', 'availableChannels.catalog.cacheRead': '缓存读取',
  'availableChannels.catalog.priceRequest': '按次', 'availableChannels.catalog.priceImage': '图片',
}[key] ?? key) }) }))

const prices = (official: number | null, site: number | null = official): CatalogPriceCollection => ({
  input: official == null && site == null ? null : { official, site, peakSite: null }, output: null,
  cacheWrite: null, cacheRead: null, imageInput: null, imageOutput: null, request: null,
})
function offering(key: string, channel: string, group: string, official: number | null, site = official): CatalogModelOffering {
  const collection = prices(official, site)
  const model = { key, groupKey: group, name: 'same-model', platform: 'gemini', billingMode: 'token',
    hasPricing: official != null || site != null, normalRate: 1, defaultRate: 1, userRate: null,
    peakFactor: null, prices: collection, intervals: [] }
  return { key, channelKey: channel, channelName: channel, groupKey: group, groupName: group,
    platform: 'gemini', hasPricing: model.hasPricing, model, prices: collection, intervals: [] }
}

function setBilling(offering: CatalogModelOffering, mode: string, field: keyof CatalogPriceCollection, official: number, site: number) {
  offering.model.billingMode = mode
  offering.prices.input = null
  offering.prices[field] = { official, site, peakSite: null }
  offering.model.prices = offering.prices
  offering.hasPricing = true
  offering.model.hasPricing = true
  return offering
}
function entry(key: string, offerings: CatalogModelOffering[]): CatalogModelListEntry {
  return { key, name: key, offerings, channelCount: new Set(offerings.map(x => x.channelKey)).size,
    groupCount: new Set(offerings.map(x => x.groupKey)).size,
    platforms: ['gemini', 'openai-with-a-very-long-platform-name'], hasPricedOffering: offerings.some(x => x.hasPricing) }
}
const stubs = { AvailableChannelModelPrice: { props: ['model'], template: '<div data-testid="full-price">{{ model.key }}</div>' } }

describe('AvailableChannelModelList', () => {
  it('shows metadata and representative prices before expansion', () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [entry('gemini-pro', [offering('a', 'Alpha', 'Retail', 2, 7.2)])] }, global: { stubs } })
    expect(wrapper.text()).toContain('gemini-pro'); expect(wrapper.text()).toContain('1 个渠道'); expect(wrapper.text()).toContain('1 个分组')
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('$2000000')
    expect(wrapper.get('[data-testid="representative-site"]').text()).toContain('¥7200000')
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('/ 1M token')
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('代表价')
    expect(wrapper.find('[data-testid="offering-details"]').exists()).toBe(false)
  })
  it('renders a colorful card grid with immediate official/site savings comparison', () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [entry('gemini-pro', [offering('a', 'Alpha', 'Retail', 0.00001, 0.000004)])] }, global: { stubs } })
    const card = wrapper.get('[data-testid="model-card"]')
    expect(wrapper.get('[data-testid="model-card-grid"]').classes()).toContain('grid')
    expect(card.find('[data-testid="brand-icon"]').attributes('data-brand')).toBe('gemini')
    expect(card.get('[data-testid="primary-site-price"]').text()).toContain('¥4.00')
    expect(card.get('[data-testid="primary-official-price"]').text()).toContain('$10.00')
    expect(card.get('[data-testid="primary-official-price"]').classes()).toContain('line-through')
    expect(card.get('[data-testid="savings-badge"]').text()).toContain('60%')
    expect(card.text()).toContain('输入')
  })
  it('shows input, output and cache comparisons directly on the card', () => {
    const priced = offering('a', 'Alpha', 'Retail', 0.00001, 0.000004)
    priced.prices.output = { official: 0.00002, site: 0.000008, peakSite: null }
    priced.prices.cacheRead = { official: 0.000001, site: 0, peakSite: null }
    priced.model.prices = priced.prices
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [entry('gemini-pro', [priced])] }, global: { stubs } })
    const rows = wrapper.findAll('[data-testid="price-dimension-row"]')
    expect(rows).toHaveLength(3)
    expect(rows[0].text()).toContain('输入')
    expect(rows[0].text()).toContain('$10.00')
    expect(rows[0].text()).toContain('¥4.00')
    expect(rows[1].text()).toContain('输出')
    expect(rows[1].text()).toContain('$20.00')
    expect(rows[1].text()).toContain('¥8.00')
    expect(rows[2].text()).toContain('缓存读取')
    expect(rows[2].text()).toContain('¥0.00')
  })
  it('uses billing-aware units for request and image summaries', () => {
    const request = setBilling(offering('request', 'A', 'G1', null), 'per_request', 'request', 0.2, 0.4)
    const image = setBilling(offering('image', 'B', 'G2', null), 'image', 'request', 0.3, 0.6)
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [entry('request-model', [request]), entry('image-model', [image])] }, global: { stubs } })
    const official = wrapper.findAll('[data-testid="representative-official"]')
    expect(official[0].text()).toContain('$0.20 / 次')
    expect(official[1].text()).toContain('$0.30 / 张')
  })
  it('keeps a request unit when token mode falls back to request-only pricing', () => {
    const fallback = setBilling(offering('fallback', 'A', 'G1', null), 'token', 'request', 0.2, 0.4)
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [entry('fallback-model', [fallback])] }, global: { stubs } })
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('$0.20 / 次')
  })
  it('shows a tier summary instead of unpriced for tier-only offerings and preserves zero', () => {
    const tiered = offering('tiered', 'A', 'G1', null)
    tiered.intervals = [{ key: 'tier:1', minTokens: 0, maxTokens: 1000, tierLabel: null, prices: prices(0.000002, 0.000004) }]
    tiered.model.intervals = tiered.intervals
    tiered.hasPricing = true
    tiered.model.hasPricing = true
    const free = offering('free', 'B', 'G2', 0, 0)
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [entry('tier', [tiered]), entry('free', [free])] }, global: { stubs } })
    const rows = wrapper.findAll('[data-testid="model-card"]')
    expect(rows[0].text()).toContain('阶梯价 · 展开查看')
    expect(rows[0].text()).not.toContain('暂未定价')
    expect(rows[1].get('[data-testid="representative-official"]').text()).toContain('$0.00 / 1M token')
  })
  it('chooses first priced offering, preserves zero, and marks unpriced', () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [
      entry('mixed', [offering('empty', 'A', 'G1', null), offering('free', 'B', 'G2', 0, 0), offering('paid', 'C', 'G3', 3)]),
      entry('none', [offering('none-1', 'A', 'G1', null)]),
    ] }, global: { stubs } })
    const rows = wrapper.findAll('[data-testid="model-card"]')
    expect(rows[0].get('[data-testid="representative-official"]').text()).toContain('$0')
    expect(rows[0].get('[data-testid="representative-site"]').text()).toContain('¥0')
    expect(rows[0].text()).toContain('还有 2 个方案'); expect(rows[1].text()).toContain('暂未定价')
  })
  it('expands exact offerings by mouse and keyboard with unique aria', async () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [
      entry('one', [offering('a', 'Alpha', 'Retail', 1), offering('b', 'Beta', 'Pro', 2)]),
      entry('two', [offering('c', 'Gamma', 'Main', 3)]),
    ] }, global: { stubs } })
    const toggles = wrapper.findAll('[data-testid="model-offering-toggle"]')
    expect(toggles[0].attributes('aria-controls')).not.toBe(toggles[1].attributes('aria-controls'))
    await toggles[0].trigger('keydown', { key: 'Enter' })
    expect(toggles[0].attributes('aria-expanded')).toBe('true')
    const details = wrapper.get('[data-testid="offering-details"]')
    expect(details.text()).toContain('Alpha'); expect(details.text()).toContain('Retail'); expect(details.text()).toContain('Beta')
    expect(details.findAll('[data-testid="full-price"]')).toHaveLength(2)
    await toggles[0].trigger('keydown', { key: ' ' }); expect(wrapper.find('[data-testid="offering-details"]').exists()).toBe(false)
  })
  it('uses collision-safe details ids and summarizes request/image pricing', async () => {
    const request = offering('request', 'A', 'G1', null)
    request.hasPricing = true
    request.model.billingMode = 'per_request'
    request.prices.input = null
    request.prices.request = { official: 0.2, site: 0.4, peakSite: null }
    request.model.prices = request.prices
    const image = offering('image', 'B', 'G2', null)
    image.hasPricing = true
    image.model.billingMode = 'image'
    image.prices.input = null
    image.prices.imageInput = { official: 0, site: 0, peakSite: null }
    image.model.prices = image.prices
    const first = entry('a/b', [request])
    const second = entry('a?b', [image])
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [first, second] }, global: { stubs } })
    const toggles = wrapper.findAll('[data-testid="model-offering-toggle"]')
    expect(toggles[0].attributes('aria-controls')).not.toBe(toggles[1].attributes('aria-controls'))
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('$0.2')
    expect(wrapper.get('[data-testid="representative-site"]').text()).toContain('¥0.4')
    expect(wrapper.findAll('[data-testid="representative-official"]')[1].text()).toContain('$0')
  })
  it('keeps expansion by stable key after reorder', async () => {
    const one = entry('one', [offering('a', 'Alpha', 'Retail', 1)]); const two = entry('two', [offering('b', 'Beta', 'Pro', 2)])
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [one, two] }, global: { stubs } })
    await wrapper.findAll('[data-testid="model-offering-toggle"]')[0].trigger('click'); await wrapper.setProps({ entries: [two, one] })
    expect(wrapper.findAll('[data-testid="model-card"]')[1].get('[data-testid="model-offering-toggle"]').attributes('aria-expanded')).toBe('true')
  })
  it('handles empty input and has one responsive, wrapping, touch-friendly DOM', () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [] }, global: { stubs } })
    expect(wrapper.find('[data-testid="model-card"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="model-list-empty"]').text()).toContain('该渠道暂无可用模型')
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../AvailableChannelModelList.vue'), 'utf8')
    expect(source).toContain('md:grid-cols-2'); expect(source).toContain('2xl:grid-cols-3'); expect(source).toContain('min-h-11')
    expect(source).toContain('[overflow-wrap:anywhere]'); expect(source).toContain('motion-reduce:transition-none')
    expect(source).not.toMatch(/fetch\(|axios|\/api\//); expect(source.match(/v-for="entry in entries"/g)).toHaveLength(1)
    expect(source).not.toContain('Math.random')
  })
  it('uses unique stable instance-scoped detail ids', async () => {
    const data = [entry('a/b', [offering('a', 'A', 'G', 1)]), entry('a?b', [offering('b', 'B', 'G', 1)])]
    const first = mount(AvailableChannelModelList, { props: { entries: data }, global: { stubs } })
    const before = first.findAll('[data-testid="model-offering-toggle"]').map(item => item.attributes('aria-controls'))
    await first.setProps({ entries: [...data].reverse() })
    const after = first.findAll('[data-testid="model-offering-toggle"]').map(item => item.attributes('aria-controls'))
    expect(new Set(before).size).toBe(2)
    expect(after).toEqual([...before].reverse())
    const second = mount(AvailableChannelModelList, { props: { entries: data }, global: { stubs } })
    expect(second.findAll('[data-testid="model-offering-toggle"]')[0].attributes('aria-controls')).not.toBe(before[0])
  })
  it('ships complete Chinese and English catalog labels', () => {
    const zh = zhDashboard.availableChannels.catalog
    const en = enDashboard.availableChannels.catalog
    const keys = ['channelsCount', 'moreOfferings', 'showOfferings', 'hideOfferings', 'offeringsColumn', 'representativePrice', 'tieredSummary', 'noModels'] as const
    for (const key of keys) {
      expect(zh[key]).toBeTruthy()
      expect(en[key]).toBeTruthy()
      expect(en[key]).not.toMatch(/[\u3400-\u9fff]/u)
      expect(en[key]).not.toContain(`availableChannels.catalog.${key}`)
    }
  })
})
