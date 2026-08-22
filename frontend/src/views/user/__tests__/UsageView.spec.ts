import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import UsageView from '../UsageView.vue'

const {
  query,
  getStats,
  getDashboardModels,
  getDashboardSnapshotV2,
  listRecentEmptyResponses,
  list,
  getAvailable,
  submitEmptyResponseClaim,
  showError,
  showWarning,
  showSuccess,
  showInfo,
} = vi.hoisted(() => ({
  query: vi.fn(),
  getStats: vi.fn(),
  getDashboardModels: vi.fn(),
  getDashboardSnapshotV2: vi.fn(),
  listRecentEmptyResponses: vi.fn(),
  list: vi.fn(),
  getAvailable: vi.fn(),
  submitEmptyResponseClaim: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

const messages: Record<string, string> = {
  'admin.dashboard.timeRange': 'Time range',
  'admin.dashboard.granularity': 'Granularity',
  'admin.dashboard.day': 'Day',
  'admin.dashboard.hour': 'Hour',
  'admin.users.columnSettings': 'Columns',
  'admin.usage.group': 'Group',
  'admin.usage.billingType': 'Billing type',
  'admin.usage.billingMode': 'Billing mode',
  'admin.usage.allTypes': 'All types',
  'admin.usage.allBillingTypes': 'All billing types',
  'admin.usage.billingTypeBalance': 'Balance',
  'admin.usage.billingTypeSubscription': 'Subscription',
  'admin.usage.allBillingModes': 'All billing modes',
  'admin.usage.billingModeToken': 'Token',
  'admin.usage.billingModePerRequest': 'Per request',
  'admin.usage.billingModeImage': 'Image',
  'admin.usage.allGroups': 'All groups',
  'admin.usage.allModels': 'All models',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.model': 'Model',
  'usage.type': 'Type',
  'usage.ws': 'WS',
  'usage.stream': 'Stream',
  'usage.sync': 'Sync',
  'usage.exporting': 'Exporting',
  'usage.exportCsv': 'Export CSV',
  'usage.failedToLoad': 'Failed to load',
  'usage.tabs.usage': 'Usage',
  'usage.tabs.errors': 'Error Requests',
  'usage.tabs.emptyResponses': 'Empty responses',
  'usage.emptyResponse.bulk.title': 'Recent empty responses',
  'usage.emptyResponse.bulk.subtitle': 'Last 7 days',
  'usage.emptyResponse.bulk.action': 'Claim empty responses',
  'usage.emptyResponse.bulk.claiming': 'Claiming...',
  'usage.emptyResponse.bulk.empty': 'No empty responses',
  'usage.emptyResponse.bulk.loadFailed': 'Failed to load empty responses',
  'usage.emptyResponse.bulk.claimSuccess': 'Compensated {count}, skipped {skipped}',
  'usage.emptyResponse.bulk.claimFailed': 'Failed to claim empty responses',
  'usage.emptyResponse.bulk.dailyRemaining': '{count} claims left today',
  'usage.emptyResponse.statusLabel': 'Status',
  'usage.emptyResponse.status.claimable': 'Claimable',
  'usage.emptyResponse.status.compensated': 'Compensated',
  'usage.emptyResponse.status.daily_limited': 'Daily limit',
  'usage.emptyResponse.tokens': 'Tokens',
  'usage.emptyResponse.tokenDetail': 'In {input} / Out {output} / Cache {cache} / Total {total}',
  'usage.emptyResponse.claimOne': 'Claim',
  'usage.emptyResponse.claimingOne': 'Claiming...',
  'usage.emptyResponse.singleClaimSuccess': 'Empty response compensated',
  'usage.emptyResponse.dailyLimitReached': 'Daily limit reached',
  'usage.emptyResponse.claimRules.dailyLimit': 'Up to 15 automatic compensations per day',
  'usage.emptyResponse.claimRules.tokenLimit': 'Output Token must be 10 or less',
  'usage.emptyResponse.reasonCode.pure_empty': 'Empty output',
  'usage.emptyResponse.reasonCode.daily_limit_manual_review': 'Daily limit reached',
  'usage.emptyResponse.originalCharge': 'Original charge',
  'usage.emptyResponse.refunded': 'Refunded',
  'usage.noDataToExport': 'No data',
  'usage.preparingExport': 'Preparing export',
  'usage.exportSuccess': 'Export success',
  'usage.exportFailed': 'Export failed',
  'common.refresh': 'Refresh',
  'common.reset': 'Reset',
  'common.actions': 'Actions',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStats,
    getDashboardModels,
    getDashboardSnapshotV2,
    listRecentEmptyResponses,
    submitEmptyResponseClaim,
  },
  keysAPI: {
    list,
  },
  userGroupsAPI: {
    getAvailable,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        let text = messages[key] ?? key
        if (params) {
          for (const [name, value] of Object.entries(params)) {
            text = text.replaceAll(`{${name}}`, String(value))
          }
        }
        return text
      },
    }),
  }
})

const simpleStub = { template: '<div><slot /></div>' }
const chartStub = { template: '<div />' }
const usageTableStub = {
  props: ['showCompensationAction'],
  template: `
    <div data-testid="usage-table-stub" :data-show-compensation-action="showCompensationAction ? 'true' : 'false'">
      <button v-if="showCompensationAction" data-testid="legacy-empty-response-action">申请补空回</button>
    </div>
  `,
}

const usageLog = {
  id: 1,
  request_id: 'req-user-export',
  actual_cost: 0.092883,
  total_cost: 0.092883,
  rate_multiplier: 1,
  service_tier: 'priority',
  input_cost: 0.020285,
  output_cost: 0.00303,
  cache_creation_cost: 0.000001,
  cache_read_cost: 0.069568,
  input_tokens: 4057,
  output_tokens: 101,
  cache_creation_tokens: 4,
  cache_read_tokens: 278272,
  cache_creation_5m_tokens: 0,
  cache_creation_1h_tokens: 0,
  image_count: 0,
  image_size: null,
  first_token_ms: 12,
  duration_ms: 345,
  created_at: '2026-03-08T00:00:00Z',
  model: 'gpt-5.4',
  reasoning_effort: null,
  ip_address: '203.0.113.10',
  api_key: { name: 'demo-key' },
  billing_mode: 'token',
  request_type: 'sync',
  stream: false,
}

function mountUsageView() {
  return mount(UsageView, {
    global: {
      stubs: {
        AppLayout: simpleStub,
        Pagination: true,
        Select: true,
        DateRangePicker: true,
        Icon: true,
        UsageStatsCards: chartStub,
        UsageTable: usageTableStub,
        ModelDistributionChart: chartStub,
        GroupDistributionChart: chartStub,
        EndpointDistributionChart: chartStub,
        TokenUsageTrend: chartStub,
      },
    },
  })
}

describe('user UsageView', () => {
  beforeEach(() => {
    query.mockReset()
    getStats.mockReset()
    getDashboardModels.mockReset()
    getDashboardSnapshotV2.mockReset()
    listRecentEmptyResponses.mockReset()
    list.mockReset()
    getAvailable.mockReset()
    submitEmptyResponseClaim.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()

    query.mockResolvedValue({ items: [usageLog], total: 1, pages: 1 })
    getStats.mockResolvedValue({
      total_requests: 1,
      total_input_tokens: 10,
      total_output_tokens: 20,
      total_cache_tokens: 0,
      total_tokens: 30,
      total_cost: 0.1,
      total_actual_cost: 0.08,
      average_duration_ms: 12,
      endpoints: [],
      upstream_endpoints: [],
      endpoint_paths: [],
    })
    getDashboardModels.mockResolvedValue({
      models: [{ model: 'gpt-5.4', requests: 1, input_tokens: 10, output_tokens: 20, cache_creation_tokens: 0, cache_read_tokens: 0, total_tokens: 30, cost: 0.1, actual_cost: 0.08 }],
      start_date: '2026-03-08',
      end_date: '2026-03-08',
    })
    getDashboardSnapshotV2.mockResolvedValue({
      generated_at: '2026-03-08T00:00:00Z',
      start_date: '2026-03-08',
      end_date: '2026-03-08',
      granularity: 'hour',
      trend: [],
      groups: [],
    })
    listRecentEmptyResponses.mockResolvedValue([
      {
        usage_log_id: 77,
        model: 'claude-opus-4-6',
        api_key_name: 'cli',
        group_name: 'cc',
        inbound_endpoint: '/v1/messages',
        actual_cost: 1.25,
        input_tokens: 1234,
        output_tokens: 0,
        cache_tokens: 46,
        total_tokens: 1280,
        refunded_amount: 0,
        status: 'claimable',
        reason_code: 'pure_empty',
        created_at: '2026-03-08T00:00:00Z',
      },
    ])
    list.mockResolvedValue({ items: [{ id: 1, name: 'demo-key' }] })
    getAvailable.mockResolvedValue([{ id: 1, name: 'default' }])
		submitEmptyResponseClaim.mockResolvedValue({
			id: 9,
			usage_log_id: 1,
			status: 'compensated',
			reason_code: 'pure_empty',
			estimated_refund: 0.092883,
			refunded_amount: 0.092883,
		})
  })

  it('loads logs, stats, model stats, and snapshot on first render', async () => {
    mountUsageView()
    await flushPromises()

    expect(query).toHaveBeenCalled()
    expect(getStats).toHaveBeenCalled()
    expect(getDashboardModels).toHaveBeenCalled()
    expect(getDashboardSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      include_trend: true,
      include_model_stats: false,
      include_group_stats: true,
    }))
    expect(list).toHaveBeenCalledWith(1, 100)
    expect(getAvailable).toHaveBeenCalled()
  })

  it('does not expose the legacy empty-response claim action in usage details', async () => {
    query.mockResolvedValueOnce({
      items: [{
        ...usageLog,
        compensation_eligible: true,
        compensation_eligibility: 'eligible',
      }],
      total: 1,
      pages: 1,
    })

    const wrapper = mountUsageView()
    await flushPromises()

    expect(wrapper.get('[data-testid="usage-table-stub"]').attributes('data-show-compensation-action')).toBe('false')
    expect(wrapper.find('[data-testid="legacy-empty-response-action"]').exists()).toBe(false)
  })

  it('exports csv with current filters and without admin-only fields', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    let exportedBlob: Blob | null = null
    let csvContent = ''
    const OriginalBlob = globalThis.Blob
    vi.stubGlobal('Blob', vi.fn((parts: BlobPart[], options?: BlobPropertyBag) => {
      csvContent = parts.map((part) => String(part)).join('')
      return new OriginalBlob(parts, options)
    }))
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:usage-export'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    await (wrapper.vm as any).exportToCSV()

    expect(exportedBlob).not.toBeNull()
    expect(query).toHaveBeenCalledWith(expect.objectContaining({
      page_size: 100,
      sort_by: 'created_at',
      sort_order: 'desc',
    }))
    expect(clickSpy).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()
    expect(csvContent.startsWith('\uFEFF')).toBe(true)
    expect(csvContent.slice(1)).toBe([
      'Time,API Key Name,Model,Reasoning Effort,Inbound Endpoint,IP Address,Type,Billing Mode,Input Tokens,Output Tokens,Cache Read Tokens,Cache Creation Tokens,Rate Multiplier,Billed Cost,Original Cost,First Token (ms),Duration (ms)',
      '2026-03-08T00:00:00Z,demo-key,gpt-5.4,"\'-",,203.0.113.10,Sync,Token,4057,101,278272,4,1,0.09288300,0.09288300,12,345',
    ].join('\n'))
    expect(csvContent).toContain('IP Address')
    expect(csvContent).toContain('203.0.113.10')
    expect(csvContent).toContain('Billed Cost')
    expect(csvContent).toContain('Original Cost')
    expect(csvContent).not.toContain('Upstream Endpoint')
    expect(csvContent).not.toContain('account_cost')
    expect(csvContent).not.toContain('account_rate_multiplier')

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    vi.unstubAllGlobals()
    clickSpy.mockRestore()
  })

  it('exports historical image rows with image billing mode derived from image_count', async () => {
    query.mockResolvedValue({
      items: [
        {
          ...usageLog,
          request_id: 'req-user-export-legacy-image',
          actual_cost: 0.2,
          total_cost: 0.2,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          image_count: 1,
          model: 'gpt-image-2',
          billing_mode: null,
          ip_address: null,
        },
      ],
      total: 1,
      pages: 1,
    })

    const wrapper = mountUsageView()
    await flushPromises()

    let csvContent = ''
    const OriginalBlob = globalThis.Blob
    vi.stubGlobal('Blob', vi.fn((parts: BlobPart[], options?: BlobPropertyBag) => {
      csvContent = parts.map((part) => String(part)).join('')
      return new OriginalBlob(parts, options)
    }))
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn(() => 'blob:usage-export') as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    await (wrapper.vm as any).exportToCSV()

    expect(csvContent).toContain('Billing Mode')
    expect(csvContent).toContain('Image')
    expect(csvContent).not.toContain(',Token,0,0,0,0,')

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    vi.unstubAllGlobals()
    clickSpy.mockRestore()
  })

	it('submits an eligible claim and updates only the matching usage row in place', async () => {
		query.mockResolvedValue({
			items: [{
				...usageLog,
				compensation_eligible: true,
				compensation_eligibility: 'eligible',
				compensated_cost: 0,
				net_actual_cost: usageLog.actual_cost,
			}],
			total: 1,
			pages: 1,
		})
		const wrapper = mountUsageView()
		await flushPromises()

		;(wrapper.vm as any).openEmptyResponseClaim((wrapper.vm as any).usageLogs[0])
		await (wrapper.vm as any).submitEmptyResponseClaim('empty response')
		await flushPromises()

		expect(submitEmptyResponseClaim).toHaveBeenCalledWith(1, { reason: 'empty response' })
		expect((wrapper.vm as any).usageLogs[0]).toMatchObject({
			claim_status: 'compensated',
			compensation_eligible: false,
			compensated_cost: usageLog.actual_cost,
			net_actual_cost: 0,
		})
		expect(query).toHaveBeenCalledTimes(1)
	})

  it('shows recent empty responses with rules and token details', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.find('[data-testid="empty-response-tab"]').trigger('click')
    await flushPromises()

    expect(listRecentEmptyResponses).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('Recent empty responses')
    expect(wrapper.text()).toContain('Up to 15 automatic compensations per day')
    expect(wrapper.text()).toContain('Output Token must be 10 or less')
    expect(wrapper.text()).toContain('claude-opus-4-6')
    expect(wrapper.text()).toContain('cli')
    expect(wrapper.text()).toContain('$1.250000')
    expect(wrapper.find('[data-testid="empty-response-token-77"]').text()).toContain('1,234')
    expect(wrapper.find('[data-testid="empty-response-token-77"]').text()).toContain('0')
    expect(wrapper.find('[data-testid="empty-response-token-77"]').text()).toContain('46')
    expect(wrapper.find('[data-testid="empty-response-token-77"]').text()).toContain('1,280')
    expect(wrapper.find('[data-testid="claim-empty-responses"]').exists()).toBe(false)
  })

  it('keeps the empty response table fixed-width and horizontally scrollable', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.find('[data-testid="empty-response-tab"]').trigger('click')
    await flushPromises()

    const scroll = wrapper.find('[data-testid="empty-response-table-scroll"]')
    const table = wrapper.find('[data-testid="empty-response-table"]')

    expect(scroll.exists()).toBe(true)
    expect(scroll.classes()).toEqual(expect.arrayContaining(['overflow-x-auto', 'touch-pan-x', 'overscroll-x-contain']))
    expect(table.exists()).toBe(true)
    expect(table.classes()).toEqual(expect.arrayContaining(['table-fixed', 'min-w-[1280px]']))
    expect(wrapper.find('[data-testid="empty-response-model-77"]').classes()).toContain('truncate')
    expect(wrapper.find('[data-testid="claim-empty-response-77"]').classes()).toEqual(expect.arrayContaining(['min-w-[72px]', 'whitespace-nowrap']))
  })

  it('puts empty response actions in the first column for mobile access', async () => {
    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.find('[data-testid="empty-response-tab"]').trigger('click')
    await flushPromises()

    const headerCells = wrapper.findAll('[data-testid="empty-response-table"] thead th')
    const firstBodyCells = wrapper.findAll('[data-testid="empty-response-table"] tbody tr:first-child td')

    expect(headerCells[0]?.text()).toBe('Actions')
    expect(firstBodyCells[0]?.find('[data-testid="claim-empty-response-77"]').exists()).toBe(true)
  })

  it('claims a single empty response row and updates that row only', async () => {
    submitEmptyResponseClaim.mockResolvedValueOnce({
      id: 701,
      usage_log_id: 77,
      status: 'compensated',
      reason_code: 'pure_empty',
      estimated_refund: 1.25,
      refunded_amount: 1.25,
    })
    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.find('[data-testid="empty-response-tab"]').trigger('click')
    await flushPromises()
    await wrapper.find('[data-testid="claim-empty-response-77"]').trigger('click')
    await flushPromises()

    expect(submitEmptyResponseClaim).toHaveBeenCalledWith(77, { reason: '' })
    expect(showSuccess).toHaveBeenCalledWith('Empty response compensated')
    const row = (wrapper.vm as any).emptyResponseRows[0]
    expect(row).toMatchObject({
      claim_id: 701,
      status: 'compensated',
      refunded_amount: 1.25,
    })
  })

  it('does not render claim buttons for rows that are no longer claimable', async () => {
    listRecentEmptyResponses.mockResolvedValueOnce([
      {
        usage_log_id: 77,
        claim_id: 701,
        model: 'claude-opus-4-6',
        api_key_name: 'cli',
        group_name: 'cc',
        inbound_endpoint: '/v1/messages',
        actual_cost: 1.25,
        input_tokens: 1234,
        output_tokens: 0,
        cache_tokens: 46,
        total_tokens: 1280,
        refunded_amount: 1.25,
        status: 'compensated',
        reason_code: 'pure_empty',
        created_at: '2026-03-08T00:00:00Z',
      },
      {
        usage_log_id: 78,
        model: 'claude-opus-4-6',
        api_key_name: 'cli',
        group_name: 'cc',
        inbound_endpoint: '/v1/messages',
        actual_cost: 1.25,
        input_tokens: 1234,
        output_tokens: 0,
        cache_tokens: 46,
        total_tokens: 1280,
        refunded_amount: 0,
        status: 'daily_limited',
        reason_code: 'daily_limit_manual_review',
        created_at: '2026-03-08T00:00:00Z',
      },
    ])
    const wrapper = mountUsageView()
    await flushPromises()

    await wrapper.find('[data-testid="empty-response-tab"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="claim-empty-response-77"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="claim-empty-response-78"]').exists()).toBe(false)
    expect(submitEmptyResponseClaim).not.toHaveBeenCalled()
  })
})
