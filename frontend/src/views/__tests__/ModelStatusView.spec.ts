import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { ref } from 'vue'
import ModelStatusView from '../ModelStatusView.vue'
import type { ModelStatusBucket, ModelStatusResponse } from '@/api/modelStatus'

const { getModelStatus, authStore, appStore } = vi.hoisted(() => ({
  getModelStatus: vi.fn(),
  authStore: { isAuthenticated: false },
  appStore: { fetchPublicSettings: vi.fn().mockResolvedValue({}) },
}))

vi.mock('@/api/modelStatus', () => ({ getModelStatus }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => authStore }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({
    locale: ref('en'),
    t: (key: string, params?: Record<string, unknown>) => `${key}${params ? ` ${Object.values(params).join(' ')}` : ''}`,
  }),
}))

const metrics = {
  total: 10, success: 1, failure: 0, empty: 9, unknown: 0,
  success_rate: 10, avg_ttft_ms: null, avg_duration_ms: 1500,
  ttft_samples: 0, duration_samples: 1,
}

function report(): ModelStatusResponse {
  const emptyBuckets = (): ModelStatusBucket[] => Array.from({ length: 20 }, (_, index) => ({
    start_at: new Date(Date.UTC(2026, 8, 6, 0, index * 15)).toISOString(),
    end_at: new Date(Date.UTC(2026, 8, 6, 0, index * 15 + 15)).toISOString(),
    total: 0, success: 0, failure: 0, empty: 0, unknown: 0, requests: [],
  }))
  const activeBuckets = emptyBuckets()
  activeBuckets[0] = { ...activeBuckets[0], total: 1, empty: 1, requests: [{ at: '2026-09-06T03:55:00Z', outcome: 'empty' }] }
  activeBuckets[1] = { ...activeBuckets[1], total: 1, success: 1, requests: [{ at: '2026-09-06T03:56:00Z', outcome: 'success' }] }
  return {
    generated_at: '2026-09-06T04:00:00Z',
    snapshot_at: '2026-09-06T04:00:00Z',
    bucket_count: 20,
    bucket_interval_minutes: 15,
    refresh_interval_seconds: 30,
    coverage: { status: 'partial', terminal_errors_enabled: false, reasons: ['best_effort_recording', 'terminal_errors_disabled'] },
    summary: metrics,
    groups: [
      { id: 1, name: 'Public A', platform: 'openai', metrics, models: [
        { name: 'same-model', platform: 'openai', status: 'unavailable', metrics, buckets: activeBuckets },
        { name: 'another-model', platform: 'openai', status: 'insufficient_data', metrics: { ...metrics, success_rate: null }, buckets: emptyBuckets() },
      ] },
      { id: 2, name: 'Public B', platform: 'openai', metrics, models: [
        { name: 'same-model', platform: 'openai', status: 'healthy', metrics: { ...metrics, success_rate: 100 }, buckets: emptyBuckets() },
      ] },
    ],
  }
}

const wrappers: VueWrapper[] = []
function render() {
  const wrapper = mount(ModelStatusView, { global: { stubs: {
    AppLayout: { template: '<div data-testid="app-layout"><slot /></div>' },
    PlazaNavBar: { template: '<nav data-testid="public-nav" />' },
    PlatformIcon: true,
    Select: {
      props: ['modelValue', 'options'],
      emits: ['update:modelValue'],
      template: '<select :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>',
    },
  } } })
  wrappers.push(wrapper)
  return wrapper
}

describe('ModelStatusView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-09-06T04:00:00Z'))
    authStore.isAuthenticated = false
    getModelStatus.mockReset().mockResolvedValue(report())
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    vi.useRealTimers()
  })

  it('shows the public layout and preserves separate groups for the same model', async () => {
    const wrapper = render()
    await flushPromises()

    expect(wrapper.find('[data-testid="public-nav"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="app-layout"]').exists()).toBe(false)
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(3)
    expect(wrapper.text()).toContain('Public A')
    expect(wrapper.text()).toContain('Public B')
    expect(wrapper.find('.status-summary').exists()).toBe(false)
    expect(wrapper.find('.status-timestamps').exists()).toBe(false)
    expect(wrapper.find('.coverage-line').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('modelStatus.coverage.terminal_errors_disabled')
    expect(wrapper.text()).not.toContain('modelStatus.snapshot')
    expect(wrapper.text()).not.toContain('modelStatus.snapshotWindow')
    expect(wrapper.findAll('[data-outcome]').map(bar => bar.attributes('data-outcome'))).toEqual(['empty', 'success'])
    expect(wrapper.find('[data-outcome="empty"]').attributes('title')).toContain('modelStatus.outcome.empty')
  })

  it('uses the sidebar layout for authenticated users', async () => {
    authStore.isAuthenticated = true
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('[data-testid="app-layout"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="public-nav"]').exists()).toBe(false)
  })

  it('opens bucket details when a signal light is clicked', async () => {
    const wrapper = render()
    await flushPromises()
    const bucket = wrapper.find('[data-testid="status-bucket"]')
    expect(wrapper.get('.bucket-help').attributes('title')).toContain('modelStatus.bucketClickHint')
    await bucket.trigger('click')
    expect(bucket.classes()).toContain('bucket-pressed')
    await flushPromises()
    expect(document.body.textContent).toContain('modelStatus.bucketDetails')
    expect(document.body.textContent).toContain('modelStatus.requestDetails')
    expect(document.body.textContent).toContain('modelStatus.outcome.empty')
    const close = document.body.querySelector('.brand-floating-close') as HTMLElement | null
    close?.click()
    await vi.advanceTimersByTimeAsync(400)
    expect(document.body.querySelector('.bucket-detail')).toBeNull()
  })

  it('marks low-failure buckets as degraded instead of failure', async () => {
    const data = report()
    data.groups[0].models[0].buckets![0] = {
      ...data.groups[0].models[0].buckets![0],
      total: 100,
      success: 99,
      failure: 1,
      empty: 0,
      unknown: 0,
      requests: Array.from({ length: 100 }, (_, index) => ({
        at: `2026-09-06T03:${String(index).padStart(2, '0')}:00Z`,
        outcome: index === 99 ? 'failure' : 'success',
      })),
    }
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    const bucket = wrapper.get('[data-testid="status-bucket"]')
    expect(bucket.classes()).toContain('bucket-degraded')
    expect(bucket.attributes('data-outcome')).toBe('degraded')
    expect(bucket.attributes('title')).toContain('modelStatus.bucketStatus.degraded')
  })

  it('separates incomplete records from outcomes without rendering a global summary', async () => {
    const data = report()
    const incompleteMetrics = { ...metrics, total: 13, unknown: 3 }
    data.summary = incompleteMetrics
    data.groups[0].metrics = incompleteMetrics
    data.groups[0].models[0].metrics = incompleteMetrics
    data.groups[0].models[0].buckets![0] = { ...data.groups[0].models[0].buckets![0], total: 1, unknown: 1, requests: [{ at: '2026-09-06T03:54:00Z', outcome: 'unknown' }] }
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.findAll('.model-row .incomplete-note')).toHaveLength(1)
    expect(wrapper.find('[data-testid="incomplete-records"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('modelStatus.outcome.unknown')
    expect(wrapper.find('[data-outcome="unknown"]').exists()).toBe(false)
    expect(wrapper.find('.bucket-unknown').exists()).toBe(true)
    expect(wrapper.findAll('[data-outcome]').map(bar => bar.attributes('data-outcome'))).toEqual(['empty', 'success'])
    expect(wrapper.findAll('.recent-bars').at(0)?.element.children).toHaveLength(20)

    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="incomplete-records"]').exists()).toBe(false)
    expect(wrapper.find('.recent-incomplete').exists()).toBe(false)
  })

  it('keeps incomplete-only model records out of the failure rate', async () => {
    const data = report()
    data.summary = { ...metrics, total: 2, success: 0, empty: 0, unknown: 2, success_rate: null }
    data.groups[0].models[0].metrics = data.summary
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()
    expect(wrapper.find('.status-summary').exists()).toBe(false)
    expect(wrapper.find('[data-testid="incomplete-records"]').exists()).toBe(false)
    expect(wrapper.get('.model-row .model-rate strong').text()).toBe('-')
    expect(wrapper.get('.model-row .incomplete-note').text()).toBe('modelStatus.incompleteRecords 2')
  })

  it('filters by group and model without merging names or changing model metrics', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="model-search"]').setValue('SAME-model')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(2)
    await wrapper.get('select').setValue('2')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="model-row"]').text()).toContain('100%')
    await wrapper.get('[data-testid="model-search"]').setValue('missing')
    expect(wrapper.text()).toContain('modelStatus.noMatches')
  })

  it('keeps the last successful report visible when refresh fails, then clears the warning on retry', async () => {
    const wrapper = render()
    await flushPromises()
    getModelStatus.mockRejectedValueOnce(new Error('private upstream error'))
    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('modelStatus.refreshFailed')
    expect(wrapper.text()).not.toContain('private upstream error')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(3)
    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
  })

  it('shows initial loading and a retryable error without inventing zero traffic', async () => {
    let reject!: (error: Error) => void
    getModelStatus.mockReturnValueOnce(new Promise((_, rejectPromise) => { reject = rejectPromise }))
    const wrapper = render()
    expect(wrapper.text()).toContain('modelStatus.loading')
    reject(new Error('unavailable'))
    await flushPromises()
    expect(wrapper.text()).toContain('modelStatus.loadFailed')
    await wrapper.get('[data-testid="retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(3)
  })

  it('polls every 30 seconds, warns on stale live reports, and stops after unmounting', async () => {
    getModelStatus.mockResolvedValue({ ...report(), snapshot_at: undefined })
    const wrapper = render()
    await flushPromises()
    await vi.advanceTimersByTimeAsync(120000)
    expect(getModelStatus).toHaveBeenCalledTimes(5)
    expect(wrapper.text()).toContain('modelStatus.staleData')
    wrapper.unmount()
    await vi.advanceTimersByTimeAsync(30000)
    expect(getModelStatus).toHaveBeenCalledTimes(5)
  })

  it('renders an empty catalog separately from a filter with no matches', async () => {
    getModelStatus.mockResolvedValueOnce({ ...report(), groups: [] })
    const wrapper = render()
    await flushPromises()
    expect(wrapper.text()).toContain('modelStatus.noModels')
    expect(wrapper.text()).not.toContain('modelStatus.noMatches')
  })

  it('renders all 20 time buckets across the five-hour window', async () => {
    const data = report()
    const model = data.groups[0].models[0]
    model.metrics = { ...metrics, total: 30, success: 30, empty: 0, success_rate: 100 }
    model.buckets = Array.from({ length: 20 }, (_, index) => ({
      start_at: new Date(Date.UTC(2026, 8, 5, 1, index * 15)).toISOString(),
      end_at: new Date(Date.UTC(2026, 8, 5, 1, index * 15 + 15)).toISOString(),
      total: 1, success: 1, failure: 0, empty: 0, unknown: 0,
      requests: [{ at: new Date(Date.UTC(2026, 8, 5, 1, index * 15)).toISOString(), outcome: 'success' as const }],
    }))
    data.groups = [{ ...data.groups[0], models: [model] }]
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.findAll('[data-testid="status-bucket"]')).toHaveLength(20)
    expect(wrapper.find('.recent-placeholder').exists()).toBe(false)
    expect(wrapper.get('.recent-heading').text()).toContain('modelStatus.fifteenMinuteBuckets')
    expect(wrapper.get('.recent-heading').text()).toContain('20/20')
    expect(wrapper.get('.model-rate strong').text()).toBe('100%')
    expect(wrapper.findAll('[data-testid="status-bucket"]')[0].attributes('title')).toContain('9/5')
  })

  it('renders large catalogs in batches and appends the next batch on demand', async () => {
    const data = report()
    const baseModel = data.groups[0].models[0]
    data.groups = [{
      ...data.groups[0],
      models: Array.from({ length: 45 }, (_, index) => ({ ...baseModel, name: `model-${index}` })),
    }]
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(40)
    expect(wrapper.find('.load-more-models').exists()).toBe(true)
    await wrapper.get('.load-more-models').trigger('click')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(45)
    expect(wrapper.find('.load-more-models').exists()).toBe(false)
  })

})
