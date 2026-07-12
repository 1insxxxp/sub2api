import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ImageStudioSettingsPanel from '../ImageStudioSettingsPanel.vue'

const {
  getImageStudioSettings,
  updateImageStudioSettings,
  testImageStudioStorage,
  fetchPublicSettings,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getImageStudioSettings: vi.fn(),
  updateImageStudioSettings: vi.fn(),
  testImageStudioStorage: vi.fn(),
  fetchPublicSettings: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getImageStudioSettings,
      updateImageStudioSettings,
      testImageStudioStorage,
    },
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    fetchPublicSettings,
  }),
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: () => 'api error',
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, string | number>) =>
      key.replace(/\{(\w+)\}/g, (_, token) => String(params?.[token] ?? `{${token}}`)),
  }),
}))

const baseSettings = {
  enabled: true,
  allowed_models: ['gpt-image-1'],
  default_model: 'gpt-image-1',
  storage_driver: 'local',
  local_root_dir: '',
  local_public_base_url: 'http://127.0.0.1:18080/api/v1/user/images/assets',
  r2_public_base_url: '',
  retention_days: 30,
  max_images_per_user: 100,
  max_reference_image_mb: 20,
  aspect_ratios: [
    { ratio: '1:1', size: '1024x1024', billing_tier: '1k' },
    { ratio: '16:9', size: '1536x864', billing_tier: '2k' },
  ],
  storage_status: {
    driver: 'local',
    status: 'ok',
    configured: true,
    message: '',
  },
}

function mountPanel() {
  return mount(ImageStudioSettingsPanel, {
    global: {
      stubs: {
        Icon: true,
      },
    },
  })
}

describe('ImageStudioSettingsPanel', () => {
  beforeEach(() => {
    getImageStudioSettings.mockReset()
    updateImageStudioSettings.mockReset()
    testImageStudioStorage.mockReset()
    fetchPublicSettings.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getImageStudioSettings.mockResolvedValue({ ...baseSettings })
    updateImageStudioSettings.mockImplementation(async (payload) => ({
      ...payload,
      storage_status: {
        driver: payload.storage_driver,
        status: 'ok',
        configured: true,
      },
    }))
    testImageStudioStorage.mockResolvedValue({
      driver: 'r2',
      status: 'ok',
      configured: true,
      message: 'ok',
    })
    fetchPublicSettings.mockResolvedValue(null)
  })

  it('loads the current image studio configuration', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(getImageStudioSettings).toHaveBeenCalledTimes(1)
    expect((wrapper.get('[data-test="image-studio-enabled"]').element as HTMLInputElement).checked).toBe(true)
    expect((wrapper.get('[data-test="image-studio-models"]').element as HTMLTextAreaElement).value).toBe('gpt-image-1')
    expect(wrapper.text()).toContain('1024x1024')
    expect(wrapper.text()).toContain('admin.settings.imageStudio.storageStatus.ok')
  })

  it('shows the real render sizes for each billing quality tier', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('1K')
    expect(text).toContain('2K')
    expect(text).toContain('4K')
    expect(text).toContain('1024x576')
    expect(text).toContain('2048x1152')
    expect(text).toContain('3840x2160')
    expect(text).toContain('admin.settings.imageStudio.renderSizeMatrixHint')
  })

  it('saves normalized models and storage settings', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="image-studio-models"]').setValue('gpt-image-1\nflux-dev\ngpt-image-1')
    await wrapper.get('[data-test="image-studio-default-model"]').setValue('flux-dev')
    await wrapper.get('[data-test="image-studio-storage-driver"]').setValue('r2')
    await wrapper.get('[data-test="image-studio-r2-public-base-url"]').setValue('https://images.example.com/')
    await wrapper.get('[data-test="image-studio-save"]').trigger('click')
    await flushPromises()

    expect(updateImageStudioSettings).toHaveBeenCalledTimes(1)
    const payload = updateImageStudioSettings.mock.calls[0][0]
    expect(payload.allowed_models).toEqual(['gpt-image-1', 'flux-dev'])
    expect(payload.default_model).toBe('flux-dev')
    expect(payload.storage_driver).toBe('r2')
    expect(payload.r2_public_base_url).toBe('https://images.example.com')
    expect(fetchPublicSettings).toHaveBeenCalledWith(true)
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.imageStudio.saveSuccess')
  })

  it('tests the configured storage target', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await wrapper.get('[data-test="image-studio-test-storage"]').trigger('click')
    await flushPromises()

    expect(testImageStudioStorage).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('admin.settings.imageStudio.storageStatus.ok')
    expect(showSuccess).toHaveBeenCalledWith('admin.settings.imageStudio.storageTestSuccess')
  })
})
