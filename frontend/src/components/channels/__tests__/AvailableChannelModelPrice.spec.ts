import { existsSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { mount, type VueWrapper } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { CatalogModelEntry, CatalogPriceCollection } from '../availableChannelCatalog'

const labels: Record<string, string> = {
  'availableChannels.catalog.officialPrice': '官方价',
  'availableChannels.catalog.sitePrice': '本站价',
  'availableChannels.catalog.effectiveRate': '实际倍率',
  'availableChannels.catalog.startingFrom': '起',
  'availableChannels.catalog.peakPrice': '高峰价格',
  'availableChannels.catalog.regularPrice': '常规价格',
  'availableChannels.catalog.showDetails': '展开价格详情',
  'availableChannels.catalog.hideDetails': '收起价格详情',
  'availableChannels.catalog.unpriced': '暂未定价',
  'availableChannels.catalog.cacheWrite': '缓存写入',
  'availableChannels.catalog.cacheRead': '缓存读取',
  'availableChannels.catalog.imageInput': '图片输入',
  'availableChannels.catalog.imageOutput': '图片输出',
  'availableChannels.catalog.input': '输入',
  'availableChannels.catalog.output': '输出',
  'availableChannels.catalog.perMillion': '/ 1M token',
  'availableChannels.catalog.perRequest': '/ 次',
  'availableChannels.catalog.perImage': '/ 张',
  'availableChannels.catalog.tieredPricing': '阶梯定价',
  'availableChannels.catalog.billingMode.token': '按 Token',
  'availableChannels.catalog.billingMode.per_request': '按次',
  'availableChannels.catalog.billingMode.image': '按图片',
  'availableChannels.catalog.billingMode.unknown': '未知计费',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (key === 'availableChannels.catalog.rangeBetween') {
          return `${params?.min}–${params?.max} token`
        }
        if (key === 'availableChannels.catalog.rangeFrom') {
          return `${params?.min}+ token`
        }
        return labels[key] ?? key
      },
    }),
  }
})

const componentPath = resolve(
  dirname(fileURLToPath(import.meta.url)),
  '../AvailableChannelModelPrice.vue',
)

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

function price(
  official: number | null,
  site: number | null,
  peakSite: number | null = null,
) {
  return { official, site, peakSite }
}

function makeModel(overrides: Partial<CatalogModelEntry> = {}): CatalogModelEntry {
  return {
    key: 'channel:test:group:1:model:gpt-test:0',
    groupKey: 'channel:test:group:1',
    name: 'gpt-test',
    platform: 'openai',
    billingMode: 'token',
    hasPricing: true,
    normalRate: 0.5,
    defaultRate: 0.5,
    userRate: null,
    peakFactor: null,
    prices: {
      ...emptyPrices(),
      input: price(0.000002, 0.000007),
      output: price(0.000008, 0.000028),
    },
    intervals: [],
    ...overrides,
  }
}

async function mountPrice(model: CatalogModelEntry): Promise<VueWrapper> {
  const component = (await import('../AvailableChannelModelPrice.vue')).default
  return mount(component, { props: { model } })
}

describe('AvailableChannelModelPrice', () => {
  it('exists as the shared price comparison component', () => {
    expect(existsSync(componentPath)).toBe(true)
  })

  it.runIf(existsSync(componentPath))(
    'shows token official and site input/output prices before interaction',
    async () => {
      const wrapper = await mountPrice(makeModel())

      expect(wrapper.text()).toContain('gpt-test')
      expect(wrapper.text()).toContain('按 Token')
      expect(wrapper.text()).toContain('实际倍率')
      expect(wrapper.text()).toContain('0.5×')

      const official = wrapper.get('[data-testid="official-price"]')
      const site = wrapper.get('[data-testid="site-price"]')
      expect(official.text()).toContain('输入')
      expect(official.text()).toContain('$2.00')
      expect(official.text()).toContain('输出')
      expect(official.text()).toContain('$8.00')
      expect(site.text()).toContain('输入')
      expect(site.text()).toContain('¥7.00')
      expect(site.text()).toContain('输出')
      expect(site.text()).toContain('¥28.00')
    },
  )

  it.runIf(existsSync(componentPath))('does not expose an empty detail control', async () => {
    const wrapper = await mountPrice(makeModel())

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it.runIf(existsSync(componentPath))('distinguishes unavailable site prices from true zero', async () => {
    const wrapper = await mountPrice(makeModel({
      prices: {
        ...emptyPrices(),
        input: price(0.000001, null),
        output: price(0, 0),
      },
    }))
    const site = wrapper.get('[data-testid="site-price"]')

    expect(site.text()).toContain('-')
    expect(site.text()).toContain('¥0.00')
    expect(site.text()).not.toContain('¥-')
  })

  it.runIf(existsSync(componentPath))('uses one request value and request units for per-request billing', async () => {
    const wrapper = await mountPrice(makeModel({
      billingMode: 'per_request',
      prices: { ...emptyPrices(), request: price(0.5, 2.25) },
    }))

    expect(wrapper.get('[data-testid="official-price"]').text()).toContain('$0.50 / 次')
    expect(wrapper.get('[data-testid="site-price"]').text()).toContain('¥2.25 / 次')
    expect(wrapper.get('[data-testid="official-price"]').text()).not.toContain('输入')
    expect(wrapper.get('[data-testid="site-price"]').text()).not.toContain('输出')
  })

  it.runIf(existsSync(componentPath))('uses one request value and image units for image billing', async () => {
    const wrapper = await mountPrice(makeModel({
      billingMode: 'image',
      prices: { ...emptyPrices(), request: price(0.08, 0.4) },
    }))

    expect(wrapper.get('[data-testid="official-price"]').text()).toContain('$0.08 / 张')
    expect(wrapper.get('[data-testid="site-price"]').text()).toContain('¥0.40 / 张')
  })

  it.runIf(existsSync(componentPath))('shows an inline unpriced state without a tooltip', async () => {
    const wrapper = await mountPrice(makeModel({
      billingMode: null,
      hasPricing: false,
      prices: emptyPrices(),
    }))

    expect(wrapper.get('[data-testid="unpriced-state"]').text()).toContain('暂未定价')
    expect(wrapper.find('[data-testid="official-price"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="site-price"]').exists()).toBe(false)
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it.runIf(existsSync(componentPath))('owns accessible open and close detail state with a safe DOM id', async () => {
    const wrapper = await mountPrice(makeModel({
      key: 'group:model:<unsafe id>/模型?#',
      prices: {
        ...emptyPrices(),
        input: price(0.000002, 0.000007),
        output: price(0.000008, 0.000028),
        cacheRead: price(0.0000002, 0.0000007),
      },
    }))
    const button = wrapper.get('button')
    const controls = button.attributes('aria-controls')

    expect(controls).toMatch(/^available-channel-model-price-[a-z0-9-]+$/)
    expect(controls).not.toContain('<')
    expect(button.classes()).toContain('min-h-11')
    expect(wrapper.find(`#${controls}`).exists()).toBe(false)

    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('true')
    expect(button.text()).toContain('收起价格详情')
    expect(wrapper.get(`#${controls}`).text()).toContain('缓存读取')

    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('false')
    expect(wrapper.find(`#${controls}`).exists()).toBe(false)
  })

  it.runIf(existsSync(componentPath))('shows available cache and peak prices only in expanded details', async () => {
    const wrapper = await mountPrice(makeModel({
      peakFactor: 1.8,
      prices: {
        ...emptyPrices(),
        input: price(0.000002, 0.000007, 0.0000126),
        output: price(0.000008, 0.000028, 0.0000504),
        cacheWrite: price(0.000001, 0.0000035, 0.0000063),
        cacheRead: price(0.0000002, 0.0000007, 0.00000126),
        imageInput: price(0.000003, 0.0000105, 0.0000189),
      },
    }))

    expect(wrapper.text()).not.toContain('缓存写入')
    expect(wrapper.get('[data-testid="site-price"]').text()).toContain('常规价格')
    expect(wrapper.get('[data-testid="site-price"]').text()).toContain('高峰价格')

    await wrapper.get('button').trigger('click')
    const details = wrapper.get('[data-testid="price-details"]')
    expect(details.text()).toContain('缓存写入')
    expect(details.text()).toContain('缓存读取')
    expect(details.text()).toContain('图片输入')
    expect(details.text()).not.toContain('图片输出')
    expect(details.text()).toContain('常规价格')
    expect(details.text()).toContain('高峰价格')
    expect(details.text()).toContain('¥6.30')
  })

  it.runIf(existsSync(componentPath))('shows the first tier as starting price and every tier in source order', async () => {
    const wrapper = await mountPrice(makeModel({
      prices: emptyPrices(),
      intervals: [
        {
          key: 'tier-a',
          minTokens: 0,
          maxTokens: 1_000,
          tierLabel: 'Starter',
          prices: {
            ...emptyPrices(),
            input: price(0.000001, 0.0000035),
            output: price(0.000004, 0.000014),
          },
        },
        {
          key: 'tier-b',
          minTokens: 1_001,
          maxTokens: 2_000,
          tierLabel: null,
          prices: {
            ...emptyPrices(),
            input: price(0.0000015, 0.00000525),
            output: price(0.000006, 0.000021),
          },
        },
        {
          key: 'tier-c',
          minTokens: 2_001,
          maxTokens: null,
          tierLabel: null,
          prices: {
            ...emptyPrices(),
            input: price(0.000002, 0.000007),
            output: price(0.000008, 0.000028),
          },
        },
      ],
    }))

    expect(wrapper.text()).toContain('阶梯定价')
    expect(wrapper.text()).toContain('起')
    expect(wrapper.get('[data-testid="official-price"]').text()).toContain('$1.00')
    expect(wrapper.get('[data-testid="site-price"]').text()).toContain('¥3.50')

    await wrapper.get('button').trigger('click')
    const tiers = wrapper.findAll('[data-testid="pricing-tier"]')
    expect(tiers).toHaveLength(3)
    expect(tiers.map((tier) => tier.text())).toEqual(expect.arrayContaining([
      expect.stringContaining('Starter'),
      expect.stringContaining('1,001–2,000 token'),
      expect.stringContaining('2,001+ token'),
    ]))
    expect(tiers[0].text()).toContain('$1.00')
    expect(tiers[2].text()).toContain('¥28.00')
  })

  it.runIf(existsSync(componentPath))('degrades unknown billing safely to available request pricing', async () => {
    const wrapper = await mountPrice(makeModel({
      billingMode: null,
      prices: { ...emptyPrices(), request: price(0.25, 1.75) },
    }))

    expect(wrapper.text()).toContain('未知计费')
    expect(wrapper.get('[data-testid="official-price"]').text()).toContain('$0.25 / 次')
    expect(wrapper.get('[data-testid="site-price"]').text()).toContain('¥1.75 / 次')
  })

  it.runIf(existsSync(componentPath))('uses one responsive semantic comparison surface without duplicate prices', async () => {
    const wrapper = await mountPrice(makeModel())
    const root = wrapper.get('article')
    const comparison = wrapper.get('[data-testid="price-comparison"]')

    expect(root.classes()).toContain('w-full')
    expect(root.classes()).toContain('overflow-hidden')
    expect(comparison.classes()).toContain('grid-cols-2')
    expect(comparison.classes()).toContain('lg:grid-cols-2')
    expect(wrapper.findAll('[data-testid="official-price"]')).toHaveLength(1)
    expect(wrapper.findAll('[data-testid="site-price"]')).toHaveLength(1)
    expect(wrapper.text().match(/\$2\.00/g)).toHaveLength(1)
    expect(wrapper.text().match(/¥7\.00/g)).toHaveLength(1)
  })

  it.runIf(existsSync(componentPath))('does not depend on legacy chips or overlay pricing', () => {
    const source = readFileSync(componentPath, 'utf8')
    expect(source).not.toMatch(/SupportedModelChip/)
    expect(source).not.toMatch(/PricingRow/)
    expect(source).not.toMatch(/Popover|Teleport/)
  })
})
