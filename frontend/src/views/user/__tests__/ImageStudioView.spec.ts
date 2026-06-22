import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import ImageStudioView from '../ImageStudioView.vue'

const { getConfig, getOptions, generate, edit, list, deleteImage, refreshUser, showError, showSuccess } =
  vi.hoisted(() => ({
    getConfig: vi.fn(),
    getOptions: vi.fn(),
    generate: vi.fn(),
    edit: vi.fn(),
    list: vi.fn(),
    deleteImage: vi.fn(),
    refreshUser: vi.fn(),
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }))

vi.mock('@/api/images', () => ({
  imageStudioAPI: {
    getConfig,
    getOptions,
    generate,
    edit,
    list,
    delete: deleteImage,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { balance: 20 },
    refreshUser,
  }),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const config = {
  enabled: true,
  allowed_models: ['gpt-image-1', 'gpt-image-2'],
  default_model: 'gpt-image-1',
  aspect_ratios: [
    { ratio: '1:1', size: '1024x1024', billing_tier: '1K' },
    { ratio: '16:9', size: '1536x864', billing_tier: '2K' },
  ],
  max_reference_image_mb: 20,
  retention_days: 30,
  max_images_per_user: 100,
}

const options = {
  enabled: true,
  default_group_id: 9,
  default_model: 'gpt-image-2',
  groups: [
    {
      id: 9,
      name: 'Image Pro',
      description: 'Pro image pool',
      platform: 'openai',
      models: [
        {
          model: 'gpt-image-2',
          label: 'gpt-image-2',
          capabilities: ['generation', 'edit'],
        },
      ],
      qualities: [
        { quality: '1K', label: '1K', billing_tier: '1K', estimated_cost: 0.03 },
        { quality: '2K', label: '2K', billing_tier: '2K', estimated_cost: 0.07 },
        { quality: '4K', label: '4K', billing_tier: '4K', estimated_cost: 0.16 },
      ],
      prices: [
        {
          ratio: '16:9',
          quality: '4K',
          size: '3840x2160',
          billing_tier: '4K',
          estimated_cost: 0.16,
        },
      ],
    },
  ],
}

describe('ImageStudioView', () => {
  beforeEach(() => {
    getConfig.mockReset()
    getOptions.mockReset()
    generate.mockReset()
    edit.mockReset()
    list.mockReset()
    deleteImage.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getConfig.mockResolvedValue(config)
    getOptions.mockResolvedValue(options)
    list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 12, pages: 0 })
    generate.mockResolvedValue({
      id: 9,
      mode: 'generation',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '1:1',
      size: '1024x1024',
      image_url: 'https://assets.example.com/generated.png',
      cost: 0.1,
      bytes: 100,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })
  })

  it('renders enabled config controls and appends generated images to gallery', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('gpt-image-2')
    expect(wrapper.text()).toContain('Image Pro')
    expect(wrapper.text()).toContain('4K')
    expect(wrapper.text()).toContain('1:1')
    expect(wrapper.text()).toContain('16:9')

    await wrapper.get('[data-testid="image-studio-quality-4K"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-ratio-16:9"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-prompt"]').setValue('blue gateway')
    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await flushPromises()

    expect(generate).toHaveBeenCalledWith({
      group_id: 9,
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '4K',
    })
    expect(wrapper.find('img[src="https://assets.example.com/generated.png"]').exists()).toBe(true)
    expect(refreshUser).toHaveBeenCalled()
  })

  it('shows a disabled state when image studio is off', async () => {
    getConfig.mockResolvedValueOnce({ ...config, enabled: false })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('imageStudio.disabledTitle')
    expect(wrapper.get('[data-testid="image-studio-generate-button"]').attributes('disabled')).toBeDefined()
  })
})
