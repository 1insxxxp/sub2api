import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ImageStudioView from '../ImageStudioView.vue'

const { getConfig, getOptions, generate, createTask, getTask, listTasks, edit, list, deleteImage, listKeys, refreshUser, showError, showSuccess } =
  vi.hoisted(() => ({
    getConfig: vi.fn(),
    getOptions: vi.fn(),
    generate: vi.fn(),
    createTask: vi.fn(),
    getTask: vi.fn(),
    listTasks: vi.fn(),
    edit: vi.fn(),
    list: vi.fn(),
    deleteImage: vi.fn(),
    listKeys: vi.fn(),
    refreshUser: vi.fn(),
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }))

vi.mock('@/api/images', () => ({
  imageStudioAPI: {
    getConfig,
    getOptions,
    generate,
    createTask,
    getTask,
    listTasks,
    edit,
    list,
    delete: deleteImage,
  },
}))

vi.mock('@/api/keys', () => ({
  keysAPI: {
    list: listKeys,
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

const apiKeys = {
  items: [
    {
      id: 15,
      user_id: 42,
      key: 'sk-local-image-key',
      name: 'Image Key',
      group_id: 9,
      status: 'active',
      ip_whitelist: [],
      ip_blacklist: [],
      last_used_at: null,
      quota: 0,
      quota_used: 0,
      expires_at: null,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
      group: {
        id: 9,
        name: 'Image Pro',
        description: 'Pro image pool',
        platform: 'openai',
        rate_multiplier: 1,
        is_exclusive: false,
        status: 'active',
        subscription_type: 'standard',
        daily_limit_usd: null,
        weekly_limit_usd: null,
        monthly_limit_usd: null,
        allow_image_generation: true,
        image_rate_independent: false,
        image_rate_multiplier: 1,
        image_price_1k: null,
        image_price_2k: null,
        image_price_4k: null,
        claude_code_only: false,
        fallback_group_id: null,
        fallback_group_id_on_invalid_request: null,
        require_oauth_only: false,
        require_privacy_set: false,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      },
      rate_limit_5h: 0,
      rate_limit_1d: 0,
      rate_limit_7d: 0,
      usage_5h: 0,
      usage_1d: 0,
      usage_7d: 0,
      window_5h_start: null,
      window_1d_start: null,
      window_7d_start: null,
      reset_5h_at: null,
      reset_1d_at: null,
      reset_7d_at: null,
    },
  ],
  total: 1,
  page: 1,
  page_size: 100,
  pages: 1,
}

function vueSource(): string {
  return readFileSync('src/views/user/ImageStudioView.vue', 'utf-8')
}

function cssRulesFor(selector: string): string[] {
  const escapedSelector = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const matches = vueSource().matchAll(new RegExp(`${escapedSelector}\\s*\\{([^}]*)\\}`, 'g'))
  return Array.from(matches, (match) => match[1] ?? '')
}

describe('ImageStudioView', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
    vi.useRealTimers()
  })

  beforeEach(() => {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn((file: File) => `blob:${file.name}`),
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: vi.fn(),
    })

    getConfig.mockReset()
    getOptions.mockReset()
    generate.mockReset()
    createTask.mockReset()
    getTask.mockReset()
    listTasks.mockReset()
    edit.mockReset()
    list.mockReset()
    deleteImage.mockReset()
    listKeys.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getConfig.mockResolvedValue(config)
    getOptions.mockResolvedValue(options)
    listKeys.mockResolvedValue(apiKeys)
    list.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 12, pages: 0 })
    listTasks.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 5, pages: 0 })
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
    createTask.mockResolvedValue({
      id: 22,
      mode: 'generation',
      status: 'queued',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '4K',
      size: '3840x2160',
      estimated_cost: 0.16,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })
    getTask.mockResolvedValue({
      id: 22,
      mode: 'generation',
      status: 'succeeded',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '4K',
      size: '3840x2160',
      estimated_cost: 0.16,
      source_image_count: 0,
      image: {
        id: 9,
        mode: 'generation',
        model: 'gpt-image-2',
        prompt: 'blue gateway',
        aspect_ratio: '16:9',
        size: '3840x2160',
        image_url: 'https://assets.example.com/generated.png',
        cost: 0.1,
        bytes: 100,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      },
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })
  })

  it('renders enabled config controls and appends generated images to gallery', async () => {
    vi.useFakeTimers()
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
    expect(wrapper.text()).toContain('Image Key')
    expect(wrapper.text()).toContain('Image Pro')
    expect(wrapper.text()).toContain('4K')
    expect(wrapper.text()).toContain('imageStudio.quality4KRisk')
    expect(wrapper.text()).toContain('1:1')
    expect(wrapper.text()).toContain('16:9')

    await wrapper.get('[data-testid="image-studio-quality-4K"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-ratio-16:9"]').trigger('click')
    expect(wrapper.text()).toContain('imageStudio.quality4KInlineHint')
    await wrapper.get('[data-testid="image-studio-prompt"]').setValue('blue gateway')
    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    expect(createTask).toHaveBeenCalledWith({
      mode: 'generation',
      api_key_id: 15,
      group_id: 9,
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '4K',
    })
    expect(getTask).toHaveBeenCalledWith(22)
    expect(wrapper.find('img[src="https://assets.example.com/generated.png"]').exists()).toBe(true)
    expect(refreshUser).toHaveBeenCalled()
  })

  it('sends output format and background options with each generated image request', async () => {
    vi.useFakeTimers()
    getOptions.mockResolvedValueOnce({
      ...options,
      default_model: 'gpt-image-1',
      groups: [
        {
          ...options.groups[0],
          models: [
            {
              model: 'gpt-image-1',
              label: 'gpt-image-1',
              capabilities: ['generation', 'edit'],
            },
          ],
        },
      ],
    })
    createTask
      .mockResolvedValueOnce({
        id: 31,
        mode: 'generation',
        status: 'queued',
        model: 'gpt-image-1',
        prompt: 'batch neon gateway',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 32,
        mode: 'generation',
        status: 'queued',
        model: 'gpt-image-1',
        prompt: 'batch neon gateway',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
    getTask
      .mockResolvedValueOnce({
        id: 31,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-1',
        prompt: 'batch neon gateway',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: {
          id: 31,
          mode: 'generation',
          model: 'gpt-image-1',
          prompt: 'batch neon gateway',
          aspect_ratio: '16:9',
          size: '2048x1152',
          image_url: 'https://assets.example.com/batch-1.webp',
          cost: 0.07,
          bytes: 100,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:00Z',
          updated_at: '2026-06-22T00:00:00Z',
        },
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 32,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-1',
        prompt: 'batch neon gateway',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: {
          id: 32,
          mode: 'generation',
          model: 'gpt-image-1',
          prompt: 'batch neon gateway',
          aspect_ratio: '16:9',
          size: '2048x1152',
          image_url: 'https://assets.example.com/batch-2.webp',
          cost: 0.07,
          bytes: 100,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:00Z',
          updated_at: '2026-06-22T00:00:00Z',
        },
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="image-studio-quality-2K"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-ratio-16:9"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-output-count-2"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-output-format-webp"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-output-background-transparent"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-prompt"]').setValue('batch neon gateway')
    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await vi.advanceTimersByTimeAsync(2600)
    await flushPromises()

    expect(createTask).toHaveBeenCalledTimes(2)
    expect(createTask).toHaveBeenNthCalledWith(1, {
      mode: 'generation',
      api_key_id: 15,
      group_id: 9,
      model: 'gpt-image-1',
      prompt: 'batch neon gateway',
      aspect_ratio: '16:9',
      quality: '2K',
      output_format: 'webp',
      background: 'transparent',
    })
    expect(createTask).toHaveBeenNthCalledWith(2, {
      mode: 'generation',
      api_key_id: 15,
      group_id: 9,
      model: 'gpt-image-1',
      prompt: 'batch neon gateway',
      aspect_ratio: '16:9',
      quality: '2K',
      output_format: 'webp',
      background: 'transparent',
    })
    expect(wrapper.find('img[src="https://assets.example.com/batch-1.webp"]').exists()).toBe(true)
    expect(wrapper.find('img[src="https://assets.example.com/batch-2.webp"]').exists()).toBe(true)
  })

  it('disables transparent background when jpeg output is selected', async () => {
    getOptions.mockResolvedValueOnce({
      ...options,
      default_model: 'gpt-image-1',
      groups: [
        {
          ...options.groups[0],
          models: [
            {
              model: 'gpt-image-1',
              label: 'gpt-image-1',
              capabilities: ['generation', 'edit'],
            },
          ],
        },
      ],
    })
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="image-studio-output-background-transparent"]').trigger('click')
    expect(wrapper.get('[data-testid="image-studio-output-background-transparent"]').classes()).toContain('active')

    await wrapper.get('[data-testid="image-studio-output-format-jpeg"]').trigger('click')
    const transparentButton = wrapper.get('[data-testid="image-studio-output-background-transparent"]')

    expect(transparentButton.attributes('disabled')).toBeDefined()
    expect(transparentButton.classes()).not.toContain('active')
    expect(wrapper.get('[data-testid="image-studio-output-background-auto"]').classes()).toContain('active')
  })

  it('disables transparent background for gpt-image-2', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const transparentButton = wrapper.get('[data-testid="image-studio-output-background-transparent"]')
    expect(transparentButton.attributes('disabled')).toBeDefined()
    expect(transparentButton.classes()).not.toContain('active')
    expect(wrapper.get('[data-testid="image-studio-output-background-auto"]').classes()).toContain('active')
  })

  it('shows an inline recovery panel and retries failed generation at 1K', async () => {
    vi.useFakeTimers()
    getTask.mockResolvedValueOnce({
      id: 22,
      mode: 'generation',
      status: 'failed',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '4K',
      size: '3840x2160',
      estimated_cost: 0.16,
      source_image_count: 0,
      error_reason: 'IMAGE_PROVIDER_TIMEOUT_OR_DISCONNECT',
      error_message: 'image provider timed out or disconnected before returning an image; this request was not charged',
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })
    createTask.mockResolvedValueOnce({
      id: 23,
      mode: 'generation',
      status: 'queued',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '1K',
      size: '1024x576',
      estimated_cost: 0.03,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })
    getTask.mockResolvedValueOnce({
      id: 23,
      mode: 'generation',
      status: 'succeeded',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '1K',
      size: '1024x576',
      estimated_cost: 0.03,
      source_image_count: 0,
      image: {
        id: 11,
        user_id: 42,
        mode: 'generation',
        model: 'gpt-image-2',
        prompt: 'blue gateway',
        aspect_ratio: '16:9',
        size: '1024x576',
        image_url: 'https://assets.example.com/retry.png',
        storage_driver: 'local',
        storage_object_key: 'images/user-42/retry.png',
        mime_type: 'image/png',
        cost: 0.03,
        bytes: 100,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      },
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="image-studio-quality-4K"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-ratio-16:9"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-prompt"]').setValue('blue gateway')
    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    expect(wrapper.get('[data-testid="image-studio-failure-panel"]').text()).toContain('imageStudio.failureTitle')
    expect(wrapper.text()).toContain('imageStudio.failureNotCharged')
    expect(wrapper.find('[data-testid="image-studio-retry-button"]').exists()).toBe(true)

    await wrapper.get('[data-testid="image-studio-retry-1k-button"]').trigger('click')
    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    expect(createTask).toHaveBeenLastCalledWith({
      mode: 'generation',
      api_key_id: 15,
      group_id: 9,
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      quality: '1K',
    })
    expect(wrapper.find('img[src="https://assets.example.com/retry.png"]').exists()).toBe(true)
  })

  it('resumes the latest unfinished generation task after a page reload', async () => {
    vi.useFakeTimers()
    listTasks.mockResolvedValueOnce({
      items: [
        {
          id: 31,
          mode: 'generation',
          status: 'running',
          api_key_id: 15,
          group_id: 9,
          model: 'gpt-image-2',
          prompt: 'restored neon station',
          aspect_ratio: '16:9',
          quality: '2K',
          size: '2048x1152',
          estimated_cost: 0.07,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:00Z',
          updated_at: '2026-06-22T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 5,
      pages: 1,
    })
    getTask.mockResolvedValueOnce({
      id: 31,
      mode: 'generation',
      status: 'succeeded',
      model: 'gpt-image-2',
      prompt: 'restored neon station',
      aspect_ratio: '16:9',
      quality: '2K',
      size: '2048x1152',
      estimated_cost: 0.07,
      source_image_count: 0,
      image: {
        id: 13,
        mode: 'generation',
        model: 'gpt-image-2',
        prompt: 'restored neon station',
        aspect_ratio: '16:9',
        size: '2048x1152',
        image_url: 'https://assets.example.com/restored.png',
        cost: 0.07,
        bytes: 100,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      },
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(listTasks).toHaveBeenCalledWith({ page: 1, page_size: 5 })
    expect(wrapper.text()).toContain('imageStudio.taskRunningHint')

    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    expect(getTask).toHaveBeenCalledWith(31)
    expect(wrapper.find('img[src="https://assets.example.com/restored.png"]').exists()).toBe(true)
    expect(refreshUser).toHaveBeenCalled()
  })

  it('uses a compact adaptive layout without hero or balance summary cards', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.image-studio-hero').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-status-strip"]').exists()).toBe(false)
    expect(wrapper.find('.image-studio-shell').classes()).not.toContain('max-w-7xl')
  })

  it('organizes generation settings in a streamlined command surface', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('.image-studio-command-surface').exists()).toBe(true)
    expect(wrapper.find('.image-studio-prompt-section').exists()).toBe(true)
    expect(wrapper.find('.image-studio-foundation-row').exists()).toBe(true)
    expect(wrapper.find('.image-studio-output-row').exists()).toBe(true)
    expect(wrapper.find('.image-studio-control-dock').exists()).toBe(true)
    expect(wrapper.find('.image-studio-action-bar').exists()).toBe(true)
    expect(wrapper.find('.image-studio-stage-panel').exists()).toBe(true)
    expect(wrapper.find('.image-studio-gallery-rail').exists()).toBe(true)
    expect(wrapper.find('.image-studio-settings-panel').exists()).toBe(false)
    expect(wrapper.find('.image-studio-choice-picker').exists()).toBe(true)
    expect(wrapper.find('.image-studio-quality-picker').exists()).toBe(true)
    expect(wrapper.find('.image-studio-ratio-picker').exists()).toBe(true)
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-workbench"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-control-console"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-canvas-stage"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-canvas-stage"] .image-studio-gallery-rail').exists()).toBe(true)

    const apiKeySelect = wrapper.get('[data-testid="image-studio-api-key-select"]')
    expect(apiKeySelect.text()).toContain('Image Key')

    const modelSelect = wrapper.get('[data-testid="image-studio-model-select"]')
    expect(modelSelect.classes()).toContain('image-studio-select-trigger')

    await modelSelect.trigger('click')

    expect(wrapper.find('[data-testid="image-studio-model-menu"]').exists()).toBe(true)
    expect(wrapper.find('.image-studio-select-option.is-selected').text()).toContain('gpt-image-2')
  })

  it('renumbers command steps by mode without skipping the reference step', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const generationSteps = wrapper.findAll('.image-studio-step-heading').map((heading) => ({
      number: heading.find('span').text(),
      label: heading.find('strong').text(),
    }))

    expect(generationSteps).toEqual([
      { number: '00', label: 'imageStudio.stepPrompt' },
      { number: '01', label: 'imageStudio.stepOutput' },
      { number: '02', label: 'imageStudio.stepConnection' },
    ])

    await wrapper.findAll('.image-studio-mode-switch button')[1].trigger('click')

    const editSteps = wrapper.findAll('.image-studio-step-heading').map((heading) => ({
      number: heading.find('span').text(),
      label: heading.find('strong').text(),
    }))

    expect(editSteps).toEqual([
      { number: '00', label: 'imageStudio.stepPrompt' },
      { number: '01', label: 'imageStudio.stepReference' },
      { number: '02', label: 'imageStudio.stepOutput' },
      { number: '03', label: 'imageStudio.stepConnection' },
    ])
  })

  it('lets select menus open upward when the lower viewport is cramped', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const modelSelect = wrapper.get('[data-testid="image-studio-model-select"]').element as HTMLElement
    vi.spyOn(modelSelect, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 680,
      width: 360,
      height: 44,
      top: 680,
      right: 360,
      bottom: 724,
      left: 0,
      toJSON: () => ({}),
    } as DOMRect)
    vi.stubGlobal('innerHeight', 760)

    await wrapper.get('[data-testid="image-studio-model-select"]').trigger('click')

    expect(wrapper.get('[data-testid="image-studio-model-select-root"]').classes()).toContain('is-drop-up')
    const dropUpRule = cssRulesFor('.image-studio-select.is-drop-up .image-studio-select-menu')[0] ?? ''
    expect(dropUpRule).toMatch(/top:\s*auto/)
    expect(dropUpRule).toMatch(/bottom:\s*calc\(100%\s*\+\s*0\.45rem\)/)
  })

  it('uses a narrow secondary left rail with a fixed bottom generate card', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const shellRule = cssRulesFor('.image-studio-shell').find((rule) => rule.includes('--studio-viewport-height')) ?? ''
    const workbenchRule = cssRulesFor('.image-studio-workbench').find((rule) => rule.includes('height: 100%')) ?? ''
    const workspaceRule = cssRulesFor('.image-studio-workspace').find((rule) => rule.includes('grid-template-rows')) ?? ''
    const commandSurfaceRule = cssRulesFor('.image-studio-command-surface').find((rule) => rule.includes('overflow-y')) ?? ''
    const canvasRule = cssRulesFor('.image-studio-canvas-stage').find((rule) => rule.includes('grid-template-rows')) ?? ''
    const previewRule = cssRulesFor('.image-studio-preview-panel').find((rule) => rule.includes('grid-template-rows')) ?? ''
    const previewOpenRule =
      cssRulesFor('.image-studio-stage-panel .image-studio-preview-open').find((rule) => rule.includes('height: 100%')) ?? ''
    const currentActionsRule = cssRulesFor('.image-studio-current-actions').find((rule) => rule.includes('margin-top')) ?? ''
    const stagePreviewImageRule = cssRulesFor('.image-studio-stage-panel .image-studio-preview-open img')[0] ?? ''
    const stagePreviewHoverRule =
      vueSource().match(
        /\.image-studio-stage-panel \.image-studio-preview-open:hover img,\s*\.image-studio-stage-panel \.image-studio-preview-open:focus-visible img\s*\{([^}]*)\}/,
      )?.[1] ?? ''
    const galleryRule = cssRulesFor('.image-studio-gallery').find((rule) => rule.includes('max-height')) ?? ''
    const galleryGridRule =
      vueSource().match(/\.image-studio-gallery-grid,\s*\.image-studio-gallery-loading\s*\{([^}]*)\}/)?.[1] ?? ''

    expect(shellRule).toMatch(/height:\s*var\(--studio-viewport-height\)/)
    expect(shellRule).toMatch(/overflow:\s*hidden/)
    expect(workbenchRule).toMatch(/align-items:\s*stretch/)
    expect(workbenchRule).toMatch(/grid-template-columns:\s*minmax\(18\.5rem,\s*0\.56fr\) minmax\(38rem,\s*1\.44fr\)/)
    expect(workspaceRule).toMatch(/height:\s*100%/)
    expect(workspaceRule).toMatch(/grid-template-rows:\s*auto minmax\(0,\s*1fr\) auto/)
    expect(commandSurfaceRule).toMatch(/height:\s*100%/)
    expect(commandSurfaceRule).toMatch(/overflow-y:\s*auto/)
    expect(canvasRule).toMatch(/height:\s*100%/)
    expect(canvasRule).toMatch(/grid-template-rows:\s*minmax\(0,\s*1fr\) auto/)
    expect(previewRule).toMatch(/height:\s*100%/)
    expect(previewRule).toMatch(/grid-template-rows:\s*auto minmax\(0,\s*1fr\) auto/)
    expect(previewOpenRule).toMatch(/height:\s*100%/)
    expect(previewOpenRule).toMatch(/width:\s*100%/)
    expect(previewOpenRule).toMatch(/max-height:\s*100%/)
    expect(previewOpenRule).toMatch(/max-width:\s*100%/)
    expect(previewOpenRule).toMatch(/display:\s*flex/)
    expect(previewOpenRule).toMatch(/align-items:\s*center/)
    expect(previewOpenRule).toMatch(/justify-content:\s*center/)
    expect(currentActionsRule).toMatch(/margin-top:\s*clamp\(1\.35rem,\s*2vh,\s*1\.8rem\)/)
    expect(stagePreviewImageRule).toMatch(/height:\s*auto/)
    expect(stagePreviewImageRule).toMatch(/width:\s*auto/)
    expect(stagePreviewImageRule).toMatch(/max-height:\s*min\(100%,\s*clamp\(20rem,\s*54dvh,\s*36rem\)\)/)
    expect(stagePreviewImageRule).toMatch(/max-width:\s*min\(100%,\s*clamp\(26rem,\s*56vw,\s*56rem\)\)/)
    expect(stagePreviewImageRule).toMatch(/object-fit:\s*contain/)
    expect(stagePreviewHoverRule).toMatch(/transform:\s*none/)
    expect(galleryRule).toMatch(/max-height:\s*clamp\(11rem,\s*22dvh,\s*15rem\)/)
    expect(galleryGridRule).toMatch(/grid-auto-columns:\s*minmax\(11rem,\s*13rem\)/)
    expect(wrapper.get('.image-studio-workspace > .image-studio-action-bar').exists()).toBe(true)
    expect(wrapper.find('.image-studio-command-surface .image-studio-action-bar').exists()).toBe(false)
  })

  it('keeps the cost and generate action panel in normal flow with an opaque surface', () => {
    const actionBarRule = cssRulesFor('.image-studio-action-bar').find((rule) => rule.includes('display: grid')) ?? ''
    const darkActionBarRule = cssRulesFor('.dark .image-studio-action-bar').find((rule) => rule.includes('background')) ?? ''

    expect(actionBarRule).not.toMatch(/position:\s*sticky/)
    expect(actionBarRule).not.toMatch(/bottom:\s*0/)
    expect(actionBarRule).toMatch(/box-shadow:/)
    expect(actionBarRule).toMatch(/rgba\(255,\s*255,\s*255,\s*0\.9[5-9]\)/)
    expect(darkActionBarRule).toMatch(/rgba\(15,\s*23,\s*42,\s*0\.9[5-9]\)/)
  })

  it('attaches the current generated image when continuing edit mode', async () => {
    const generatedImage = {
      id: 9,
      user_id: 42,
      mode: 'generation',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '1:1',
      size: '1024x1024',
      image_url: '/api/v1/user/images/files/images/user-42/2026/06/generated.png',
      storage_driver: 'local',
      storage_object_key: 'images/user-42/2026/06/generated.png',
      mime_type: 'image/png',
      cost: 0.1,
      bytes: 100,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    }
    const fetchMock = vi.fn(async () => ({
      ok: true,
      blob: async () => new Blob(['png-bytes'], { type: 'image/png' }),
    }))
    vi.stubGlobal('fetch', fetchMock)
    list.mockResolvedValueOnce({ items: [generatedImage], total: 1, page: 1, page_size: 12, pages: 1 })
    edit.mockResolvedValue({ ...generatedImage, id: 10, mode: 'edit', source_image_count: 1 })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const continueButton = wrapper.findAll('button').find((button) => button.text().includes('imageStudio.useAsReference'))
    expect(continueButton).toBeTruthy()
    await continueButton!.trigger('click')
    await flushPromises()

    expect(fetchMock).toHaveBeenCalledWith(generatedImage.image_url, { credentials: 'same-origin' })
    expect(wrapper.text()).not.toContain('passion-api-image-9.png')
    const referencePreview = wrapper.get('[data-testid="image-studio-reference-preview"]')
    expect(referencePreview.attributes('src')).toBe('blob:passion-api-image-9.png')
    expect(wrapper.get('.image-studio-reference-item').text()).toContain('9 Bytes')

    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await flushPromises()

    expect(edit).toHaveBeenCalledWith(
      expect.objectContaining({
        api_key_id: 15,
        group_id: 9,
        model: 'gpt-image-2',
        prompt: 'blue gateway',
        aspect_ratio: '1:1',
        quality: '1K',
        images: [expect.objectContaining({ name: 'passion-api-image-9.png', type: 'image/png' })],
      }),
    )
  })

  it('renders uploaded reference images as compact thumbnails with bottom breathing room', () => {
    const referenceListRule = cssRulesFor('.image-studio-reference-list')[0] ?? ''
    const referenceItemRule = cssRulesFor('.image-studio-reference-item')[0] ?? ''
    const referenceThumbRule = cssRulesFor('.image-studio-reference-thumb')[0] ?? ''

    expect(referenceListRule).toMatch(/margin-bottom:\s*1rem/)
    expect(referenceItemRule).toMatch(/grid-template-columns:\s*auto minmax\(0,\s*1fr\) auto/)
    expect(referenceThumbRule).toMatch(/width:\s*3\.25rem/)
    expect(referenceThumbRule).toMatch(/height:\s*3\.25rem/)
    expect(referenceThumbRule).toMatch(/object-fit:\s*cover/)
  })

  it('opens an image preview dialog from the latest result and closes it with escape', async () => {
    const generatedImage = {
      id: 9,
      user_id: 42,
      mode: 'generation',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      size: '1536x864',
      image_url: 'https://assets.example.com/generated.png',
      storage_driver: 'r2',
      storage_object_key: 'images/user-42/2026/06/generated.png',
      mime_type: 'image/png',
      cost: 0.16,
      bytes: 100,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    }
    list.mockResolvedValueOnce({ items: [generatedImage], total: 1, page: 1, page_size: 12, pages: 1 })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="image-studio-current-preview"]').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-testid="image-studio-image-preview-dialog"]')
    expect(dialog.attributes('role')).toBe('dialog')
    expect(dialog.text()).toContain('gpt-image-2')
    expect(dialog.text()).toContain('16:9')
    expect(dialog.find('img').attributes('src')).toBe(generatedImage.image_url)
    expect(wrapper.find('[data-testid="image-studio-preview-copy"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-preview-download"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-preview-reference"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-preview-delete"]').exists()).toBe(true)
    expect(wrapper.find('.image-studio-gallery-rail .image-studio-image-card-actions').exists()).toBe(false)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()

    expect(wrapper.find('[data-testid="image-studio-image-preview-dialog"]').exists()).toBe(false)
  })

  it('uses a themed confirmation dialog before deleting a generated image', async () => {
    const generatedImage = {
      id: 9,
      user_id: 42,
      mode: 'generation',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '16:9',
      size: '1536x864',
      image_url: 'https://assets.example.com/generated.png',
      storage_driver: 'r2',
      storage_object_key: 'images/user-42/2026/06/generated.png',
      mime_type: 'image/png',
      cost: 0.16,
      bytes: 100,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    }
    list.mockResolvedValueOnce({ items: [generatedImage], total: 1, page: 1, page_size: 12, pages: 1 })
    deleteImage.mockResolvedValue(undefined)

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    await wrapper.get('.image-studio-gallery-rail .image-studio-image-thumb').trigger('click')
    await flushPromises()
    await wrapper.get('[data-testid="image-studio-preview-delete"]').trigger('click')
    await flushPromises()

    expect(deleteImage).not.toHaveBeenCalled()
    const dialog = wrapper.get('[data-testid="image-studio-delete-dialog"]')
    expect(dialog.attributes('role')).toBe('dialog')
    expect(dialog.text()).toContain('imageStudio.deleteTitle')
    expect(dialog.text()).toContain('blue gateway')

    await wrapper.get('[data-testid="image-studio-delete-confirm"]').trigger('click')
    await flushPromises()

    expect(deleteImage).toHaveBeenCalledWith(9)
    expect(wrapper.find('[data-testid="image-studio-delete-dialog"]').exists()).toBe(false)
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

  it('requires an active image-enabled API key before generating', async () => {
    listKeys.mockResolvedValueOnce({ ...apiKeys, items: [], total: 0, pages: 0 })

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('imageStudio.noApiKeysTitle')
    expect(wrapper.get('[data-testid="image-studio-generate-button"]').attributes('disabled')).toBeDefined()
  })
})
