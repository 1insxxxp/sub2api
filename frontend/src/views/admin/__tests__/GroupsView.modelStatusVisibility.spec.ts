import { defineComponent } from "vue";
import { flushPromises, mount } from "@vue/test-utils";
import { createPinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AdminGroup } from "@/types";
import GroupsView from "@/views/admin/GroupsView.vue";

const {
  listGroups,
  createGroup,
  updateGroup,
  showError,
  getModelAllowlistCandidates,
  getUsageSummary,
  getCapacitySummary,
  getLiveCapability,
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
  showError: vi.fn(),
  getModelAllowlistCandidates: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getLiveCapability: vi.fn(),
}));

vi.mock("@/api/admin", () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      getAll: vi.fn().mockResolvedValue([]),
      previewPricingCoverage: vi.fn(async ({ models }: { models: string[] }) => ({ models: models.map(model => ({ model, status: "priced" })) })),
      getModelAllowlistCandidates,
      getUsageSummary,
      getCapacitySummary,
      getLiveCapability,
      create: createGroup,
      update: updateGroup,
      delete: vi.fn(),
      duplicate: vi.fn(),
      updateSortOrder: vi.fn(),
    },
    accounts: {
      list: vi.fn(),
      getById: vi.fn(),
    },
  },
}));

vi.mock("@/stores/app", () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
  }),
}));

vi.mock("@/stores/onboarding", () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn(),
  }),
}));

vi.mock("vue-i18n", async () => {
  const actual = await vi.importActual<typeof import("vue-i18n")>("vue-i18n");
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  };
});

const sourceGroup = {
  id: 42,
  name: "OpenAI",
  description: null,
  platform: "openai",
  rate_multiplier: 1,
  rpm_limit: 0,
  is_exclusive: false,
  status: "active",
  subscription_type: "standard",
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
  peak_start: "",
  peak_end: "",
  peak_rate_multiplier: 1,
  claude_code_only: false,
  fallback_group_id: null,
  fallback_group_id_on_invalid_request: null,
  allow_messages_dispatch: false,
  allow_live: false,
  require_oauth_only: false,
  require_privacy_set: false,
  created_at: "2026-09-05T00:00:00Z",
  updated_at: "2026-09-05T00:00:00Z",
  model_routing: null,
  model_routing_enabled: false,
  mcp_xml_inject: true,
  supported_model_scopes: [],
  account_count: 1,
  active_account_count: 1,
  rate_limited_account_count: 0,
  model_allowlist: { enabled: true, models: ["gpt-*"] },
  model_status_visibility: { enabled: true, models: ["gpt-5.4"] },
  codex_models_manifest_config: {
    enabled: false,
    account_ids: [],
    fallback_to_scheduler: false,
  },
  sort_order: 10,
} satisfies AdminGroup;

const AppLayoutStub = defineComponent({
  template: "<main><slot /></main>",
});

const TablePageLayoutStub = defineComponent({
  template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>',
});

const DataTableStub = defineComponent({
  props: {
    data: { type: Array, default: () => [] },
  },
  template: '<div><div v-if="data.length"><slot name="cell-actions" :row="data[0]" /></div></div>',
});

const BaseDialogStub = defineComponent({
  props: {
    show: { type: Boolean, default: false },
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
});

const mountView = () =>
  mount(GroupsView, {
    global: {
      plugins: [createPinia()],
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        EmptyState: true,
        Select: true,
        PlatformIcon: true,
        Icon: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        ReasoningEffortPolicyFields: defineComponent({ template: "<div />", setup(_props, { expose }) { expose({ validate: () => true, resetValidation: () => undefined }); } }),
        CodexManifestAccountsField: true,
        PricingEntryCard: true,
        VueDraggable: true,
      },
    },
  });


const edit = async (wrapper: ReturnType<typeof mountView>) => {
  await flushPromises();
  await wrapper.findAll('button').find(button => button.text().includes('common.edit'))!.trigger('click');
  await flushPromises();
};
const submit = async (wrapper: ReturnType<typeof mountView>) => {
  await wrapper.get('form').trigger('submit');
  await flushPromises();
};

describe('GroupsView model status visibility', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.clearAllMocks();
    getModelAllowlistCandidates.mockReset();
    getModelAllowlistCandidates.mockResolvedValue(['gpt-5.4', 'gpt-image-2']);
    listGroups.mockResolvedValue({items: [structuredClone(sourceGroup)], total: 1, page: 1, page_size: 20, pages: 1});
    getUsageSummary.mockResolvedValue([]);
    getCapacitySummary.mockResolvedValue([]);
    getLiveCapability.mockResolvedValue({supported: false});
    createGroup.mockResolvedValue(sourceGroup);
    updateGroup.mockResolvedValue(sourceGroup);
  });

  it('submits visibility independently while keeping image requests in the request allowlist', async () => {
    const wrapper = mountView();
    await edit(wrapper);
    const field = wrapper.get('[data-testid="model-status-visibility-field"]');
    expect((field.get('input[aria-label="gpt-5.4"]').element as HTMLInputElement).checked).toBe(true);
    expect((field.get('input[aria-label="gpt-image-2"]').element as HTMLInputElement).checked).toBe(false);
    await submit(wrapper);
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      model_status_visibility: {enabled: true, models: ['gpt-5.4']},
      model_allowlist: {enabled: true, models: ['gpt-*']},
    }));
    expect(sourceGroup.model_status_visibility.models).toEqual(['gpt-5.4']);
    wrapper.unmount();
  });

  it('blocks enabled empty selections and lets disabling restore all without changing request restrictions', async () => {
    const wrapper = mountView();
    await edit(wrapper);
    await wrapper.get('[data-testid="model-status-visibility-clear"]').trigger('click');
    await submit(wrapper);
    expect(updateGroup).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalledWith('admin.groups.modelStatusVisibility.emptySelectionError');
    await wrapper.get('[data-testid="model-status-visibility-toggle"]').trigger('click');
    await submit(wrapper);
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      model_status_visibility: {enabled: false, models: []},
      model_allowlist: {enabled: true, models: ['gpt-*']},
    }));
    wrapper.unmount();
  });

  it('retains saved selections when candidates fail to load', async () => {
    const group = structuredClone(sourceGroup);
    group.model_allowlist.enabled = false;
    listGroups.mockResolvedValue({items: [group], total: 1, page: 1, page_size: 20, pages: 1});
    getModelAllowlistCandidates.mockRejectedValue(new Error('test unavailable'));
    const wrapper = mountView();
    await edit(wrapper);
    expect(wrapper.find('[data-testid="model-status-visibility-load-error"]').exists()).toBe(true);
    expect((wrapper.get('[data-testid="model-status-visibility-field"] input[aria-label="gpt-5.4"]').element as HTMLInputElement).checked).toBe(true);
    await submit(wrapper);
    expect(updateGroup).toHaveBeenCalledWith(42, expect.objectContaining({
      model_status_visibility: {enabled: true, models: ['gpt-5.4']},
      model_allowlist: {enabled: false, models: ['gpt-*']},
    }));
    wrapper.unmount();
  });

  it('creates a group with all models visible by default', async () => {
    const wrapper = mountView();
    await flushPromises();
    await wrapper.findAll('button').find(button => button.text().includes('admin.groups.createGroup'))!.trigger('click');
    await flushPromises();
    expect(wrapper.get('[data-testid="model-status-visibility-toggle"]').attributes('aria-checked')).toBe('false');
    await wrapper.get('form input').setValue('New group');
    await submit(wrapper);
    expect(createGroup).toHaveBeenCalledWith(expect.objectContaining({model_status_visibility:{enabled:false,models:[]}}));
    wrapper.unmount();
  });
});
