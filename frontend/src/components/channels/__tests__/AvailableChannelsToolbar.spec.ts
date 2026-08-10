import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

const labels: Record<string, string> = {
  'availableChannels.searchPlaceholder': '搜索渠道或模型...',
  'availableChannels.catalog.platformFilter': '平台筛选',
  'availableChannels.catalog.allPlatforms': '全部平台',
  'availableChannels.catalog.pricedOnly': '仅显示有价模型',
  'common.refresh': '刷新',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => labels[key] ?? key }),
}))

async function mountToolbar(overrides: Record<string, unknown> = {}) {
  const component = (await import('../AvailableChannelsToolbar.vue')).default
  return mount(component, {
    props: {
      search: '',
      platform: '',
      pricedOnly: false,
      platforms: ['anthropic', 'openai'],
      loading: false,
      ...overrides,
    },
    global: {
      stubs: {
        Icon: { template: '<i />' },
      },
    },
  })
}

describe('AvailableChannelsToolbar', () => {
  it('keeps search, platform, and priced-only values synchronized', async () => {
    const wrapper = await mountToolbar()

    await wrapper.get('[data-testid="channel-search"]').setValue('claude')
    await wrapper.get('[data-testid="platform-filter"]').setValue('anthropic')
    await wrapper.get('[data-testid="priced-only-filter"]').setValue(true)

    expect(wrapper.emitted('update:search')).toEqual([['claude']])
    expect(wrapper.emitted('update:platform')).toEqual([['anthropic']])
    expect(wrapper.emitted('update:pricedOnly')).toEqual([[true]])
  })

  it('emits refresh from a fixed touch target', async () => {
    const wrapper = await mountToolbar()
    const refresh = wrapper.get('[data-testid="channel-refresh"]')

    expect(refresh.classes()).toEqual(expect.arrayContaining(['h-11', 'w-11']))
    await refresh.trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('disables refresh and exposes its loading state', async () => {
    const wrapper = await mountToolbar({ loading: true })
    const refresh = wrapper.get('[data-testid="channel-refresh"]')

    expect(refresh.attributes('disabled')).toBeDefined()
    expect(refresh.attributes('aria-busy')).toBe('true')
  })

  it('uses deterministic responsive rows and retains the all-platform option', async () => {
    const wrapper = await mountToolbar({ platforms: [] })
    const toolbar = wrapper.get('[data-testid="available-channels-toolbar"]')

    expect(toolbar.classes()).toEqual(expect.arrayContaining([
      'grid-cols-[minmax(0,1fr)_auto_auto]',
      'sm:grid-cols-[minmax(0,1fr)_minmax(10rem,13rem)_auto_auto]',
    ]))
    expect(wrapper.get('[data-testid="channel-search-shell"]').classes()).toEqual(expect.arrayContaining(['col-span-3', 'sm:col-span-1']))
    expect(wrapper.get('[data-testid="channel-search"]').classes()).toEqual(expect.arrayContaining(['h-11', 'w-full', 'min-w-0']))
    expect(wrapper.get('[data-testid="platform-filter"]').text()).toContain('全部平台')
  })
})
