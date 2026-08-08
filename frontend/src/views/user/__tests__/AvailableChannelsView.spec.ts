import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AvailableChannelsView from '@/views/user/AvailableChannelsView.vue'

const api = vi.hoisted(() => ({ channels: vi.fn(), rates: vi.fn() }))
vi.mock('@/api/channels', () => ({ default: { getAvailable: api.channels } }))
vi.mock('@/api/groups', () => ({ default: { getUserGroupRates: api.rates } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: { available_channels_price_cny_multiplier: 7 }, showError: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...(await importOriginal<typeof import('vue-i18n')>()), useI18n: () => ({ t: (key: string, p?: { count?: number }) => p?.count == null ? key : `${key}:${p.count}` }) }))

const channel = (name = 'Alpha') => ({ name, description: 'test', platforms: [{ platform: 'openai', groups: [{ id: 1, name: 'public', platform: 'openai', subscription_type: 'standard', rate_multiplier: 1, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false, supported_models: [{ name: 'gpt-test', platform: 'openai', pricing: { billing_mode: 'token', input_price: 1, output_price: 2, cache_write_price: null, cache_read_price: null, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [] } }] }], supported_models: [] }] })
const stubs = { AppLayout: { template: '<div><slot /></div>' }, TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/></div>' }, Icon: { template: '<span />' }, AvailableChannelPicker: { template: '<div />' }, AvailableChannelModelPrice: { template: '<div />' } }

describe('AvailableChannelsView integration', () => {
  beforeEach(() => { api.channels.mockReset(); api.rates.mockReset(); api.rates.mockResolvedValue({}) })
  it('renders model list from fetched catalog', async () => { api.channels.mockResolvedValue([channel()]); const w = mount(AvailableChannelsView, { global: { stubs } }); await flushPromises(); expect(w.find('[data-testid="available-model-list"]').exists()).toBe(true); expect(w.text()).toContain('gpt-test') })
  it('filters platform and priced-only results', async () => { api.channels.mockResolvedValue([channel()]); const w = mount(AvailableChannelsView, { global: { stubs } }); await flushPromises(); await w.find('select').setValue('openai'); expect(w.findAll('[data-testid="model-list-row"]')).toHaveLength(1); await w.find('input[type="checkbox"]').setValue(true); expect(w.findAll('[data-testid="model-list-row"]')).toHaveLength(1) })
  it('keeps old content while refreshing and distinguishes empty states', async () => { let resolve!: (v: unknown) => void; api.channels.mockResolvedValueOnce([channel()]).mockReturnValueOnce(new Promise(r => { resolve = r })); const w = mount(AvailableChannelsView, { global: { stubs } }); await flushPromises(); await w.find('button[title]').trigger('click'); expect(w.find('[data-testid="refreshing-indicator"]').exists()).toBe(true); expect(w.text()).toContain('gpt-test'); resolve([]); await flushPromises(); expect(w.find('[data-testid="catalog-empty"]').exists()).toBe(true) })
  it('shows rate fallback warning', async () => { api.channels.mockResolvedValue([channel()]); api.rates.mockRejectedValue(new Error('offline')); const w = mount(AvailableChannelsView, { global: { stubs } }); await flushPromises(); expect(w.find('[data-testid="rate-fallback-warning"]').exists()).toBe(true) })
})
