import { defineComponent } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AdminGroup, GroupPlatform } from '@/types'
import GroupsView from '@/views/admin/GroupsView.vue'

const { listGroups, createGroup, updateGroup, authState } = vi.hoisted(() => ({
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  authState: { isSimpleMode: false },
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      create: createGroup,
      update: updateGroup,
      getAll: vi.fn().mockResolvedValue([]),
      getAllIncludingInactive: vi.fn().mockResolvedValue([]),
      getModelAllowlistCandidates: vi.fn().mockResolvedValue([]),
      getUsageSummary: vi.fn().mockResolvedValue([]),
      getCapacitySummary: vi.fn().mockResolvedValue([]),
      getLiveCapability: vi.fn().mockResolvedValue({ supported: false }),
      previewPricingCoverage: vi.fn().mockResolvedValue({ models: [] }),
    },
    accounts: { list: vi.fn(), getById: vi.fn() },
  },
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => authState }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }),
}))
vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn() }),
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))

const sourceGroup = (platform: GroupPlatform, enabled?: boolean): AdminGroup => ({
  id: 42,
  name: 'Input counting group',
  description: null,
  platform,
  rate_multiplier: 1,
  empty_response_compensation_enabled: false,
  is_exclusive: false,
  status: 'active',
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
  ...(enabled === undefined ? {} : { gemini_input_counting_enabled: enabled }),
})

const SelectStub = defineComponent({
  props: ['modelValue', 'options', 'disabled'],
  emits: ['update:modelValue'],
  template: '<select :value="modelValue" :disabled="disabled" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>',
})

const wrappers: VueWrapper[] = []
const mountView = () => {
  const wrapper = mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: defineComponent({ template: '<main><slot /></main>' }),
        TablePageLayout: defineComponent({ template: '<section><slot name="filters" /><slot name="table" /></section>' }),
        DataTable: defineComponent({
          props: ['data'],
          template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>',
        }),
        BaseDialog: defineComponent({
          props: ['show'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>',
        }),
        Select: SelectStub,
        Pagination: true,
        ConfirmDialog: true,
        EmptyState: true,
        Icon: true,
        PlatformIcon: true,
        GroupTagField: true,
        ModelAliasesField: true,
        GroupModelStatusVisibilityField: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        PricingEntryCard: true,
        ReasoningEffortPolicyFields: defineComponent({
          template: '<div />',
          setup(_props, { expose }) {
            expose({ validate: () => true, resetValidation: () => undefined })
          },
        }),
        CodexManifestAccountsField: true,
        VueDraggable: true,
      },
    },
  })
  wrappers.push(wrapper)
  return wrapper
}

const openCreate = async (wrapper: VueWrapper, platform: GroupPlatform) => {
  await flushPromises()
  await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
  await wrapper.get('[data-tour="group-form-platform"]').setValue(platform)
  await wrapper.get('[data-tour="group-form-name"]').setValue('New input group')
  await flushPromises()
}
const openEdit = async (wrapper: VueWrapper) => {
  await flushPromises()
  await wrapper.findAll('button').find(button => button.text() === 'common.edit')!.trigger('click')
  await flushPromises()
}
const submit = async (wrapper: VueWrapper, form: 'create' | 'edit') => {
  await wrapper.get(`#${form}-group-form`).trigger('submit')
  await flushPromises()
}

describe('GroupsView Gemini input counting policy', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
    authState.isSimpleMode = false
    listGroups.mockResolvedValue({ items: [sourceGroup('gemini')], total: 1, page: 1, page_size: 20, pages: 1 })
    createGroup.mockResolvedValue(sourceGroup('gemini'))
    updateGroup.mockResolvedValue(sourceGroup('gemini'))
  })

  afterEach(() => {
    for (const wrapper of wrappers.splice(0)) wrapper.unmount()
  })

  it.each(['gemini', 'composite'] as const)('creates %s groups with the policy off by default and explicitly enables it', async (platform) => {
    const wrapper = mountView()
    await openCreate(wrapper, platform)
    expect(wrapper.get<HTMLInputElement>('[data-testid="create-gemini-input-counting"]').element.checked).toBe(false)
    await submit(wrapper, 'create')
    expect(createGroup).toHaveBeenLastCalledWith(expect.objectContaining({ platform, gemini_input_counting_enabled: false }))

    await openCreate(wrapper, platform)
    expect(wrapper.get<HTMLInputElement>('[data-testid="create-gemini-input-counting"]').element.checked).toBe(false)
    await wrapper.get('[data-testid="create-gemini-input-counting"]').setValue(true)
    await submit(wrapper, 'create')
    expect(createGroup).toHaveBeenLastCalledWith(expect.objectContaining({ platform, gemini_input_counting_enabled: true }))
    await openCreate(wrapper, platform)
    expect(wrapper.get<HTMLInputElement>('[data-testid="create-gemini-input-counting"]').element.checked).toBe(false)
  })

  it.each(['gemini', 'composite'] as const)('restores the saved %s policy, preserves it on unrelated edits, and saves an explicit disable', async (platform) => {
    const group = sourceGroup(platform, true)
    listGroups.mockResolvedValue({ items: [group], total: 1, page: 1, page_size: 20, pages: 1 })
    updateGroup.mockResolvedValue(group)
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.get<HTMLInputElement>('[data-testid="edit-gemini-input-counting"]').element.checked).toBe(true)
    await wrapper.get('[data-tour="edit-group-form-name"]').setValue('Renamed group')
    await submit(wrapper, 'edit')
    expect(updateGroup).toHaveBeenLastCalledWith(42, expect.objectContaining({ gemini_input_counting_enabled: true }))
    expect(group.gemini_input_counting_enabled).toBe(true)

    await openEdit(wrapper)
    await wrapper.get('[data-testid="edit-gemini-input-counting"]').setValue(false)
    await submit(wrapper, 'edit')
    expect(updateGroup).toHaveBeenLastCalledWith(42, expect.objectContaining({ gemini_input_counting_enabled: false }))
  })

  it('loads older group responses without opting in', async () => {
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.get<HTMLInputElement>('[data-testid="edit-gemini-input-counting"]').element.checked).toBe(false)
    await submit(wrapper, 'edit')
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({ gemini_input_counting_enabled: false }))
  })

  it('clears the create opt-in when switching to an unsupported platform', async () => {
    const wrapper = mountView()
    await openCreate(wrapper, 'gemini')
    await wrapper.get('[data-testid="create-gemini-input-counting"]').setValue(true)
    await wrapper.get('[data-tour="group-form-platform"]').setValue('openai')
    expect(wrapper.find('[data-testid="create-gemini-input-counting"]').exists()).toBe(false)
    await submit(wrapper, 'create')
    expect(createGroup).toHaveBeenCalledWith(expect.objectContaining({ platform: 'openai', gemini_input_counting_enabled: false }))
    await openCreate(wrapper, 'gemini')
    expect(wrapper.get<HTMLInputElement>('[data-testid="create-gemini-input-counting"]').element.checked).toBe(false)
  })

  it('normalizes unsupported saved groups to disabled', async () => {
    listGroups.mockResolvedValue({ items: [sourceGroup('openai', true)], total: 1, page: 1, page_size: 20, pages: 1 })
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.find('[data-testid="edit-gemini-input-counting"]').exists()).toBe(false)
    await submit(wrapper, 'edit')
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({ gemini_input_counting_enabled: false }))
  })

  it('keeps the billing policy out of simple-mode edits', async () => {
    authState.isSimpleMode = true
    const wrapper = mountView()
    await openEdit(wrapper)
    expect(wrapper.find('[data-testid="edit-gemini-input-counting"]').exists()).toBe(false)
    await submit(wrapper, 'edit')
    expect(updateGroup).toHaveBeenCalledWith(42, expect.not.objectContaining({ gemini_input_counting_enabled: expect.anything() }))
  })
})
