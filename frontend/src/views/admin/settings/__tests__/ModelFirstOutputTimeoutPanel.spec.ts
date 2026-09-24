import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import ModelFirstOutputTimeoutPanel from '../ModelFirstOutputTimeoutPanel.vue'

const {
  getModelFirstOutputTimeoutSettings,
  updateModelFirstOutputTimeoutSettings,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getModelFirstOutputTimeoutSettings: vi.fn(),
  updateModelFirstOutputTimeoutSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getModelFirstOutputTimeoutSettings,
      updateModelFirstOutputTimeoutSettings,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showError, showSuccess }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback,
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const policy = (overrides = {}) => ({
  enabled: true,
  target_seconds: 30,
  switch_seconds: 120,
  hard_cap_seconds: 300,
  ...overrides,
})

const settings = () => ({
  enabled: true,
  default: policy(),
  profiles: {
    gemini_flash: policy(),
    gemini_pro: policy({ target_seconds: 45 }),
    gemini_thinking: policy({ hard_cap_seconds: 600 }),
  },
  platforms: { gemini: policy({ switch_seconds: 180 }) },
  models: {},
})

function mountPanel() {
  return mount(ModelFirstOutputTimeoutPanel, {
    global: { stubs: { Icon: true } },
  })
}

describe('ModelFirstOutputTimeoutPanel', () => {
  beforeEach(() => {
    getModelFirstOutputTimeoutSettings.mockReset()
    updateModelFirstOutputTimeoutSettings.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    getModelFirstOutputTimeoutSettings.mockResolvedValue(settings())
    updateModelFirstOutputTimeoutSettings.mockImplementation(async (payload) => payload)
  })

  it('loads and saves global, profile, platform and model policies', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(getModelFirstOutputTimeoutSettings).toHaveBeenCalledTimes(1)
    expect((wrapper.get('[data-test="model-first-output-timeout-default-target_seconds"]').element as HTMLInputElement).value).toBe('30')

    await wrapper.get('[data-test="model-first-output-timeout-model-new-key"]').setValue('gemini-2.5-flash')
    await wrapper.get('[data-test="model-first-output-timeout-model-add"]').trigger('click')
    await wrapper.get('[data-test="model-first-output-timeout-save"]').trigger('click')
    await flushPromises()

    expect(updateModelFirstOutputTimeoutSettings).toHaveBeenCalledTimes(1)
    const payload = updateModelFirstOutputTimeoutSettings.mock.calls[0][0]
    expect(payload.profiles.gemini_pro.target_seconds).toBe(45)
    expect(payload.platforms.gemini.switch_seconds).toBe(180)
    expect(payload.models['gemini-2.5-flash']).toBeTruthy()
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.modelFirstOutputTimeout.saved')
  })

  it('validates ordered positive timeout values before saving', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="model-first-output-timeout-default-switch_seconds"]').setValue('10')
    await wrapper.get('[data-test="model-first-output-timeout-save"]').trigger('click')
    await flushPromises()

    expect(updateModelFirstOutputTimeoutSettings).not.toHaveBeenCalled()
    expect(wrapper.get('[data-test="model-first-output-timeout-error"]').text()).toContain('validationOrder')
  })

  it('removes model overrides and shows load/save errors', async () => {
    const loaded = settings()
    loaded.models = { 'gemini-2.5-flash': policy() }
    getModelFirstOutputTimeoutSettings.mockResolvedValueOnce(loaded)
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="model-first-output-timeout-model-remove-gemini-2.5-flash"]').trigger('click')
    updateModelFirstOutputTimeoutSettings.mockRejectedValueOnce(new Error('save failed'))
    await wrapper.get('[data-test="model-first-output-timeout-save"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="model-first-output-timeout-error"]').text()).toContain('saveFailed')

    getModelFirstOutputTimeoutSettings.mockRejectedValueOnce(new Error('load failed'))
    const failedWrapper = mountPanel()
    await flushPromises()
    expect(failedWrapper.get('[data-test="model-first-output-timeout-load-error"]').text()).toContain('loadFailed')
    expect((failedWrapper.get('[data-test="model-first-output-timeout-enabled"]').element as HTMLInputElement).checked).toBe(false)
    expect(showError).toHaveBeenCalled()
  })
})
