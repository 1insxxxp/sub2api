import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AvailableChannelModelList from '../AvailableChannelModelList.vue'
import type { CatalogModelListEntry, CatalogModelOffering, CatalogPriceCollection } from '../availableChannelCatalog'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string, params?: Record<string, number>) => ({
  'availableChannels.catalog.modelColumn': '模型', 'availableChannels.catalog.officialPrice': '官方价',
  'availableChannels.catalog.sitePrice': '本站价', 'availableChannels.catalog.unpriced': '暂未定价',
  'availableChannels.catalog.moreOfferings': `还有 ${params?.count ?? 0} 个方案`,
  'availableChannels.catalog.channelsCount': `${params?.count ?? 0} 个渠道`,
  'availableChannels.catalog.groupsCount': `${params?.count ?? 0} 个分组`,
  'availableChannels.catalog.showOfferings': '展开方案', 'availableChannels.catalog.hideOfferings': '收起方案',
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
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('$2')
    expect(wrapper.get('[data-testid="representative-site"]').text()).toContain('¥7.2')
    expect(wrapper.get('[data-testid="representative-official"]').text()).toContain('代表')
    expect(wrapper.find('[data-testid="offering-details"]').exists()).toBe(false)
  })
  it('chooses first priced offering, preserves zero, and marks unpriced', () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [
      entry('mixed', [offering('empty', 'A', 'G1', null), offering('free', 'B', 'G2', 0, 0), offering('paid', 'C', 'G3', 3)]),
      entry('none', [offering('none-1', 'A', 'G1', null)]),
    ] }, global: { stubs } })
    const rows = wrapper.findAll('[data-testid="model-list-row"]')
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
    request.prices.input = null
    request.prices.request = { official: 0.2, site: 0.4, peakSite: null }
    request.model.prices = request.prices
    const image = offering('image', 'B', 'G2', null)
    image.hasPricing = true
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
    expect(wrapper.findAll('[data-testid="model-list-row"]')[1].get('[data-testid="model-offering-toggle"]').attributes('aria-expanded')).toBe('true')
  })
  it('handles empty input and has one responsive, wrapping, touch-friendly DOM', () => {
    const wrapper = mount(AvailableChannelModelList, { props: { entries: [] }, global: { stubs } })
    expect(wrapper.find('[data-testid="model-list-row"]').exists()).toBe(false)
    const source = readFileSync(resolve(dirname(fileURLToPath(import.meta.url)), '../AvailableChannelModelList.vue'), 'utf8')
    expect(source).toContain('xl:grid'); expect(source).toContain('xl:hidden'); expect(source).toContain('min-h-11')
    expect(source).toContain('[overflow-wrap:anywhere]'); expect(source).toContain('motion-reduce:transition-none')
    expect(source).not.toMatch(/fetch\(|axios|\/api\//); expect(source.match(/v-for="entry in entries"/g)).toHaveLength(1)
  })
})
