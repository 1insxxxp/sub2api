import { describe, expect, it } from 'vitest'

import type {
  CatalogChannelEntry,
  CatalogGroupEntry,
  CatalogModelEntry,
  CatalogPriceCollection,
} from '../availableChannelCatalog'
import { buildAvailableChannelModelList } from '../availableChannelCatalog'

function prices(official: number | null): CatalogPriceCollection {
  return {
    input: official == null ? null : { official, site: official, peakSite: null },
    output: null,
    cacheWrite: null,
    cacheRead: null,
    imageInput: null,
    imageOutput: null,
    request: null,
  }
}

function model(key: string, name: string, platform: string, official: number | null): CatalogModelEntry {
  return {
    key,
    groupKey: '',
    name,
    platform,
    billingMode: 'token',
    hasPricing: official != null,
    normalRate: 1,
    defaultRate: 1,
    userRate: null,
    peakFactor: null,
    prices: prices(official),
    intervals: [],
  }
}

function group(
  channelKey: string,
  key: string,
  name: string,
  platform: string,
  models: CatalogModelEntry[],
): CatalogGroupEntry {
  for (const entry of models) entry.groupKey = key
  return {
    key,
    channelKey,
    id: Number(key.replace(/\D/g, '')) || 1,
    name,
    platform,
    subscriptionType: 'standard',
    isExclusive: false,
    normalRate: 1,
    defaultRate: 1,
    userRate: null,
    peak: null,
    models,
    modelCount: models.length,
  }
}

function channel(key: string, name: string, groups: CatalogGroupEntry[]): CatalogChannelEntry {
  return {
    key,
    name,
    description: '',
    platforms: [...new Set(groups.map((entry) => entry.platform))],
    groups,
    groupCount: groups.length,
    modelCount: groups.reduce((count, entry) => count + entry.models.length, 0),
  }
}

describe('buildAvailableChannelModelList', () => {
  it('aggregates normalized display names while preserving every offering identity and price references', () => {
    const first = model('channel:A:group:1:model:Foo:0', 'Gemini Pro', 'gemini', 1)
    const second = model('channel:B:group:2:model:foo:0', ' gemini pro ', 'openai', 2)
    const tiered = model('channel:A:group:3:model:foo:0', 'GEMINI PRO', 'gemini', null)
    tiered.hasPricing = true
    tiered.intervals = [{
      key: `${tiered.key}:interval:0`,
      minTokens: 0,
      maxTokens: null,
      tierLabel: null,
      prices: prices(3),
    }]
    const source = [
      channel('channel:A', 'Alpha', [
        group('channel:A', 'channel:A:group:1', 'Retail', 'gemini', [first]),
        group('channel:A', 'channel:A:group:3', 'Wholesale', 'gemini', [tiered]),
      ]),
      channel('channel:B', 'Beta', [
        group('channel:B', 'channel:B:group:2', 'Pro', 'openai', [second]),
      ]),
    ]

    const [entry] = buildAvailableChannelModelList(source)

    expect(entry).toMatchObject({
      name: 'Gemini Pro',
      channelCount: 2,
      groupCount: 3,
      platforms: ['gemini', 'openai'],
      hasPricedOffering: true,
    })
    expect(entry.offerings.map((offering) => offering.key)).toEqual([
      first.key,
      tiered.key,
      second.key,
    ])
    expect(entry.offerings[0]).toMatchObject({
      channelKey: 'channel:A',
      channelName: 'Alpha',
      groupKey: 'channel:A:group:1',
      groupName: 'Retail',
      platform: 'gemini',
    })
    expect(entry.offerings[0].model).toBe(first)
    expect(entry.offerings[0].prices).toBe(first.prices)
    expect(entry.offerings[2].model).toBe(second)
    expect(entry.offerings[2].prices).toBe(second.prices)
  })

  it('keeps zero priced, unpriced, tier-priced, and repeated occurrences distinct', () => {
    const zero = model('channel:X:group:1:model:same:0', 'same', 'gemini', 0)
    const unpriced = model('channel:X:group:1:model:same:1', 'same', 'gemini', null)
    const tiered = model('channel:X:group:1:model:same:2', 'same', 'gemini', null)
    tiered.hasPricing = true
    tiered.intervals = [{
      key: `${tiered.key}:interval:0`,
      minTokens: 10,
      maxTokens: null,
      tierLabel: 'large',
      prices: prices(4),
    }]
    const source = [channel('channel:X', 'Only', [
      group('channel:X', 'channel:X:group:1', 'Main', 'gemini', [zero, unpriced, tiered]),
    ])]

    const [entry] = buildAvailableChannelModelList(source)

    expect(entry.hasPricedOffering).toBe(true)
    expect(entry.offerings).toHaveLength(3)
    expect(entry.offerings.map((item) => item.key)).toEqual([zero.key, unpriced.key, tiered.key])
    expect(entry.offerings[0].prices.input?.official).toBe(0)
    expect(entry.offerings[1].hasPricing).toBe(false)
    expect(entry.offerings[2].intervals).toBe(tiered.intervals)
  })

  it('sorts display models and offerings stably regardless of input order', () => {
    const alphaUpper = model('channel:b:group:2:model:A:0', ' Alpha ', 'openai', 1)
    const alphaLower = model('channel:a:group:1:model:a:0', 'alpha', 'gemini', 1)
    const model2 = model('channel:c:group:3:model:2:0', 'Model 2', 'gemini', 1)
    const model10 = model('channel:c:group:3:model:10:0', 'model 10', 'gemini', 1)
    const source = [
      channel('channel:c', 'Charlie', [group('channel:c', 'channel:c:group:3', 'G3', 'gemini', [model10, model2])]),
      channel('channel:b', 'beta', [group('channel:b', 'channel:b:group:2', 'G2', 'openai', [alphaUpper])]),
      channel('channel:a', 'Alpha channel', [group('channel:a', 'channel:a:group:1', 'G1', 'gemini', [alphaLower])]),
    ]

    const forward = buildAvailableChannelModelList(source)
    const reversed = buildAvailableChannelModelList([...source].reverse().map((item) => ({
      ...item,
      groups: [...item.groups].reverse().map((entry) => ({ ...entry, models: [...entry.models].reverse() })),
    })))

    expect(forward.map((entry) => entry.name)).toEqual(['Alpha', 'Model 2', 'model 10'])
    expect(reversed.map((entry) => entry.key)).toEqual(forward.map((entry) => entry.key))
    expect(reversed.map((entry) => entry.offerings.map((item) => item.key)))
      .toEqual(forward.map((entry) => entry.offerings.map((item) => item.key)))
    expect(forward[0].offerings.map((item) => item.channelName)).toEqual(['Alpha channel', 'beta'])
  })

  it('returns an empty list without recreating filtered-out catalog data', () => {
    expect(buildAvailableChannelModelList([])).toEqual([])
    expect(buildAvailableChannelModelList([
      channel('channel:empty', 'Visible empty channel', [
        group('channel:empty', 'channel:empty:group:1', 'Visible empty group', 'gemini', []),
      ]),
    ])).toEqual([])
  })
})
