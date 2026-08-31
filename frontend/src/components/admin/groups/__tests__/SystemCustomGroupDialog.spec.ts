import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SystemCustomGroupDialog from '../SystemCustomGroupDialog.vue'

const {
  createSystemCustomGroup,
  deleteSystemCustomGroup,
  getSystemCustomGroup,
  getSystemCustomGroupCandidates,
  updateSystemCustomGroup
} = vi.hoisted(() => ({
  createSystemCustomGroup: vi.fn(),
  deleteSystemCustomGroup: vi.fn(),
  getSystemCustomGroup: vi.fn(),
  getSystemCustomGroupCandidates: vi.fn(),
  updateSystemCustomGroup: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      createSystemCustomGroup,
      deleteSystemCustomGroup,
      getSystemCustomGroup,
      getSystemCustomGroupCandidates,
      updateSystemCustomGroup
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) =>
        params?.count === undefined ? key : `${key}:${params.count}`
    })
  }
})

const candidates = [
  {
    group: { id: 11, name: '酒馆甲', platform: 'anthropic' as const, status: 'active' as const },
    models: ['claude-sonnet-4', 'claude-haiku-4']
  },
  {
    group: { id: 22, name: '酒馆乙', platform: 'openai' as const, status: 'active' as const },
    models: ['claude-sonnet-4', 'gpt-5']
  }
]

const existingGroup = {
  group: {
    id: 90,
    name: '酒馆综合月卡',
    description: '多来源路由',
    platform: 'composite' as const,
    rate_multiplier: 1,
    is_exclusive: true,
    status: 'active' as const,
    subscription_type: 'subscription' as const,
    system_custom_routing_enabled: true as const,
    daily_limit_usd: 20,
    weekly_limit_usd: null,
    monthly_limit_usd: 300,
    default_validity_days: 30,
    created_at: '2026-08-01T00:00:00Z',
    updated_at: '2026-08-01T00:00:00Z'
  },
  sources: [
    {
      id: 1,
      group_id: 90,
      source_group_id: 22,
      priority: 0,
      group: candidates[1].group,
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z'
    },
    {
      id: 2,
      group_id: 90,
      source_group_id: 11,
      priority: 1,
      group: candidates[0].group,
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z'
    }
  ],
  summary: {
    unique_models: 3,
    fallback_routes: 1,
    unavailable_sources: 0,
    unpriced_routes: 0
  },
  models: []
}

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(SystemCustomGroupDialog, {
    props: {
      show: true,
      groupId: null,
      ...props
    },
    global: {
      stubs: {
        BaseDialog: {
          props: ['show', 'width'],
          emits: ['close'],
          template: '<div v-if="show"><slot /><slot name="footer" /></div>'
        },
        Icon: true
      }
    }
  })
}

function sourceCheckbox(wrapper: ReturnType<typeof mountDialog>, sourceID: number) {
  return wrapper.get(`[data-testid="system-custom-source-select"][data-source-id="${sourceID}"]`)
}

describe('SystemCustomGroupDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getSystemCustomGroupCandidates.mockResolvedValue(candidates)
    getSystemCustomGroup.mockResolvedValue(existingGroup)
    createSystemCustomGroup.mockResolvedValue(existingGroup)
    updateSystemCustomGroup.mockResolvedValue(existingGroup)
    deleteSystemCustomGroup.mockResolvedValue({ id: 90, deleted: true })
  })

  it('saves only ordered source group IDs and never exposes a model routing editor', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-testid="system-custom-name"]').setValue('酒馆综合月卡')
    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)
    await wrapper
      .get('[data-testid="system-custom-priority-row"][data-source-id="22"] [data-testid="system-custom-priority-up"]')
      .trigger('click')
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()

    expect(createSystemCustomGroup).toHaveBeenCalledWith({
      name: '酒馆综合月卡',
      description: null,
      daily_limit_usd: null,
      weekly_limit_usd: null,
      monthly_limit_usd: null,
      default_validity_days: 30,
      source_group_ids: [22, 11]
    })
    expect(wrapper.find('[data-testid="system-custom-model-row"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="system-custom-sync"]').exists()).toBe(false)
  })

  it('loads persisted source priority and recalculates the dynamic catalog summary', async () => {
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()

    const priorities = wrapper.findAll('[data-testid="system-custom-priority-row"]')
    expect(priorities.map((row) => row.attributes('data-source-id'))).toEqual(['22', '11'])
    expect(wrapper.get('[data-testid="system-custom-selected-source-count"]').text()).toBe('2')
    expect(wrapper.get('[data-testid="system-custom-unique-model-count"]').text()).toBe('3')
    expect(wrapper.get('[data-testid="system-custom-fallback-count"]').text()).toBe('1')
  })

  it('keeps source order and form state after a save error', async () => {
    createSystemCustomGroup.mockRejectedValueOnce({
      response: { data: { message: 'source group is unavailable' } }
    })
    const wrapper = mountDialog()
    await flushPromises()

    await wrapper.get('[data-testid="system-custom-name"]').setValue('酒馆综合月卡')
    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)
    await wrapper
      .get('[data-testid="system-custom-priority-row"][data-source-id="22"] [data-testid="system-custom-priority-up"]')
      .trigger('click')
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="system-custom-error"]').text()).toContain('source group is unavailable')
    expect(wrapper.get('[data-testid="system-custom-name"]').element).toHaveProperty('value', '酒馆综合月卡')
    expect(wrapper.findAll('[data-testid="system-custom-priority-row"]').map((row) => row.attributes('data-source-id'))).toEqual(['22', '11'])
  })

  it('uses a one-column mobile-first source flow with full-width footer actions', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.get('[data-testid="system-custom-source-workspace"]').classes()).toContain('lg:grid-cols-[minmax(0,1fr)_minmax(20rem,0.9fr)]')
    expect(wrapper.get('[data-testid="system-custom-save"]').classes()).toEqual(expect.arrayContaining(['flex-1', 'sm:flex-none']))
  })
})
