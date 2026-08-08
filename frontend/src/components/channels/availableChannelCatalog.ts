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
  return encodeURIComponent(value.trim().toLowerCase())
}

function unique(values: string[]): string[] {
  return [...new Set(values)]
}

function priceValue(rawValue: number | null, context: PriceContext): CatalogPriceValue | null {
  if (rawValue == null) return null

  const site = rawValue * context.cnyMultiplier * context.normalRate
  return {
    official: rawValue,
    site,
    peakSite: context.peakFactor == null ? null : site * context.peakFactor,
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
    interval.min_tokens,
    interval.max_tokens,
    interval.tier_label ?? null,
    interval.input_price,
    interval.output_price,
    interval.cache_write_price,
    interval.cache_read_price,
    interval.per_request_price,
  ]
}

function pricingFingerprint(modelPricing: UserSupportedModelPricing | null): unknown {
  if (modelPricing == null) return null
  return [
    modelPricing.billing_mode,
    modelPricing.input_price,
    modelPricing.output_price,
    modelPricing.cache_write_price,
    modelPricing.cache_read_price,
    modelPricing.image_input_price,
    modelPricing.image_output_price,
    modelPricing.per_request_price,
    (modelPricing.intervals ?? []).map(intervalFingerprint),
  ]
}

function modelFingerprint(model: UserSupportedModel): string {
  return JSON.stringify([model.name, model.platform, pricingFingerprint(model.pricing)])
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
  return intervals.map((interval, index) => ({
    key: `${modelKey}:interval:${index}:${interval.min_tokens}:${interval.max_tokens ?? 'max'}`,
    minTokens: interval.min_tokens,
    maxTokens: interval.max_tokens,
    tierLabel: interval.tier_label ?? null,
    prices: priceCollection(interval, context),
  }))
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
): CatalogModelEntry {
  const billingMode = source.pricing?.billing_mode ?? null
  const peakFactor =
    billingMode === BILLING_MODE_TOKEN && group.peak_rate_enabled
      ? group.peak_rate_multiplier
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
    platform: source.platform || group.platform,
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
  sectionPlatform: string,
  sectionIndex: number,
  groupIndex: number,
  userGroupRates: Record<number, number>,
  cnyMultiplier: number,
): CatalogGroupEntry {
  const defaultRate = source.rate_multiplier ?? 1
  const userRate = userGroupRates[source.id] ?? null
  const normalRate = userGroupRates[source.id] ?? source.rate_multiplier ?? 1
  const platform = source.platform || sectionPlatform
  const groupKey = `${channelKey}:section:${sectionIndex}:${keySegment(sectionPlatform)}:group:${source.id}:${groupIndex}`
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
    )
  })

  return {
    key: groupKey,
    channelKey,
    id: source.id,
    name: source.name,
    platform,
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
          factor: source.peak_rate_multiplier,
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
  return rows.map((channel, channelIndex) => {
    const channelKey = `channel:${channelIndex}:${keySegment(channel.name)}`
    const groups = channel.platforms.flatMap((section, sectionIndex) => {
      const hasGroupScopedModels = section.groups.some((item) => Array.isArray(item.supported_models))

      return section.groups.map((item, groupIndex) => {
        const models = hasGroupScopedModels
          ? item.supported_models ?? []
          : groupIndex === 0
            ? section.supported_models ?? []
            : []
        return normalizeGroup(
          item,
          models,
          channelKey,
          section.platform,
          sectionIndex,
          groupIndex,
          userGroupRates,
          cnyMultiplier,
        )
      })
    })

    return {
      key: channelKey,
      name: channel.name,
      description: channel.description,
      platforms: unique(channel.platforms.map((section) => section.platform)),
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
      let models = search === '' || channelMatches || groupMatches
        ? group.models
        : group.models.filter((model) => includes(model.name, search))

      if (filters.pricedOnly) {
        models = models.filter((model) => model.hasPricing)
      }
      if (models.length === 0) return []

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
