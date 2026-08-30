import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AdminAffiliateSummaryView from '../affiliates/AdminAffiliateSummaryView.vue'

const { listInviterSummaries, push } = vi.hoisted(() => ({
  listInviterSummaries: vi.fn(),
  push: vi.fn(),
}))

vi.mock('@/api/admin/affiliates', () => {
  const api = { listInviterSummaries }
  return { affiliatesAPI: api, default: api }
})

vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('@/utils/apiError', () => ({ extractI18nErrorMessage: () => 'error' }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))
vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div v-for="row in data" :key="row.inviter_id">
        <slot name="cell-inviter" :row="row" />
        <slot name="cell-invited_count" :row="row" />
        <slot name="cell-qualified_invitee_count" :row="row" />
        <slot name="cell-total_rebate" :row="row" />
        <slot name="cell-available_quota" :row="row" />
        <slot name="cell-transferred_amount" :row="row" />
        <slot name="cell-rebate_record_count" :row="row" />
        <slot name="cell-last_invited_at" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `,
}

function mountView() {
  return mount(AdminAffiliateSummaryView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: { template: '<div><slot name="filters"/><slot name="table"/><slot name="pagination"/></div>' },
        DataTable: DataTableStub,
        Pagination: true,
        Icon: true,
      },
    },
  })
}

describe('AdminAffiliateSummaryView', () => {
  beforeEach(() => {
    localStorage.clear()
    push.mockReset()
    listInviterSummaries.mockReset()
    listInviterSummaries.mockResolvedValue({
      items: [{
        inviter_id: 42,
        inviter_email: 'owner@example.com',
        inviter_username: 'owner',
        aff_code: 'OWNER',
        invited_count: 12,
        qualified_invitee_count: 9,
        total_rebate: 18.5,
        available_quota: 3.5,
        transferred_amount: 15,
        rebate_record_count: 7,
        last_invited_at: '2026-08-28T08:00:00Z',
      }],
      total: 1,
    })
  })

  it('loads inviters with most invitations first and shows rebate totals', async () => {
    const wrapper = mountView()
    await flushPromises()

    expect(listInviterSummaries).toHaveBeenCalledWith(expect.objectContaining({
      sort_by: 'invited_count',
      sort_order: 'desc',
    }))
    expect(wrapper.text()).toContain('owner@example.com')
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('9')
    expect(wrapper.text()).toContain('$18.50')
    expect(wrapper.text()).toContain('$3.50')
    expect(wrapper.text()).toContain('$15.00')
    expect(wrapper.text()).toContain('7')
  })

  it('opens filtered invitation and rebate records for an inviter', async () => {
    const wrapper = mountView()
    await flushPromises()

    const inviteButton = wrapper.get('[data-test="view-invite-records"]')
    const rebateButton = wrapper.get('[data-test="view-rebate-records"]')
    await inviteButton.trigger('click')
    await rebateButton.trigger('click')

    expect(push).toHaveBeenNthCalledWith(1, {
      path: '/admin/affiliates/invites',
      query: { search: 'owner@example.com' },
    })
    expect(push).toHaveBeenNthCalledWith(2, {
      path: '/admin/affiliates/rebates',
      query: { search: 'owner@example.com' },
    })
  })

  it('uses the persisted sortable column for the initial request', async () => {
    localStorage.setItem('admin-affiliate-summary-table-sort', JSON.stringify({
      key: 'total_rebate',
      order: 'asc',
    }))

    mountView()
    await flushPromises()

    expect(listInviterSummaries).toHaveBeenCalledWith(expect.objectContaining({
      sort_by: 'total_rebate',
      sort_order: 'asc',
    }))
  })
})
