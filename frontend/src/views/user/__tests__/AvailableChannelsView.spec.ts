import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AvailableChannelsView.vue')
const source = readFileSync(viewPath, 'utf8')

describe('AvailableChannelsView catalog integration', () => {
  it('renders the pricing catalog instead of the legacy table', () => {
    expect(source).toContain('AvailableChannelCatalog')
    expect(source).toContain(':channels="filteredCatalog"')
    expect(source).not.toContain('AvailableChannelsTable')
  })

  it('keeps search filtering and distinguishes refresh from first load', () => {
    expect(source).toContain('filterAvailableChannelCatalog')
    expect(source).toContain('search: searchQuery.value')
    expect(source).toContain(':refreshing="loading && channels.length > 0"')
    expect(source).toContain(":empty-kind=\"channels.length > 0 && filteredCatalog.length === 0 ? 'no-results' : 'no-data'\"")
  })

  it('uses the configured public multiplier and user rates without recalculating prices in the view', () => {
    expect(source).toContain('buildAvailableChannelCatalog(channels.value, userGroupRates.value, priceCnyMultiplier.value)')
    expect(source).not.toContain('input_price *')
    expect(source).not.toContain('output_price *')
  })
})
