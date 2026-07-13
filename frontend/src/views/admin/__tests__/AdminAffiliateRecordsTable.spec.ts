import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminAffiliateRecordsTable from '../affiliates/AdminAffiliateRecordsTable.vue'

const { listInviteRecords, getUserOverview } = vi.hoisted(() => ({
  listInviteRecords: vi.fn(),
  getUserOverview: vi.fn(),
}))

vi.mock('@/api/admin/affiliates', () => {
  const api = {
    listInviteRecords,
    listRebateRecords: vi.fn(),
    listTransferRecords: vi.fn(),
    getUserOverview,
  }
  return { affiliatesAPI: api, default: api }
})

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return { ...actual, useI18n: () => ({ t: (key: string) => key }) }
})

vi.mock('@/utils/apiError', () => ({ extractI18nErrorMessage: () => 'error' }))

const DataTableStub = {
  props: ['columns', 'data'],
  template: `<div><div v-for="row in data" :key="row.invitee_id"><slot name="cell-inviter" :row="row" /><slot name="cell-invitee" :row="row" /><slot name="cell-aff_code" :row="row" /><slot name="cell-tier" :row="row" /><slot name="cell-qualified" :row="row" /><slot name="cell-rate" :row="row" /></div></div>`,
}

function mountTable() {
  return mount(AdminAffiliateRecordsTable, {
    props: { type: 'invites' },
    global: { stubs: {
      AppLayout: { template: '<div><slot /></div>' },
      TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
      DataTable: DataTableStub,
      Pagination: true,
      Icon: true,
      BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
      OrderStatusBadge: true,
    } },
  })
}

describe('AdminAffiliateRecordsTable promotion reporting', () => {
  beforeEach(() => {
    listInviteRecords.mockResolvedValue({ items: [{
      inviter_id: 1, inviter_email: 'owner@example.com', inviter_username: 'owner',
      invitee_id: 2, invitee_email: 'member@example.com', invitee_username: 'member',
      aff_code: 'OWNER', total_rebate: 2, qualifying_payment_amount: 50,
      qualified: true, qualified_at: '2026-07-13T00:00:00Z', invited_count: 12,
      qualified_invitee_count: 10, automatic_level: 'silver',
      automatic_rebate_rate_percent: 12, custom_rebate_rate_percent: 18,
      effective_rebate_rate_percent: 18, created_at: '2026-07-13T00:00:00Z',
    }], total: 1 })
    getUserOverview.mockResolvedValue({
      user_id: 1, email: 'owner@example.com', username: 'owner', aff_code: 'OWNER',
      rebate_rate_percent: 18, invited_count: 12, rebated_invitee_count: 4,
      available_quota: 3, history_quota: 5, automatic_level: 'silver',
      automatic_rebate_rate_percent: 12, effective_rebate_rate_percent: 18,
      has_custom_rebate_rate: true, custom_rebate_rate_percent: 18,
      qualified_invitee_count: 10, qualification_amount: 50,
      next_level_invitee_threshold: 30, remaining_qualified_invitees: 20,
    })
  })

  it('shows tier, qualification, custom override, and effective rate on invite records', async () => {
    const wrapper = mountTable()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.affiliates.tiers.silver')
    expect(wrapper.text()).toContain('admin.affiliates.records.qualified')
    expect(wrapper.text()).toContain('18%')
    expect(wrapper.text()).toContain('admin.affiliates.records.customOverride')
  })

  it('shows promotion details in the user overview', async () => {
    const wrapper = mountTable()
    await flushPromises()
    const userButton = wrapper.findAll('button').find((button) => button.text() === 'owner@example.com')
    expect(userButton).toBeDefined()
    await userButton?.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('admin.affiliates.overview.automaticTier')
    expect(wrapper.text()).toContain('10 / 12')
    expect(wrapper.text()).toContain('admin.affiliates.overview.customOverride')
    expect(wrapper.text()).toContain('admin.affiliates.overview.effectiveRate')
  })

  it('collapses secondary invite columns on mobile without a fixed minimum width', async () => {
    const wrapper = mountTable()
    await flushPromises()
    const secondary = wrapper.vm.$.setupState.columns.filter((column: { key: string }) => ['aff_code', 'tier', 'qualified'].includes(column.key))
    expect(secondary.every((column: { class?: string }) => column.class?.includes('hidden') && column.class?.includes('md:table-cell'))).toBe(true)
    expect(wrapper.html()).not.toMatch(/min-w-\[/)
  })
})
