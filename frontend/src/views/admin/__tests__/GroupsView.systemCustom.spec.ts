import { defineComponent } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import GroupsView from '../GroupsView.vue'

const { list, getModelsListCandidates, getUsageSummary, getCapacitySummary, showSuccess } =
  vi.hoisted(() => ({
    list: vi.fn(),
    getModelsListCandidates: vi.fn(),
    getUsageSummary: vi.fn(),
    getCapacitySummary: vi.fn(),
    showSuccess: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list,
      getModelsListCandidates,
      getUsageSummary,
      getCapacitySummary
    },
    accounts: {
      list: vi.fn(),
      getById: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError: vi.fn() })
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({ nextStep: vi.fn() })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const systemGroup = {
  id: 90,
  name: '酒馆综合月卡',
  platform: 'composite',
  subscription_type: 'subscription',
  system_custom_routing_enabled: true,
  daily_limit_usd: 20,
  weekly_limit_usd: null,
  monthly_limit_usd: 300,
  status: 'active',
  is_exclusive: true,
  rate_multiplier: 1
}

const ordinaryComposite = {
  ...systemGroup,
  id: 91,
  name: '普通 Composite',
  subscription_type: 'standard',
  system_custom_routing_enabled: false
}

const SystemCustomDialogStub = defineComponent({
  name: 'SystemCustomGroupDialog',
  props: {
    show: Boolean,
    groupId: { type: Number, default: null }
  },
  emits: ['close', 'saved', 'deleted'],
  template: '<div v-if="show" data-testid="system-custom-dialog"></div>'
})

function mountView() {
  return shallowMount(GroupsView, {
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        TablePageLayout: {
          template:
            '<section><slot name="filters"/><slot name="table"/><slot name="pagination"/></section>'
        },
        DataTable: {
          props: ['data'],
          template:
            '<div><div v-for="row in data" :key="row.id" :data-row-id="row.id"><slot name="cell-billing_type" :row="row"/><slot name="cell-actions" :row="row"/></div></div>'
        },
        SystemCustomGroupDialog: SystemCustomDialogStub,
        BaseDialog: true,
        ConfirmDialog: true,
        Pagination: true,
        Select: true,
        Icon: true,
        EmptyState: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true,
        GroupCapacityBadge: true,
        ReasoningEffortPolicyFields: true,
        VueDraggable: true
      }
    }
  })
}

describe('GroupsView system custom rows', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    list.mockResolvedValue({
      items: [systemGroup, ordinaryComposite],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getModelsListCandidates.mockResolvedValue([])
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
  })

  it('renders dedicated system actions while ordinary Composite keeps ordinary controls', async () => {
    const wrapper = mountView()
    await flushPromises()

    const systemRow = wrapper.get('[data-row-id="90"]')
    expect(systemRow.find('[data-testid="system-custom-type-badge"]').exists()).toBe(true)
    expect(systemRow.find('[data-testid="system-custom-manage"]').exists()).toBe(true)
    const systemActions = systemRow.findAll('button').map((button) => button.text())
    expect(systemActions).not.toContain('common.edit')
    expect(systemActions).not.toContain('admin.groups.duplicate')
    expect(systemActions).not.toContain('admin.groups.compositeRoutes.action')
    expect(systemActions).not.toContain('common.delete')

    const ordinaryRow = wrapper.get('[data-row-id="91"]')
    expect(ordinaryRow.find('[data-testid="system-custom-type-badge"]').exists()).toBe(false)
    expect(ordinaryRow.find('[data-testid="system-custom-manage"]').exists()).toBe(false)
    const ordinaryActions = ordinaryRow.findAll('button').map((button) => button.text())
    expect(ordinaryActions).toEqual(
      expect.arrayContaining([
        'common.edit',
        'admin.groups.duplicate',
        'admin.groups.compositeRoutes.action',
        'common.delete'
      ])
    )
  })

  it('keeps both create actions visible in the responsive toolbar', async () => {
    const wrapper = mountView()
    await flushPromises()

    const toolbar = wrapper.get('[data-testid="groups-toolbar"]')
    expect(toolbar.classes()).toEqual(
      expect.arrayContaining(['lg:flex-col', 'lg:items-stretch', '2xl:flex-row', '2xl:items-center'])
    )

    const actions = wrapper.get('[data-testid="groups-toolbar-actions"]')
    expect(actions.classes()).toEqual(
      expect.arrayContaining(['lg:w-full', '2xl:w-auto', '2xl:flex-none'])
    )

    const createActions = wrapper.get('[data-testid="groups-create-actions"]')
    expect(createActions.get('[data-testid="system-custom-create"]').classes()).toContain(
      'whitespace-nowrap'
    )
    expect(createActions.get('[data-tour="groups-create-btn"]').classes()).toContain(
      'whitespace-nowrap'
    )
  })

  it('opens the dedicated dialog and refreshes after saved, deleted, and close events', async () => {
    const wrapper = mountView()
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-row-id="90"] [data-testid="system-custom-manage"]').trigger('click')
    const dialog = wrapper.findComponent(SystemCustomDialogStub)
    expect(dialog.props('show')).toBe(true)
    expect(dialog.props('groupId')).toBe(90)
    dialog.vm.$emit('saved')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(2)

    await wrapper.get('[data-row-id="90"] [data-testid="system-custom-manage"]').trigger('click')
    dialog.vm.$emit('deleted')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(3)

    await wrapper.get('[data-testid="system-custom-create"]').trigger('click')
    dialog.vm.$emit('close')
    await flushPromises()
    expect(list).toHaveBeenCalledTimes(4)
  })
})
