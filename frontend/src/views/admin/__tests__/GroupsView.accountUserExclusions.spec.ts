import { reactive } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import GroupsView from '../GroupsView.vue'

const mocks = vi.hoisted(() => ({ list: vi.fn(), getUsageSummary: vi.fn(), getCapacitySummary: vi.fn(), getLiveCapability: vi.fn() }))
const auth = reactive({ role: 'admin', isSimpleMode: false })
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ get isAdmin() { return auth.role === 'admin' }, get isSimpleMode() { return auth.isSimpleMode } }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))
vi.mock('@/stores/onboarding', () => ({ useOnboardingStore: () => ({ isCurrentStep: () => false, nextStep: vi.fn() }) }))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: {
  ...mocks,
  getAll: vi.fn().mockResolvedValue([]),
  getAllIncludingInactive: vi.fn().mockResolvedValue([]),
  getModelAllowlistCandidates: vi.fn().mockResolvedValue([])
} } }))
vi.mock('vue-i18n', async importOriginal => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)

function create() {
  return mount(GroupsView, { global: { stubs: {
    AppLayout: { template: '<main><slot /></main>' },
    TablePageLayout: { template: '<section><slot name="filters" /><slot name="table" /><slot name="pagination" /></section>' },
    DataTable: { props: ['data'], template: '<div><div v-for="row in data" :key="row.id"><slot name="cell-actions" :row="row" /></div></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
    GroupAccountUserExclusionsModal: { props: ['show', 'group'], template: '<div v-if="show" data-test="account-exclusions-dialog">{{ group.name }}</div>' },
    Pagination: true, ConfirmDialog: true, EmptyState: true, Select: true, PlatformIcon: true, Icon: true,
    GroupCapacityBadge: true, GroupRateMultipliersModal: true, GroupRPMOverridesModal: true, VueDraggable: true,
    SystemCustomGroupDialog: true, CodexManifestAccountsField: true, PricingEntryCard: true, ReasoningEffortPolicyFields: true
  } } })
}

beforeEach(() => {
  vi.clearAllMocks()
  auth.role = 'admin'
  auth.isSimpleMode = false
  mocks.list.mockResolvedValue({ items: [{ id: 42, name: 'Restricted group', platform: 'anthropic', status: 'active' }], total: 1, page: 1, page_size: 20, pages: 1 })
  mocks.getUsageSummary.mockResolvedValue([])
  mocks.getCapacitySummary.mockResolvedValue([])
  mocks.getLiveCapability.mockResolvedValue({ supported: false })
})

describe('group account user exclusion entry authorization', () => {
  it('opens the selected group only when the super administrator clicks the entry', async () => {
    const wrapper = create()
    await flushPromises()
    expect(wrapper.find('[data-test="account-exclusions-dialog"]').exists()).toBe(false)
    await wrapper.get('[data-testid="group-account-user-exclusions"]').trigger('click')
    expect(wrapper.get('[data-test="account-exclusions-dialog"]').text()).toContain('Restricted group')
    auth.role = 'sub_admin'
    await flushPromises()
    expect(wrapper.find('[data-testid="group-account-user-exclusions"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="account-exclusions-dialog"]').exists()).toBe(false)
  })

  it.each(['sub_admin', 'user'])('does not expose the entry or dialog to %s', async role => {
    auth.role = role
    const wrapper = create()
    await flushPromises()
    expect(wrapper.find('[data-testid="group-account-user-exclusions"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="account-exclusions-dialog"]').exists()).toBe(false)
  })
})
