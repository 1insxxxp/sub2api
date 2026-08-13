import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import SystemCustomGroupDialog from '../SystemCustomGroupDialog.vue'

const {
  createSystemCustomGroup,
  deleteSystemCustomGroup,
  getSystemCustomGroup,
  getSystemCustomGroupCandidates,
  getSystemCustomGroupSyncPreview,
  updateSystemCustomGroup
} = vi.hoisted(() => ({
  createSystemCustomGroup: vi.fn(),
  deleteSystemCustomGroup: vi.fn(),
  getSystemCustomGroup: vi.fn(),
  getSystemCustomGroupCandidates: vi.fn(),
  getSystemCustomGroupSyncPreview: vi.fn(),
  updateSystemCustomGroup: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      createSystemCustomGroup,
      deleteSystemCustomGroup,
      getSystemCustomGroup,
      getSystemCustomGroupCandidates,
      getSystemCustomGroupSyncPreview,
      updateSystemCustomGroup
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (params?.model) return `${key}:${params.model}`
        if (params?.source) return `${key}:${params.source}`
        return key
      }
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
  models: [
    {
      id: 1,
      group_id: 90,
      public_model: 'claude-sonnet-4',
      source_group_id: 11,
      source_model: 'claude-sonnet-4',
      enabled: true,
      source_group: candidates[0].group,
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z'
    },
    {
      id: 2,
      group_id: 90,
      public_model: 'legacy-haiku',
      source_group_id: 11,
      source_model: 'legacy-haiku',
      enabled: true,
      source_group: candidates[0].group,
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z'
    },
    {
      id: 3,
      group_id: 90,
      public_model: 'gpt-5@B',
      source_group_id: 22,
      source_model: 'gpt-5',
      enabled: false,
      source_group: candidates[1].group,
      created_at: '2026-08-01T00:00:00Z',
      updated_at: '2026-08-01T00:00:00Z'
    }
  ]
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
          props: ['show'],
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

function modelRow(
  wrapper: ReturnType<typeof mountDialog>,
  sourceID: number,
  sourceModel: string
) {
  return wrapper.get(
    `[data-testid="system-custom-model-row"][data-source-id="${sourceID}"][data-source-model="${sourceModel}"]`
  )
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

  it('loads candidates, keeps unique names and sends the complete selected route snapshot', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(getSystemCustomGroupCandidates).toHaveBeenCalledTimes(1)
    await wrapper.get('[data-testid="system-custom-name"]').setValue('酒馆综合月卡')
    await wrapper.get('[data-testid="system-custom-daily-limit"]').setValue('20')
    await wrapper.get('[data-testid="system-custom-monthly-limit"]').setValue('300')
    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)

    const uniqueRow = modelRow(wrapper, 11, 'claude-haiku-4')
    await uniqueRow.get('input[type="checkbox"]').setValue(true)
    expect(uniqueRow.get('[data-testid="system-custom-public-model"]').element).toHaveProperty(
      'value',
      'claude-haiku-4'
    )

    const duplicateA = modelRow(wrapper, 11, 'claude-sonnet-4')
    const duplicateB = modelRow(wrapper, 22, 'claude-sonnet-4')
    await duplicateA.get('input[type="checkbox"]').setValue(true)
    await duplicateB.get('input[type="checkbox"]').setValue(true)
    await duplicateB.get('[data-testid="system-custom-public-model"]').setValue(
      'CLAUDE-SONNET-4'
    )

    expect(wrapper.get('[data-testid="system-custom-conflict"]').text()).toContain(
      'claude-sonnet-4'
    )
    expect(wrapper.get('[data-testid="system-custom-save"]').attributes('disabled')).toBeDefined()

    await duplicateB.get('[data-testid="system-custom-public-model"]').setValue(
      'claude-sonnet-4@乙'
    )
    expect(wrapper.find('[data-testid="system-custom-conflict"]').exists()).toBe(false)

    // Source filters only control visibility. Draft aliases survive deselection/reselection.
    await sourceCheckbox(wrapper, 22).setValue(false)
    await sourceCheckbox(wrapper, 22).setValue(true)
    expect(
      modelRow(wrapper, 22, 'claude-sonnet-4')
        .get('[data-testid="system-custom-public-model"]')
        .element
    ).toHaveProperty('value', 'claude-sonnet-4@乙')

    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()

    expect(createSystemCustomGroup).toHaveBeenCalledWith({
      name: '酒馆综合月卡',
      description: null,
      daily_limit_usd: 20,
      weekly_limit_usd: null,
      monthly_limit_usd: 300,
      default_validity_days: 30,
      models: [
        {
          public_model: 'claude-sonnet-4',
          source_group_id: 11,
          source_model: 'claude-sonnet-4',
          enabled: true
        },
        {
          public_model: 'claude-haiku-4',
          source_group_id: 11,
          source_model: 'claude-haiku-4',
          enabled: true
        },
        {
          public_model: 'claude-sonnet-4@乙',
          source_group_id: 22,
          source_model: 'claude-sonnet-4',
          enabled: true
        }
      ]
    })
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })

  it('loads edit details and preserves enabled flags in an update snapshot', async () => {
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()

    expect(getSystemCustomGroup).toHaveBeenCalledWith(90)
    expect(wrapper.get('[data-testid="system-custom-name"]').element).toHaveProperty(
      'value',
      '酒馆综合月卡'
    )
    expect(
      modelRow(wrapper, 22, 'gpt-5').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', true)

    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()

    expect(updateSystemCustomGroup).toHaveBeenCalledWith(
      90,
      expect.objectContaining({
        name: '酒馆综合月卡',
        models: expect.arrayContaining([
          {
            public_model: 'gpt-5@B',
            source_group_id: 22,
            source_model: 'gpt-5',
            enabled: false
          }
        ])
      })
    )
  })

  it('keeps hidden source drafts but excludes them from conflicts and the saved snapshot', async () => {
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-name"]').setValue('酒馆综合月卡')
    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)

    const routeA = modelRow(wrapper, 11, 'claude-sonnet-4')
    const routeB = modelRow(wrapper, 22, 'claude-sonnet-4')
    await routeA.get('input[type="checkbox"]').setValue(true)
    await routeB.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.find('[data-testid="system-custom-conflict"]').exists()).toBe(true)

    await sourceCheckbox(wrapper, 22).setValue(false)
    expect(wrapper.find('[data-testid="system-custom-conflict"]').exists()).toBe(false)
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(createSystemCustomGroup.mock.calls.at(-1)?.[0].models).toEqual([
      {
        public_model: 'claude-sonnet-4',
        source_group_id: 11,
        source_model: 'claude-sonnet-4',
        enabled: true
      }
    ])

    await sourceCheckbox(wrapper, 22).setValue(true)
    expect(
      modelRow(wrapper, 22, 'claude-sonnet-4')
        .get('[data-testid="system-custom-public-model"]')
        .element
    ).toHaveProperty('value', 'claude-sonnet-4')
    expect(wrapper.find('[data-testid="system-custom-conflict"]').exists()).toBe(true)
  })

  it('normalizes route identity without locale-sensitive casing', async () => {
    const localeLower = vi
      .spyOn(String.prototype, 'toLocaleLowerCase')
      .mockImplementation(() => {
        throw new Error('locale-sensitive casing must not be used')
      })
    try {
      const wrapper = mountDialog()
      await flushPromises()
      await sourceCheckbox(wrapper, 11).setValue(true)
      await sourceCheckbox(wrapper, 22).setValue(true)
      await modelRow(wrapper, 11, 'claude-sonnet-4')
        .get('input[type="checkbox"]')
        .setValue(true)
      await modelRow(wrapper, 22, 'claude-sonnet-4')
        .get('input[type="checkbox"]')
        .setValue(true)
      expect(wrapper.get('[data-testid="system-custom-conflict"]').text()).toContain(
        'claude-sonnet-4'
      )
    } finally {
      localeLower.mockRestore()
    }
  })

  it('previews added, missing and conflicting routes without silently applying them', async () => {
    getSystemCustomGroupSyncPreview.mockResolvedValue({
      added: [
        {
          public_model: 'claude-haiku-4',
          source_group_id: 11,
          source_model: 'claude-haiku-4',
          selected: false
        }
      ],
      missing: [existingGroup.models[1]],
      conflicting: [
        {
          public_model: 'claude-sonnet-4',
          source_group_id: 22,
          source_model: 'claude-sonnet-4',
          reason: 'duplicate public model'
        }
      ]
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()

    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()

    const added = wrapper.get('[data-testid="system-custom-sync-added"]')
    expect(added.text()).toContain('claude-haiku-4')
    expect(added.get('input[type="checkbox"]').element).toHaveProperty('checked', false)

    const missing = wrapper.get('[data-testid="system-custom-sync-missing"]')
    expect(missing.text()).toContain('legacy-haiku')
    expect(missing.get('input[type="checkbox"]').element).toHaveProperty('checked', false)
    expect(wrapper.get('[data-testid="system-custom-sync-conflicting"]').text()).toContain(
      'claude-sonnet-4'
    )

    await added.get('input[type="checkbox"]').setValue(true)
    await missing.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()

    const snapshot = updateSystemCustomGroup.mock.calls.at(-1)?.[1]
    expect(snapshot.models).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ source_model: 'claude-haiku-4', enabled: true }),
        expect.objectContaining({ source_model: 'legacy-haiku', enabled: false })
      ])
    )
  })

  it('selecting a sync-added route activates its valid source and rejects removed sources', async () => {
    getSystemCustomGroup.mockResolvedValueOnce({
      ...existingGroup,
      models: [existingGroup.models[0]]
    })
    getSystemCustomGroupSyncPreview.mockResolvedValueOnce({
      added: [
        {
          public_model: 'gpt-5',
          source_group_id: 22,
          source_model: 'gpt-5',
          selected: false
        },
        {
          public_model: 'retired-model',
          source_group_id: 99,
          source_model: 'retired-model',
          selected: false
        }
      ],
      missing: [],
      conflicting: []
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    expect(sourceCheckbox(wrapper, 22).element).toHaveProperty('checked', false)
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()

    const addedRows = wrapper.findAll('[data-testid="system-custom-sync-added"]')
    await addedRows[0].get('input[type="checkbox"]').setValue(true)
    expect(sourceCheckbox(wrapper, 22).element).toHaveProperty('checked', true)
    expect(modelRow(wrapper, 22, 'gpt-5').text()).toContain('gpt-5')

    await addedRows[1].get('input[type="checkbox"]').setValue(true)
    expect(wrapper.get('[data-testid="system-custom-error"]').text()).toContain('retired-model')
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(updateSystemCustomGroup.mock.calls.at(-1)?.[1].models).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ source_group_id: 99 })])
    )
  })

  it('deletes edit groups only after explicit confirmation and reports backend details', async () => {
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()

    await wrapper.get('[data-testid="system-custom-delete"]').trigger('click')
    expect(deleteSystemCustomGroup).not.toHaveBeenCalled()
    await wrapper.get('[data-testid="system-custom-delete-confirm"]').trigger('click')
    await flushPromises()
    expect(deleteSystemCustomGroup).toHaveBeenCalledWith(90)
    expect(wrapper.emitted('deleted')).toHaveLength(1)

    deleteSystemCustomGroup.mockRejectedValueOnce({
      response: {
        data: {
          message: 'system custom group is in use',
          metadata: { public_model: 'claude-sonnet-4' }
        }
      }
    })
    await wrapper.get('[data-testid="system-custom-delete"]').trigger('click')
    await wrapper.get('[data-testid="system-custom-delete-confirm"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="system-custom-error"]').text()).toContain(
      'claude-sonnet-4'
    )
  })

  it('shows backend reason and concrete metadata even without status or code', async () => {
    createSystemCustomGroup.mockRejectedValue({
      response: {
        data: {
          message: 'validation failed',
          reason: 'duplicate public model',
          metadata: { public_model: 'claude-sonnet-4' }
        }
      }
    })
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-name"]').setValue('酒馆综合月卡')
    await sourceCheckbox(wrapper, 11).setValue(true)
    const row = modelRow(wrapper, 11, 'claude-haiku-4')
    await row.get('input[type="checkbox"]').setValue(true)
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()

    const error = wrapper.get('[data-testid="system-custom-error"]').text()
    expect(error).toContain('duplicate public model')
    expect(error).toContain('claude-sonnet-4')
    expect(error).not.toBe('internal error')

    createSystemCustomGroup.mockRejectedValueOnce({
      response: {
        data: {
          message: 'invalid route',
          metadata: 'source model gpt-5-mini is unavailable'
        }
      }
    })
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="system-custom-error"]').text()).toContain('gpt-5-mini')
  })
})
