import {
  BILLING_MODE_IMAGE,
  BILLING_MODE_PER_REQUEST,
  BILLING_MODE_TOKEN,
} from '@/constants/channel'
import type {
  CatalogModelOffering,
  CatalogPriceCollection,
  CatalogPriceValue,
} from './availableChannelCatalog'

export interface CatalogSummaryPrice {
  kind: 'price' | 'tiered' | 'unpriced'
  value: CatalogPriceValue | null
  scale: number
  unitKey: 'perMillion' | 'perRequest' | 'perImage'
}

export interface CatalogPriceComparison extends CatalogSummaryPrice {
  savingsPercent: number | null
  savingsPercentMax: number | null
  savingsText: string | null
}

const perMillion = 1_000_000

function hasValue(value: CatalogPriceValue | null): boolean {
  return value != null
    && (value.official != null
      || value.site != null
      || value.siteMax != null
      || value.peakSite != null
      || value.peakSiteMax != null)
}

function firstMeaningful(prices: CatalogPriceCollection) {
  const tokenFields = [prices.input, prices.output, prices.cacheWrite, prices.cacheRead, prices.imageInput, prices.imageOutput]
  return tokenFields.find(hasValue) ?? null
}

function summaryFromCollection(
  prices: CatalogPriceCollection,
  billingMode: string | null,
): Omit<CatalogSummaryPrice, 'kind'> | null {
  if (billingMode === BILLING_MODE_PER_REQUEST && hasValue(prices.request)) {
    return { value: prices.request, scale: 1, unitKey: 'perRequest' }
  }
  if (billingMode === BILLING_MODE_IMAGE && hasValue(prices.request)) {
    return { value: prices.request, scale: 1, unitKey: 'perImage' }
  }
  if (billingMode === BILLING_MODE_TOKEN) {
    const value = [prices.input, prices.output].find(hasValue) ?? firstMeaningful(prices)
    if (value) return { value, scale: 1_000_000, unitKey: 'perMillion' }
    if (hasValue(prices.request)) return { value: prices.request, scale: 1, unitKey: 'perRequest' }
  }
  if (hasValue(prices.request)) {
    return { value: prices.request, scale: 1, unitKey: 'perRequest' }
  }
  const value = firstMeaningful(prices)
  return value ? { value, scale: perMillion, unitKey: 'perMillion' } : null
}

export function summarizeOfferingPrice(offering: CatalogModelOffering): CatalogSummaryPrice {
  const direct = summaryFromCollection(offering.prices, offering.model.billingMode)
  if (direct) return { kind: 'price', ...direct }
  if (offering.intervals.some(interval => summaryFromCollection(interval.prices, offering.model.billingMode))) {
    return { kind: 'tiered', value: null, scale: 1, unitKey: 'perMillion' }
  }
  return { kind: 'unpriced', value: null, scale: 1, unitKey: 'perMillion' }
}

export function compareOfferingPrice(offering: CatalogModelOffering): CatalogPriceComparison {
  const summary = summarizeOfferingPrice(offering)
  const officialCny = summary.value?.officialCny
  const savings = summary.kind === 'price'
    ? [summary.value?.site, summary.value?.siteMax]
      .map(site => savingsAt(officialCny, site))
      .filter((value): value is number => value != null)
      .sort((left, right) => left - right)
    : []
  const uniqueSavings = [...new Set(savings)]
  const savingsPercent = uniqueSavings[0] ?? null
  const savingsPercentMax = uniqueSavings.at(-1) ?? null
  const savingsText = savingsPercent == null
    ? null
    : formatCatalogSavingsRange(savingsPercent, savingsPercentMax)
  return { ...summary, savingsPercent, savingsPercentMax, savingsText }
}

export function formatCatalogSavingsRange(
  minimum: number,
  maximum: number | null | undefined,
): string {
  const formatted = formatCatalogValueRange(minimum, maximum, value => String(value), '')
  return formatted.replace('–', '%–')
}

function savingsAt(officialCny: number | null | undefined, site: number | null | undefined): number | null {
  if (officialCny == null || site == null
    || !Number.isFinite(officialCny) || !Number.isFinite(site)
    || officialCny <= 0 || site < 0 || site >= officialCny) {
    return null
  }
  const savings = Math.round((1 - site / officialCny) * 100)
  return savings > 0 ? savings : null
}

export function formatCatalogValueRange(
  first: number | null | undefined,
  second: number | null | undefined,
  formatter: (value: number) => string,
  unavailable = '-',
): string {
  const values = [first, second]
    .filter((value): value is number => value != null && Number.isFinite(value))
    .sort((left, right) => left - right)
  const formatted = [...new Set(values.map(formatter).filter(value => value !== unavailable))]
  if (formatted.length === 0) return unavailable
  if (formatted.length === 1) return formatted[0]
  return `${formatted[0]}–${formatted.at(-1)}`
}

export function formatCatalogMoneyRange(
  first: number | null | undefined,
  second: number | null | undefined,
  scale: number,
  symbol: '$' | '¥',
  minimumFractionDigits = 2,
): string {
  return formatCatalogValueRange(
    first,
    second,
    value => formatCatalogMoney(value, scale, symbol, minimumFractionDigits),
  )
}

export function formatCatalogMoney(
  value: number | null,
  scale: number,
  symbol: '$' | '¥',
  minimumFractionDigits = 2,
): string {
  if (value == null) return '-'
  const scaled = value * scale
  if (!Number.isFinite(scaled)) return '-'
  if (scaled === 0) return `${symbol}0.${'0'.repeat(minimumFractionDigits)}`

  const [rawMantissa, rawExponent] = scaled.toPrecision(10).split('e')
  let mantissa = rawMantissa.includes('.')
    ? rawMantissa.replace(/0+$/, '').replace(/\.$/, '')
    : rawMantissa
  if (rawExponent != null) {
    const exponent = Number(rawExponent)
    return `${symbol}${mantissa}e${exponent >= 0 ? '+' : ''}${exponent}`
  }
  if (minimumFractionDigits > 0) {
    const decimalIndex = mantissa.indexOf('.')
    const digits = decimalIndex === -1 ? 0 : mantissa.length - decimalIndex - 1
    if (digits < minimumFractionDigits) {
      mantissa = (decimalIndex === -1 ? `${mantissa}.` : mantissa)
        + '0'.repeat(minimumFractionDigits - digits)
    }
  }
  return `${symbol}${mantissa}`
}
