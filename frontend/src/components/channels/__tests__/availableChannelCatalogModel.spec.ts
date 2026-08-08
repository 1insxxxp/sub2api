import { describe, expect, it } from 'vitest'

import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserSupportedModel,
  UserSupportedModelPricing,
} from '@/api/channels'
import {
  buildAvailableChannelCatalog,
  filterAvailableChannelCatalog,
} from '../availableChannelCatalog'

function pricing(
  overrides: Partial<UserSupportedModelPricing> = {},
): UserSupportedModelPricing {
  return {
    billing_mode: 'token',
    input_price: 5,
    output_price: 10,
    cache_write_price: 2,
    cache_read_price: 1,
    image_input_price: 0.5,
    image_output_price: 0.75,
    per_request_price: null,
    intervals: [],
    ...overrides,
  }
}

function model(
  name: string,
  modelPricing: UserSupportedModelPricing | null,
  platform = 'anthropic',
): UserSupportedModel {
  return { name, platform, pricing: modelPricing }
}

function group(
  id: number,
  name: string,
  overrides: Partial<UserAvailableGroup> = {},
): UserAvailableGroup {
  return {
    id,
    name,
    platform: 'anthropic',
    subscription_type: 'standard',
    rate_multiplier: 1,
    peak_rate_enabled: false,
    peak_start: '',
    peak_end: '',
    peak_rate_multiplier: 1,
    is_exclusive: false,
    supported_models: [],
    ...overrides,
  }
}

function channel(
  name: string,
  groups: UserAvailableGroup[],
  overrides: Partial<UserAvailableChannel> = {},
): UserAvailableChannel {
  return {
    name,
    description: '',
    platforms: [
      {
        platform: 'anthropic',
        groups,
        supported_models: [],
      },
    ],
    ...overrides,
  }
}

describe('buildAvailableChannelCatalog', () => {
  it('normalizes token prices with user rate, peak prices, intervals, and group metadata', () => {
    const tokenPricing = pricing({
      per_request_price: 3,
      intervals: [
        {
          min_tokens: 0,
          max_tokens: 100_000,
          tier_label: 'Standard context',
          input_price: 4,
          output_price: 8,
          cache_write_price: 1.5,
          cache_read_price: 0.4,
          per_request_price: 2.5,
        },
      ],
    })
    const source = channel('Official Anthropic', [
      group(7, 'Pro', {
        subscription_type: 'subscription',
        rate_multiplier: 0.8,
        peak_rate_enabled: true,
        peak_start: '08:00',
        peak_end: '10:00',
        peak_rate_multiplier: 2,
        is_exclusive: true,
        supported_models: [model('claude-opus', tokenPricing)],
      }),
    ], { description: 'Fast premium route' })

    const catalog = buildAvailableChannelCatalog([source], { 7: 0.5 }, 7.2)
    const catalogChannel = catalog[0]
    const catalogGroup = catalogChannel.groups[0]
    const entry = catalogGroup.models[0]

    expect(catalogChannel).toMatchObject({
      name: 'Official Anthropic',
      description: 'Fast premium route',
      platforms: ['anthropic'],
      groupCount: 1,
      modelCount: 1,
    })
    expect(catalogGroup).toMatchObject({
      id: 7,
      name: 'Pro',
      platform: 'anthropic',
      subscriptionType: 'subscription',
      isExclusive: true,
      defaultRate: 0.8,
      userRate: 0.5,
      normalRate: 0.5,
      modelCount: 1,
      peak: { enabled: true, start: '08:00', end: '10:00', factor: 2 },
    })
    expect(entry).toMatchObject({
      name: 'claude-opus',
      platform: 'anthropic',
      billingMode: 'token',
      hasPricing: true,
      defaultRate: 0.8,
      userRate: 0.5,
      normalRate: 0.5,
      peakFactor: 2,
    })
    expect(entry.groupKey).toBe(catalogGroup.key)
    expect(entry.prices.input).toEqual({ official: 5, site: 18, peakSite: 36 })
    expect(entry.prices.output).toEqual({ official: 10, site: 36, peakSite: 72 })
    expect(entry.prices.cacheWrite?.site).toBeCloseTo(7.2)
    expect(entry.prices.cacheWrite?.peakSite).toBeCloseTo(14.4)
    expect(entry.prices.cacheRead?.site).toBeCloseTo(3.6)
    expect(entry.prices.cacheRead?.peakSite).toBeCloseTo(7.2)
    expect(entry.prices.imageInput?.site).toBeCloseTo(1.8)
    expect(entry.prices.imageInput?.peakSite).toBeCloseTo(3.6)
    expect(entry.prices.imageOutput?.site).toBeCloseTo(2.7)
    expect(entry.prices.imageOutput?.peakSite).toBeCloseTo(5.4)
    expect(entry.prices.request).toEqual({ official: 3, site: 10.8, peakSite: null })
    expect(entry.intervals[0]).toMatchObject({
      minTokens: 0,
      maxTokens: 100_000,
      tierLabel: 'Standard context',
    })
    expect(entry.intervals[0].prices.input?.site).toBeCloseTo(14.4)
    expect(entry.intervals[0].prices.input?.peakSite).toBeCloseTo(28.8)
    expect(entry.intervals[0].prices.output?.site).toBeCloseTo(28.8)
    expect(entry.intervals[0].prices.output?.peakSite).toBeCloseTo(57.6)
    expect(entry.intervals[0].prices.cacheWrite?.site).toBeCloseTo(5.4)
    expect(entry.intervals[0].prices.cacheWrite?.peakSite).toBeCloseTo(10.8)
    expect(entry.intervals[0].prices.cacheRead?.site).toBeCloseTo(1.44)
    expect(entry.intervals[0].prices.cacheRead?.peakSite).toBeCloseTo(2.88)
    expect(entry.intervals[0].prices.request).toEqual({
      official: 2.5,
      site: 9,
      peakSite: null,
    })
    expect(catalogChannel.key).toMatch(/^channel:/)
    expect(catalogGroup.channelKey).toBe(catalogChannel.key)
    expect(catalogGroup.key).toContain(catalogChannel.key)
    expect(entry.key).toContain(catalogGroup.key)
  })

  it('uses the default rate when the user has no override and preserves zero prices', () => {
    const source = channel('Default-rate channel', [
      group(8, 'Default rate', {
        rate_multiplier: 0.25,
        supported_models: [
          model('free-input', pricing({ input_price: 0, output_price: null })),
        ],
      }),
    ])

    const entry = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups[0].models[0]

    expect(entry.normalRate).toBe(0.25)
    expect(entry.userRate).toBeNull()
    expect(entry.prices.input).toEqual({ official: 0, site: 0, peakSite: null })
    expect(entry.prices.output).toBeNull()
  })

  it('disables site prices for zero or non-finite CNY multipliers while preserving official prices', () => {
    const source = channel('CNY disabled', [
      group(81, 'Published prices', {
        peak_rate_enabled: true,
        peak_rate_multiplier: 2,
        supported_models: [model('priced', pricing({ input_price: 5, output_price: 0 }))],
      }),
    ])

    for (const multiplier of [0, Number.NaN, Number.POSITIVE_INFINITY]) {
      const entry = buildAvailableChannelCatalog([source], {}, multiplier)[0].groups[0].models[0]

      expect(entry.prices.input).toEqual({ official: 5, site: null, peakSite: null })
      expect(entry.prices.output).toEqual({ official: 0, site: null, peakSite: null })
    }

    const validEntry = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups[0].models[0]
    expect(validEntry.prices.output).toEqual({ official: 0, site: 0, peakSite: 0 })
  })

  it('normalizes invalid prices and rates without exposing NaN or Infinity', () => {
    const invalidRaw = channel('Invalid raw prices', [
      group(82, 'Invalid raw', {
        supported_models: [model('broken-price', pricing({
          input_price: Number.NaN,
          output_price: Number.POSITIVE_INFINITY,
          cache_write_price: Number.MAX_VALUE,
        }))],
      }),
    ])
    const invalidRates = channel('Invalid rates', [
      group(83, 'Fallback rate', {
        rate_multiplier: 0.25,
        peak_rate_enabled: true,
        peak_rate_multiplier: Number.POSITIVE_INFINITY,
        supported_models: [model('valid-price', pricing())],
      }),
      group(84, 'Invalid default rate', {
        rate_multiplier: Number.POSITIVE_INFINITY,
        supported_models: [model('fallback-default', pricing())],
      }),
    ])

    const rawEntry = buildAvailableChannelCatalog([invalidRaw], {}, 7.2)[0].groups[0].models[0]
    const rateGroups = buildAvailableChannelCatalog([invalidRates], { 83: Number.NaN }, 7.2)[0].groups

    expect(rawEntry.prices.input).toBeNull()
    expect(rawEntry.prices.output).toBeNull()
    expect(rawEntry.prices.cacheWrite).toEqual({
      official: Number.MAX_VALUE,
      site: null,
      peakSite: null,
    })
    expect(rateGroups[0]).toMatchObject({ defaultRate: 0.25, userRate: null, normalRate: 0.25 })
    expect(rateGroups[0].peak).toMatchObject({ factor: 1 })
    expect(rateGroups[0].models[0].prices.input?.site).toBe(9)
    expect(rateGroups[0].models[0].prices.input?.peakSite).toBe(9)
    expect(rateGroups[1]).toMatchObject({ defaultRate: 1, userRate: null, normalRate: 1 })
    expect(rateGroups[1].models[0].prices.input?.site).toBe(36)
  })

  it('normalizes image and per-request prices without applying peak multipliers', () => {
    const source = channel('Media', [
      group(9, 'Media peak group', {
        rate_multiplier: 0.5,
        peak_rate_enabled: true,
        peak_rate_multiplier: 3,
        supported_models: [
          model('image-model', pricing({
            billing_mode: 'image',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            image_input_price: 0.2,
            image_output_price: 0.8,
            per_request_price: 1,
          })),
          model('request-model', pricing({
            billing_mode: 'per_request',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            image_input_price: null,
            image_output_price: null,
            per_request_price: 2,
            intervals: [
              {
                min_tokens: 0,
                max_tokens: null,
                input_price: null,
                output_price: null,
                cache_write_price: null,
                cache_read_price: null,
                per_request_price: 1.5,
              },
            ],
          })),
        ],
      }),
    ])

    const [imageEntry, requestEntry] = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups[0].models

    expect(imageEntry.peakFactor).toBeNull()
    expect(imageEntry.prices.imageInput?.site).toBeCloseTo(0.72)
    expect(imageEntry.prices.imageInput?.peakSite).toBeNull()
    expect(imageEntry.prices.imageOutput?.site).toBeCloseTo(2.88)
    expect(imageEntry.prices.imageOutput?.peakSite).toBeNull()
    expect(imageEntry.prices.request).toEqual({ official: 1, site: 3.6, peakSite: null })
    expect(requestEntry.peakFactor).toBeNull()
    expect(requestEntry.prices.request).toEqual({ official: 2, site: 7.2, peakSite: null })
    expect(requestEntry.intervals[0].prices.request?.official).toBe(1.5)
    expect(requestEntry.intervals[0].prices.request?.site).toBeCloseTo(5.4)
    expect(requestEntry.intervals[0].prices.request?.peakSite).toBeNull()
  })

  it('marks null and empty pricing payloads as unpriced without coercing missing values to zero', () => {
    const emptyPricing = pricing({
      input_price: null,
      output_price: null,
      cache_write_price: null,
      cache_read_price: null,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
    })
    const source = channel('Unpriced', [
      group(10, 'No prices', {
        supported_models: [
          model('no-payload', null),
          model('empty-payload', emptyPricing),
        ],
      }),
    ])

    const entries = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups[0].models

    expect(entries.map((entry) => entry.hasPricing)).toEqual([false, false])
    expect(entries[0].prices.input).toBeNull()
    expect(entries[1].prices.input).toBeNull()
  })

  it('preserves same-name models across groups and removes only exact duplicates within one group', () => {
    const shared = model('same-model', pricing())
    const source = channel('Duplicates', [
      group(11, 'Low rate', {
        rate_multiplier: 0.5,
        supported_models: [shared, { ...shared }, model('same-model', pricing({ input_price: 6 }))],
      }),
      group(12, 'High rate', {
        rate_multiplier: 2,
        supported_models: [shared],
      }),
    ])

    const groups = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups

    expect(groups[0].models).toHaveLength(2)
    expect(groups[1].models).toHaveLength(1)
    expect(groups[0].models[0].name).toBe('same-model')
    expect(groups[1].models[0].name).toBe('same-model')
    expect(groups[0].models[0].groupKey).not.toBe(groups[1].models[0].groupKey)
    expect(groups[0].models[0].prices.input?.site).toBe(18)
    expect(groups[1].models[0].prices.input?.site).toBe(72)
    expect(new Set(groups[0].models.map((entry) => entry.key)).size).toBe(2)
  })

  it('gives case and whitespace variants unique keys when they are not exact duplicates', () => {
    const source = channel('Key collisions', [
      group(15, 'Variants', {
        supported_models: [
          model('GPT-4', pricing()),
          model('gpt-4', pricing()),
          model(' GPT-4 ', pricing()),
        ],
      }),
    ])

    const entries = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups[0].models

    expect(entries.map((entry) => entry.name)).toEqual(['GPT-4', 'gpt-4', ' GPT-4 '])
    expect(new Set(entries.map((entry) => entry.key)).size).toBe(3)
  })

  it('resolves composite group and model platforms to the concrete section platform', () => {
    const composite = group(16, 'Composite route', {
      platform: 'composite',
      supported_models: [model('claude-composite', pricing(), '')],
    })
    const explicit = group(17, 'Explicit route', {
      platform: 'openai',
      supported_models: [model('openai-explicit', pricing(), '')],
    })
    const source = channel('Expanded platforms', [composite, explicit])

    const catalog = buildAvailableChannelCatalog([source], {}, 7.2)

    expect(catalog[0].groups[0].platform).toBe('anthropic')
    expect(catalog[0].groups[0].models[0].platform).toBe('anthropic')
    expect(catalog[0].groups[1].platform).toBe('openai')
    expect(catalog[0].groups[1].models[0].platform).toBe('openai')
    expect(filterAvailableChannelCatalog(catalog, {
      search: '', platform: 'anthropic', pricedOnly: false,
    })[0].groups.map((entry) => entry.id)).toEqual([16])
  })

  it('keeps channel, group, and model keys stable when unrelated rows are inserted or reordered', () => {
    const targetGroup = group(18, 'Target group', {
      supported_models: [model('target-model', pricing())],
    })
    const target = channel('Target channel', [targetGroup])
    const unrelated = channel('Unrelated channel', [
      group(19, 'Unrelated group', { supported_models: [model('other', pricing())] }),
    ])
    const baseTarget = buildAvailableChannelCatalog([target], {}, 7.2)[0]
    const insertedTarget = buildAvailableChannelCatalog([unrelated, target], {}, 7.2)[1]

    expect(insertedTarget.key).toBe(baseTarget.key)
    expect(insertedTarget.groups[0].key).toBe(baseTarget.groups[0].key)
    expect(insertedTarget.groups[0].models[0].key).toBe(baseTarget.groups[0].models[0].key)

    const sibling = group(20, 'Sibling', {
      platform: 'openai',
      supported_models: [model('sibling', pricing(), 'openai')],
    })
    const reordered = buildAvailableChannelCatalog([channel('Target channel', [sibling, targetGroup])], {}, 7.2)[0]
    const reorderedTarget = reordered.groups.find((entry) => entry.id === 18)
    expect(reorderedTarget?.key).toBe(baseTarget.groups[0].key)
    expect(reorderedTarget?.models[0].key).toBe(baseTarget.groups[0].models[0].key)

    const reversed = buildAvailableChannelCatalog([channel('Target channel', [targetGroup, sibling])], {}, 7.2)[0]
    const reversedTarget = reversed.groups.find((entry) => entry.id === 18)
    const removedSibling = buildAvailableChannelCatalog([target], {}, 7.2)[0]
    expect(reversed.key).toBe(baseTarget.key)
    expect(reversedTarget?.key).toBe(baseTarget.groups[0].key)
    expect(removedSibling.key).toBe(baseTarget.key)
    expect(removedSibling.groups[0].key).toBe(baseTarget.groups[0].key)
  })

  it('adds occurrences only for duplicate stable channel and group identities', () => {
    const duplicateGroupA = group(90, 'Duplicate A', { supported_models: [model('a', pricing())] })
    const duplicateGroupB = group(90, 'Duplicate B', { supported_models: [model('b', pricing())] })
    const duplicateChannelA = channel('Same channel', [duplicateGroupA, duplicateGroupB])
    const duplicateChannelB = channel(' same CHANNEL ', [group(91, 'Other', {
      supported_models: [model('c', pricing())],
    })])

    const catalog = buildAvailableChannelCatalog([duplicateChannelA, duplicateChannelB], {}, 7.2)
    const exactDuplicates = buildAvailableChannelCatalog([
      duplicateChannelA,
      structuredClone(duplicateChannelA),
    ], {}, 7.2)

    expect(new Set(catalog.map((entry) => entry.key)).size).toBe(2)
    expect(new Set(catalog[0].groups.map((entry) => entry.key)).size).toBe(2)
    expect(exactDuplicates[1].key).toBe(`${exactDuplicates[0].key}:occ:1`)
  })

  it('keeps case-variant channel keys stable when those channels reorder or grow', () => {
    const first = channel('Shared channel', [
      group(93, 'First route', { supported_models: [model('model-a', pricing())] }),
    ], { description: 'First upstream route' })
    const second = channel(' shared CHANNEL ', [
      group(94, 'Second route', { supported_models: [model('model-b', pricing({ input_price: 7 }))] }),
    ], { description: 'Second upstream route' })
    const inserted = channel('SHARED CHANNEL', [
      group(95, 'Inserted route', { supported_models: [model('model-c', pricing({ output_price: 12 }))] }),
    ], { description: 'Inserted upstream route' })

    const beforeDuplicateExists = buildAvailableChannelCatalog([first], {}, 7.2)
    const initial = buildAvailableChannelCatalog([first, second], {}, 7.2)
    const reordered = buildAvailableChannelCatalog([second, first], {}, 7.2)
    const grown = buildAvailableChannelCatalog([inserted, second, first], {}, 7.2)
    const initialByDescription = new Map(initial.map((entry) => [entry.description, entry.key]))
    const reorderedByDescription = new Map(reordered.map((entry) => [entry.description, entry.key]))
    const grownByDescription = new Map(grown.map((entry) => [entry.description, entry.key]))

    expect(initialByDescription.get('First upstream route')).toBe(beforeDuplicateExists[0].key)
    expect(reorderedByDescription.get('First upstream route')).toBe(
      initialByDescription.get('First upstream route'),
    )
    expect(reorderedByDescription.get('Second upstream route')).toBe(
      initialByDescription.get('Second upstream route'),
    )
    expect(grownByDescription.get('First upstream route')).toBe(
      initialByDescription.get('First upstream route'),
    )
    expect(grownByDescription.get('Second upstream route')).toBe(
      initialByDescription.get('Second upstream route'),
    )
    expect(new Set(initial.map((entry) => entry.key)).size).toBe(2)
    expect(new Set(grown.map((entry) => entry.key)).size).toBe(3)
  })

  it('keeps channel identity stable across pricing, rate, and model refreshes', () => {
    const before = channel('Mutable catalog data', [
      group(96, 'Stable group', {
        rate_multiplier: 0.5,
        supported_models: [model('model-a', pricing({ input_price: 5 }))],
      }),
    ], { description: 'Updated route copy' })
    const after = channel('Mutable catalog data', [
      group(96, 'Renamed display group', {
        rate_multiplier: 1.25,
        peak_rate_enabled: true,
        peak_start: '09:00',
        peak_end: '11:00',
        peak_rate_multiplier: 2,
        supported_models: [
          model('model-a', pricing({ input_price: 99, output_price: 101 })),
          model('new-model', pricing({ input_price: 3 })),
        ],
      }),
    ], { description: 'Stable route identity' })

    const original = buildAvailableChannelCatalog([before], {}, 7.2)[0]
    const refreshed = buildAvailableChannelCatalog([after], { 96: 0.25 }, 8)[0]

    expect(refreshed.key).toBe(original.key)
    expect(refreshed.groups[0].key).toBe(original.groups[0].key)
  })

  it('preserves case-sensitive channel identities and namespaces shared group ids by channel', () => {
    const upper = channel('Foo', [
      group(97, 'Shared shape', { supported_models: [model('same-model', pricing())] }),
    ], { description: 'Same description' })
    const lower = channel('foo', [
      group(97, 'Shared shape', { supported_models: [model('same-model', pricing())] }),
    ], { description: 'Same description' })
    const sharedGroupFirst = channel('Shared group A', [
      group(99, 'Shared id', { supported_models: [model('shared-model', pricing())] }),
    ], { description: 'Route A' })
    const sharedGroupSecond = channel('Shared group B', [
      group(99, 'Shared id', { supported_models: [model('shared-model', pricing())] }),
    ], { description: 'Route B' })

    const caseVariants = buildAvailableChannelCatalog([upper, lower], {}, 7.2)
    const reorderedCaseVariants = buildAvailableChannelCatalog([lower, upper], {}, 7.2)
    const sharedGroups = buildAvailableChannelCatalog([sharedGroupFirst, sharedGroupSecond], {}, 7.2)

    expect(caseVariants[0].key).not.toBe(caseVariants[1].key)
    expect(reorderedCaseVariants.find((entry) => entry.name === 'Foo')?.key).toBe(caseVariants[0].key)
    expect(reorderedCaseVariants.find((entry) => entry.name === 'foo')?.key).toBe(caseVariants[1].key)
    expect(caseVariants.every((entry) => !entry.key.includes(':fp:'))).toBe(true)
    expect(sharedGroups[0].groups[0].key).not.toBe(sharedGroups[1].groups[0].key)
    expect(sharedGroups[0].groups[0].models[0].key).not.toBe(
      sharedGroups[1].groups[0].models[0].key,
    )
  })

  it('keeps an exact-name channel key stable as a case-variant peer appears and disappears', () => {
    const target = channel('Peer-sensitive channel', [
      group(100, 'Target route', { supported_models: [model('target-model', pricing())] }),
    ], { description: 'Shared base identity' })
    const peer = channel(' peer-sensitive CHANNEL ', [
      group(101, 'Peer route', { supported_models: [model('peer-model', pricing())] }),
    ], { description: 'Shared base identity' })

    const aloneBefore = buildAvailableChannelCatalog([target], {}, 7.2)[0]
    const withPeer = buildAvailableChannelCatalog([target, peer], {}, 7.2)
    const reordered = buildAvailableChannelCatalog([peer, target], {}, 7.2)
    const aloneAfter = buildAvailableChannelCatalog([target], {}, 7.2)[0]
    const targetWithPeer = withPeer.find((entry) => entry.groups[0].id === 100)
    const targetReordered = reordered.find((entry) => entry.groups[0].id === 100)

    expect(targetWithPeer?.key).toBe(aloneBefore.key)
    expect(targetReordered?.key).toBe(aloneBefore.key)
    expect(aloneAfter.key).toBe(aloneBefore.key)
  })

  it('safely creates keys from upstream names containing lone UTF-16 surrogates', () => {
    const source = channel('Broken\uD800 channel', [
      group(92, 'Safe group', {
        supported_models: [model('Broken\uDFFF model', pricing())],
      }),
    ])

    expect(() => buildAvailableChannelCatalog([source], {}, 7.2)).not.toThrow()
    const catalog = buildAvailableChannelCatalog([source], {}, 7.2)
    expect(catalog[0].key).toContain('channel:')
    expect(catalog[0].groups[0].models[0].key).toContain(':model:')
  })

  it('attaches section fallback models only to the first group when group-scoped arrays are unavailable', () => {
    const first = group(13, 'First', { rate_multiplier: 0.5 })
    const second = group(14, 'Second', { rate_multiplier: 2 })
    delete first.supported_models
    delete second.supported_models
    const source = channel('Legacy fallback', [first, second])
    source.platforms[0].supported_models = [model('legacy-model', pricing())]

    const groups = buildAvailableChannelCatalog([source], {}, 7.2)[0].groups

    expect(groups[0].models.map((entry) => entry.name)).toEqual(['legacy-model'])
    expect(groups[0].models[0].prices.input?.site).toBe(18)
    expect(groups[1].models).toEqual([])
  })
})

describe('filterAvailableChannelCatalog', () => {
  const catalog = buildAvailableChannelCatalog([
    channel('Premium Routes', [
      group(21, 'Claude Pro', {
        supported_models: [
          model('claude-opus', pricing()),
          model('claude-unpriced', null),
        ],
      }),
      group(22, 'Shared Budget', {
        supported_models: [model('claude-haiku', pricing({ input_price: 1 }))],
      }),
    ], { description: 'Low latency west coast' }),
    {
      name: 'OpenAI Direct',
      description: 'Official GPT access',
      platforms: [
        {
          platform: 'openai',
          groups: [
            group(31, 'GPT Standard', {
              platform: 'openai',
              supported_models: [
                model('gpt-priced', pricing(), 'openai'),
                model('gpt-unpriced', null, 'openai'),
              ],
            }),
          ],
          supported_models: [],
        },
      ],
    },
  ], {}, 7.2)

  it('preserves all descendants when channel name or description matches', () => {
    const byName = filterAvailableChannelCatalog(catalog, {
      search: 'premium', platform: 'all', pricedOnly: false,
    })
    const byDescription = filterAvailableChannelCatalog(catalog, {
      search: 'west coast', platform: 'all', pricedOnly: false,
    })

    expect(byName).toHaveLength(1)
    expect(byName[0].groupCount).toBe(2)
    expect(byName[0].modelCount).toBe(3)
    expect(byDescription[0].modelCount).toBe(3)
  })

  it('preserves all models in a matching group and narrows model-only matches', () => {
    const byGroup = filterAvailableChannelCatalog(catalog, {
      search: 'claude pro', platform: 'all', pricedOnly: false,
    })
    const byModel = filterAvailableChannelCatalog(catalog, {
      search: 'opus', platform: 'all', pricedOnly: false,
    })

    expect(byGroup[0].groups).toHaveLength(1)
    expect(byGroup[0].groups[0].models.map((entry) => entry.name)).toEqual([
      'claude-opus',
      'claude-unpriced',
    ])
    expect(byGroup[0].modelCount).toBe(2)
    expect(byModel[0].groups).toHaveLength(1)
    expect(byModel[0].groups[0].models.map((entry) => entry.name)).toEqual(['claude-opus'])
    expect(byModel[0].modelCount).toBe(1)
  })

  it('composes platform and priced-only filters and recomputes descendant counts', () => {
    const result = filterAvailableChannelCatalog(catalog, {
      search: '', platform: 'anthropic', pricedOnly: true,
    })

    expect(result).toHaveLength(1)
    expect(result[0].platforms).toEqual(['anthropic'])
    expect(result[0].groupCount).toBe(2)
    expect(result[0].modelCount).toBe(2)
    expect(result[0].groups.flatMap((entry) => entry.models).every((entry) => entry.hasPricing)).toBe(true)
  })

  it('removes groups and channels emptied by composed filters', () => {
    const result = filterAvailableChannelCatalog(catalog, {
      search: 'unpriced', platform: 'openai', pricedOnly: true,
    })

    expect(result).toEqual([])
  })

  it('retains raw empty groups by default, with platform filters, and on metadata matches', () => {
    const emptyCatalog = buildAvailableChannelCatalog([
      channel('Empty Channel', [group(41, 'Empty Group')]),
    ], {}, 7.2)

    const defaults = filterAvailableChannelCatalog(emptyCatalog, {
      search: '', platform: 'all', pricedOnly: false,
    })
    const byPlatform = filterAvailableChannelCatalog(emptyCatalog, {
      search: '', platform: 'anthropic', pricedOnly: false,
    })
    const byChannel = filterAvailableChannelCatalog(emptyCatalog, {
      search: 'empty channel', platform: 'all', pricedOnly: false,
    })
    const byGroup = filterAvailableChannelCatalog(emptyCatalog, {
      search: 'empty group', platform: 'all', pricedOnly: false,
    })

    for (const result of [defaults, byPlatform, byChannel, byGroup]) {
      expect(result).toHaveLength(1)
      expect(result[0]).toMatchObject({ groupCount: 1, modelCount: 0 })
      expect(result[0].groups[0]).toMatchObject({ id: 41, modelCount: 0, models: [] })
    }
  })

  it('removes raw empty groups on unmatched model search and platform mismatch', () => {
    const emptyCatalog = buildAvailableChannelCatalog([
      channel('Empty Channel', [group(42, 'Empty Group')]),
    ], {}, 7.2)

    expect(filterAvailableChannelCatalog(emptyCatalog, {
      search: 'missing-model', platform: 'all', pricedOnly: false,
    })).toEqual([])
    expect(filterAvailableChannelCatalog(emptyCatalog, {
      search: '', platform: 'openai', pricedOnly: false,
    })).toEqual([])
  })

  it('removes raw empty groups when only priced models are requested', () => {
    const emptyCatalog = buildAvailableChannelCatalog([
      channel('Empty Channel', [group(44, 'Empty Group')]),
    ], {}, 7.2)

    expect(filterAvailableChannelCatalog(emptyCatalog, {
      search: '', platform: 'all', pricedOnly: true,
    })).toEqual([])
  })

  it('removes originally nonempty groups when model search or priced-only empties them', () => {
    const nonemptyCatalog = buildAvailableChannelCatalog([
      channel('Unpriced Channel', [
        group(43, 'Unpriced Group', { supported_models: [model('unpriced-model', null)] }),
      ]),
    ], {}, 7.2)

    expect(filterAvailableChannelCatalog(nonemptyCatalog, {
      search: 'different-model', platform: 'all', pricedOnly: false,
    })).toEqual([])
    expect(filterAvailableChannelCatalog(nonemptyCatalog, {
      search: '', platform: 'all', pricedOnly: true,
    })).toEqual([])
  })

  it('does not mutate the source catalog while filtering', () => {
    filterAvailableChannelCatalog(catalog, {
      search: 'opus', platform: 'anthropic', pricedOnly: true,
    })

    expect(catalog[0].groupCount).toBe(2)
    expect(catalog[0].modelCount).toBe(3)
    expect(catalog[0].groups[0].models).toHaveLength(2)
  })
})
