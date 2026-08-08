import type {
  UserAvailableChannel,
  UserAvailableGroup,
  UserPricingInterval,
  UserSupportedModel,
  UserSupportedModelPricing,
} from '@/api/channels'
import type { BillingMode } from '@/constants/channel'
import { BILLING_MODE_TOKEN } from '@/constants/channel'

export interface CatalogPriceValue {
  official: number | null
  site: number | null
  peakSite: number | null
}

export interface CatalogPriceCollection {
  input: CatalogPriceValue | null
  output: CatalogPriceValue | null
  cacheWrite: CatalogPriceValue | null
  cacheRead: CatalogPriceValue | null
  imageInput: CatalogPriceValue | null
  imageOutput: CatalogPriceValue | null
  request: CatalogPriceValue | null
}

export interface CatalogPricingInterval {
  key: string
  minTokens: number
  maxTokens: number | null
  tierLabel: string | null
  prices: CatalogPriceCollection
}

export interface CatalogPeakMetadata {
  enabled: true
  start: string
  end: string
  factor: number
}

export interface CatalogModelEntry {
  key: string
  groupKey: string
  name: string
  platform: string
  billingMode: BillingMode | null
  hasPricing: boolean
  normalRate: number
  defaultRate: number
  userRate: number | null
  peakFactor: number | null
  prices: CatalogPriceCollection
  intervals: CatalogPricingInterval[]
}

export interface CatalogGroupEntry {
  key: string
  channelKey: string
  id: number
  name: string
  platform: string
  subscriptionType: string
  isExclusive: boolean
  normalRate: number
  defaultRate: number
  userRate: number | null
  peak: CatalogPeakMetadata | null
  models: CatalogModelEntry[]
  modelCount: number
}

export interface CatalogChannelEntry {
  key: string
  name: string
  description: string
  platforms: string[]
  groups: CatalogGroupEntry[]
  groupCount: number
  modelCount: number
}

export interface AvailableChannelCatalogFilters {
  search: string
  platform: string
  pricedOnly: boolean
}

interface PriceContext {
  cnyMultiplier: number
  normalRate: number
  peakFactor: number | null
}

function keySegment(value: string): string {
  let wellFormed = ''
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    if (code >= 0xd800 && code <= 0xdbff) {
      const next = value.charCodeAt(index + 1)
      if (next >= 0xdc00 && next <= 0xdfff) {
        wellFormed += value[index] + value[index + 1]
        index += 1
      } else {
        wellFormed += '\ufffd'
      }
    } else if (code >= 0xdc00 && code <= 0xdfff) {
      wellFormed += '\ufffd'
    } else {
      wellFormed += value[index]
    }
  }
  return encodeURIComponent(wellFormed.trim().toLowerCase())
}

function unique(values: string[]): string[] {
  return [...new Set(values)]
}

function finiteRate(value: number | null | undefined): number | null {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : null
}

function priceValue(rawValue: number | null, context: PriceContext): CatalogPriceValue | null {
  if (rawValue == null || !Number.isFinite(rawValue)) return null

  if (
    !Number.isFinite(context.cnyMultiplier)
    || context.cnyMultiplier <= 0
    || !Number.isFinite(context.normalRate)
    || context.normalRate < 0
  ) {
    return { official: rawValue, site: null, peakSite: null }
  }

  const site = rawValue * context.cnyMultiplier * context.normalRate
  if (!Number.isFinite(site)) {
    return { official: rawValue, site: null, peakSite: null }
  }

  const peakSite = context.peakFactor == null
    || !Number.isFinite(context.peakFactor)
    || context.peakFactor < 0
    ? null
    : site * context.peakFactor
  return {
    official: rawValue,
    site,
    peakSite: peakSite != null && Number.isFinite(peakSite) ? peakSite : null,
  }
}

function priceCollection(
  source: {
    input_price: number | null
    output_price: number | null
    cache_write_price: number | null
    cache_read_price: number | null
    image_input_price?: number | null
    image_output_price?: number | null
    per_request_price: number | null
  },
  context: PriceContext,
): CatalogPriceCollection {
  return {
    input: priceValue(source.input_price, context),
    output: priceValue(source.output_price, context),
    cacheWrite: priceValue(source.cache_write_price, context),
    cacheRead: priceValue(source.cache_read_price, context),
    imageInput: priceValue(source.image_input_price ?? null, context),
    imageOutput: priceValue(source.image_output_price ?? null, context),
    request: priceValue(source.per_request_price, { ...context, peakFactor: null }),
  }
}

function collectionHasPricing(prices: CatalogPriceCollection): boolean {
  return Object.values(prices).some((value) => value?.official != null)
}

function intervalFingerprint(interval: UserPricingInterval): unknown[] {
  return [
    fingerprintNumber(interval.min_tokens),
    fingerprintNumber(interval.max_tokens),
    interval.tier_label ?? null,
    fingerprintNumber(interval.input_price),
    fingerprintNumber(interval.output_price),
    fingerprintNumber(interval.cache_write_price),
    fingerprintNumber(interval.cache_read_price),
    fingerprintNumber(interval.per_request_price),
  ]
}

function fingerprintNumber(value: number | null): number | string | null {
  if (value == null || Number.isFinite(value)) return value
  return String(value)
}

function pricingFingerprint(modelPricing: UserSupportedModelPricing | null): unknown {
  if (modelPricing == null) return null
  return [
    modelPricing.billing_mode,
    fingerprintNumber(modelPricing.input_price),
    fingerprintNumber(modelPricing.output_price),
    fingerprintNumber(modelPricing.cache_write_price),
    fingerprintNumber(modelPricing.cache_read_price),
    fingerprintNumber(modelPricing.image_input_price),
    fingerprintNumber(modelPricing.image_output_price),
    fingerprintNumber(modelPricing.per_request_price),
    (modelPricing.intervals ?? []).map(intervalFingerprint),
  ]
}

function modelFingerprint(model: UserSupportedModel): string {
  return JSON.stringify([model.name, model.platform, pricingFingerprint(model.pricing)])
}

function stableHash(value: string): string {
  let fnv = 2166136261
  let djb = 5381
  for (let index = 0; index < value.length; index += 1) {
    const code = value.charCodeAt(index)
    fnv ^= code
    fnv = Math.imul(fnv, 16777619)
    djb = Math.imul(djb, 33) ^ code
  }
  return `${(fnv >>> 0).toString(36)}-${(djb >>> 0).toString(36)}`
}

function channelFingerprint(channel: UserAvailableChannel): string {
  const sections = channel.platforms.map((section) => {
    const groups = section.groups.map((group) => JSON.stringify([
      group.id,
      group.name.trim(),
      group.platform.trim().toLocaleLowerCase(),
      group.subscription_type,
      fingerprintNumber(group.rate_multiplier),
      group.peak_rate_enabled,
      group.peak_start,
      group.peak_end,
      fingerprintNumber(group.peak_rate_multiplier),
      group.is_exclusive,
      (group.supported_models ?? []).map(modelFingerprint).sort(),
    ])).sort()
    return JSON.stringify([
      section.platform.trim().toLocaleLowerCase(),
      groups,
      section.supported_models.map(modelFingerprint).sort(),
    ])
  }).sort()

  return JSON.stringify([
    channel.name.trim().toLocaleLowerCase(),
    channel.description.trim(),
    sections,
  ])
}

function deduplicateModels(models: UserSupportedModel[]): UserSupportedModel[] {
  const seen = new Set<string>()
  return models.filter((model) => {
    const fingerprint = modelFingerprint(model)
    if (seen.has(fingerprint)) return false
    seen.add(fingerprint)
    return true
  })
}

function normalizeIntervals(
  intervals: UserPricingInterval[],
  modelKey: string,
  context: PriceContext,
): CatalogPricingInterval[] {
  const occurrences = new Map<string, number>()
  return intervals.map((interval) => {
    const identity = [
      interval.min_tokens,
      interval.max_tokens ?? 'max',
      keySegment(interval.tier_label ?? ''),
    ].join(':')
    const occurrence = occurrences.get(identity) ?? 0
    occurrences.set(identity, occurrence + 1)
    return {
      key: `${modelKey}:interval:${identity}${occurrence > 0 ? `:occ:${occurrence}` : ''}`,
      minTokens: interval.min_tokens,
      maxTokens: interval.max_tokens,
      tierLabel: interval.tier_label ?? null,
      prices: priceCollection(interval, context),
    }
  })
}

function resolvePlatform(platform: string | null | undefined, fallback: string): string {
  const normalized = platform?.trim() ?? ''
  return normalized === '' || normalized.toLowerCase() === 'composite'
    ? fallback
    : normalized
}

function normalizeModel(
  source: UserSupportedModel,
  group: UserAvailableGroup,
  groupKey: string,
  occurrence: number,
  cnyMultiplier: number,
  defaultRate: number,
  userRate: number | null,
  normalRate: number,
  resolvedGroupPlatform: string,
  configuredPeakFactor: number | null,
): CatalogModelEntry {
  const billingMode = source.pricing?.billing_mode ?? null
  const peakFactor =
    billingMode === BILLING_MODE_TOKEN && group.peak_rate_enabled
      ? configuredPeakFactor
      : null
  const context: PriceContext = { cnyMultiplier, normalRate, peakFactor }
  const modelKey = `${groupKey}:model:${keySegment(source.name)}:${occurrence}`
  const prices = source.pricing
    ? priceCollection(source.pricing, context)
    : priceCollection({
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        image_output_price: null,
        per_request_price: null,
      }, context)
  const intervals = normalizeIntervals(source.pricing?.intervals ?? [], modelKey, context)

  return {
    key: modelKey,
    groupKey,
    name: source.name,
    platform: resolvePlatform(source.platform, resolvedGroupPlatform),
    billingMode,
    hasPricing: collectionHasPricing(prices)
      || intervals.some((interval) => collectionHasPricing(interval.prices)),
    normalRate,
    defaultRate,
    userRate,
    peakFactor,
    prices,
    intervals,
  }
}

function normalizeGroup(
  source: UserAvailableGroup,
  models: UserSupportedModel[],
  channelKey: string,
  groupKey: string,
  resolvedPlatform: string,
  userGroupRates: Record<number, number>,
  cnyMultiplier: number,
): CatalogGroupEntry {
  const defaultRate = finiteRate(source.rate_multiplier) ?? 1
  const userRate = finiteRate(userGroupRates[source.id])
  const normalRate = userRate ?? defaultRate
  const configuredPeakFactor = source.peak_rate_enabled
    ? finiteRate(source.peak_rate_multiplier) ?? 1
    : null
  const occurrences = new Map<string, number>()
  const normalizedModels = deduplicateModels(models).map((item) => {
    const modelIdentity = keySegment(item.name)
    const occurrence = occurrences.get(modelIdentity) ?? 0
    occurrences.set(modelIdentity, occurrence + 1)
    return normalizeModel(
      item,
      source,
      groupKey,
      occurrence,
      cnyMultiplier,
      defaultRate,
      userRate,
      normalRate,
      resolvedPlatform,
      configuredPeakFactor,
    )
  })

  return {
    key: groupKey,
    channelKey,
    id: source.id,
    name: source.name,
    platform: resolvedPlatform,
    subscriptionType: source.subscription_type,
    isExclusive: source.is_exclusive,
    normalRate,
    defaultRate,
    userRate,
    peak: source.peak_rate_enabled
      ? {
          enabled: true,
          start: source.peak_start,
          end: source.peak_end,
          factor: configuredPeakFactor ?? 1,
        }
      : null,
    models: normalizedModels,
    modelCount: normalizedModels.length,
  }
}

export function buildAvailableChannelCatalog(
  rows: UserAvailableChannel[],
  userGroupRates: Record<number, number>,
  cnyMultiplier: number,
): CatalogChannelEntry[] {
  const channelOccurrences = new Map<string, number>()
  return rows.map((channel) => {
    const channelName = keySegment(channel.name)
    const fingerprint = channelFingerprint(channel)
    const channelIdentity = `${channelName}:fp:${stableHash(fingerprint)}`
    const occurrenceIdentity = `${channelName}\u0000${fingerprint}`
    const channelOccurrence = channelOccurrences.get(occurrenceIdentity) ?? 0
    channelOccurrences.set(occurrenceIdentity, channelOccurrence + 1)
    const channelKey = `channel:${channelIdentity}${channelOccurrence > 0 ? `:occ:${channelOccurrence}` : ''}`
    const groupOccurrences = new Map<string, number>()
    const groups = channel.platforms.flatMap((section) => {
      const hasGroupScopedModels = section.groups.some((item) => Array.isArray(item.supported_models))

      return section.groups.map((item, groupIndex) => {
        const models = hasGroupScopedModels
          ? item.supported_models ?? []
          : groupIndex === 0
            ? section.supported_models ?? []
            : []
        const resolvedPlatform = resolvePlatform(item.platform, section.platform)
        const groupIdentity = `${keySegment(resolvedPlatform)}:${item.id}`
        const groupOccurrence = groupOccurrences.get(groupIdentity) ?? 0
        groupOccurrences.set(groupIdentity, groupOccurrence + 1)
        const groupKey = `group:${item.id}:platform:${keySegment(resolvedPlatform)}${groupOccurrence > 0 ? `:occ:${groupOccurrence}` : ''}`
        return normalizeGroup(
          item,
          models,
          channelKey,
          groupKey,
          resolvedPlatform,
          userGroupRates,
          cnyMultiplier,
        )
      })
    })

    return {
      key: channelKey,
      name: channel.name,
      description: channel.description || '',
      platforms: unique(
        groups.length > 0
          ? groups.map((group) => group.platform)
          : channel.platforms.map((section) => section.platform),
      ),
      groups,
      groupCount: groups.length,
      modelCount: groups.reduce((count, item) => count + item.modelCount, 0),
    }
  })
}

function includes(value: string, search: string): boolean {
  return value.toLocaleLowerCase().includes(search)
}

function platformMatches(group: CatalogGroupEntry, platform: string): boolean {
  const normalized = platform.trim().toLocaleLowerCase()
  return normalized === '' || normalized === 'all' || group.platform.toLocaleLowerCase() === normalized
}

export function filterAvailableChannelCatalog(
  catalog: CatalogChannelEntry[],
  filters: AvailableChannelCatalogFilters,
): CatalogChannelEntry[] {
  const search = filters.search.trim().toLocaleLowerCase()

  return catalog.flatMap((channel) => {
    const channelMatches = search !== ''
      && (includes(channel.name, search) || includes(channel.description, search))
    const groups = channel.groups.flatMap((group) => {
      if (!platformMatches(group, filters.platform)) return []

      const groupMatches = search !== '' && includes(group.name, search)
      const metadataMatches = search === '' || channelMatches || groupMatches
      let models = metadataMatches
        ? group.models
        : group.models.filter((model) => includes(model.name, search))

      if (filters.pricedOnly) {
        models = models.filter((model) => model.hasPricing)
      }
      const isRawEmptyGroup = group.models.length === 0
      if (
        models.length === 0
        && !(isRawEmptyGroup && metadataMatches && !filters.pricedOnly)
      ) return []

      return [{ ...group, models: [...models], modelCount: models.length }]
    })

    if (groups.length === 0) return []
    return [{
      ...channel,
      platforms: unique(groups.map((group) => group.platform)),
      groups,
      groupCount: groups.length,
      modelCount: groups.reduce((count, group) => count + group.modelCount, 0),
    }]
  })
}
