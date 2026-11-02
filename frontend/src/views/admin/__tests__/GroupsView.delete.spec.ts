import { defineComponent } from 'vue'
import { createI18n } from 'vue-i18n'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup } from '@/types'
import GroupsView from '@/views/admin/GroupsView.vue'
import { adminAPI } from '@/api/admin'
import zhAdminOverview from '@/i18n/locales/zh/admin/overview'

const { listGroups, deleteGroup, showSuccess, showError } = vi.hoisted(() => ({
  listGroups: vi.fn(),
  deleteGroup: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      delete: deleteGroup,
      getAll: vi.fn().mockResolvedValue([]),
      getAllIncludingInactive: vi.fn().mockResolvedValue([]),
      getModelAllowlistCandidates: vi.fn().mockResolvedValue([]),
      getUsageSummary: vi.fn().mockResolvedValue([]),
      getCapacitySummary: vi.fn().mockResolvedValue([]),
      getLiveCapability: vi.fn().mockResolvedValue({ supported: false }),
    },
    accounts: { list: vi.fn(), getById: vi.fn() },
  },
}))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isSimpleMode: false }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))
vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn() }),
}))

const group: AdminGroup = {
  id: 102,
  name: '【酒馆】量Claude-awsb cc',
  description: null,
  platform: 'anthropic',
  rate_multiplier: 1,
  empty_response_compensation_enabled: false,
  is_exclusive: false,
  status: 'inactive',
  subscription_type: 'standard',
  system_custom_routing_enabled: false,
  daily_limit_usd: null,
  weekly_limit_usd: null,
  monthly_limit_usd: null,
  long_context_pricing_enabled: true,
  force_openai_fast: false,
  free_openai_fast: false,
  model_pricing: [],
  profit_control_enabled: false,
  profit_min_margin: 0,
  profit_safety_buffer: 0,
  allow_image_generation: false,
  allow_batch_image_generation: false,
  image_count_billing: false,
  image_rate_independent: false,
  image_rate_multiplier: 1,
  batch_image_discount_multiplier: 0.5,
  batch_image_hold_multiplier: 0.6,
  image_price_1k: null,
  image_price_2k: null,
  image_price_4k: null,
  video_rate_independent: false,
  video_rate_multiplier: 1,
  video_price_480p: null,
  video_price_720p: null,
  video_price_1080p: null,
  web_search_price_per_call: null,
  search_price_per_1k: null,
  audio_realtime_price_per_min: null,
  audio_tts_price_per_million_chars: null,
  audio_stt_price_per_hour: null,
  peak_rate_enabled: false,
  peak_start: '',
  peak_end: '',
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  default_reasoning_effort: '',
  allow_live: false,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: '2026-11-01T00:00:00Z',
  updated_at: '2026-11-01T00:00:00Z',
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: false,
  simulate_claude_max_enabled: false,
  sort_order: 0,
}

const wrappers: VueWrapper[] = []
function mountView(subscription = false) {
  listGroups.mockResolvedValue({
    items: [{ ...group, subscription_type: subscription ? 'subscription' : 'standard' }],
    total: 1, page: 1, page_size: 20, pages: 1,
  })
  const wrapper = mount(GroupsView, {
    global: {
      plugins: [createI18n({
        legacy: false,
        locale: 'zh',
        missingWarn: false,
        fallbackWarn: false,
        messages: { zh: { common: { delete: '删除', cancel: '取消' }, admin: zhAdminOverview } },
      })],
      stubs: {
        AppLayout: defineComponent({ template: '<main><slot /></main>' }),
        TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
        DataTable: defineComponent({
          props: ['data'],
          template: '<div data-testid="group-table"><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>',
        }),
        BaseDialog: defineComponent({
          props: ['show', 'title'],
          template: '<section v-if="show" role="dialog"><h2>{{ title }}</h2><slot /><slot name="footer" /></section>',
        }),
        Select: true,
        Pagination: true,
        EmptyState: true,
        Icon: true,
        PlatformIcon: true,
        GroupTagField: true,
        ModelAliasesField: true,
        GroupModelStatusVisibilityField: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        GroupAccountUserExclusionsModal: true,
        SystemCustomGroupDialog: true,
        PricingEntryCard: true,
        ReasoningEffortPolicyFields: true,
        CodexManifestAccountsField: true,
        VueDraggable: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

async function openDelete(wrapper: VueWrapper) {
  await flushPromises()
  const trigger = wrapper.get('[data-testid="group-table"]').findAll('button').find(button => button.text() === '删除')
  expect(trigger).toBeDefined()
  await trigger!.trigger('click')
  return wrapper.get('[role="dialog"]')
}

describe('GroupsView group deletion', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    vi.mocked(adminAPI.groups.getAll).mockResolvedValue([])
    vi.mocked(adminAPI.groups.getAllIncludingInactive).mockResolvedValue([])
    vi.mocked(adminAPI.groups.getModelAllowlistCandidates).mockResolvedValue([])
    vi.mocked(adminAPI.groups.getUsageSummary).mockResolvedValue([])
    vi.mocked(adminAPI.groups.getCapacitySummary).mockResolvedValue([])
    vi.mocked(adminAPI.groups.getLiveCapability).mockResolvedValue({ supported: false })
    vi.spyOn(console, 'error').mockImplementation(() => undefined)
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
    vi.restoreAllMocks()
  })

  it.each([
    ['normalized API client', { status: 409, message: '分组当前正在更新，请稍后重试' }, '分组当前正在更新，请稍后重试'],
    ['standard response envelope', { response: { data: { message: '分组不存在，请刷新列表' } } }, '分组不存在，请刷新列表'],
    ['backend message over transport summary', { message: 'Request failed with status code 409', response: { data: { message: '分组操作冲突，请稍后重试', detail: '旧版错误详情' } } }, '分组操作冲突，请稍后重试'],
    ['legacy detail', { response: { data: { detail: '请求超时，请稍后重试' } } }, '请求超时，请稍后重试'],
    ['missing message', {}, '删除分组失败'],
  ])('shows the %s error and keeps confirmation available', async (_name, error, message) => {
    deleteGroup.mockRejectedValue(error)
    const wrapper = mountView()
    const dialog = await openDelete(wrapper)
    expect(dialog.text()).toContain(group.name)
    await dialog.findAll('button').find(button => button.text() === '删除')!.trigger('click')
    await flushPromises()
    expect(deleteGroup).toHaveBeenCalledTimes(1)
    expect(deleteGroup).toHaveBeenCalledWith(102)
    expect(showError).toHaveBeenCalledTimes(1)
    expect(showError).toHaveBeenCalledWith(message)
    expect(showSuccess).not.toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(true)
    expect(listGroups).toHaveBeenCalledTimes(1)
  })

  it.each([false, true])('explains reference cleanup before deleting and supports cancellation (subscription: %s)', async (subscription) => {
    const wrapper = mountView(subscription)
    const dialog = await openDelete(wrapper)
    expect(dialog.text()).toContain('自定义模型配置')
    expect(dialog.text()).toContain('其他来源的模型')
    if (subscription) expect(dialog.text()).toContain('订阅记录')
    await dialog.findAll('button').find(button => button.text() === '取消')!.trigger('click')
    expect(deleteGroup).not.toHaveBeenCalled()
    expect(wrapper.find('[role="dialog"]').exists()).toBe(false)
  })
})
