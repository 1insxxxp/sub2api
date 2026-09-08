import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import ModelStatusView from '../ModelStatusView.vue'
import type { ModelStatusBucket, ModelStatusResponse } from '@/api/modelStatus'

const { getModelStatus, listCustomGroups, authStore, appStore } = vi.hoisted(() => ({
  getModelStatus: vi.fn(),
  listCustomGroups: vi.fn(),
  authStore: { isAuthenticated: false, user: null as { id: number } | null },
  appStore: { fetchPublicSettings: vi.fn().mockResolvedValue({}) },
}))

vi.mock('@/api/modelStatus', () => ({ getModelStatus }))
vi.mock('@/api/customGroups', () => ({ customGroupsAPI: { list: listCustomGroups } }))
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
    authStore.user = null
    localStorage.removeItem('model-status-group-filter')
    sessionStorage.removeItem('model-status-bucket-hint-seen')
    getModelStatus.mockReset().mockResolvedValue(report())
    listCustomGroups.mockReset().mockResolvedValue([])
  })

  afterEach(() => {
    wrappers.splice(0).forEach(wrapper => wrapper.unmount())
    window.history.replaceState({}, '', '/model-status')
    document.documentElement.classList.remove('model-status-mobile-header-hidden')
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 1024 })
    vi.useRealTimers()
  })

  it('uses the latest 15-minute bucket for the continuous signal light', async () => {
    const data = report()
    const baseModel = data.groups[0].models[0]
    data.groups = [{
      ...data.groups[0],
      models: [
        { ...baseModel, name: 'mostly-successful', status: 'degraded', metrics: { ...metrics, total: 100, success: 95, failure: 5, empty: 0, success_rate: 95 } },
        { ...baseModel, name: 'balanced', status: 'degraded', metrics: { ...metrics, total: 100, success: 50, failure: 0, empty: 50, success_rate: 50 } },
        { ...baseModel, name: 'mostly-failed', status: 'unavailable', metrics: { ...metrics, total: 100, success: 0, failure: 100, empty: 0, success_rate: 0 } },
        { ...baseModel, name: 'no-data', status: 'no_data', metrics: { ...metrics, total: 0, success: 0, failure: 0, empty: 0, unknown: 0, success_rate: null } },
      ].map(model => ({
        ...model,
        status: 'healthy' as const,
        metrics: { ...metrics, total: 100, success: 100, failure: 0, empty: 0, success_rate: 100 },
        buckets: baseModel.buckets!.map((bucket, index) => index === 19 ? { ...bucket, ...model.metrics } : bucket),
      })),
    }]
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    const lights = wrapper.findAll('.health-light')
    expect(lights[0].attributes('style')).toContain('--health-hue: 114deg')
    expect(lights[1].attributes('style')).toContain('--health-hue: 60deg')
    expect(lights[2].attributes('style')).toContain('--health-hue: 0deg')
    expect(lights[3].attributes('style')).toBeUndefined()
    expect(wrapper.findAll('.health-badge')[0].classes()).toContain('badge-warning')
    expect(wrapper.findAll('.health-badge')[2].classes()).toContain('badge-danger')
  })

  it.each([
    { success: 0, failure: 0, empty: 0, unknown: 0, status: 'no_data', rate: '-', hue: null },
    { success: 0, failure: 0, empty: 0, unknown: 5, status: 'unknown', rate: '-', hue: null },
    { success: 4, failure: 0, empty: 0, unknown: 20, status: 'insufficient_data', rate: '100%', hue: 120 },
    { success: 5, failure: 0, empty: 0, unknown: 0, status: 'healthy', rate: '100%', hue: 120 },
    { success: 99, failure: 1, empty: 0, unknown: 0, status: 'healthy', rate: '99%', hue: 119 },
    { success: 98, failure: 2, empty: 0, unknown: 0, status: 'degraded', rate: '98%', hue: 118 },
    { success: 4, failure: 1, empty: 0, unknown: 10, status: 'degraded', rate: '80%', hue: 96 },
    { success: 79, failure: 21, empty: 0, unknown: 0, status: 'unavailable', rate: '79%', hue: 95 },
    { success: 0, failure: 5, empty: 0, unknown: 0, status: 'unavailable', rate: '0%', hue: 0 },
    { success: 0, failure: 0, empty: 5, unknown: 0, status: 'unavailable', rate: '0%', hue: 0 },
  ])('derives $status from the latest 15-minute bucket ($success/$failure/$empty/$unknown)', async ({ success, failure, empty, unknown, status, rate, hue }) => {
    const data = report()
    const model = data.groups[0].models[0]
    model.status = status === 'healthy' ? 'unavailable' : 'healthy'
    model.metrics = status === 'healthy'
      ? { ...metrics, total: 100, success: 0, failure: 100, empty: 0, success_rate: 0 }
      : { ...metrics, total: 100, success: 100, failure: 0, empty: 0, success_rate: 100 }
    model.buckets![19] = {
      ...model.buckets![19], total: success + failure + empty + unknown,
      success, failure, empty, unknown,
    }
    data.groups = [{ ...data.groups[0], models: [model] }]
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('.health-badge').text()).toBe(`modelStatus.health.${status}`)
    expect(wrapper.get('.health-light').attributes('data-health')).toBe(status)
    expect(wrapper.get('.model-rate strong').text()).toBe(rate)
    expect(wrapper.get('.health-light').classes().includes('health-light-observed')).toBe(hue !== null)
    if (hue === null) {
      expect(wrapper.get('.health-light').attributes('style')).toBeUndefined()
    } else {
      expect(wrapper.get('.health-light').attributes('style')).toContain(`--health-hue: ${hue}deg`)
    }
    expect(wrapper.findAll('[data-testid="status-bucket"]')).toHaveLength(20)
  })

  it('keeps legacy health metrics when a cached response has no buckets', async () => {
    const data = report()
    const model = data.groups[0].models[0]
    delete model.buckets
    data.groups = [{ ...data.groups[0], models: [model] }]
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('.health-badge').text()).toBe('modelStatus.health.unavailable')
    expect(wrapper.get('.health-light').attributes('style')).toContain('--health-hue: 12deg')
    expect(wrapper.get('.model-rate strong').text()).toBe('10%')
  })

  it('shows a countdown that decrements each second and resets after a manual refresh', async () => {
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('[data-testid="refresh-countdown"]').text()).toContain('30')

    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.get('[data-testid="refresh-countdown"]').text()).toContain('29')

    getModelStatus.mockResolvedValueOnce(report())
    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="refresh-countdown"]').text()).toContain('30')
  })

  it('plays the bucket sweep once after every successful refresh', async () => {
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('.recent-bars').classes()).toContain('bucket-hint-active')
    await vi.advanceTimersByTimeAsync(1400)
    expect(wrapper.get('.recent-bars').classes()).not.toContain('bucket-hint-active')

    getModelStatus.mockResolvedValueOnce(report())
    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('.recent-bars').classes()).toContain('bucket-hint-active')
  })

  it('keeps the full refresh label on mobile and does not render a model search field', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 375 })
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('[data-testid="refresh-countdown"]').text()).toContain('modelStatus.autoRefreshIn')
    expect(wrapper.find('[data-testid="model-search"]').exists()).toBe(false)
  })

  it('restores and persists the selected public group', async () => {
    localStorage.setItem('model-status-group-filter', '2')
    const wrapper = render()
    await flushPromises()

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('2')
    expect(wrapper.findAll('.group-heading h2').map(heading => heading.text())).toEqual(['Public B'])

    await wrapper.get('select').setValue('1')
    expect(localStorage.getItem('model-status-group-filter')).toBe('1')
  })

  it('does not request private custom groups for guests', async () => {
    const wrapper = render()
    await flushPromises()

    expect(listCustomGroups).not.toHaveBeenCalled()
    expect(wrapper.find('option[value="custom:21"]').exists()).toBe(false)
  })

  it('adds active custom-group presets and matches source group and model exactly', async () => {
    authStore.isAuthenticated = true
    authStore.user = { id: 7 }
    listCustomGroups.mockResolvedValueOnce([
      {
        id: 21, user_id: 7, name: '精选模型', status: 'active',
        models: [
          { id: 1, custom_group_id: 21, public_model: 'alias', source_group_id: 1, source_model: 'SAME-MODEL', source_available: true },
          { id: 2, custom_group_id: 21, public_model: 'alias', source_group_id: 2, source_model: 'same-model' },
          { id: 3, custom_group_id: 21, public_model: 'alias', source_group_id: 1, source_model: 'another-model', source_available: false },
        ], created_at: '', updated_at: '',
      },
      {
        id: 22, user_id: 7, name: '已停用', status: 'disabled', models: [], created_at: '', updated_at: '',
      },
    ])
    const wrapper = render()
    await flushPromises()

    expect(wrapper.find('option[value="custom:21"]').text()).toContain('精选模型')
    expect(wrapper.find('option[value="custom:22"]').exists()).toBe(false)
    await wrapper.get('select').setValue('custom:21')
    expect(wrapper.findAll('.group-heading h2').map(heading => heading.text())).toEqual(['Public A', 'Public B'])
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(2)
    expect(wrapper.findAll('.model-title h3').map(title => title.text())).toEqual(['same-model', 'same-model'])
  })

  it('waits for custom groups before restoring a persisted custom preset', async () => {
    authStore.isAuthenticated = true
    authStore.user = { id: 7 }
    localStorage.setItem('model-status-group-filter', 'custom:21')
    let resolveCustomGroups!: (groups: unknown[]) => void
    listCustomGroups.mockReturnValueOnce(new Promise(resolve => { resolveCustomGroups = resolve }))
    const wrapper = render()
    await flushPromises()

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('custom:21')
    resolveCustomGroups([{
      id: 21, user_id: 7, name: '精选模型', status: 'active',
      models: [{ id: 1, custom_group_id: 21, public_model: 'alias', source_group_id: 1, source_model: 'same-model' }],
      created_at: '', updated_at: '',
    }])
    await flushPromises()
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('custom:21')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(1)
  })

  it('falls back to public groups on custom-group failure and recovers on refresh', async () => {
    authStore.isAuthenticated = true
    authStore.user = { id: 7 }
    listCustomGroups.mockRejectedValueOnce(new Error('private endpoint unavailable'))
    const wrapper = render()
    await flushPromises()

    expect(wrapper.find('[data-testid="custom-group-load-warning"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(3)
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('')

    listCustomGroups.mockResolvedValueOnce([{
      id: 21, user_id: 7, name: '精选模型', status: 'active',
      models: [{ id: 1, custom_group_id: 21, public_model: 'alias', source_group_id: 1, source_model: 'same-model' }],
      created_at: '', updated_at: '',
    }])
    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="custom-group-load-warning"]').exists()).toBe(false)
    expect(wrapper.find('option[value="custom:21"]').exists()).toBe(true)
  })

  it('keeps an empty custom preset selected and shows a no-match state', async () => {
    authStore.isAuthenticated = true
    authStore.user = { id: 7 }
    listCustomGroups.mockResolvedValueOnce([{
      id: 21, user_id: 7, name: '暂无匹配', status: 'active',
      models: [{ id: 1, custom_group_id: 21, public_model: 'alias', source_group_id: 99, source_model: 'missing' }],
      created_at: '', updated_at: '',
    }])
    const wrapper = render()
    await flushPromises()
    await wrapper.get('select').setValue('custom:21')

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('custom:21')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(0)
    expect(wrapper.text()).toContain('modelStatus.noCustomGroupMatches')
  })

  it('resynchronizes custom presets on manual and visibility refresh without blocking public data', async () => {
    authStore.isAuthenticated = true
    authStore.user = { id: 7 }
    let resolveCustomGroups!: (groups: unknown[]) => void
    listCustomGroups.mockReturnValueOnce(new Promise(resolve => { resolveCustomGroups = resolve }))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(3)

    resolveCustomGroups([])
    await flushPromises()
    listCustomGroups.mockResolvedValue([])
    await wrapper.get('[data-testid="refresh"]').trigger('click')
    await flushPromises()
    expect(listCustomGroups).toHaveBeenCalledTimes(2)

    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(listCustomGroups).toHaveBeenCalledTimes(3)
    expect(getModelStatus.mock.calls.length).toBeGreaterThanOrEqual(2)
  })

  it('hides both mobile navigation variants while scrolling and provides a back-to-top action', async () => {
    Object.defineProperty(window, 'innerWidth', { configurable: true, value: 375 })
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 0, writable: true })
    const scrollTo = vi.fn()
    const originalMatchMedia = window.matchMedia
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: () => ({ matches: false }) })
    Object.defineProperty(window, 'scrollTo', { configurable: true, value: scrollTo })
    const wrapper = render()
    await flushPromises()

    expect(wrapper.find('[data-testid="back-to-top"]').exists()).toBe(false)
    Object.defineProperty(window, 'scrollY', { configurable: true, value: 100, writable: true })
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.documentElement.classList.contains('model-status-mobile-header-hidden')).toBe(true)

    Object.defineProperty(window, 'scrollY', { configurable: true, value: 400, writable: true })
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    const backToTop = wrapper.get('[data-testid="back-to-top"]')
    expect(backToTop.exists()).toBe(true)
    expect(wrapper.get('.status-filters').classes()).toContain('status-filters')
    await backToTop.trigger('click')
    expect(scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'smooth' })

    Object.defineProperty(window, 'scrollY', { configurable: true, value: 396, writable: true })
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.documentElement.classList.contains('model-status-mobile-header-hidden')).toBe(true)

    Object.defineProperty(window, 'scrollY', { configurable: true, value: 300, writable: true })
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.documentElement.classList.contains('model-status-mobile-header-hidden')).toBe(true)

    Object.defineProperty(window, 'scrollY', { configurable: true, value: 1, writable: true })
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.documentElement.classList.contains('model-status-mobile-header-hidden')).toBe(true)

    Object.defineProperty(window, 'scrollY', { configurable: true, value: 0, writable: true })
    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.documentElement.classList.contains('model-status-mobile-header-hidden')).toBe(false)

    wrapper.unmount()
    expect(document.documentElement.classList.contains('model-status-mobile-header-hidden')).toBe(false)
    Object.defineProperty(window, 'matchMedia', { configurable: true, value: originalMatchMedia })
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
    expect(wrapper.find('.bucket-help').exists()).toBe(false)
    expect(bucket.element.tagName).toBe('BUTTON')
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

  it.each([12, 10, 3, 0])('shows only the latest ten of %i bucket requests without changing totals', async count => {
    const data = report()
    const requests = Array.from({ length: count }, (_, index) => ({
      at: new Date(Date.UTC(2026, 8, 6, 0, index)).toISOString(),
      outcome: 'success' as const,
      status_code: 200 + index,
    }))
    const originalOrder = requests.map(request => request.at)
    data.groups[0].models[0].buckets![0] = {
      ...data.groups[0].models[0].buckets![0],
      total: count, success: count, failure: 0, empty: 0, unknown: 0, requests,
    }
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[data-testid="status-bucket"]').trigger('click')
    await flushPromises()

    const rows = [...document.body.querySelectorAll('.bucket-request-row')]
    expect(rows).toHaveLength(Math.min(count, 10))
    expect(rows.map(row => row.querySelector('.bucket-request-status')?.textContent)).toEqual(
      requests.slice(-10).reverse().map(request => `modelStatus.statusCode ${request.status_code}`),
    )
    expect(document.body.querySelector('.bucket-detail-stats strong')?.textContent).toBe(String(count))
    const header = document.body.querySelector('.bucket-detail-list-header')?.textContent
    if (count > 10) expect(header).toContain(`modelStatus.showingLatest 10 ${count}`)
    else expect(header).not.toContain('modelStatus.showingLatest')
    expect(requests.map(request => request.at)).toEqual(originalOrder)
  })

  it('colors recent buckets by their success-rate thresholds', async () => {
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
    expect(bucket.classes()).toContain('bucket-success')
    expect(bucket.attributes('data-outcome')).toBe('success')
    expect(bucket.attributes('title')).toContain('modelStatus.outcome.success')
  })

  it.each([
    { success: 80, failure: 20, expected: 'bucket-degraded' },
    { success: 49, failure: 51, expected: 'bucket-failure' },
  ])('uses $expected at $success% success', async ({ success, failure, expected }) => {
    const data = report()
    data.groups[0].models[0].buckets![0] = {
      ...data.groups[0].models[0].buckets![0],
      total: 100,
      success,
      failure,
      empty: 0,
      unknown: 0,
    }
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.get('[data-testid="status-bucket"]').classes()).toContain(expected)
  })

  it('separates incomplete records from outcomes without rendering a global summary', async () => {
    const data = report()
    const incompleteMetrics = { ...metrics, total: 13, unknown: 3 }
    data.summary = incompleteMetrics
    data.groups[0].metrics = incompleteMetrics
    data.groups[0].models[0].metrics = incompleteMetrics
    data.groups[0].models[0].buckets![0] = { ...data.groups[0].models[0].buckets![0], total: 1, unknown: 1, requests: [{ at: '2026-09-06T03:54:00Z', outcome: 'unknown', status_code: 499 }] }
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.findAll('.model-row .incomplete-note')).toHaveLength(0)
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

  it('shows the request status code in bucket details', async () => {
    const data = report()
    data.groups[0].models[0].buckets![0].requests = [{ at: '2026-09-06T03:54:00Z', outcome: 'failure', status_code: 502 }]
    data.groups[0].models[0].buckets![0].total = 1
    data.groups[0].models[0].buckets![0].failure = 1
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    await wrapper.get('[data-testid="status-bucket"]').trigger('click')
    await flushPromises()
    expect(document.body.textContent).toContain('modelStatus.statusCode 502')
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
    expect(wrapper.find('.model-row .incomplete-note').exists()).toBe(false)
  })

  it('filters by group without merging names or changing model metrics', async () => {
    const data = report()
    data.groups[1].models[0].buckets![19] = { ...data.groups[1].models[0].buckets![19], total: 1, success: 1, requests: [
      { at: '2026-09-06T04:00:00Z', outcome: 'success' },
    ] }
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(3)
    await wrapper.get('select').setValue('2')
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(1)
    expect(wrapper.find('[data-testid="model-row"]').text()).toContain('100%')
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
    model.buckets[19] = { ...model.buckets[19], total: 4, success: 3, failure: 1, requests: [
      { at: '2026-09-05T05:45:00Z', outcome: 'failure' },
      { at: '2026-09-05T05:44:00Z', outcome: 'success' },
      { at: '2026-09-05T05:43:00Z', outcome: 'success' },
      { at: '2026-09-05T05:42:00Z', outcome: 'success' },
    ] }
    data.groups = [{ ...data.groups[0], models: [model] }]
    getModelStatus.mockResolvedValueOnce(data)
    const wrapper = render()
    await flushPromises()

    expect(wrapper.findAll('[data-testid="status-bucket"]')).toHaveLength(20)
    expect(wrapper.find('.recent-placeholder').exists()).toBe(false)
    expect(wrapper.get('.recent-heading').text()).toContain('modelStatus.fifteenMinuteBuckets')
    expect(wrapper.get('.recent-heading').text()).toContain('20/20')
    expect(wrapper.get('.model-rate strong').text()).toBe('75%')
    expect(wrapper.get('.outcome-counts').text()).toContain('3')
    expect(wrapper.get('.outcome-counts').text()).toContain('1')
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

  it('renders the complete catalog and exposes a readiness marker in capture mode', async () => {
    window.history.pushState({}, '', '/model-status?capture=all')
    localStorage.setItem('model-status-group-filter', '2')
    const data = report()
    const baseModel = data.groups[0].models[0]
    data.groups = [{
      ...data.groups[0],
      models: Array.from({ length: 45 }, (_, index) => ({ ...baseModel, name: `capture-model-${index}` })),
    }, data.groups[1]]
    getModelStatus.mockResolvedValueOnce(data)

    const wrapper = render()
    await flushPromises()

    expect(wrapper.find('[data-testid="model-status-ready"]').exists()).toBe(true)
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(46)
    expect(wrapper.findAll('.group-heading h2').map(heading => heading.text())).toEqual(['Public A', 'Public B'])
    expect(wrapper.find('.load-more-models').exists()).toBe(false)
    expect(wrapper.find('[data-testid="public-nav"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="refresh"]').exists()).toBe(false)
  })

  it.each([
    ['public a', ['Public A'], 2],
    ['ANOTHER-MODEL', ['Public A'], 2],
    ['same-model', ['Public A', 'Public B'], 3],
    ['不存在的分组', [], 0],
  ])('filters capture by keyword %s while keeping complete matching groups', async (keyword, names, count) => {
    window.history.pushState({}, '', `/model-status?capture=all&search=${encodeURIComponent(keyword)}`)
    localStorage.setItem('model-status-group-filter', '2')
    const wrapper = render()
    await flushPromises()
    expect(wrapper.findAll('.group-heading h2').map(heading => heading.text())).toEqual(names)
    expect(wrapper.findAll('[data-testid="model-row"]')).toHaveLength(count)
    expect(wrapper.find('[data-testid="model-status-ready"]').exists()).toBe(true)
  })

})
