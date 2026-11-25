import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import CheckinRiskPanel from '../CheckinRiskPanel.vue'

const { reviews, decide, associations, config, saveConfig, whitelist, changeWhitelist } = vi.hoisted(() => ({
  reviews: vi.fn(), decide: vi.fn(), associations: vi.fn(), config: vi.fn(),
  saveConfig: vi.fn(), whitelist: vi.fn(), changeWhitelist: vi.fn(),
}))
vi.mock('@/api/admin/checkinRisk', () => ({ riskReasons: ['browser', 'invitation'], checkinRisk: { reviews, decide, associations, config, saveConfig, whitelist, changeWhitelist } }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))

const review = { user_id: 987654, user_email: 'review@example.com', owner_email: 'first@example.com', browser_hash: 'private-browser', checkin_date: '2026-11-26', status: 'pending', created_at: '2026-11-26T00:00:00Z' }
const page = (items: unknown[]) => ({ items, total: items.length, page: 1, page_size: 20, pages: 1 })
const settings = { checkin_mode: 'observe', registration_mode: 'observe', window_minutes: 5, challenge_count: 3, cooldown_count: 6, cooldown_minutes: 15 }
const mountPanel = () => mount(CheckinRiskPanel, { global: { stubs: {
  DataTable: { props: ['columns', 'data'], template: '<table><tr v-for="row in data"><td v-for="col in columns"><slot :name="\'cell-\'+col.key" :row="row" :value="row[col.key]">{{row[col.key]}}</slot></td></tr></table>' },
  Pagination: true, Icon: true, BaseDialog: { props: ['show'], template: '<div v-if="show"><slot/><slot name="footer"/></div>' },
} } })

describe('check-in risk management', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    reviews.mockResolvedValue(page([review]))
    config.mockResolvedValue(settings)
    associations.mockResolvedValue(page([]))
    whitelist.mockResolvedValue(page([]))
  })
  it('shows both emails, requires a reason and submits the selected review identity', async () => {
    const wrapper = mountPanel()
    await wrapper.get('[data-test="risk-open"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain(review.user_email)
    expect(wrapper.text()).toContain(review.owner_email)
    expect(wrapper.text()).not.toContain(String(review.user_id))
    expect(wrapper.text()).not.toContain(review.browser_hash)
    await wrapper.get('[data-test="approve-review"]').trigger('click')
    expect(wrapper.get('[data-test="confirm-risk-action"]').attributes('disabled')).toBeDefined()
    await wrapper.get('[data-test="risk-action-reason"]').setValue('Shared family computer')
    await wrapper.get('[data-test="risk-action-form"]').trigger('submit')
    await flushPromises()
    expect(decide).toHaveBeenCalledWith({ user_id: review.user_id, browser_hash: review.browser_hash, checkin_date: review.checkin_date, decision: 'approved', reason: 'Shared family computer' })
    expect(reviews).toHaveBeenCalledTimes(2)
  })
  it('shows observed records without a decision action', async () => {
    reviews.mockResolvedValue(page([{ ...review, status: 'observed' }]))
    const wrapper = mountPanel()
    await wrapper.get('[data-test="risk-open"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="approve-review"]').exists()).toBe(false)
  })
  it('offers an appeal only for a rejected review dated today in Beijing', async () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    try {
      vi.setSystemTime(new Date('2026-11-25T16:05:00Z'))
      reviews.mockResolvedValue(page([
        { ...review, status: 'rejected' },
        { ...review, checkin_date: '2026-11-25', status: 'rejected' },
        { ...review, status: 'approved' },
        { ...review, status: 'observed' },
      ]))
      const wrapper = mountPanel()
      await wrapper.get('[data-test="risk-open"]').trigger('click')
      await flushPromises()
      expect(wrapper.findAll('[data-test="appeal-review"]')).toHaveLength(1)
      await wrapper.get('[data-test="appeal-review"]').trigger('click')
      expect(wrapper.get('[data-test="confirm-risk-action"]').attributes('disabled')).toBeDefined()
      await wrapper.get('[data-test="risk-action-reason"]').setValue('Shared device verified')
      await wrapper.get('[data-test="risk-action-form"]').trigger('submit')
      await flushPromises()
      expect(decide).toHaveBeenCalledWith({ user_id: review.user_id, browser_hash: review.browser_hash, checkin_date: '2026-11-26', decision: 'approved', reason: 'Shared device verified' })
    } finally {
      vi.useRealTimers()
    }
  })
  it('shows the matched emails and signals and filters both ends of the date range', async () => {
    associations.mockResolvedValue(page([{
      user_email: 'a@example.com', related_email: 'b@example.com', inviter_email: 'a@example.com',
      first_seen_at: '2026-11-24T08:00:00Z', last_seen_at: '2026-11-26T08:00:00Z',
      high_risk: true, reasons: ['browser', 'invitation'], status: 'pending',
      checkin_count: 1, related_checkin_count: 2, reward_amount: 1.25, related_reward_amount: 2.5,
    }]))
    const wrapper = mountPanel()
    await wrapper.get('[data-test="risk-open"]').trigger('click')
    await wrapper.get('[data-test="risk-associations"]').trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('a@example.com')
    expect(wrapper.text()).toContain('b@example.com')
    expect(wrapper.text()).toContain('admin.checkinRisk.highRisk')
    expect(wrapper.text()).toContain('admin.checkinRisk.signals.invitation')
    const filters = wrapper.get('form')
    await filters.get('input[type=search]').setValue('a@example.com')
    await filters.findAll('input[type=date]')[0].setValue('2026-11-22')
    await filters.findAll('input[type=date]')[1].setValue('2026-11-26')
    await filters.trigger('submit')
    expect(associations).toHaveBeenLastCalledWith(expect.objectContaining({
      search: 'a@example.com', start_date: '2026-11-22', end_date: '2026-11-26', page: 1,
    }))
  })
  it('records a reason and the chosen email when removing an allowlist entry', async () => {
    whitelist.mockResolvedValue(page([{ email: 'family@example.com', reason: 'verified', created_at: '2026-11-26T08:00:00Z' }]))
    const wrapper = mountPanel()
    await wrapper.get('[data-test="risk-open"]').trigger('click')
    await wrapper.get('[data-test="risk-whitelist"]').trigger('click')
    await flushPromises()
    await wrapper.get('td button').trigger('click')
    await wrapper.get('[data-test="risk-action-reason"]').setValue('Expired exception')
    await wrapper.get('[data-test="risk-action-form"]').trigger('submit')
    await flushPromises()
    expect(changeWhitelist).toHaveBeenCalledWith({ enabled: false, email: 'family@example.com', reason: 'Expired exception' })
    expect(whitelist).toHaveBeenCalledTimes(2)
  })
  it('sends only editable settings and retains the other mode when enabling check-in enforcement', async () => {
    config.mockResolvedValue({ ...settings, registration_hits_today: { challenge: 8, cooldown: 1 } })
    saveConfig.mockResolvedValue({ ...settings, checkin_mode: 'enforce' })
    const wrapper = mountPanel()
    await wrapper.get('[data-test="risk-open"]').trigger('click')
    await wrapper.get('[data-test="risk-settings"]').trigger('click')
    await flushPromises()
    await wrapper.get('[name="checkin_mode"]').setValue('enforce')
    await wrapper.get('[data-test="risk-settings-form"]').trigger('submit')
    await flushPromises()
    expect(saveConfig).toHaveBeenCalledWith({ ...settings, checkin_mode: 'enforce' })
  })
})
