import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import CheckinRewardCampaignDialog from '../CheckinRewardCampaignDialog.vue'
import type {
  AdminCheckinRewardCampaign,
  CheckinRewardCampaignLifecycle,
  CheckinRewardCampaignStatus,
  CheckinRewardTier,
} from '@/api/admin'

const {
  getCampaign,
  createCampaign,
  updateCampaign,
  copyCampaign,
} = vi.hoisted(() => ({
  getCampaign: vi.fn(),
  createCampaign: vi.fn(),
  updateCampaign: vi.fn(),
  copyCampaign: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    checkins: {
      getCampaign,
      createCampaign,
      updateCampaign,
      copyCampaign,
    },
  },
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

const BaseDialogStub = {
  props: ['show', 'title', 'closeOnEscape', 'showCloseButton'],
  emits: ['close'],
  template: `
    <section v-if="show" data-test="base-dialog" :data-close-enabled="showCloseButton">
      <h2>{{ title }}</h2>
      <button data-test="base-dialog-close" type="button" @click="$emit('close')">close</button>
      <slot />
      <slot name="footer" />
    </section>
  `,
}

const defaultTiers: CheckinRewardTier[] = [
  { amount: 1, probability: 25, sort_order: 1 },
  { amount: 3, probability: 75, sort_order: 2 },
]

const readOnlyCampaigns: [CheckinRewardCampaignLifecycle, CheckinRewardCampaignStatus][] = [
  ['active', 'enabled'],
  ['upcoming', 'enabled'],
  ['ended', 'enabled'],
]

function campaign(
  overrides: Partial<AdminCheckinRewardCampaign> = {}
): AdminCheckinRewardCampaign {
  return {
    id: 42,
    name: 'Summer bonus',
    status: 'draft',
    lifecycle_status: 'draft',
    start_date: '2026-08-16',
    end_date: '2026-08-18',
    reward_tiers: defaultTiers.map((tier) => ({ ...tier })),
    probability_total: 100,
    preview: { min_reward: 1, max_reward: 3, average_reward: 2.5 },
    created_by: 7,
    updated_by: null,
    created_at: '2026-08-15T01:02:03Z',
    updated_at: '2026-08-15T01:02:03Z',
    ...overrides,
  }
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

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(CheckinRewardCampaignDialog, {
    props: {
      show: true,
      mode: 'create',
      defaultTiers,
      ...props,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Icon: true,
      },
    },
  })
}

describe('CheckinRewardCampaignDialog', () => {
  beforeEach(() => {
    getCampaign.mockReset()
    createCampaign.mockReset()
    updateCampaign.mockReset()
    copyCampaign.mockReset()
  })

  it('deep copies current daily tiers for a new draft and previews min, max, and weighted average', async () => {
    const wrapper = mountDialog()

    expect(wrapper.get<HTMLInputElement>('[data-test="campaign-tier-amount-0"]').element.value).toBe('1')
    expect(wrapper.get<HTMLInputElement>('[data-test="campaign-tier-probability-1"]').element.value).toBe('75')
    expect(wrapper.get('[data-test="campaign-preview-min"]').text()).toContain('$1.00')
    expect(wrapper.get('[data-test="campaign-preview-max"]').text()).toContain('$3.00')
    expect(wrapper.get('[data-test="campaign-preview-average"]').text()).toContain('$2.50')

    await wrapper.get('[data-test="campaign-tier-amount-0"]').setValue('9')
    expect(defaultTiers[0]?.amount).toBe(1)
  })

  it('adds, edits, and removes reward tiers with stable controls', async () => {
    const wrapper = mountDialog()

    await wrapper.get('[data-test="campaign-add-tier"]').trigger('click')
    expect(wrapper.findAll('[data-test^="campaign-tier-row-"]')).toHaveLength(3)
    await wrapper.get('[data-test="campaign-tier-amount-2"]').setValue('5')
    await wrapper.get('[data-test="campaign-remove-tier-1"]').trigger('click')

    expect(wrapper.findAll('[data-test^="campaign-tier-row-"]')).toHaveLength(2)
    expect(wrapper.get<HTMLInputElement>('[data-test="campaign-tier-amount-1"]').element.value).toBe('5')
  })

  it.each([
    ['non-positive amount', '0', '25', 'admin.checkins.campaigns.validation.positiveValues'],
    ['amount precision', '1.001', '25', 'admin.checkins.campaigns.validation.twoDecimals'],
    ['probability precision', '1', '25.001', 'admin.checkins.campaigns.validation.twoDecimals'],
    ['duplicate amount', '3', '25', 'admin.checkins.campaigns.validation.uniqueAmounts'],
  ])('rejects %s', async (_label, amount, probability, expectedKey) => {
    const wrapper = mountDialog()
    await wrapper.get('[data-test="campaign-name"]').setValue('Campaign')
    await wrapper.get('[data-test="campaign-start-date"]').setValue('2026-08-16')
    await wrapper.get('[data-test="campaign-end-date"]').setValue('2026-08-18')
    await wrapper.get('[data-test="campaign-tier-amount-0"]').setValue(amount)
    await wrapper.get('[data-test="campaign-tier-probability-0"]').setValue(probability)
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')

    expect(createCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="campaign-validation-error"]').text()).toContain(expectedKey)
  })

  it('requires probabilities to total exactly 100 percent', async () => {
    const wrapper = mountDialog()
    await wrapper.get('[data-test="campaign-name"]').setValue('Campaign')
    await wrapper.get('[data-test="campaign-start-date"]').setValue('2026-08-16')
    await wrapper.get('[data-test="campaign-end-date"]').setValue('2026-08-18')
    await wrapper.get('[data-test="campaign-tier-probability-1"]').setValue('74.99')
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')

    expect(createCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="campaign-validation-error"]').text()).toContain(
      'admin.checkins.campaigns.validation.probabilityTotal'
    )
  })

  it('allows no more than 20 tiers', async () => {
    const twentyTiers = Array.from({ length: 20 }, (_, index) => ({
      amount: index + 1,
      probability: 5,
      sort_order: index + 1,
    }))
    const wrapper = mountDialog({ defaultTiers: twentyTiers })

    expect(wrapper.findAll('[data-test^="campaign-tier-row-"]')).toHaveLength(20)
    expect(wrapper.get('[data-test="campaign-add-tier"]').attributes('disabled')).toBeDefined()
  })

  it('blocks an inverted date range and submits original YYYY-MM-DD strings unchanged', async () => {
    const wrapper = mountDialog()
    await wrapper.get('[data-test="campaign-name"]').setValue('Campaign')
    await wrapper.get('[data-test="campaign-start-date"]').setValue('2026-08-18')
    await wrapper.get('[data-test="campaign-end-date"]').setValue('2026-08-16')
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')

    expect(createCampaign).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="campaign-validation-error"]').text()).toContain(
      'admin.checkins.campaigns.validation.dateRange'
    )

    createCampaign.mockResolvedValue(campaign())
    await wrapper.get('[data-test="campaign-end-date"]').setValue('2026-08-20')
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')
    await flushPromises()

    expect(createCampaign).toHaveBeenCalledWith({
      name: 'Campaign',
      start_date: '2026-08-18',
      end_date: '2026-08-20',
      reward_tiers: defaultTiers,
    })
  })

  it.each(readOnlyCampaigns)('renders %s campaigns read-only', async (lifecycle, status) => {
    getCampaign.mockResolvedValue(campaign({ lifecycle_status: lifecycle, status }))
    const wrapper = mountDialog({ mode: 'view', campaignId: 42 })
    await flushPromises()

    expect(wrapper.get('[data-test="campaign-name"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="campaign-start-date"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="campaign-submit"]').exists()).toBe(false)
    expect(wrapper.get('[data-test="campaign-read-only-note"]').text()).toContain(
      'admin.checkins.campaigns.readOnly'
    )
  })

  it('falls back to view-only when a draft becomes active before edit detail loads', async () => {
    getCampaign.mockResolvedValue(campaign({ lifecycle_status: 'active', status: 'enabled' }))
    const wrapper = mountDialog({ mode: 'edit', campaignId: 42 })
    await flushPromises()

    expect(wrapper.get('[data-test="campaign-name"]').attributes('disabled')).toBeDefined()
    expect(wrapper.find('[data-test="campaign-submit"]').exists()).toBe(false)
  })

  it('opens a copy as a new draft with only the generated name editable', async () => {
    getCampaign.mockResolvedValue(campaign())
    copyCampaign.mockResolvedValue(campaign({ id: 43, name: 'Summer bonus copy' }))
    const wrapper = mountDialog({ mode: 'copy', campaignId: 42 })
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('[data-test="campaign-name"]').element.value).toContain('Summer bonus')
    expect(wrapper.get('[data-test="campaign-name"]').attributes('disabled')).toBeUndefined()
    expect(wrapper.get('[data-test="campaign-start-date"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="campaign-name"]').setValue('Fresh draft')
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')
    await flushPromises()

    expect(copyCampaign).toHaveBeenCalledWith(42, { name: 'Fresh draft' })
  })

  it('shows sanitized API details and structured overlap metadata', async () => {
    createCampaign.mockRejectedValue({
      reason: 'CHECKIN_REWARD_CAMPAIGN_OVERLAP',
      message: '<img src=x onerror=alert(1)>',
      metadata: {
        conflict_campaign_id: '9',
        conflict_campaign_name: 'Existing campaign',
        conflict_start_date: '2026-08-20',
        conflict_end_date: '2026-08-25',
      },
    })
    const wrapper = mountDialog()
    await wrapper.get('[data-test="campaign-name"]').setValue('Campaign')
    await wrapper.get('[data-test="campaign-start-date"]').setValue('2026-08-20')
    await wrapper.get('[data-test="campaign-end-date"]').setValue('2026-08-21')
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')
    await flushPromises()

    const error = wrapper.get('[data-test="campaign-api-error"]')
    expect(error.text()).toContain('<img src=x onerror=alert(1)>')
    expect(error.find('img').exists()).toBe(false)
    expect(wrapper.get('[data-test="campaign-overlap-conflict"]').text()).toContain('Existing campaign')
    expect(wrapper.get('[data-test="campaign-overlap-conflict"]').text()).toContain('2026-08-20')
    expect(wrapper.get('[data-test="campaign-overlap-conflict"]').text()).toContain('2026-08-25')
  })

  it('ignores stale detail results after the dialog is reopened for another campaign', async () => {
    const first = deferred<AdminCheckinRewardCampaign>()
    const second = deferred<AdminCheckinRewardCampaign>()
    getCampaign.mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const wrapper = mountDialog({ mode: 'view', campaignId: 41 })
    await wrapper.setProps({ campaignId: 42 })

    second.resolve(campaign({ id: 42, name: 'Current campaign' }))
    await flushPromises()
    first.resolve(campaign({ id: 41, name: 'Stale campaign' }))
    await flushPromises()

    expect(wrapper.get<HTMLInputElement>('[data-test="campaign-name"]').element.value).toBe('Current campaign')
    expect(getCampaign.mock.calls[0]?.[1]?.signal).toBeInstanceOf(AbortSignal)
  })

  it('uses one saving state to disable mutation controls and prevent closing', async () => {
    const pending = deferred<AdminCheckinRewardCampaign>()
    createCampaign.mockReturnValue(pending.promise)
    const wrapper = mountDialog()
    await wrapper.get('[data-test="campaign-name"]').setValue('Campaign')
    await wrapper.get('[data-test="campaign-start-date"]').setValue('2026-08-16')
    await wrapper.get('[data-test="campaign-end-date"]').setValue('2026-08-18')
    await wrapper.get('[data-test="campaign-submit"]').trigger('click')

    expect(wrapper.get('[data-test="campaign-submit"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="campaign-cancel"]').attributes('disabled')).toBeDefined()
    expect(wrapper.get('[data-test="base-dialog"]').attributes('data-close-enabled')).toBe('false')
    await wrapper.get('[data-test="base-dialog-close"]').trigger('click')
    expect(wrapper.emitted('close')).toBeUndefined()

    pending.resolve(campaign())
    await flushPromises()
    expect(wrapper.emitted('saved')).toHaveLength(1)
  })
})
