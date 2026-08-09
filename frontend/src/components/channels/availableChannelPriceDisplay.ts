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

const perMillion = 1_000_000

function hasValue(value: CatalogPriceValue | null): boolean {
  return value != null
    && (value.official != null || value.site != null || value.peakSite != null)
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
