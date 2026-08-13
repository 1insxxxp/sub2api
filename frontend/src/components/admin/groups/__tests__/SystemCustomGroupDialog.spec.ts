import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
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
          props: ['show', 'width'],
          emits: ['close'],
          template:
            '<div v-if="show" :data-dialog-width="width"><button data-testid="base-dialog-close" @click="$emit(\'close\')"/><slot /><slot name="footer" /></div>'
        },
        Icon: true
      }
    }
  })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

const groupDetail = (id: number, name: string) => ({
  ...existingGroup,
  group: { ...existingGroup.group, id, name },
  models: existingGroup.models.map((model) => ({ ...model, group_id: id }))
})

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

  it('scrolls source and model columns independently while rendering every selected source', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)

    const sourceScroll = wrapper.get('[data-testid="system-custom-source-scroll"]')
    const modelScroll = wrapper.get('[data-testid="system-custom-model-scroll"]')
    expect(sourceScroll.classes()).toEqual(
      expect.arrayContaining(['lg:overflow-y-auto', 'lg:overscroll-contain'])
    )
    expect(modelScroll.classes()).toEqual(
      expect.arrayContaining(['lg:overflow-y-auto', 'lg:overscroll-contain'])
    )

    const sourceSections = wrapper.findAll('[data-testid="system-custom-source-section"]')
    expect(sourceSections).toHaveLength(2)
    expect(sourceSections.map((section) => section.attributes('data-source-id'))).toEqual([
      '11',
      '22'
    ])
    expect(sourceSections[0].text()).toContain('酒馆甲')
    expect(sourceSections[0].text()).toContain('claude-haiku-4')
    expect(sourceSections[1].text()).toContain('酒馆乙')
    expect(sourceSections[1].text()).toContain('gpt-5')

    const sourceNavigation = wrapper.findAll('[data-testid="system-custom-source-nav"]')
    expect(sourceNavigation).toHaveLength(2)
    expect(sourceNavigation.map((button) => button.attributes('data-source-id'))).toEqual([
      '11',
      '22'
    ])
  })

  it('renders every selected source as a normal-flow route card', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)

    expect(wrapper.get('[data-testid="system-custom-model-scroll"]').classes()).toContain(
      '[overflow-anchor:none]'
    )
    const sourceSections = wrapper.findAll('[data-testid="system-custom-source-section"]')
    expect(sourceSections).toHaveLength(2)
    for (const section of sourceSections) {
      expect(section.classes()).toEqual(
        expect.arrayContaining(['overflow-hidden', 'rounded-xl', 'border'])
      )
      expect(section.get('[data-testid="system-custom-source-section-header"]').classes()).not.toContain(
        'sticky'
      )
    }
  })

  it('selects all visible model routes and exposes a mixed state', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    const selectAll = wrapper.get('[data-testid="system-custom-model-select-all"]')
    expect(selectAll.attributes('disabled')).toBeDefined()

    await sourceCheckbox(wrapper, 11).setValue(true)
    expect(selectAll.attributes('disabled')).toBeUndefined()
    await selectAll.setValue(true)

    expect(
      modelRow(wrapper, 11, 'claude-sonnet-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', true)
    expect(
      modelRow(wrapper, 11, 'claude-haiku-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', true)

    await sourceCheckbox(wrapper, 22).setValue(true)
    expect(
      modelRow(wrapper, 22, 'claude-sonnet-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', false)
    expect(selectAll.element).toHaveProperty('indeterminate', true)
    expect(selectAll.attributes('aria-checked')).toBe('mixed')

    await selectAll.setValue(true)
    for (const row of wrapper.findAll('[data-testid="system-custom-model-row"]')) {
      expect(row.get('input[type="checkbox"]').element).toHaveProperty('checked', true)
    }

    await modelRow(wrapper, 22, 'gpt-5').get('input[type="checkbox"]').setValue(false)
    expect(selectAll.element).toHaveProperty('indeterminate', true)

    await selectAll.setValue(true)
    await selectAll.setValue(false)
    for (const row of wrapper.findAll('[data-testid="system-custom-model-row"]')) {
      expect(row.get('input[type="checkbox"]').element).toHaveProperty('checked', false)
    }
  })

  it('selects routes independently for each source with a tri-state group control', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)

    const sourceSelectAll = (sourceID: number) =>
      wrapper.get(
        `[data-testid="system-custom-source-select-all"][data-source-id="${sourceID}"]`
      )
    const sourceACheckbox = sourceSelectAll(11)
    const sourceBCheckbox = sourceSelectAll(22)

    await sourceACheckbox.setValue(true)
    expect(
      modelRow(wrapper, 11, 'claude-sonnet-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', true)
    expect(
      modelRow(wrapper, 11, 'claude-haiku-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', true)
    expect(
      modelRow(wrapper, 22, 'gpt-5').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', false)
    expect(sourceBCheckbox.element).toHaveProperty('checked', false)

    await modelRow(wrapper, 11, 'claude-haiku-4')
      .get('input[type="checkbox"]')
      .setValue(false)
    expect(sourceACheckbox.element).toHaveProperty('indeterminate', true)
    expect(sourceACheckbox.attributes('aria-checked')).toBe('mixed')

    await sourceACheckbox.setValue(true)
    await sourceACheckbox.setValue(false)
    expect(
      modelRow(wrapper, 11, 'claude-sonnet-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', false)
    expect(
      modelRow(wrapper, 22, 'gpt-5').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', false)
  })

  it('collapses each source independently without changing its route draft', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)
    await modelRow(wrapper, 11, 'claude-sonnet-4')
      .get('input[type="checkbox"]')
      .setValue(true)

    const sourceToggle = (sourceID: number) =>
      wrapper.get(
        `[data-testid="system-custom-source-collapse"][data-source-id="${sourceID}"]`
      )
    const sourceModels = (sourceID: number) =>
      wrapper.get(
        `[data-testid="system-custom-source-models"][data-source-id="${sourceID}"]`
      )

    expect(sourceToggle(11).attributes('aria-expanded')).toBe('true')
    expect(sourceToggle(22).attributes('aria-expanded')).toBe('true')

    await sourceToggle(11).trigger('click')
    expect(sourceToggle(11).attributes('aria-expanded')).toBe('false')
    expect(sourceModels(11).attributes('style')).toContain('display: none')
    expect(sourceToggle(22).attributes('aria-expanded')).toBe('true')
    expect(sourceModels(22).attributes('style') ?? '').not.toContain('display: none')
    expect(
      wrapper
        .get('[data-testid="system-custom-source-section"][data-source-id="11"]')
        .text()
    ).toContain('1 / 2')

    await sourceToggle(11).trigger('click')
    expect(sourceToggle(11).attributes('aria-expanded')).toBe('true')
    expect(
      modelRow(wrapper, 11, 'claude-sonnet-4').get('input[type="checkbox"]').element
    ).toHaveProperty('checked', true)

    await sourceToggle(11).trigger('click')
    const modelScroll = wrapper.get('[data-testid="system-custom-model-scroll"]')
      .element as HTMLElement
    modelScroll.scrollTo = vi.fn()
    await wrapper
      .get('[data-testid="system-custom-source-nav"][data-source-id="11"]')
      .trigger('click')
    await flushPromises()
    expect(sourceToggle(11).attributes('aria-expanded')).toBe('true')
  })

  it('prioritizes the route workspace and collapses optional settings on create', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.attributes('data-dialog-width')).toBe('full')
    expect(wrapper.get('[data-testid="system-custom-route-workspace"]').classes()).toContain(
      'lg:h-[min(64vh,46rem)]'
    )
    expect(
      wrapper.get('[data-testid="system-custom-advanced-settings"]').attributes('style')
    ).toContain('display: none')

    const advancedToggle = wrapper.get('[data-testid="system-custom-advanced-toggle"]')
    expect(advancedToggle.attributes('aria-expanded')).toBe('false')
    await advancedToggle.trigger('click')
    expect(advancedToggle.attributes('aria-expanded')).toBe('true')
    expect(
      wrapper.get('[data-testid="system-custom-advanced-settings"]').attributes('style') ?? ''
    ).not.toContain('display: none')
  })

  it('warns when selected sources use different protocols', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await sourceCheckbox(wrapper, 11).setValue(true)
    expect(wrapper.find('[data-testid="system-custom-cross-protocol-warning"]').exists()).toBe(
      false
    )

    await sourceCheckbox(wrapper, 22).setValue(true)
    expect(wrapper.get('[data-testid="system-custom-cross-protocol-warning"]').text()).toContain(
      'admin.groups.systemCustom.crossProtocolHint'
    )

    await sourceCheckbox(wrapper, 22).setValue(false)
    expect(wrapper.find('[data-testid="system-custom-cross-protocol-warning"]').exists()).toBe(
      false
    )
  })

  it('navigates inside the model column without changing the selected sources', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await sourceCheckbox(wrapper, 11).setValue(true)
    await sourceCheckbox(wrapper, 22).setValue(true)

    const sourceScroll = wrapper.get('[data-testid="system-custom-source-scroll"]')
      .element as HTMLElement
    const modelScroll = wrapper.get('[data-testid="system-custom-model-scroll"]')
      .element as HTMLElement
    const targetSection = wrapper.get(
      '[data-testid="system-custom-source-section"][data-source-id="22"]'
    ).element as HTMLElement
    const sourceScrollTo = vi.fn()
    const modelScrollTo = vi.fn()
    sourceScroll.scrollTo = sourceScrollTo
    modelScroll.scrollTo = modelScrollTo
    Object.defineProperty(modelScroll, 'scrollTop', { configurable: true, value: 40 })
    vi.spyOn(modelScroll, 'getBoundingClientRect').mockReturnValue({
      top: 100,
      bottom: 500,
      left: 0,
      right: 800,
      width: 800,
      height: 400,
      x: 0,
      y: 100,
      toJSON: () => ({})
    })
    vi.spyOn(targetSection, 'getBoundingClientRect').mockReturnValue({
      top: 320,
      bottom: 520,
      left: 0,
      right: 800,
      width: 800,
      height: 200,
      x: 0,
      y: 320,
      toJSON: () => ({})
    })

    await wrapper
      .get('[data-testid="system-custom-source-nav"][data-source-id="22"]')
      .trigger('click')

    expect(modelScrollTo).toHaveBeenCalledWith({ top: 260, behavior: 'auto' })
    expect(sourceScrollTo).not.toHaveBeenCalled()
    expect(sourceCheckbox(wrapper, 11).element).toHaveProperty('checked', true)
    expect(sourceCheckbox(wrapper, 22).element).toHaveProperty('checked', true)
    expect(wrapper.findAll('[data-testid="system-custom-source-section"]')).toHaveLength(2)
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

  it('ignores stale out-of-order load responses after switching dialog sessions', async () => {
    const candidatesA = deferred<typeof candidates>()
    const detailA = deferred<ReturnType<typeof groupDetail>>()
    const candidatesB = deferred<typeof candidates>()
    const detailB = deferred<ReturnType<typeof groupDetail>>()
    getSystemCustomGroupCandidates
      .mockReturnValueOnce(candidatesA.promise)
      .mockReturnValueOnce(candidatesB.promise)
    getSystemCustomGroup
      .mockReturnValueOnce(detailA.promise)
      .mockReturnValueOnce(detailB.promise)

    const wrapper = mountDialog({ groupId: 90 })
    await wrapper.setProps({ groupId: 91 })
    candidatesB.resolve(candidates)
    detailB.resolve(groupDetail(91, 'session-B'))
    await flushPromises()
    expect(wrapper.get('[data-testid="system-custom-name"]').element).toHaveProperty(
      'value',
      'session-B'
    )

    candidatesA.resolve(candidates)
    detailA.resolve(groupDetail(90, 'stale-session-A'))
    await flushPromises()
    expect(wrapper.get('[data-testid="system-custom-name"]').element).toHaveProperty(
      'value',
      'session-B'
    )
  })

  it('ignores a stale sync preview after the dialog moves to another group', async () => {
    const syncCandidates = deferred<typeof candidates>()
    const syncPreview = deferred<{
      added: Array<{
        public_model: string
        source_group_id: number
        source_model: string
        selected: boolean
      }>
      missing: never[]
      conflicting: never[]
    }>()
    getSystemCustomGroupCandidates
      .mockResolvedValueOnce(candidates)
      .mockReturnValueOnce(syncCandidates.promise)
    getSystemCustomGroupSyncPreview.mockReturnValueOnce(syncPreview.promise)
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await wrapper.setProps({ groupId: 91 })
    await flushPromises()
    syncCandidates.resolve(candidates)
    syncPreview.resolve({
      added: [
        {
          public_model: 'stale-sync-model',
          source_group_id: 11,
          source_model: 'stale-sync-model',
          selected: false
        }
      ],
      missing: [],
      conflicting: []
    })
    await flushPromises()
    expect(wrapper.text()).not.toContain('stale-sync-model')
  })

  it('does not let stale save or delete completions emit into a reopened session', async () => {
    const saveResult = deferred<typeof existingGroup>()
    updateSystemCustomGroup.mockReturnValueOnce(saveResult.promise)
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, groupId: 91 })
    await flushPromises()
    saveResult.resolve(existingGroup)
    await flushPromises()
    expect(wrapper.emitted('saved')).toBeUndefined()
    expect(wrapper.get('[data-testid="system-custom-name"]').element).toHaveProperty(
      'value',
      '酒馆综合月卡'
    )

    const deleteResult = deferred<{ id: number; deleted: boolean }>()
    deleteSystemCustomGroup.mockReturnValueOnce(deleteResult.promise)
    await wrapper.get('[data-testid="system-custom-delete"]').trigger('click')
    await wrapper.get('[data-testid="system-custom-delete-confirm"]').trigger('click')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, groupId: 92 })
    await flushPromises()
    deleteResult.resolve({ id: 91, deleted: true })
    await flushPromises()
    expect(wrapper.emitted('deleted')).toBeUndefined()
  })

  it('refreshes candidates with sync so newly discovered models can enter the snapshot', async () => {
    const initialCandidates = [
      { ...candidates[0], models: ['claude-sonnet-4'] },
      candidates[1]
    ]
    const freshCandidates = [
      { ...candidates[0], models: ['claude-sonnet-4', 'brand-new-model'] },
      candidates[1]
    ]
    getSystemCustomGroupCandidates
      .mockResolvedValueOnce(initialCandidates)
      .mockResolvedValueOnce(freshCandidates)
    getSystemCustomGroupSyncPreview.mockResolvedValueOnce({
      added: [
        {
          public_model: 'brand-new-model',
          source_group_id: 11,
          source_model: 'brand-new-model',
          selected: false
        }
      ],
      missing: [],
      conflicting: []
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    expect(wrapper.text()).not.toContain('brand-new-model')
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()
    expect(getSystemCustomGroupCandidates).toHaveBeenCalledTimes(2)
    const added = wrapper.get('[data-testid="system-custom-sync-added"]')
    await added.get('input[type="checkbox"]').setValue(true)
    expect(modelRow(wrapper, 11, 'brand-new-model').text()).toContain('brand-new-model')
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(updateSystemCustomGroup.mock.calls.at(-1)?.[1].models).toEqual(
      expect.arrayContaining([expect.objectContaining({ source_model: 'brand-new-model' })])
    )
  })

  it('uses route drafts as the single truth across sync and main model controls', async () => {
    getSystemCustomGroupSyncPreview.mockResolvedValueOnce({
      added: [
        {
          public_model: 'claude-haiku-4',
          source_group_id: 11,
          source_model: 'claude-haiku-4',
          selected: false
        }
      ],
      missing: [existingGroup.models[1]],
      conflicting: []
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()
    const added = wrapper.get('[data-testid="system-custom-sync-added"]')
    await added.get('input[type="checkbox"]').setValue(true)
    const addedMain = modelRow(wrapper, 11, 'claude-haiku-4')
    await addedMain.get('[data-testid="system-custom-public-model"]').setValue('custom-haiku')
    await addedMain.get('input[type="checkbox"]').setValue(false)
    expect(added.get('input[type="checkbox"]').element).toHaveProperty('checked', false)
    await addedMain.get('input[type="checkbox"]').setValue(true)
    expect(added.get('input[type="checkbox"]').element).toHaveProperty('checked', true)
    expect(addedMain.get('[data-testid="system-custom-public-model"]').element).toHaveProperty(
      'value',
      'custom-haiku'
    )

    const missing = wrapper.get('[data-testid="system-custom-sync-missing"]')
    const missingMain = modelRow(wrapper, 11, 'legacy-haiku')
    await missingMain.get('[data-testid="system-custom-public-model"]').setValue('legacy-alias')
    await missing.get('input[type="checkbox"]').setValue(true)
    expect(missingMain.get('[data-testid="system-custom-public-model"]').element).toHaveProperty(
      'value',
      'legacy-alias'
    )
    expect(missingMain.findAll('input[type="checkbox"]')[1].element).toHaveProperty(
      'checked',
      false
    )
  })

  it('preserves an existing sync-added route draft when it is unchecked and selected again', async () => {
    getSystemCustomGroupSyncPreview.mockResolvedValueOnce({
      added: [
        {
          public_model: 'preview-haiku',
          source_group_id: 11,
          source_model: 'claude-haiku-4',
          selected: false
        }
      ],
      missing: [],
      conflicting: []
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()

    const added = wrapper.get('[data-testid="system-custom-sync-added"]')
    await added.get('input[type="checkbox"]').setValue(true)
    const main = modelRow(wrapper, 11, 'claude-haiku-4')
    await main.get('[data-testid="system-custom-public-model"]').setValue('custom-haiku')
    await main.findAll('input[type="checkbox"]')[1].setValue(false)
    await added.get('input[type="checkbox"]').setValue(false)
    await added.get('input[type="checkbox"]').setValue(true)

    expect(main.get('[data-testid="system-custom-public-model"]').element).toHaveProperty(
      'value',
      'custom-haiku'
    )
    expect(main.findAll('input[type="checkbox"]')[1].element).toHaveProperty('checked', false)
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(updateSystemCustomGroup.mock.calls.at(-1)?.[1].models).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          public_model: 'custom-haiku',
          source_group_id: 11,
          source_model: 'claude-haiku-4',
          enabled: false
        })
      ])
    )
  })

  it('renders already-disabled missing routes as an applied read-only suggestion', async () => {
    getSystemCustomGroupSyncPreview.mockResolvedValueOnce({
      added: [],
      missing: [existingGroup.models[2]],
      conflicting: []
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()

    const missing = wrapper.get('[data-testid="system-custom-sync-missing"]')
    expect(missing.get('input[type="checkbox"]').element).toHaveProperty('checked', true)
    expect(missing.get('input[type="checkbox"]').attributes('disabled')).toBeDefined()
    expect(missing.text()).toContain('admin.groups.systemCustom.alreadyDisabled')
  })

  it('lets an active missing suggestion be applied and undone without losing its route state', async () => {
    getSystemCustomGroupSyncPreview.mockResolvedValueOnce({
      added: [],
      missing: [existingGroup.models[1]],
      conflicting: []
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-sync"]').trigger('click')
    await flushPromises()

    const missing = wrapper.get('[data-testid="system-custom-sync-missing"]')
    const input = missing.get('input[type="checkbox"]')
    expect(input.element).toHaveProperty('checked', false)
    expect(input.attributes('disabled')).toBeUndefined()
    await input.setValue(true)
    expect(input.element).toHaveProperty('checked', true)
    expect(input.attributes('disabled')).toBeUndefined()
    await input.setValue(false)
    expect(input.element).toHaveProperty('checked', false)
    expect(modelRow(wrapper, 11, 'legacy-haiku').findAll('input[type="checkbox"]')[1].element).toHaveProperty(
      'checked',
      true
    )

    await modelRow(wrapper, 11, 'legacy-haiku').findAll('input[type="checkbox"]')[1].setValue(false)
    expect(input.element).toHaveProperty('checked', true)
    expect(input.attributes('disabled')).toBeDefined()
    expect(missing.text()).toContain('admin.groups.systemCustom.alreadyDisabled')
  })

  it.each([
    ['daily', 'system-custom-daily-limit', '-1'],
    ['validity fractional', 'system-custom-validity-days', '1.5'],
    ['validity zero', 'system-custom-validity-days', '0'],
    ['validity too high', 'system-custom-validity-days', '3651']
  ])('blocks invalid numeric input: %s', async (_label, testID, value) => {
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-name"]').setValue('numeric-test')
    await sourceCheckbox(wrapper, 11).setValue(true)
    await modelRow(wrapper, 11, 'claude-sonnet-4')
      .get('input[type="checkbox"]')
      .setValue(true)
    await wrapper.get(`[data-testid="${testID}"]`).setValue(value)
    expect(wrapper.get('[data-testid="system-custom-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    expect(createSystemCustomGroup).not.toHaveBeenCalled()
  })

  it.each([Infinity, Number.NaN, 'not-a-number', ' '])(
    'blocks non-finite or non-numeric limit state: %j',
    async (invalid) => {
      const wrapper = mountDialog()
      await flushPromises()
      await wrapper.get('[data-testid="system-custom-name"]').setValue('numeric-state-test')
      await sourceCheckbox(wrapper, 11).setValue(true)
      await modelRow(wrapper, 11, 'claude-sonnet-4')
        .get('input[type="checkbox"]')
        .setValue(true)
      ;(wrapper.vm as any).$.setupState.form.monthly_limit_usd = invalid
      await nextTick()
      expect(wrapper.get('[data-testid="system-custom-save"]').attributes('disabled')).toBeDefined()
      await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
      expect(createSystemCustomGroup).not.toHaveBeenCalled()
    }
  )

  it('makes mutations and close mutually exclusive for the active session', async () => {
    const deleteResult = deferred<{ id: number; deleted: boolean }>()
    deleteSystemCustomGroup.mockReturnValueOnce(deleteResult.promise)
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-delete"]').trigger('click')
    expect(wrapper.get('[data-testid="system-custom-save"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="system-custom-delete-confirm"]').trigger('click')
    expect(wrapper.get('[data-testid="system-custom-sync"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-testid="base-dialog-close"]').trigger('click')
    expect(wrapper.emitted('close')).toBeUndefined()
  })

  it('renders orphaned existing sources so an admin can remove them from the snapshot', async () => {
    const orphan = {
      ...existingGroup.models[1],
      id: 99,
      source_group_id: 99,
      source_model: 'orphan-model',
      public_model: 'orphan-alias',
      source_group: { id: 99, name: 'retired-source', status: 'inactive' as const }
    }
    getSystemCustomGroup.mockResolvedValueOnce({
      ...existingGroup,
      models: [existingGroup.models[0], orphan]
    })
    const wrapper = mountDialog({ groupId: 90 })
    await flushPromises()
    expect(sourceCheckbox(wrapper, 99).element.parentElement?.textContent).toContain(
      'retired-source'
    )
    expect(wrapper.find('[data-testid="system-custom-source-unavailable"]').exists()).toBe(true)
    expect(modelRow(wrapper, 99, 'orphan-model').text()).toContain('orphan-model')
    await sourceCheckbox(wrapper, 99).setValue(false)
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(updateSystemCustomGroup.mock.calls.at(-1)?.[1].models).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ source_group_id: 99 })])
    )
  })

  it.each([
    ['', 'admin.groups.systemCustom.saveFailed'],
    ['   ', 'admin.groups.systemCustom.saveFailed'],
    ['internal error', 'admin.groups.systemCustom.saveFailed'],
    ['502 Bad Gateway', 'admin.groups.systemCustom.saveFailed'],
    ['Bad Gateway', 'admin.groups.systemCustom.saveFailed'],
    ['Internal Server Error', 'admin.groups.systemCustom.saveFailed'],
    ['<html><body>proxy error</body></html>', 'admin.groups.systemCustom.saveFailed'],
    ['safe upstream validation', 'safe upstream validation']
  ])('sanitizes raw API string errors: %j', async (raw, expected) => {
    createSystemCustomGroup.mockRejectedValueOnce({ response: { data: raw } })
    const wrapper = mountDialog()
    await flushPromises()
    await wrapper.get('[data-testid="system-custom-name"]').setValue('error-test')
    await sourceCheckbox(wrapper, 11).setValue(true)
    await modelRow(wrapper, 11, 'claude-sonnet-4')
      .get('input[type="checkbox"]')
      .setValue(true)
    await wrapper.get('[data-testid="system-custom-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="system-custom-error"]').text()).toBe(expected)
  })
})
