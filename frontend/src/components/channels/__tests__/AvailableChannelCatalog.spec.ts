import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { createPinia } from 'pinia'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { UserAvailableChannel } from '@/api/channels'
import type {
  CatalogChannelEntry,
  CatalogGroupEntry,
  CatalogModelEntry,
  CatalogPriceCollection,
  CatalogModelListEntry,
} from '../availableChannelCatalog'
import { buildAvailableChannelCatalog, projectModelEntriesForChannel } from '../availableChannelCatalog'
import { useAppStore } from '@/stores/app'

const labels: Record<string, string> = {
  'availableChannels.catalog.officialPrice': '官方价',
  'availableChannels.catalog.sitePrice': '本站价',
  'availableChannels.catalog.effectiveRate': '实际倍率',
  'availableChannels.catalog.groupsCount': '{count} 个分组',
  'availableChannels.catalog.channelsCount': '{count} 个渠道',
  'availableChannels.catalog.modelsCount': '{count} 个模型',
  'availableChannels.catalog.moreOfferings': '还有 {count} 个方案',
  'availableChannels.catalog.showOfferings': '展开方案',
  'availableChannels.catalog.hideOfferings': '收起方案',
  'availableChannels.catalog.offeringsColumn': '方案',
  'availableChannels.catalog.representativePrice': '代表价',
  'availableChannels.catalog.tieredSummary': '阶梯价 · 展开查看',
  'availableChannels.catalog.noModels': '该渠道暂无可用模型',
  'availableChannels.catalog.noModelsInGroup': '该分组暂无模型',
  'availableChannels.catalog.rateFallback': '专属倍率暂不可用，当前显示默认倍率',
  'availableChannels.catalog.loading': '正在加载可用渠道',
  'availableChannels.catalog.refreshing': '正在更新渠道数据',
  'availableChannels.catalog.empty': '暂无可用渠道',
  'availableChannels.catalog.noChannels': '暂无可用渠道',
  'availableChannels.catalog.noMatchingResults': '没有匹配的渠道或模型',
  'availableChannels.catalog.selectChannel': '选择渠道',
  'availableChannels.catalog.channelNavigation': '渠道导航',
  'availableChannels.catalog.publicGroup': '公开',
  'availableChannels.catalog.exclusiveGroup': '专属',
  'availableChannels.catalog.subscriptionGroup': '订阅',
  'availableChannels.catalog.modelColumn': '模型',
  'availableChannels.catalog.detailsColumn': '详情',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        const value = labels[key] ?? key
        return Object.entries(params ?? {}).reduce(
          (result, [name, replacement]) => result.replace(`{${name}}`, String(replacement)),
          value,
        )
      },
    }),
  }
})

const testDirectory = dirname(fileURLToPath(import.meta.url))
const catalogPath = resolve(testDirectory, '../AvailableChannelCatalog.vue')
const groupSectionPath = resolve(testDirectory, '../AvailableChannelGroupSection.vue')
const modelPricePath = resolve(testDirectory, '../AvailableChannelModelPrice.vue')
const desktopPriceGrid = 'xl:grid-cols-[minmax(0,1.35fr)_minmax(0,1fr)_minmax(0,1fr)_minmax(88px,0.5fr)_minmax(72px,0.4fr)]'

afterEach(() => {
  document.body.innerHTML = ''
})

function emptyPrices(): CatalogPriceCollection {
  return {
    input: null,
    output: null,
    cacheWrite: null,
    cacheRead: null,
    imageInput: null,
    imageOutput: null,
    request: null,
  }
}

function model(key: string, groupKey: string, name: string, platform = 'openai'): CatalogModelEntry {
  return {
    key,
    groupKey,
    name,
    platform,
    billingMode: 'token',
    hasPricing: true,
    normalRate: 0.5,
    defaultRate: 0.8,
    userRate: 0.5,
    peakFactor: null,
    prices: {
      ...emptyPrices(),
      input: { official: 0.000001, site: 0.0000036, peakSite: null },
      output: { official: 0.000004, site: 0.0000144, peakSite: null },
    },
    intervals: [],
  }
}

function group(
  key: string,
  channelKey: string,
  name: string,
  models: CatalogModelEntry[],
  overrides: Partial<CatalogGroupEntry> = {},
): CatalogGroupEntry {
  return {
    key,
    channelKey,
    id: Number(key.replace(/\D/g, '')) || 1,
    name,
    platform: 'openai',
    subscriptionType: 'standard',
    isExclusive: false,
    normalRate: 0.5,
    defaultRate: 0.8,
    userRate: 0.5,
    peak: null,
    models,
    modelCount: models.length,
    ...overrides,
  }
}

const alphaKey = 'channel:alpha'
const alphaGroupOneKey = `${alphaKey}:platform:openai:group:1`
const alphaGroupTwoKey = `${alphaKey}:platform:anthropic:group:2`
const alphaGroupOne = group(
  alphaGroupOneKey,
  alphaKey,
  'Very long OpenAI primary group name that must wrap safely on narrow screens',
  [model(`${alphaGroupOneKey}:model:shared:0`, alphaGroupOneKey, 'shared-model')],
  {
    isExclusive: true,
    subscriptionType: 'subscription',
    peak: { enabled: true, start: '08:00', end: '10:00', factor: 1.5 },
  },
)
const alphaGroupTwo = group(
  alphaGroupTwoKey,
  alphaKey,
  'Anthropic public',
  [model(`${alphaGroupTwoKey}:model:shared:0`, alphaGroupTwoKey, 'shared-model', 'anthropic')],
  { platform: 'anthropic', normalRate: 1, defaultRate: 1, userRate: null },
)
const betaKey = 'channel:beta'
const betaEmptyGroup = group(
  `${betaKey}:platform:gemini:group:3`,
  betaKey,
  'Gemini empty',
  [],
  { platform: 'gemini', normalRate: 1, defaultRate: 1, userRate: null },
)

const channels: CatalogChannelEntry[] = [
  {
    key: alphaKey,
    name: 'Alpha channel with an exceptionally long name that must wrap',
    description: 'Primary route with a long description that remains readable without widening the page.',
    platforms: ['openai', 'anthropic'],
    groups: [alphaGroupOne, alphaGroupTwo],
    groupCount: 2,
    modelCount: 2,
  },
  {
    key: betaKey,
    name: 'Beta channel',
    description: 'Secondary route',
    platforms: ['gemini'],
    groups: [betaEmptyGroup],
    groupCount: 1,
    modelCount: 0,
  },
]

function installPinia() {
  const pinia = createPinia()
  useAppStore(pinia).$patch({ cachedPublicSettings: { server_utc_offset: '+08:00' } })
  return pinia
}

async function mountCatalog(
  props: Partial<{
    channels: CatalogChannelEntry[]
    loading: boolean
    refreshing: boolean
    rateFallback: boolean
    emptyKind: 'no-data' | 'no-results'
    modelEntries: CatalogModelListEntry[]
  }> = {},
  slots: Record<string, string> = {},
): Promise<VueWrapper> {
  const component = (await import('../AvailableChannelCatalog.vue')).default
  return mount(component, {
    attachTo: document.body,
    props: {
      channels,
      loading: false,
      refreshing: false,
      rateFallback: false,
      emptyKind: 'no-data',
      ...props,
    },
    slots,
    global: {
      plugins: [installPinia()],
      stubs: {
        PlatformIcon: {
          props: ['platform'],
          template: '<i data-platform-icon :data-platform="platform" />',
        },
        GroupBadge: {
          props: ['name', 'platform', 'subscriptionType', 'rateMultiplier', 'userRateMultiplier'],
          template:
            '<span data-group-badge :data-platform="platform" :data-subscription="subscriptionType" :data-default-rate="rateMultiplier" :data-user-rate="userRateMultiplier">{{ name }}</span>',
        },
        AvailableChannelModelPrice: {
          props: ['model'],
          template: '<article data-model-price :data-model-key="model.key">{{ model.name }}</article>',
        },
      },
    },
  })
}

function rawChannelWithSharedGroup(name: string, description: string): UserAvailableChannel {
  return {
    name,
    description,
    platforms: [{
      platform: 'openai',
      groups: [{
        id: 77,
        name: `${name} group`,
        platform: 'openai',
        subscription_type: 'standard',
        rate_multiplier: 1,
        peak_rate_enabled: false,
        peak_start: '',
        peak_end: '',
        peak_rate_multiplier: 1,
        is_exclusive: false,
        supported_models: [{ name: 'shared-model', platform: 'openai', pricing: null }],
      }],
      supported_models: [],
    }],
  }
}

describe('AvailableChannelCatalog', () => {
  it('provides the catalog and group-section components', () => {
    expect(existsSync(catalogPath)).toBe(true)
    expect(existsSync(groupSectionPath)).toBe(true)
  })

  it.runIf(existsSync(catalogPath))('selects the first channel and changes detail on click', async () => {
    const wrapper = await mountCatalog()
    const options = wrapper.findAll('[data-testid="channel-nav-item"]')

    expect(options).toHaveLength(2)
    expect(options[0].attributes('aria-selected')).toBe('true')
    expect(options[1].attributes('aria-selected')).toBe('false')
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Alpha channel')

    await options[1].trigger('click')
    expect(options[0].attributes('aria-selected')).toBe('false')
    expect(options[1].attributes('aria-selected')).toBe('true')
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Beta channel')
    expect(wrapper.get('[data-testid="channel-detail"]').text()).not.toContain('Alpha channel')
  })

  it.runIf(existsSync(catalogPath))('shows channel name, platforms, group count, and model count in each nav item', async () => {
    const wrapper = await mountCatalog()
    const options = wrapper.findAll('[data-testid="channel-nav-item"]')

    expect(options[0].text()).toContain('Alpha channel')
    expect(options[0].text()).toContain('openai')
    expect(options[0].text()).toContain('anthropic')
    expect(options[0].text()).toContain('2 个分组')
    expect(options[0].text()).toContain('2 个模型')
    expect(options[1].text()).toContain('gemini')
    expect(options[1].text()).toContain('1 个分组')
    expect(options[1].text()).toContain('0 个模型')
  })

  it.runIf(existsSync(catalogPath))('preserves selection across cloned refreshes and unrelated insertions', async () => {
    const wrapper = await mountCatalog()
    await wrapper.findAll('[data-testid="channel-nav-item"]')[1].trigger('click')

    await wrapper.setProps({ channels: structuredClone(channels) })
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Beta channel')

    const inserted: CatalogChannelEntry = {
      key: 'channel:inserted',
      name: 'Inserted channel',
      description: '',
      platforms: ['openai'],
      groups: [],
      groupCount: 0,
      modelCount: 0,
    }
    await wrapper.setProps({ channels: [inserted, ...structuredClone(channels)] })
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Beta channel')
    expect(wrapper.findAll('[data-testid="channel-nav-item"]')[2].attributes('aria-selected')).toBe('true')
  })

  it.runIf(existsSync(catalogPath))('falls back when selection disappears and clears selection when empty', async () => {
    const wrapper = await mountCatalog()
    await wrapper.findAll('[data-testid="channel-nav-item"]')[1].trigger('click')

    await wrapper.setProps({ channels: [structuredClone(channels[0])] })
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Alpha channel')

    await wrapper.setProps({ channels: [] })
    expect(wrapper.find('[data-testid="channel-detail"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="catalog-empty"]').text()).toContain('暂无可用渠道')
  })

  it.runIf(existsSync(catalogPath))('renders duplicate model names once in each source group', async () => {
    const wrapper = await mountCatalog()
    const prices = wrapper.findAll('[data-model-price]')

    expect(prices).toHaveLength(2)
    expect(prices.map((entry) => entry.text())).toEqual(['shared-model', 'shared-model'])
    expect(new Set(prices.map((entry) => entry.attributes('data-model-key'))).size).toBe(2)
  })

  it.runIf(existsSync(catalogPath))('renders a distinct state for a raw empty group', async () => {
    const wrapper = await mountCatalog()
    await wrapper.findAll('[data-testid="channel-nav-item"]')[1].trigger('click')
    expect(wrapper.get('[data-testid="group-empty"]').text()).toContain('该分组暂无模型')
    expect(wrapper.find('[data-model-price]').exists()).toBe(false)
  })

  it.runIf(existsSync(catalogPath))('recomputes model metadata after selecting one channel', async () => {
    const sharedModelEntries: CatalogModelListEntry[] = [{
      key: 'shared', name: 'shared', offerings: [
        { key: 'a', channelKey: alphaKey, channelName: 'Alpha', groupKey: 'ga', groupName: 'GA', platform: 'openai', hasPricing: false, model: model('a', 'ga', 'shared'), prices: emptyPrices(), intervals: [] },
        { key: 'b', channelKey: betaKey, channelName: 'Beta', groupKey: 'gb', groupName: 'GB', platform: 'gemini', hasPricing: true, model: model('b', 'gb', 'shared', 'gemini'), prices: emptyPrices(), intervals: [] },
      ], channelCount: 2, groupCount: 2, platforms: ['gemini', 'openai'], hasPricedOffering: true,
    }]
    sharedModelEntries[0].offerings[0].model.hasPricing = false
    const projected = projectModelEntriesForChannel(sharedModelEntries, alphaKey)
    expect(projected[0]).toMatchObject({ channelCount: 1, groupCount: 1, platforms: ['openai'], hasPricedOffering: false })

    const wrapper = await mountCatalog({ modelEntries: sharedModelEntries })
    expect(wrapper.get('[data-testid="model-card"]').text()).toContain('1 个渠道')
    expect(wrapper.get('[data-testid="model-card"]').text()).toContain('1 个分组')
    expect(wrapper.get('[data-testid="model-card"]').text()).not.toContain('gemini')
  })

  it.runIf(existsSync(catalogPath))('shows an explicit model-list empty state for a selected channel with no models', async () => {
    const wrapper = await mountCatalog({ modelEntries: [] })
    expect(wrapper.get('[data-testid="channel-detail-name"]').text()).toContain('Alpha')
    expect(wrapper.get('[data-testid="model-list-empty"]').text()).toContain('该渠道暂无可用模型')
  })

  it.runIf(existsSync(catalogPath))('aligns toolbar, navigation, and detail on one responsive grid', async () => {
    const wrapper = await mountCatalog({}, { toolbar: '<div>toolbar fixture</div>' })
    const layout = wrapper.get('[data-testid="channel-catalog-layout"]')
    const heading = wrapper.get('[data-testid="channel-navigation-heading"]')
    const toolbar = wrapper.get('[data-testid="channel-toolbar-region"]')
    const rail = wrapper.get('[data-testid="channel-navigation"]')
    const detail = wrapper.get('[data-testid="channel-detail"]')

    expect(layout.classes()).toContain('xl:grid')
    expect(layout.classes()).toContain('xl:grid-cols-[260px_minmax(0,1fr)]')
    expect(heading.classes()).toEqual(expect.arrayContaining(['hidden', 'xl:block', 'xl:col-start-1']))
    expect(heading.text()).toContain('渠道导航')
    expect(heading.text()).toContain('2 个渠道')
    expect(toolbar.classes()).toContain('xl:col-start-2')
    expect(toolbar.text()).toContain('toolbar fixture')
    expect(rail.classes()).toContain('hidden')
    expect(rail.classes()).toContain('xl:block')
    expect(rail.classes()).toContain('xl:col-start-1')
    expect(rail.classes()).toContain('xl:sticky')
    expect(rail.classes()).toContain('xl:overflow-y-auto')
    expect(detail.classes()).toContain('xl:col-start-2')
    expect(wrapper.get('[data-testid="channel-picker-trigger"]').classes()).toContain('xl:hidden')
    expect(wrapper.findAll('[data-testid="channel-detail"]')).toHaveLength(1)
  })

  it.runIf(existsSync(catalogPath))('mounts directly on the TablePageLayout scroll-host contract', async () => {
    const wrapper = await mountCatalog()
    const host = wrapper.get('[data-testid="catalog-scroll-host"]')
    const rail = wrapper.get('[data-testid="channel-navigation"]')
    const source = readFileSync(catalogPath, 'utf8')

    expect(wrapper.element).toBe(host.element)
    expect(host.classes()).toContain('table-wrapper')
    expect(host.classes()).not.toContain('h-full')
    expect(host.classes()).toContain('min-h-0')
    expect(host.classes()).not.toContain('[container-type:size]')
    expect(host.classes()).toContain('xl:h-full')
    expect(host.classes()).toContain('xl:[container-type:size]')
    expect(rail.classes()).toContain('xl:max-h-[calc(100cqh-2rem)]')
    expect(source).not.toMatch(/100(?:d|s|l)?vh/)
  })

  it.runIf(existsSync(catalogPath))('opens only the first mobile group and keeps all bodies desktop-visible', async () => {
    const wrapper = await mountCatalog()
    const toggles = wrapper.findAll('[data-testid="group-toggle"]')
    const bodies = wrapper.findAll('[data-testid="group-body"]')

    expect(toggles).toHaveLength(2)
    expect(toggles[0].attributes('aria-expanded')).toBe('true')
    expect(toggles[1].attributes('aria-expanded')).toBe('false')
    expect(toggles[0].classes()).toContain('min-h-11')
    expect(toggles[0].attributes('aria-controls')).toMatch(/^available-channel-group-[a-z0-9-]+$/)
    expect(bodies[0].classes()).not.toContain('hidden')
    expect(bodies[1].classes()).toContain('hidden')
    expect(bodies.every((body) => body.classes().includes('xl:grid'))).toBe(true)

    await toggles[1].trigger('click')
    expect(toggles[1].attributes('aria-expanded')).toBe('true')
    expect(wrapper.findAll('[data-testid="group-body"]')[1].classes()).not.toContain('hidden')
  })

  it.runIf(existsSync(catalogPath))('separates the mobile accordion from a noninteractive desktop header', async () => {
    const wrapper = await mountCatalog()
    const groups = wrapper.findAll('[data-testid="channel-group"]')

    expect(groups).toHaveLength(2)
    for (const group of groups) {
      const mobileToggle = group.get('[data-testid="group-toggle"]')
      const desktopHeader = group.get('[data-testid="group-desktop-header"]')

      expect(mobileToggle.element.tagName).toBe('BUTTON')
      expect(mobileToggle.classes()).toContain('xl:hidden')
      expect(desktopHeader.element.tagName).toBe('HEADER')
      expect(desktopHeader.classes()).toContain('hidden')
      expect(desktopHeader.classes()).toContain('xl:flex')
      expect(desktopHeader.find('button').exists()).toBe(false)
    }

    const secondToggle = groups[1].get('[data-testid="group-toggle"]')
    expect(secondToggle.attributes('aria-expanded')).toBe('false')
    await secondToggle.trigger('click')
    expect(secondToggle.attributes('aria-expanded')).toBe('true')
  })

  it.runIf(existsSync(catalogPath))('renders one desktop price heading row per group', async () => {
    const wrapper = await mountCatalog()
    const headings = wrapper.findAll('[data-testid="desktop-price-columns"]')
    const bodies = wrapper.findAll('[data-testid="group-body"]')

    expect(headings).toHaveLength(2)
    for (const heading of headings) {
      expect(heading.classes()).toContain('hidden')
      expect(heading.classes()).toContain('xl:grid')
      expect(heading.classes()).toContain(desktopPriceGrid)
      expect(heading.text()).toContain('模型')
      expect(heading.text()).toContain('官方价')
      expect(heading.text()).toContain('本站价')
      expect(heading.text()).toContain('实际倍率')
      expect(heading.text()).toContain('详情')
    }
    expect(bodies).toHaveLength(2)
    expect(bodies.every((body) => !body.classes().includes('2xl:grid-cols-2'))).toBe(true)
  })

  it.runIf(existsSync(catalogPath))('uses xl consistently for catalog, group, and model desktop surfaces', () => {
    const source = [catalogPath, groupSectionPath, modelPricePath]
      .map((path) => readFileSync(path, 'utf8'))
      .join('\n')

    expect(source).not.toMatch(/\blg:/)
    expect(source).toContain('xl:grid-cols-[260px_minmax(0,1fr)]')
    expect(source.match(/xl:grid-cols-\[minmax\(0,1\.35fr\)_minmax\(0,1fr\)_minmax\(0,1fr\)_minmax\(88px,0\.5fr\)_minmax\(72px,0\.4fr\)\]/g)).toHaveLength(2)
  })

  it.runIf(existsSync(catalogPath))('opens a group when it becomes first without resetting an unchanged user toggle', async () => {
    const wrapper = await mountCatalog()
    const secondToggle = wrapper.findAll('[data-testid="group-toggle"]')[1]

    expect(secondToggle.attributes('aria-expanded')).toBe('false')
    await secondToggle.trigger('click')
    expect(secondToggle.attributes('aria-expanded')).toBe('true')
    await wrapper.setProps({ channels: structuredClone(channels) })
    expect(wrapper.findAll('[data-testid="group-toggle"]')[1].attributes('aria-expanded')).toBe('true')

    const reorderedWrapper = await mountCatalog()
    const reordered = structuredClone(channels)
    reordered[0].groups = [reordered[0].groups[1], reordered[0].groups[0]]
    expect(reorderedWrapper.findAll('[data-testid="group-toggle"]')[1].attributes('aria-expanded')).toBe('false')
    await reorderedWrapper.setProps({ channels: reordered })
    const reorderedToggles = reorderedWrapper.findAll('[data-testid="group-toggle"]')
    expect(reorderedToggles[0].attributes('aria-expanded')).toBe('true')
    expect(reorderedToggles[1].attributes('aria-expanded')).toBe('false')
  })

  it.runIf(existsSync(catalogPath))('preserves manual accordion choices when the default group changes', async () => {
    const manuallyOpen = await mountCatalog()
    let toggles = manuallyOpen.findAll('[data-testid="group-toggle"]')
    await toggles[0].trigger('click')
    await toggles[0].trigger('click')
    const reorderedOpen = structuredClone(channels)
    reorderedOpen[0].groups = [reorderedOpen[0].groups[1], reorderedOpen[0].groups[0]]
    await manuallyOpen.setProps({ channels: reorderedOpen })
    toggles = manuallyOpen.findAll('[data-testid="group-toggle"]')
    expect(toggles[0].attributes('aria-expanded')).toBe('true')
    expect(toggles[1].attributes('aria-expanded')).toBe('true')

    const manuallyClosed = await mountCatalog()
    toggles = manuallyClosed.findAll('[data-testid="group-toggle"]')
    await toggles[1].trigger('click')
    await toggles[1].trigger('click')
    const reorderedClosed = structuredClone(channels)
    reorderedClosed[0].groups = [reorderedClosed[0].groups[1], reorderedClosed[0].groups[0]]
    await manuallyClosed.setProps({ channels: reorderedClosed })
    toggles = manuallyClosed.findAll('[data-testid="group-toggle"]')
    expect(toggles[0].attributes('aria-expanded')).toBe('false')
    expect(toggles[1].attributes('aria-expanded')).toBe('false')
  })

  it.runIf(existsSync(catalogPath))('isolates mobile group expansion when channels share a group id and platform', async () => {
    const sharedIdChannels = buildAvailableChannelCatalog([
      rawChannelWithSharedGroup('Channel A', 'First route'),
      rawChannelWithSharedGroup('Channel B', 'Second route'),
    ], {}, 7.2)
    const wrapper = await mountCatalog({ channels: sharedIdChannels })

    const firstToggle = wrapper.get('[data-testid="group-toggle"]')
    expect(firstToggle.attributes('aria-expanded')).toBe('true')
    await firstToggle.trigger('click')
    expect(firstToggle.attributes('aria-expanded')).toBe('false')

    await wrapper.findAll('[data-testid="channel-nav-item"]')[1].trigger('click')
    expect(wrapper.get('[data-testid="group-toggle"]').attributes('aria-expanded')).toBe('true')

    await wrapper.findAll('[data-testid="channel-nav-item"]')[0].trigger('click')
    expect(wrapper.get('[data-testid="group-toggle"]').attributes('aria-expanded')).toBe('true')
  })

  it.runIf(existsSync(catalogPath))('labels channel and group regions with one semantic heading each', async () => {
    const wrapper = await mountCatalog()
    const detail = wrapper.get('[data-testid="channel-detail"]')
    const detailHeading = wrapper.get('[data-testid="channel-detail-name"]')
    const groups = wrapper.findAll('[data-testid="channel-group"]')

    expect(detail.element.tagName).toBe('SECTION')
    expect(detail.attributes('aria-labelledby')).toBe(detailHeading.attributes('id'))
    for (const group of groups) {
      const headings = group.findAll('[data-testid="group-semantic-heading"]')
      expect(headings).toHaveLength(1)
      expect(headings[0].element.tagName).toBe('H3')
      expect(group.attributes('aria-labelledby')).toBe(headings[0].attributes('id'))
      expect(group.findAll('h3')).toHaveLength(1)
    }
  })

  it.runIf(existsSync(groupSectionPath) && existsSync(modelPricePath))('renders a real h3 group, h4 model, and h5 price outline', async () => {
    const component = (await import('../AvailableChannelGroupSection.vue')).default
    const semanticGroupKey = 'channel:outline:platform:openai:group:88'
    const semanticModel = model(
      `${semanticGroupKey}:model:outline:0`,
      semanticGroupKey,
      'Outline model',
    )
    semanticModel.prices.cacheRead = { official: 0.0000001, site: 0.00000036, peakSite: null }
    semanticModel.intervals = [{
      key: `${semanticModel.key}:interval:0`,
      minTokens: 0,
      maxTokens: 1000,
      tierLabel: 'Short context',
      prices: semanticModel.prices,
    }]
    const semanticGroup = group(
      semanticGroupKey,
      'channel:outline',
      'Outline group',
      [semanticModel],
    )
    const wrapper = mount(component, {
      attachTo: document.body,
      props: { group: semanticGroup, defaultExpanded: true },
      global: {
        plugins: [installPinia()],
        stubs: {
          PlatformIcon: true,
          GroupBadge: {
            props: ['name'],
            template: '<span>{{ name }}</span>',
          },
        },
      },
    })

    expect(wrapper.get('[data-testid="group-semantic-heading"]').element.tagName).toBe('H3')
    const priceRow = wrapper.get('[data-testid="model-price-row"]')
    expect(priceRow.get('header h4').text()).toBe('Outline model')
    expect(priceRow.get('[data-testid="official-price"] h5').exists()).toBe(true)
    expect(priceRow.get('[data-testid="site-price"] h5').exists()).toBe(true)
    expect(priceRow.find('h3').exists()).toBe(false)

    await priceRow.get('[data-testid="price-detail-toggle"]').trigger('click')
    expect(priceRow.get('[data-testid="price-details"]').findAll('h5').length).toBeGreaterThan(0)
    expect(priceRow.get('[data-testid="price-details"]').find('h4').exists()).toBe(false)
  })

  it.runIf(existsSync(catalogPath))('shows group rates, access, subscription, and timezone-aware peak window', async () => {
    const wrapper = await mountCatalog()
    const firstGroup = wrapper.findAll('[data-testid="channel-group"]')[0]
    const badge = firstGroup.get('[data-group-badge]')

    expect(badge.attributes('data-platform')).toBe('openai')
    expect(badge.attributes('data-subscription')).toBe('subscription')
    expect(badge.attributes('data-default-rate')).toBe('0.8')
    expect(badge.attributes('data-user-rate')).toBe('0.5')
    expect(firstGroup.text()).toContain('专属')
    expect(firstGroup.text()).toContain('订阅')
    expect(firstGroup.text()).toContain('1 个模型')
    expect(firstGroup.text()).toContain('08:00-10:00 ×1.5 (UTC+08:00)')
    expect(wrapper.findAll('[data-testid="channel-group"]')[1].text()).toContain('公开')
  })

  it.runIf(existsSync(catalogPath))('supports ArrowUp, ArrowDown, Home, and End with focus movement', async () => {
    const wrapper = await mountCatalog()
    let options = wrapper.findAll('[data-testid="channel-nav-item"]')
    options[0].element.focus()

    await options[0].trigger('keydown', { key: 'ArrowDown' })
    await flushPromises()
    options = wrapper.findAll('[data-testid="channel-nav-item"]')
    expect(options[1].attributes('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(options[1].element)

    await options[1].trigger('keydown', { key: 'ArrowUp' })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="channel-nav-item"]')[0].attributes('aria-selected')).toBe('true')

    await wrapper.findAll('[data-testid="channel-nav-item"]')[0].trigger('keydown', { key: 'End' })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="channel-nav-item"]')[1].attributes('aria-selected')).toBe('true')

    await wrapper.findAll('[data-testid="channel-nav-item"]')[1].trigger('keydown', { key: 'Home' })
    await flushPromises()
    expect(wrapper.findAll('[data-testid="channel-nav-item"]')[0].attributes('aria-selected')).toBe('true')
  })

  it.runIf(existsSync(catalogPath))('renders structural loading and explicit empty states', async () => {
    const loading = await mountCatalog({ channels: [], loading: true })
    expect(loading.get('[data-testid="catalog-loading-rail"]')).toBeTruthy()
    expect(loading.get('[data-testid="catalog-loading-detail"]')).toBeTruthy()
    expect(loading.text()).toContain('正在加载可用渠道')

    const noData = await mountCatalog({ channels: [], loading: false })
    expect(noData.get('[data-testid="catalog-empty"]').text()).toContain('暂无可用渠道')
    expect(noData.find('[data-testid="catalog-loading"]').exists()).toBe(false)

    const noResults = await mountCatalog({
      channels: [],
      loading: false,
      emptyKind: 'no-results',
    })
    expect(noResults.get('[data-testid="catalog-empty"]').text()).toContain('没有匹配的渠道或模型')
  })

  it.runIf(existsSync(catalogPath))('shares mobile picker selection with the desktop rail and detail', async () => {
    const wrapper = await mountCatalog()
    expect(wrapper.findAll('[data-testid="channel-picker-trigger"]')).toHaveLength(1)
    await wrapper.get('[data-testid="channel-picker-trigger"]').trigger('click')
    await flushPromises()
    const options = document.body.querySelectorAll<HTMLButtonElement>('[data-testid="channel-picker-option"]')
    options[1].click()
    await flushPromises()
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Beta channel')
    expect(wrapper.findAll('[data-testid="channel-nav-item"]')[1].attributes('aria-selected')).toBe('true')
    expect(document.body.querySelector('[data-testid="channel-picker-dialog"]')).toBeNull()
  })

  it.runIf(existsSync(catalogPath))('closes the mobile picker when its selected channel disappears', async () => {
    const wrapper = await mountCatalog()
    await wrapper.get('[data-testid="channel-picker-trigger"]').trigger('click')
    await flushPromises()
    expect(document.body.querySelector('[data-testid="channel-picker-dialog"]')).not.toBeNull()
    await wrapper.setProps({ channels: [structuredClone(channels[1])] })
    await flushPromises()
    expect(document.body.querySelector('[data-testid="channel-picker-dialog"]')).toBeNull()
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Beta channel')
  })

  it.runIf(existsSync(catalogPath))('restores focus to fallback only when a removed option owned focus', async () => {
    const wrapper = await mountCatalog()
    const options = wrapper.findAll('[data-testid="channel-nav-item"]')
    await options[1].trigger('click')
    options[1].element.focus()
    expect(document.activeElement).toBe(options[1].element)

    await wrapper.setProps({ channels: [structuredClone(channels[0])] })
    await flushPromises()

    const fallback = wrapper.get('[data-testid="channel-nav-item"]')
    expect(fallback.attributes('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(fallback.element)
  })

  it.runIf(existsSync(catalogPath))('does not steal external focus when selection falls back after refresh', async () => {
    const wrapper = await mountCatalog()
    await wrapper.findAll('[data-testid="channel-nav-item"]')[1].trigger('click')
    const externalInput = document.createElement('input')
    document.body.append(externalInput)
    externalInput.focus()

    await wrapper.setProps({ channels: [structuredClone(channels[0])] })
    await flushPromises()

    expect(wrapper.get('[data-testid="channel-nav-item"]').attributes('aria-selected')).toBe('true')
    expect(document.activeElement).toBe(externalInput)
  })

  it.runIf(existsSync(catalogPath))('keeps content mounted while refreshing and exposes rate fallback', async () => {
    const wrapper = await mountCatalog({ refreshing: true, rateFallback: true })

    expect(wrapper.get('[data-testid="channel-catalog-layout"]').attributes('aria-busy')).toBe('true')
    expect(wrapper.get('[data-testid="refreshing-indicator"]').text()).toContain('正在更新渠道数据')
    expect(wrapper.get('[data-testid="rate-fallback-warning"]').text()).toContain('默认倍率')
    expect(wrapper.get('[data-testid="channel-detail"]').text()).toContain('Alpha channel')
    expect(wrapper.findAll('[data-testid="channel-nav-item"]')).toHaveLength(2)
  })

  it.runIf(existsSync(catalogPath))('protects long channel, description, and group text from overflow', async () => {
    const wrapper = await mountCatalog()

    expect(wrapper.get('[data-testid="channel-detail"]').classes()).toContain('min-w-0')
    expect(wrapper.get('[data-testid="channel-detail-name"]').classes()).toContain('[overflow-wrap:anywhere]')
    expect(wrapper.get('[data-testid="channel-description"]').classes()).toContain('break-words')
    expect(wrapper.get('[data-testid="group-toggle"]').classes()).toContain('min-w-0')
  })

  it.runIf(existsSync(catalogPath))('opts animations out for reduced motion and contains large offscreen groups safely', async () => {
    const loading = await mountCatalog({ channels: [], loading: true })
    expect(loading.findAll('.animate-pulse').every((item) => item.classes().includes('motion-reduce:animate-none'))).toBe(true)

    const wrapper = await mountCatalog()
    expect(wrapper.get('[data-testid="channel-nav-item"]').classes()).toContain('motion-reduce:transition-none')
    expect(wrapper.get('[data-testid="group-toggle"] svg').classes()).toContain('motion-reduce:transition-none')
    expect(wrapper.get('[data-testid="channel-group"]').classes()).toContain('[content-visibility:auto]')
    expect(wrapper.get('[data-testid="channel-group"]').classes()).toContain('[contain-intrinsic-size:auto_480px]')
  })

  it.runIf(existsSync(catalogPath) && existsSync(groupSectionPath))('does not reintroduce old table, chips, popovers, or price calculation', () => {
    const source = `${readFileSync(catalogPath, 'utf8')}\n${readFileSync(groupSectionPath, 'utf8')}`

    expect(source).not.toMatch(/AvailableChannelsTable|SupportedModelChip|priceCnyMultiplier/)
    expect(source).not.toMatch(/buildAvailableChannelCatalog|filterAvailableChannelCatalog/)
    expect(source).not.toMatch(/popover|tooltip/i)
  })
})
