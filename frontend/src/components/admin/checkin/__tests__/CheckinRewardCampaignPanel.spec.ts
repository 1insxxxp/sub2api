import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CheckinRewardCampaignPanel from '../CheckinRewardCampaignPanel.vue'
import type { AdminCheckinRewardCampaign, CheckinRewardTier } from '@/api/admin'

const {
  listCampaigns,
  enableCampaign,
  disableCampaign,
  deleteCampaign,
  showSuccess,
  showError,
} = vi.hoisted(() => ({
  listCampaigns: vi.fn(),
  enableCampaign: vi.fn(),
  disableCampaign: vi.fn(),
  deleteCampaign: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    checkins: {
      listCampaigns,
      enableCampaign,
      disableCampaign,
      deleteCampaign,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess, showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (!params) return key
        return `${key}:${Object.values(params).join('|')}`
      },
    }),
  }
})

const DialogStub = {
  props: ['show', 'mode', 'campaignId', 'defaultTiers'],
  emits: ['close', 'saved'],
  template: `
    <div v-if="show" data-test="campaign-dialog-stub" :data-mode="mode" :data-campaign-id="campaignId">
      <button data-test="dialog-stub-close" @click="$emit('close')">close</button>
    </div>
  `,
}

const ConfirmDialogStub = {
  props: ['show', 'title', 'message', 'danger'],
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="show" data-test="campaign-confirm-dialog">
      <span>{{ title }}</span><span>{{ message }}</span>
      <button data-test="campaign-confirm-action" @click="$emit('confirm')">confirm</button>
      <button data-test="campaign-cancel-confirm" @click="$emit('cancel')">cancel</button>
    </div>
  `,
}

function campaign(
  id: number,
  lifecycle: AdminCheckinRewardCampaign['lifecycle_status'],
  overrides: Partial<AdminCheckinRewardCampaign> = {}
): AdminCheckinRewardCampaign {
  return {
    id,
    name: `Campaign ${id}`,
    status: lifecycle === 'draft' ? 'draft' : lifecycle === 'disabled' ? 'disabled' : 'enabled',
    lifecycle_status: lifecycle,
    start_date: '2026-08-16',
    end_date: '2026-08-18',
    reward_tiers: [{ amount: 1, probability: 100, sort_order: 1 }],
    probability_total: 100,
    preview: { min_reward: 1, max_reward: 1, average_reward: 1 },
    created_at: '2026-08-15T01:02:03Z',
    updated_at: '2026-08-15T01:02:03Z',
    ...overrides,
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function addCalendarDays(date: string, days: number): string {
  const [year, month, day] = date.split('-').map(Number)
  return new Date(Date.UTC(year, month - 1, day + days)).toISOString().slice(0, 10)
}

function mountPanel(overrides: {
  defaultTiers?: CheckinRewardTier[]
  defaultTiersReady?: boolean
  createDisabled?: boolean
  defaultTiersLoading?: boolean
} = {}) {
  return mount(CheckinRewardCampaignPanel, {
    props: {
      defaultTiers: [{ amount: 1, probability: 100, sort_order: 1 }],
      ...overrides,
    },
    global: {
      stubs: {
        CheckinRewardCampaignDialog: DialogStub,
        ConfirmDialog: ConfirmDialogStub,
        Icon: true,
      },
    },
  })
}

describe('CheckinRewardCampaignPanel', () => {
  beforeEach(() => {
    const today = new Intl.DateTimeFormat('en', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    })
      .formatToParts(new Date())
      .reduce<Record<string, string>>((parts, part) => {
        parts[part.type] = part.value
        return parts
      }, {})
    const beijingToday = `${today.year}-${today.month}-${today.day}`

    listCampaigns.mockReset()
    enableCampaign.mockReset()
    disableCampaign.mockReset()
    deleteCampaign.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    listCampaigns.mockResolvedValue([
      campaign(1, 'active'),
      campaign(2, 'upcoming'),
      campaign(3, 'ended'),
      campaign(4, 'draft', { start_date: beijingToday, end_date: beijingToday }),
      campaign(5, 'disabled'),
    ])
  })

  it('lists every lifecycle with status chips and raw Beijing calendar dates', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(listCampaigns).toHaveBeenCalledWith('all', expect.objectContaining({ signal: expect.any(AbortSignal) }))
    for (const lifecycle of ['active', 'upcoming', 'ended', 'draft', 'disabled']) {
      expect(wrapper.get(`[data-test="campaign-status-${lifecycle}"]`).text()).toContain(
        `admin.checkins.campaigns.lifecycle.${lifecycle}`
      )
    }
    expect(wrapper.get('[data-test="campaign-date-range-1"]').text()).toContain('2026-08-16')
    expect(wrapper.get('[data-test="campaign-date-range-1"]').text()).toContain('2026-08-18')
    expect(wrapper.get('[data-test="campaign-beijing-calendar"]').text()).toContain(
      'admin.checkins.campaigns.beijingCalendar'
    )
  })

  it('filters by lifecycle through the backend list operation', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    listCampaigns.mockClear()
    listCampaigns.mockResolvedValue([campaign(4, 'draft')])

    await wrapper.get('[data-test="campaign-filter-draft"]').trigger('click')
    await flushPromises()

    expect(listCampaigns).toHaveBeenCalledWith('draft', expect.objectContaining({ signal: expect.any(AbortSignal) }))
    expect(wrapper.find('[data-test="campaign-card-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="campaign-card-4"]').exists()).toBe(true)
  })

  it('opens create, edit, view, and copy dialogs with the expected mode', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="campaign-create"]').trigger('click')
    expect(wrapper.get('[data-test="campaign-dialog-stub"]').attributes('data-mode')).toBe('create')
    await wrapper.get('[data-test="dialog-stub-close"]').trigger('click')

    await wrapper.get('[data-test="campaign-edit-4"]').trigger('click')
    expect(wrapper.get('[data-test="campaign-dialog-stub"]').attributes('data-mode')).toBe('edit')
    await wrapper.get('[data-test="dialog-stub-close"]').trigger('click')

    await wrapper.get('[data-test="campaign-view-1"]').trigger('click')
    expect(wrapper.get('[data-test="campaign-dialog-stub"]').attributes('data-mode')).toBe('view')
    await wrapper.get('[data-test="dialog-stub-close"]').trigger('click')

    await wrapper.get('[data-test="campaign-copy-1"]').trigger('click')
    expect(wrapper.get('[data-test="campaign-dialog-stub"]').attributes('data-mode')).toBe('copy')
  })

  it('does not offer enable for an expired draft, while a draft ending today remains enableable', async () => {
    const today = new Intl.DateTimeFormat('en', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(new Date()).reduce<Record<string, string>>((parts, part) => {
      if (part.type !== 'literal') parts[part.type] = part.value
      return parts
    }, {})
    const beijingToday = `${today.year}-${today.month}-${today.day}`
    listCampaigns.mockResolvedValue([
      campaign(6, 'draft', { start_date: '2000-01-01', end_date: '2000-01-02' }),
      campaign(7, 'draft', { start_date: beijingToday, end_date: beijingToday }),
    ])

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('[data-test="campaign-enable-6"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="campaign-enable-7"]').exists()).toBe(true)
  })

  it('only offers re-enable for disabled campaigns that start after today', async () => {
    const today = new Intl.DateTimeFormat('en', {
      timeZone: 'Asia/Shanghai',
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
    }).formatToParts(new Date()).reduce<Record<string, string>>((parts, part) => {
      if (part.type !== 'literal') parts[part.type] = part.value
      return parts
    }, {})
    const beijingToday = `${today.year}-${today.month}-${today.day}`
    const tomorrow = addCalendarDays(beijingToday, 1)
    listCampaigns.mockResolvedValue([
      campaign(8, 'disabled', { start_date: beijingToday, end_date: tomorrow }),
      campaign(9, 'disabled', { start_date: tomorrow, end_date: tomorrow }),
    ])

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.find('[data-test="campaign-enable-8"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="campaign-enable-9"]').exists()).toBe(true)
  })

  it('communicates baseline reward loading and blocks empty-default campaign drafts', async () => {
    const wrapper = mountPanel({
      defaultTiers: [],
      defaultTiersReady: false,
      createDisabled: true,
      defaultTiersLoading: true,
    })
    await flushPromises()

    const createButton = wrapper.get('[data-test="campaign-create"]')
    expect(createButton.attributes('disabled')).toBeDefined()
    expect(createButton.attributes('aria-disabled')).toBe('true')
    expect(createButton.attributes('aria-busy')).toBe('true')
    expect(wrapper.get('[data-test="campaign-create-status"]').attributes('role')).toBe('status')
  })

  it('requires confirmation before enabling and disabling', async () => {
    enableCampaign.mockResolvedValue(campaign(4, 'upcoming'))
    disableCampaign.mockResolvedValue(campaign(1, 'disabled'))
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="campaign-enable-4"]').trigger('click')
    expect(enableCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="campaign-confirm-dialog"]').text()).toContain(
      'admin.checkins.campaigns.confirm.enableTitle'
    )
    await wrapper.get('[data-test="campaign-confirm-action"]').trigger('click')
    await flushPromises()
    expect(enableCampaign).toHaveBeenCalledWith(4)

    await wrapper.get('[data-test="campaign-disable-1"]').trigger('click')
    expect(disableCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="campaign-confirm-dialog"]').text()).toContain(
      'admin.checkins.campaigns.confirm.disableTitle'
    )
    await wrapper.get('[data-test="campaign-confirm-action"]').trigger('click')
    await flushPromises()
    expect(disableCampaign).toHaveBeenCalledWith(1)
  })

  it('ignores stale list responses when filters change quickly', async () => {
    const allRequest = deferred<AdminCheckinRewardCampaign[]>()
    const draftRequest = deferred<AdminCheckinRewardCampaign[]>()
    listCampaigns.mockReset()
    listCampaigns.mockReturnValueOnce(allRequest.promise).mockReturnValueOnce(draftRequest.promise)
    const wrapper = mountPanel()
    await wrapper.get('[data-test="campaign-filter-draft"]').trigger('click')

    draftRequest.resolve([campaign(4, 'draft', { name: 'Current result' })])
    await flushPromises()
    allRequest.resolve([campaign(1, 'active', { name: 'Stale result' })])
    await flushPromises()

    expect(wrapper.text()).toContain('Current result')
    expect(wrapper.text()).not.toContain('Stale result')
  })

  it('disables all mutation actions while enable is pending', async () => {
    const pending = deferred<AdminCheckinRewardCampaign>()
    enableCampaign.mockReturnValue(pending.promise)
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="campaign-enable-4"]').trigger('click')
    await wrapper.get('[data-test="campaign-confirm-action"]').trigger('click')

    expect(wrapper.get('[data-test="campaign-create"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="campaign-edit-4"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="campaign-copy-1"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="campaign-disable-1"]').attributes('disabled')).toBeDefined()

    pending.resolve(campaign(4, 'upcoming'))
    await flushPromises()
  })

  it('surfaces sanitized enable overlap errors with conflict metadata', async () => {
    enableCampaign.mockRejectedValue({
      reason: 'CHECKIN_REWARD_CAMPAIGN_OVERLAP',
      message: 'Campaign dates overlap',
      metadata: {
        conflict_campaign_name: 'Existing campaign',
        conflict_start_date: '2026-08-20',
        conflict_end_date: '2026-08-25',
      },
    })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.get('[data-test="campaign-enable-4"]').trigger('click')
    await wrapper.get('[data-test="campaign-confirm-action"]').trigger('click')
    await flushPromises()

    expect(showError).toHaveBeenCalledWith('Campaign dates overlap')
    expect(wrapper.get('[data-test="campaign-panel-overlap-conflict"]').text()).toContain('Existing campaign')
    expect(wrapper.get('[data-test="campaign-panel-overlap-conflict"]').text()).toContain('2026-08-20')
    expect(wrapper.get('[data-test="campaign-panel-overlap-conflict"]').text()).toContain('2026-08-25')
  })
})
