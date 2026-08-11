import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import Select from '@/components/common/Select.vue'

const labels: Record<string, string> = {
  'availableChannels.searchPlaceholder': '搜索渠道或模型...',
  'availableChannels.catalog.platformFilter': '平台筛选',
  'availableChannels.catalog.allPlatforms': '全部平台',
  'availableChannels.catalog.pricedOnly': '仅显示有价模型',
  'availableChannels.catalog.groupsCount': '{count} 个分组',
  'availableChannels.catalog.modelsCount': '{count} 个模型',
  'common.refresh': '刷新',
}

vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string, params?: Record<string, string | number>) => {
    const value = labels[key] ?? key
    return Object.entries(params ?? {}).reduce(
      (result, [name, replacement]) => result.replace(`{${name}}`, String(replacement)),
      value,
    )
  } }),
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
      channelName: 'Anthropic',
      channelDescription: '官渠cc满血',
      channelPlatforms: ['anthropic'],
      groupCount: 1,
      modelCount: 13,
      ...overrides,
    },
    global: {
      stubs: {
        Icon: { template: '<i />' },
        AvailableChannelPlatformBadge: {
          props: ['platform'],
          template: '<span data-platform-badge :data-platform="platform">{{ platform }}</span>',
        },
      },
    },
  })
}

describe('AvailableChannelsToolbar', () => {
  it('keeps search, platform, and priced-only values synchronized', async () => {
    const wrapper = await mountToolbar()

    await wrapper.get('[data-testid="channel-search"]').setValue('claude')
    const platformFilter = wrapper.getComponent(Select)
    expect(platformFilter.props('modelValue')).toBe('')
    await platformFilter.vm.$emit('update:modelValue', 'anthropic')
    await wrapper.get('[data-testid="priced-only-filter"]').setValue(true)

    expect(wrapper.emitted('update:search')).toEqual([['claude']])
    expect(wrapper.emitted('update:platform')).toEqual([['anthropic']])
    expect(wrapper.emitted('update:pricedOnly')).toEqual([[true]])
  })

  it('places selected channel context before one aligned filter row', async () => {
    const wrapper = await mountToolbar()
    const context = wrapper.get('[data-testid="channel-context"]')
    const filters = wrapper.get('[data-testid="channel-filter-row"]')

    expect(context.text()).toContain('Anthropic')
    expect(context.text()).toContain('官渠cc满血')
    expect(context.text()).toContain('anthropic')
    expect(context.get('[data-platform-badge]').attributes('data-platform')).toBe('anthropic')
    expect(context.text()).toContain('1 个分组')
    expect(context.text()).toContain('13 个模型')
    expect(context.element.compareDocumentPosition(filters.element) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    expect(wrapper.findAll('[data-testid="available-channels-toolbar"]')).toHaveLength(1)
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

  it('uses the shared select with deterministic responsive rows and platform options', async () => {
    const wrapper = await mountToolbar()
    const platformFilter = wrapper.getComponent(Select)
    const source = await import('../AvailableChannelsToolbar.vue?raw').then(module => module.default)

    expect(wrapper.get('[data-testid="channel-filter-row"]').classes()).toEqual(expect.arrayContaining([
      'grid-cols-[minmax(0,1fr)_auto_auto]',
      'sm:grid-cols-[minmax(0,1fr)_minmax(10rem,13rem)_auto_auto]',
    ]))
    expect(wrapper.get('[data-testid="channel-search-shell"]').classes()).toEqual(expect.arrayContaining(['col-span-3', 'sm:col-span-1']))
    expect(wrapper.get('[data-testid="channel-search"]').classes()).toEqual(expect.arrayContaining(['h-11', 'w-full', 'min-w-0']))
    expect(wrapper.find('select[data-testid="platform-filter"]').exists()).toBe(false)
    expect(platformFilter.props('searchable')).toBe(false)
    expect(platformFilter.props('ariaLabel')).toBe('平台筛选')
    expect(platformFilter.props('options')).toEqual([
      { value: '', label: '全部平台' },
      { value: 'anthropic', label: 'anthropic' },
      { value: 'openai', label: 'openai' },
    ])
    expect(wrapper.get('[data-testid="platform-filter"]').classes()).toEqual(expect.arrayContaining(['platform-filter', 'min-w-0']))
    expect(source).not.toContain('<select')
    expect(source).toContain('.platform-filter :deep(.select-trigger)')
    expect(source).toContain('@apply h-11')
  })
})
