import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ChannelsView from '../ChannelsView.vue'

const { listChannels, getGroups, updateChannel, showError } = vi.hoisted(() => ({
  listChannels: vi.fn(),
  getGroups: vi.fn(),
  updateChannel: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channels: { list: listChannels, update: updateChannel },
    groups: { getAll: getGroups },
    accounts: { getById: vi.fn().mockResolvedValue({ id: 42, name: '指定账号' }) },
    settings: { getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false }) }
  }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

function mountView() {
  return mount(ChannelsView, {
    global: {
      stubs: {
        AppLayout: defineComponent({ template: '<main><slot /></main>' }),
        TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
        DataTable: defineComponent({
          props: ['data'],
          template: '<div><slot v-for="row in data" name="cell-actions" :row="row" /></div>'
        }),
        BaseDialog: defineComponent({ props: ['show'], template: '<section v-if="show" data-test="dialog"><slot /><slot name="footer" /></section>' }),
        PricingEntryCard: true,
        Pagination: true,
        ConfirmDialog: true,
        EmptyState: true,
        Select: true,
        Icon: true,
        PlatformIcon: true,
        Toggle: true
      }
    }
  })
}

async function enablePlatform(wrapper: ReturnType<typeof mountView>, platform: string) {
  const label = wrapper.findAll('label').find(item => item.text() === `admin.groups.platforms.${platform}`)!
  await label.get('input[type="checkbox"]').setValue(true)
  await wrapper.findAll('button.channel-tab').find(item => item.text() === `admin.groups.platforms.${platform}`)!.trigger('click')
}

describe('channel pricing category default groups', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    listChannels.mockResolvedValue({ items: [], total: 0 })
    updateChannel.mockResolvedValue({})
    getGroups.mockResolvedValue([
      { id: 1, name: 'Claude 来源分组', platform: 'anthropic' },
      { id: 2, name: '普通组合分组', platform: 'composite', is_category_default: false },
      { id: 9, name: 'Gemini 分类默认入口', platform: 'composite', is_category_default: true, key_display_category: 'gemini' }
    ])
  })

  it('excludes category entries from every pricing platform while keeping ordinary composite sources', async () => {
    const wrapper = mountView()
    try {
      await flushPromises()
      await wrapper.findAll('button').find(item => item.text() === 'admin.channels.createChannel')!.trigger('click')
      await flushPromises()
      for (const platform of ['anthropic', 'gemini']) {
        await enablePlatform(wrapper, platform)
        const labels = wrapper.findAll('label').filter(item => item.isVisible()).map(item => item.text())
        expect(labels.some(text => text.includes('普通组合分组'))).toBe(true)
        expect(labels.some(text => text.includes('Gemini 分类默认入口'))).toBe(false)
        if (platform === 'anthropic') expect(labels.some(text => text.includes('Claude 来源分组'))).toBe(true)
      }
    } finally {
      wrapper.unmount()
    }
  })

  it('removes legacy category bindings while preserving source and account statistics pricing', async () => {
    const pricing = { platform: 'anthropic', models: ['claude-sonnet-4-6'], billing_mode: 'token', input_price: 0.000003, output_price: 0.000015 }
    const channel = {
      id: 11, name: '已有定价', status: 'active', group_ids: [1, 9], model_pricing: [pricing],
      model_mapping: {}, features_config: {}, apply_pricing_to_account_stats: true,
      account_stats_pricing_rules: [
        { name: '混合规则', group_ids: [1, 9], account_ids: [], pricing: [pricing] },
        { name: '仅入口规则', group_ids: [9], account_ids: [], pricing: [pricing] },
        { name: '指定账号规则', group_ids: [9], account_ids: [42], pricing: [pricing] },
        { name: '原空规则', group_ids: [], account_ids: [], pricing: [pricing] }
      ]
    }
    listChannels.mockResolvedValue({ items: [channel], total: 1 })
    const wrapper = mountView()
    try {
      await flushPromises()
      await wrapper.findAll('button').find(item => item.text() === 'common.edit')!.trigger('click')
      await flushPromises()
      const tabs = wrapper.findAll('button.channel-tab').map(item => item.text())
      expect(tabs).not.toContain('admin.groups.platforms.gemini')
      expect(wrapper.get('[data-test="dialog"]').text()).not.toContain('Gemini 分类默认入口')
      await wrapper.get('#channel-form').trigger('submit')
      await flushPromises()
      expect(showError).not.toHaveBeenCalled()
      expect(updateChannel).toHaveBeenCalledWith(11, expect.objectContaining({
        group_ids: [1],
        model_pricing: [expect.objectContaining(pricing)],
        account_stats_pricing_rules: [
          expect.objectContaining({ name: '混合规则', group_ids: [1] }),
          expect.objectContaining({ name: '指定账号规则', group_ids: [], account_ids: [42] }),
          expect.objectContaining({ name: '原空规则', group_ids: [] })
        ]
      }))
      expect(channel.group_ids).toEqual([1, 9])
      expect(channel.account_stats_pricing_rules[0].group_ids).toEqual([1, 9])
    } finally {
      wrapper.unmount()
    }
  })
})
