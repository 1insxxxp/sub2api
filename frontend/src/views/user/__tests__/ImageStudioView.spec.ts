import { flushPromises, mount } from '@vue/test-utils'
import { readFileSync } from 'node:fs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ImageStudioView from '../ImageStudioView.vue'

const { getConfig, getOptions, generate, createTask, getTask, listTasks, edit, list, deleteImage, downloadImageFile, listKeys, refreshUser, showError, showSuccess } =
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
    downloadImageFile: vi.fn(),
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
    download: downloadImageFile,
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
    window.sessionStorage.clear()
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
    downloadImageFile.mockReset()
    downloadImageFile.mockResolvedValue({
      blob: new Blob(['png-bytes'], { type: 'image/png' }),
      filename: 'passion-api-image-9.png',
    })
    listKeys.mockReset()
    refreshUser.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    window.sessionStorage.clear()

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
    expect(wrapper.find('[data-testid="image-studio-generating-overlay"]').exists()).toBe(true)
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

  it('downloads generated images through the same-origin download endpoint', async () => {
    vi.useFakeTimers()
    const click = vi.fn()
    const appendChild = vi.spyOn(document.body, 'appendChild')
    const createElement = vi.spyOn(document, 'createElement')
    vi.mocked(URL.createObjectURL).mockReturnValueOnce('blob:downloaded-image')
    createElement.mockImplementation(((tagName: string) => {
      const element = document.createElementNS('http://www.w3.org/1999/xhtml', tagName) as HTMLElement
      if (tagName.toLowerCase() === 'a') {
        Object.defineProperty(element, 'click', { configurable: true, value: click })
      }
      return element
    }) as typeof document.createElement)

    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-testid="image-studio-prompt"]').setValue('blue gateway')
    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    await wrapper.get('.image-studio-current-actions button:nth-child(2)').trigger('click')
    await flushPromises()

    expect(downloadImageFile).toHaveBeenCalledWith(9)
    expect(click).toHaveBeenCalled()
    const anchor = appendChild.mock.calls
      .map(([node]) => node)
      .find((node): node is HTMLAnchorElement => node instanceof HTMLAnchorElement)
    expect(anchor?.href).toBe('blob:downloaded-image')
    expect(anchor?.download).toBe('passion-api-image-9.png')
    expect(anchor?.href).not.toBe('https://assets.example.com/generated.png')

    createElement.mockRestore()
    appendChild.mockRestore()
  })

  it('shows an immediate pending reference placeholder while the image is being prepared for editing', async () => {
    const generatedImage = {
      id: 9,
      user_id: 42,
      mode: 'generation',
      model: 'gpt-image-2',
      prompt: 'blue gateway',
      aspect_ratio: '1:1',
      size: '1024x1024',
      image_url: 'https://assets.example.com/generated.png',
      storage_driver: 'local',
      storage_object_key: 'images/user-42/generated.png',
      mime_type: 'image/png',
      cost: 0.1,
      bytes: 100,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    }
    let resolveDownload!: (value: { blob: Blob; filename: string }) => void
    const downloadPromise = new Promise<{ blob: Blob; filename: string }>((resolve) => {
      resolveDownload = resolve
    })
    downloadImageFile.mockReturnValue(downloadPromise)
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

    const continueButton = wrapper.findAll('button').find((button) => button.text().includes('imageStudio.useAsReference'))
    expect(continueButton).toBeTruthy()
    await continueButton!.trigger('click')

    expect(wrapper.find('[data-testid="image-studio-reference-pending"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('common.loading')
    expect(wrapper.find('[data-testid="image-studio-reference-preview"]').exists()).toBe(false)

    resolveDownload!({
      blob: new Blob(['png-bytes'], { type: 'image/png' }),
      filename: 'passion-api-image-9.png',
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="image-studio-reference-pending"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="image-studio-reference-preview"]').attributes('src')).toBe('blob:passion-api-image-9.png')
  })

  it('hides background options and sends output format with each generated image request', async () => {
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
    expect(wrapper.find('[data-testid="image-studio-output-background-auto"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-output-background-opaque"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-output-background-transparent"]').exists()).toBe(false)
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
    })
    expect(wrapper.find('img[src="https://assets.example.com/batch-1.webp"]').exists()).toBe(true)
    expect(wrapper.find('img[src="https://assets.example.com/batch-2.webp"]').exists()).toBe(true)
  })

  it('shows all images from a four-image generation batch in the main result canvas', async () => {
    vi.useFakeTimers()
    const batchImages = [1, 2, 3, 4].map((item) => ({
      id: 40 + item,
      mode: 'generation',
      model: 'gpt-image-1',
      prompt: 'four blue portals',
      aspect_ratio: '16:9',
      size: '2048x1152',
      image_url: `https://assets.example.com/four-${item}.png`,
      cost: 0.07,
      bytes: 100,
      source_image_count: 0,
      created_at: '2026-06-22T00:00:00Z',
      updated_at: '2026-06-22T00:00:00Z',
    }))
    createTask
      .mockResolvedValueOnce({
        id: 41,
        mode: 'generation',
        status: 'queued',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 42,
        mode: 'generation',
        status: 'queued',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 43,
        mode: 'generation',
        status: 'queued',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 44,
        mode: 'generation',
        status: 'queued',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
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
        id: 41,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: batchImages[0],
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 42,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: batchImages[1],
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 43,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: batchImages[2],
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:00Z',
      })
      .mockResolvedValueOnce({
        id: 44,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-1',
        prompt: 'four blue portals',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: batchImages[3],
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
    await wrapper.get('[data-testid="image-studio-output-count-4"]').trigger('click')
    await wrapper.get('[data-testid="image-studio-prompt"]').setValue('four blue portals')
    await wrapper.get('[data-testid="image-studio-submit"]').trigger('submit')
    await vi.advanceTimersByTimeAsync(2600)
    await flushPromises()

    const resultGrid = wrapper.get('[data-testid="image-studio-current-result-grid"]')
    expect(createTask).toHaveBeenCalledTimes(4)
    expect(resultGrid.findAll('img')).toHaveLength(4)
    for (const image of batchImages) {
      expect(resultGrid.find(`img[src="${image.image_url}"]`).exists()).toBe(true)
    }
    expect(resultGrid.text()).toContain('imageStudio.batchSummaryComplete')
  })

  it('preserves generated image aspect ratio in the current result batch', () => {
    const batchImageRule = cssRulesFor('.image-studio-result-tile-preview img')[0] ?? ''
    const batchTileRule = cssRulesFor('.image-studio-result-tile-preview')[0] ?? ''

    expect(batchImageRule).toMatch(/object-fit:\s*contain/)
    expect(batchImageRule).not.toMatch(/object-fit:\s*cover/)
    expect(batchTileRule).toMatch(/background:/)
  })

  it('does not expose background controls when jpeg output is selected', async () => {
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

    await wrapper.get('[data-testid="image-studio-output-format-jpeg"]').trigger('click')

    expect(wrapper.find('[data-testid="image-studio-output-background-auto"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-output-background-transparent"]').exists()).toBe(false)
  })

  it('does not expose background controls for gpt-image-2', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="image-studio-output-background-auto"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-output-background-transparent"]').exists()).toBe(false)
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

  it('restores an active generation task from session storage after returning to the page', async () => {
    vi.useFakeTimers()
    window.sessionStorage.setItem('image-studio-active-generation-task-id', '22')
    getTask
      .mockResolvedValueOnce({
        id: 22,
        mode: 'generation',
        status: 'running',
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
      .mockResolvedValueOnce({
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
          id: 14,
          mode: 'generation',
          model: 'gpt-image-2',
          prompt: 'blue gateway',
          aspect_ratio: '16:9',
          size: '3840x2160',
          image_url: 'https://assets.example.com/resumed-session.png',
          cost: 0.16,
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

    expect(getTask).toHaveBeenCalledWith(22)
    expect(wrapper.find('[data-testid="image-studio-generating-overlay"]').exists()).toBe(true)

    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    expect(wrapper.find('img[src="https://assets.example.com/resumed-session.png"]').exists()).toBe(true)
    expect(refreshUser).toHaveBeenCalled()
  })

  it('resumes every unfinished generation task after a page reload', async () => {
    vi.useFakeTimers()
    listTasks.mockResolvedValueOnce({
      items: [
        {
          id: 42,
          mode: 'generation',
          status: 'running',
          api_key_id: 15,
          group_id: 9,
          model: 'gpt-image-2',
          prompt: 'restored batch station',
          aspect_ratio: '16:9',
          quality: '2K',
          size: '2048x1152',
          estimated_cost: 0.07,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:01Z',
          updated_at: '2026-06-22T00:00:01Z',
        },
        {
          id: 41,
          mode: 'generation',
          status: 'queued',
          api_key_id: 15,
          group_id: 9,
          model: 'gpt-image-2',
          prompt: 'restored batch station',
          aspect_ratio: '16:9',
          quality: '2K',
          size: '2048x1152',
          estimated_cost: 0.07,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:00Z',
          updated_at: '2026-06-22T00:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 5,
      pages: 1,
    })
    getTask
      .mockResolvedValueOnce({
        id: 42,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-2',
        prompt: 'restored batch station',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: {
          id: 42,
          mode: 'generation',
          model: 'gpt-image-2',
          prompt: 'restored batch station',
          aspect_ratio: '16:9',
          size: '2048x1152',
          image_url: 'https://assets.example.com/restored-batch-2.png',
          cost: 0.07,
          bytes: 100,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:01Z',
          updated_at: '2026-06-22T00:00:01Z',
        },
        created_at: '2026-06-22T00:00:01Z',
        updated_at: '2026-06-22T00:00:01Z',
      })
      .mockResolvedValueOnce({
        id: 41,
        mode: 'generation',
        status: 'succeeded',
        model: 'gpt-image-2',
        prompt: 'restored batch station',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        image: {
          id: 41,
          mode: 'generation',
          model: 'gpt-image-2',
          prompt: 'restored batch station',
          aspect_ratio: '16:9',
          size: '2048x1152',
          image_url: 'https://assets.example.com/restored-batch-1.png',
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

    expect(wrapper.text()).toContain('imageStudio.taskBatchRunningHint')

    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    expect(getTask).toHaveBeenCalledWith(42)
    expect(getTask).toHaveBeenCalledWith(41)
    const resultGrid = wrapper.get('[data-testid="image-studio-current-result-grid"]')
    expect(resultGrid.find('img[src="https://assets.example.com/restored-batch-2.png"]').exists()).toBe(true)
    expect(resultGrid.find('img[src="https://assets.example.com/restored-batch-1.png"]').exists()).toBe(true)
    expect(resultGrid.text()).toContain('imageStudio.batchSummaryComplete')
  })

  it('shows recovered batch images on the canvas as soon as one task finishes', async () => {
    vi.useFakeTimers()
    listTasks.mockResolvedValueOnce({
      items: [
        {
          id: 52,
          mode: 'generation',
          status: 'running',
          api_key_id: 15,
          group_id: 9,
          model: 'gpt-image-2',
          prompt: 'partial restored batch',
          aspect_ratio: '16:9',
          quality: '2K',
          size: '2048x1152',
          estimated_cost: 0.07,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:01Z',
          updated_at: '2026-06-22T00:00:01Z',
        },
        {
          id: 51,
          mode: 'generation',
          status: 'running',
          api_key_id: 15,
          group_id: 9,
          model: 'gpt-image-2',
          prompt: 'partial restored batch',
          aspect_ratio: '16:9',
          quality: '2K',
          size: '2048x1152',
          estimated_cost: 0.07,
          source_image_count: 0,
          created_at: '2026-06-22T00:00:00Z',
          updated_at: '2026-06-22T00:00:00Z',
        },
      ],
      total: 2,
      page: 1,
      page_size: 5,
      pages: 1,
    })
    getTask.mockImplementation(async (taskID: number) => {
      if (taskID === 52) {
        return {
          id: 52,
          mode: 'generation',
          status: 'succeeded',
          model: 'gpt-image-2',
          prompt: 'partial restored batch',
          aspect_ratio: '16:9',
          quality: '2K',
          size: '2048x1152',
          estimated_cost: 0.07,
          source_image_count: 0,
          image: {
            id: 52,
            mode: 'generation',
            model: 'gpt-image-2',
            prompt: 'partial restored batch',
            aspect_ratio: '16:9',
            size: '2048x1152',
            image_url: 'https://assets.example.com/restored-partial.png',
            cost: 0.07,
            bytes: 100,
            source_image_count: 0,
            created_at: '2026-06-22T00:00:01Z',
            updated_at: '2026-06-22T00:00:01Z',
          },
          created_at: '2026-06-22T00:00:01Z',
          updated_at: '2026-06-22T00:00:01Z',
        }
      }
      return {
        id: 51,
        mode: 'generation',
        status: 'running',
        model: 'gpt-image-2',
        prompt: 'partial restored batch',
        aspect_ratio: '16:9',
        quality: '2K',
        size: '2048x1152',
        estimated_cost: 0.07,
        source_image_count: 0,
        created_at: '2026-06-22T00:00:00Z',
        updated_at: '2026-06-22T00:00:02Z',
      }
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
    await vi.advanceTimersByTimeAsync(1300)
    await flushPromises()

    const frame = wrapper.get('.image-studio-preview-frame')
    expect(frame.find('img[src="https://assets.example.com/restored-partial.png"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-generating-overlay"]').exists()).toBe(true)
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
    expect(wrapper.find('[data-testid="image-studio-gallery-retention"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="image-studio-gallery-retention"]').attributes('title')).toContain('imageStudio.galleryRetentionPolicy')
    expect(wrapper.get('[data-testid="image-studio-gallery-retention"]').text()).toContain('imageStudio.galleryRetentionMax')
    expect(wrapper.get('[data-testid="image-studio-gallery-retention"]').text()).toContain('imageStudio.galleryRetentionDays')
    expect(wrapper.findAll('.image-studio-gallery-retention-token')).toHaveLength(2)
    expect(wrapper.find('.image-studio-settings-panel').exists()).toBe(false)
    expect(wrapper.find('.image-studio-choice-picker').exists()).toBe(true)
    expect(wrapper.find('.image-studio-quality-picker').exists()).toBe(true)
    expect(wrapper.find('.image-studio-ratio-picker').exists()).toBe(true)
    expect(wrapper.find('select').exists()).toBe(false)
    expect(wrapper.find('[data-testid="image-studio-workbench"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-control-console"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-canvas-stage"]').exists()).toBe(true)
    expect(wrapper.find('.image-studio-workbench > .image-studio-gallery-rail').exists()).toBe(true)
    expect(wrapper.find('[data-testid="image-studio-canvas-stage"] .image-studio-gallery-rail').exists()).toBe(false)

    const apiKeySelect = wrapper.get('[data-testid="image-studio-api-key-select"]')
    expect(apiKeySelect.text()).toContain('Image Key')

    const modelSelect = wrapper.get('[data-testid="image-studio-model-select"]')
    expect(modelSelect.classes()).toContain('image-studio-select-trigger')
    expect(vueSource()).not.toContain('selectedGroup?.platform')

    const selectTriggerRule = cssRulesFor('.image-studio-select-trigger')[0] ?? ''
    const selectTriggerBeforeRule = cssRulesFor('.image-studio-select-trigger::before')[0] ?? ''
    const selectTriggerIconRule = cssRulesFor('.image-studio-select-trigger svg')[0] ?? ''
    const selectOpenRule =
      vueSource().match(
        /\.image-studio-select-trigger:hover:not\(:disabled\),\s*\.image-studio-select-trigger:focus-visible,\s*\.image-studio-select\.is-open \.image-studio-select-trigger\s*\{([^}]*)\}/,
      )?.[1] ?? ''
    const selectMenuRule = cssRulesFor('.image-studio-select-menu')[0] ?? ''

    expect(selectTriggerRule).toMatch(/position:\s*relative/)
    expect(selectTriggerRule).toMatch(/overflow:\s*hidden/)
    expect(selectTriggerRule).toMatch(/border:\s*1px solid rgba\(37,\s*99,\s*235,\s*0\.18\)/)
    expect(selectTriggerRule).toMatch(/linear-gradient\(135deg,\s*rgba\(255,\s*255,\s*255,\s*0\.94\)/)
    expect(selectTriggerRule).toMatch(/padding:\s*0\.72rem 0\.76rem 0\.72rem 0\.9rem/)
    expect(selectTriggerBeforeRule).toMatch(/width:\s*3px/)
    expect(selectTriggerBeforeRule).toMatch(/linear-gradient\(180deg,\s*var\(--brand-600\),\s*var\(--brand-cyan\)\)/)
    expect(selectTriggerIconRule).toMatch(/border-radius:\s*999px/)
    expect(selectTriggerIconRule).toMatch(/background:\s*rgba\(239,\s*246,\s*255,\s*0\.88\)/)
    expect(selectOpenRule).toMatch(/border-color:\s*rgba\(37,\s*99,\s*235,\s*0\.48\)/)
    expect(selectOpenRule).toMatch(/background:\s*linear-gradient\(135deg,\s*rgba\(239,\s*246,\s*255,\s*0\.9\)/)
    expect(selectMenuRule).toMatch(/border:\s*1px solid rgba\(37,\s*99,\s*235,\s*0\.18\)/)
    expect(selectMenuRule).toMatch(/backdrop-filter:\s*blur\(14px\)/)

    await modelSelect.trigger('click')

    expect(wrapper.find('[data-testid="image-studio-model-menu"]').exists()).toBe(true)
    expect(wrapper.find('.image-studio-select-option.is-selected').text()).toContain('gpt-image-2')
  })

  it('keeps a single section divider without doubling it in edit mode', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const controlDockRule = cssRulesFor('.image-studio-control-dock')[0] ?? ''
    const outputRowRule = cssRulesFor('.image-studio-output-row')[0] ?? ''
    const foundationRowRule = cssRulesFor('.image-studio-foundation-row')[0] ?? ''
    const editOutputRowRule = cssRulesFor('.image-studio-control-dock.is-edit-mode .image-studio-output-row')[0] ?? ''

    expect(wrapper.find('.image-studio-upload-panel').exists()).toBe(false)
    expect(wrapper.get('.image-studio-control-dock').classes()).not.toContain('is-edit-mode')
    expect(controlDockRule).toMatch(/border-top:\s*2px solid transparent/)
    expect(controlDockRule).toMatch(/linear-gradient\(90deg,\s*transparent,\s*rgba\(37,\s*99,\s*235,\s*0\.34\)/)
    expect(controlDockRule).toMatch(/background-origin:\s*border-box/)
    expect(controlDockRule).toMatch(/padding-top:\s*1rem/)
    expect(outputRowRule).not.toMatch(/border-top:/)
    expect(foundationRowRule).not.toMatch(/border-top:\s*2px solid transparent/)
    expect(foundationRowRule).toMatch(/padding-top:\s*0\.95rem/)
    expect(editOutputRowRule).toMatch(/border-top:/)

    await wrapper.findAll('.image-studio-mode-switch button')[1].trigger('click')

    expect(wrapper.get('.image-studio-control-dock').classes()).toContain('is-edit-mode')
    expect(wrapper.find('.image-studio-upload-panel').exists()).toBe(true)
  })

  it('uses clean lightweight option groups for image settings', () => {
    const pickerRule = cssRulesFor('.image-studio-choice-picker')[0] ?? ''
    const buttonRule = cssRulesFor('.image-studio-choice-picker button')[0] ?? ''
    const activeRule = cssRulesFor('.image-studio-choice-picker button.active')[0] ?? ''
    const hoverRule =
      vueSource().match(
        /\.image-studio-choice-picker button:hover,\s*\.image-studio-choice-picker button:focus-visible\s*\{([^}]*)\}/,
      )?.[1] ?? ''

    expect(pickerRule).toMatch(/background:\s*transparent/)
    expect(pickerRule).not.toMatch(/box-shadow:\s*inset/)
    expect(pickerRule).toMatch(/gap:\s*0\.42rem/)
    expect(buttonRule).toMatch(/min-height:\s*2\.72rem/)
    expect(buttonRule).toMatch(/border:\s*1px solid rgba\(203,\s*213,\s*225,\s*0\.64\)/)
    expect(buttonRule).toMatch(/background:\s*rgba\(255,\s*255,\s*255,\s*0\.68\)/)
    expect(hoverRule).toMatch(/background:\s*rgba\(239,\s*246,\s*255,\s*0\.72\)/)
    expect(activeRule).toMatch(/border-color:\s*rgba\(37,\s*99,\s*235,\s*0\.54\)/)
    expect(activeRule).toMatch(/background:\s*rgba\(239,\s*246,\s*255,\s*0\.78\)/)
    expect(activeRule).not.toMatch(/0 10px 22px/)
    expect(activeRule).not.toMatch(/linear-gradient\(180deg/)
  })

  it('uses a vertically resizable prompt textarea without internal scrolling', async () => {
    const wrapper = mount(ImageStudioView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const textarea = wrapper.get('[data-testid="image-studio-prompt"]')
    const textareaBaseRule = cssRulesFor('.image-studio-field textarea').find((rule) => rule.includes('width: 100%')) ?? ''
    const textareaSizingRule = cssRulesFor('.image-studio-field textarea').find((rule) => rule.includes('resize')) ?? ''
    const textareaFocusRule = cssRulesFor('.image-studio-field textarea:focus')[0] ?? ''
    const textareaFocusVisibleRule = cssRulesFor('.image-studio-field textarea:focus-visible')[0] ?? ''
    const textareaResizerRule = cssRulesFor('.image-studio-field textarea::-webkit-resizer')[0] ?? ''
    const promptFieldRule = cssRulesFor('.image-studio-prompt-field')[0] ?? ''

    expect(textarea.attributes('rows')).toBe('5')
    expect(textareaBaseRule).toMatch(/width:\s*100%/)
    expect(textareaBaseRule).toMatch(/max-width:\s*100%/)
    expect(textareaBaseRule).toMatch(/border:\s*1px solid rgba\(37,\s*99,\s*235,\s*0\.24\)/)
    expect(textareaSizingRule).toMatch(/height:\s*auto/)
    expect(textareaSizingRule).toMatch(/min-height:\s*clamp\(8rem,\s*18dvh,\s*12rem\)/)
    expect(textareaSizingRule).toMatch(/resize:\s*vertical/)
    expect(textareaSizingRule).toMatch(/overflow-y:\s*hidden/)
    expect(textareaSizingRule).not.toMatch(/height:\s*100%/)
    expect(textareaSizingRule).not.toMatch(/resize:\s*none/)
    expect(textareaFocusRule).toMatch(/border-color:\s*rgba\(37,\s*99,\s*235,\s*0\.72\)/)
    expect(textareaFocusRule).toMatch(/box-shadow:/)
    expect(textareaFocusVisibleRule).toMatch(/outline:\s*none/)
    expect(textareaFocusVisibleRule).toMatch(/border-color:\s*rgba\(37,\s*99,\s*235,\s*0\.72\)/)
    expect(textareaResizerRule).toMatch(/background:/)
    expect(textareaResizerRule).toMatch(/rgba\(37,\s*99,\s*235,\s*0\.38\)/)
    expect(promptFieldRule).not.toMatch(/grid-template-rows:\s*auto minmax\(0,\s*1fr\)/)

    const element = textarea.element as HTMLTextAreaElement
    Object.defineProperty(element, 'scrollHeight', {
      configurable: true,
      value: 320,
    })

    await textarea.setValue('line 1\nline 2\nline 3')

    expect(element.style.height).toBe('320px')
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
      { number: '01', label: 'imageStudio.stepConnection' },
      { number: '02', label: 'imageStudio.stepOutput' },
    ])

    await wrapper.findAll('.image-studio-mode-switch button')[1].trigger('click')

    const editSteps = wrapper.findAll('.image-studio-step-heading').map((heading) => ({
      number: heading.find('span').text(),
      label: heading.find('strong').text(),
    }))

    expect(editSteps).toEqual([
      { number: '00', label: 'imageStudio.stepPrompt' },
      { number: '01', label: 'imageStudio.stepReference' },
      { number: '02', label: 'imageStudio.stepConnection' },
      { number: '03', label: 'imageStudio.stepOutput' },
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

  it('uses a three-column workstation with history isolated from the canvas height', async () => {
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
    const canvasRule = cssRulesFor('.image-studio-canvas-stage').find((rule) => rule.includes('height: 100%')) ?? ''
    const previewRule = cssRulesFor('.image-studio-preview-panel').find((rule) => rule.includes('grid-template-rows')) ?? ''
    const previewOpenRule =
      cssRulesFor('.image-studio-stage-panel .image-studio-preview-open').find((rule) => rule.includes('height: 100%')) ?? ''
    const currentActionsRule = cssRulesFor('.image-studio-current-actions').find((rule) => rule.includes('margin-top')) ?? ''
    const stagePreviewImageRule = cssRulesFor('.image-studio-stage-panel .image-studio-preview-open img')[0] ?? ''
    const stagePreviewHoverRule =
      vueSource().match(
        /\.image-studio-stage-panel \.image-studio-preview-open:hover img,\s*\.image-studio-stage-panel \.image-studio-preview-open:focus-visible img\s*\{([^}]*)\}/,
      )?.[1] ?? ''
    const galleryRule = cssRulesFor('.image-studio-gallery').find((rule) => rule.includes('height: 100%')) ?? ''
    const galleryRailRule = cssRulesFor('.image-studio-gallery-rail')[0] ?? ''
    const galleryRailHeaderRule = cssRulesFor('.image-studio-gallery-rail .image-studio-panel-header')[0] ?? ''
    const galleryRailHeaderContentRule = cssRulesFor('.image-studio-gallery-rail .image-studio-panel-header > div')[0] ?? ''
    const galleryRailRetentionRule = cssRulesFor('.image-studio-gallery-retention')[0] ?? ''
    const galleryRailRetentionIconRule = cssRulesFor('.image-studio-gallery-retention svg')[0] ?? ''
    const galleryRailTitleRule = cssRulesFor('.image-studio-gallery-rail .image-studio-panel-header h2')[0] ?? ''
    const galleryRailLabelRule = cssRulesFor('.image-studio-gallery-rail .image-studio-section-label')[0] ?? ''
    const galleryRailCountRule = cssRulesFor('.image-studio-gallery-rail .image-studio-gallery-count')[0] ?? ''
    const galleryRailEmptyRule = cssRulesFor('.image-studio-gallery-rail .image-studio-empty-gallery')[0] ?? ''
    const galleryRailEmptyIconRule = cssRulesFor('.image-studio-gallery-rail .image-studio-empty-gallery svg')[0] ?? ''
    const galleryRailEmptyTitleRule = cssRulesFor('.image-studio-gallery-rail .image-studio-empty-gallery strong')[0] ?? ''
    const galleryRailEmptyHintRule = cssRulesFor('.image-studio-gallery-rail .image-studio-empty-gallery span')[0] ?? ''
    const galleryRailThumbRule = cssRulesFor('.image-studio-gallery-rail .image-studio-image-thumb')[0] ?? ''
    const galleryRailThumbImageRule = cssRulesFor('.image-studio-gallery-rail .image-studio-image-thumb img')[0] ?? ''
    const galleryRailCardRule = cssRulesFor('.image-studio-gallery-rail .image-studio-image-card')[0] ?? ''
    const galleryRailBodyHiddenRule =
      cssRulesFor('.image-studio-gallery-rail .image-studio-image-card-body').find((rule) => rule.includes('display: none')) ?? ''
    const galleryRailPreviewLabelRule =
      cssRulesFor('.image-studio-gallery-rail .image-studio-image-thumb > span')[0] ?? ''
    const galleryRailPreviewHoverRule =
      vueSource().match(
        /\.image-studio-gallery-rail \.image-studio-image-thumb:hover > span,\s*\.image-studio-gallery-rail \.image-studio-image-thumb:focus-visible > span\s*\{([^}]*)\}/,
      )?.[1] ?? ''
    const galleryGridRule =
      vueSource().match(/\.image-studio-gallery-grid,\s*\.image-studio-gallery-loading\s*\{([^}]*)\}/)?.[1] ?? ''

    expect(shellRule).toMatch(/height:\s*var\(--studio-viewport-height\)/)
    expect(shellRule).toMatch(/overflow:\s*hidden/)
    expect(workbenchRule).toMatch(/align-items:\s*stretch/)
    expect(workbenchRule).toMatch(
      /grid-template-columns:\s*minmax\(19rem,\s*0\.58fr\) minmax\(42rem,\s*1\.74fr\) minmax\(8\.5rem,\s*0\.3fr\)/,
    )
    expect(workspaceRule).toMatch(/height:\s*100%/)
    expect(workspaceRule).toMatch(/grid-template-rows:\s*auto minmax\(0,\s*1fr\) auto/)
    expect(commandSurfaceRule).toMatch(/height:\s*100%/)
    expect(commandSurfaceRule).toMatch(/overflow-y:\s*auto/)
    expect(canvasRule).toMatch(/height:\s*100%/)
    expect(canvasRule).not.toMatch(/grid-template-rows:\s*minmax\(0,\s*1fr\) auto/)
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
    expect(galleryRule).toMatch(/height:\s*100%/)
    expect(galleryRule).toMatch(/padding:\s*clamp\(0\.55rem,\s*0\.72vw,\s*0\.7rem\)/)
    expect(galleryRule).not.toMatch(/max-height:\s*clamp\(11rem,\s*22dvh,\s*15rem\)/)
    expect(galleryRailRule).toMatch(/gap:\s*0\.58rem/)
    expect(galleryRailRule).toMatch(/padding:\s*clamp\(0\.62rem,\s*0\.76vw,\s*0\.78rem\)/)
    expect(galleryRailHeaderRule).toMatch(/display:\s*grid/)
    expect(galleryRailHeaderRule).toMatch(/grid-template-columns:\s*minmax\(0,\s*1fr\) auto/)
    expect(galleryRailHeaderRule).toMatch(/align-items:\s*start/)
    expect(galleryRailHeaderRule).toMatch(/gap:\s*0\.16rem 0\.36rem/)
    expect(galleryRailHeaderContentRule).toMatch(/display:\s*grid/)
    expect(galleryRailHeaderContentRule).toMatch(/gap:\s*0\.18rem/)
    expect(galleryRailRetentionRule).toMatch(/display:\s*flex/)
    expect(galleryRailRetentionRule).toMatch(/flex-wrap:\s*wrap/)
    expect(galleryRailRetentionRule).not.toMatch(/overflow:\s*hidden/)
    expect(galleryRailRetentionRule).not.toMatch(/text-overflow:\s*ellipsis/)
    expect(galleryRailRetentionIconRule).toMatch(/flex:\s*0 0 auto/)
    expect(galleryRailTitleRule).not.toMatch(/grid-column:\s*1 \/ -1/)
    expect(galleryRailTitleRule).toMatch(/white-space:\s*nowrap/)
    expect(galleryRailTitleRule).toMatch(/text-overflow:\s*ellipsis/)
    expect(galleryRailLabelRule).toMatch(/white-space:\s*nowrap/)
    expect(galleryRailCountRule).toMatch(/white-space:\s*nowrap/)
    expect(galleryRailCountRule).not.toMatch(/grid-column:\s*2/)
    expect(galleryRailEmptyRule).toMatch(/height:\s*100%/)
    expect(galleryRailEmptyRule).toMatch(/align-content:\s*center/)
    expect(galleryRailEmptyRule).toMatch(/grid-auto-rows:\s*max-content/)
    expect(galleryRailEmptyRule).toMatch(/padding:\s*0\.9rem 0\.62rem/)
    expect(galleryRailEmptyIconRule).toMatch(/width:\s*2\.15rem/)
    expect(galleryRailEmptyIconRule).toMatch(/border-radius:\s*0\.68rem/)
    expect(galleryRailEmptyTitleRule).toMatch(/max-width:\s*7rem/)
    expect(galleryRailEmptyHintRule).toMatch(/display:\s*-webkit-box/)
    expect(galleryRailEmptyHintRule).toMatch(/-webkit-line-clamp:\s*3/)
    expect(galleryRailCardRule).not.toMatch(/height:\s*100%/)
    expect(galleryRailCardRule).toMatch(/aspect-ratio:\s*16 \/ 10/)
    expect(galleryRailCardRule).toMatch(/background:\s*transparent/)
    expect(galleryRailCardRule).toMatch(/box-shadow:\s*none/)
    expect(galleryRailThumbRule).toMatch(/height:\s*100%/)
    expect(galleryRailThumbRule).toMatch(/aspect-ratio:\s*inherit/)
    expect(galleryRailThumbRule).toMatch(/overflow:\s*hidden/)
    expect(galleryRailThumbRule).toMatch(/isolation:\s*isolate/)
    expect(galleryRailThumbImageRule).toMatch(/position:\s*absolute/)
    expect(galleryRailThumbImageRule).toMatch(/inset:\s*0/)
    expect(galleryRailThumbImageRule).toMatch(/width:\s*100%/)
    expect(galleryRailThumbImageRule).toMatch(/height:\s*100%/)
    expect(galleryRailThumbImageRule).toMatch(/aspect-ratio:\s*auto/)
    expect(galleryRailThumbImageRule).toMatch(/object-fit:\s*cover/)
    expect(galleryRailThumbImageRule).toMatch(/object-position:\s*center/)
    expect(galleryRailBodyHiddenRule).toMatch(/display:\s*none/)
    expect(galleryRailPreviewLabelRule).toMatch(/top:\s*50%/)
    expect(galleryRailPreviewLabelRule).toMatch(/left:\s*50%/)
    expect(galleryRailPreviewLabelRule).toMatch(/max-width:\s*calc\(100% - 1rem\)/)
    expect(galleryRailPreviewLabelRule).toMatch(/padding:\s*0\.34rem 0\.5rem/)
    expect(galleryRailPreviewLabelRule).toMatch(/transform:\s*translate\(-50%,\s*-50%\) scale\(0\.96\)/)
    expect(galleryRailPreviewHoverRule).toMatch(/transform:\s*translate\(-50%,\s*-50%\) scale\(1\)/)
    expect(galleryGridRule).toMatch(/grid-template-columns:\s*1fr/)
    expect(galleryGridRule).toMatch(/align-content:\s*start/)
    expect(galleryGridRule).toMatch(/grid-auto-rows:\s*max-content/)
    expect(galleryGridRule).toMatch(/gap:\s*0\.42rem/)
    expect(galleryGridRule).toMatch(/overflow-y:\s*auto/)
    expect(galleryGridRule).not.toMatch(/grid-auto-flow:\s*column/)
    expect(wrapper.get('.image-studio-workspace > .image-studio-action-bar').exists()).toBe(true)
    expect(wrapper.find('.image-studio-command-surface .image-studio-action-bar').exists()).toBe(false)
  })

  it('keeps batch index badges readable over light generated images', () => {
    const indexRule = cssRulesFor('.image-studio-result-index')[0] ?? ''

    expect(indexRule).toMatch(/background:\s*linear-gradient\(135deg,\s*rgba\(15,\s*23,\s*42,\s*0\.94\)/)
    expect(indexRule).toMatch(/color:\s*white/)
    expect(indexRule).toMatch(/text-shadow:/)
    expect(indexRule).toMatch(/box-shadow:/)
  })

  it('keeps the cost and generate action panel in normal flow with an opaque surface', () => {
    const actionBarRule = cssRulesFor('.image-studio-action-bar').find((rule) => rule.includes('display: grid')) ?? ''
    const darkActionBarRule = cssRulesFor('.dark .image-studio-action-bar').find((rule) => rule.includes('background')) ?? ''

    expect(actionBarRule).not.toMatch(/position:\s*sticky/)
    expect(actionBarRule).not.toMatch(/bottom:\s*0/)
    expect(actionBarRule).toMatch(/box-shadow:/)
    expect(actionBarRule).toMatch(/margin-top:\s*1\.15rem/)
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
    const downloadMock = vi.fn().mockResolvedValue({
      blob: new Blob(['png-bytes'], { type: 'image/png' }),
      filename: 'passion-api-image-9.png',
    })
    downloadImageFile.mockImplementation(downloadMock)
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

    expect(downloadMock).toHaveBeenCalledWith(9)
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
    expect(wrapper.find('.image-studio-gallery-rail .image-studio-image-card-actions').exists()).toBe(true)
    expect(wrapper.findAll('.image-studio-gallery-rail .image-studio-image-card-actions button')).toHaveLength(3)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    await flushPromises()

    expect(wrapper.find('[data-testid="image-studio-image-preview-dialog"]').exists()).toBe(false)
  })

  it('keeps preview actions reachable when the prompt is long', () => {
    const dialogRule = cssRulesFor('.image-studio-preview-dialog')[0] ?? ''
    const detailsRule = cssRulesFor('.image-studio-preview-details')[0] ?? ''
    const promptRule = cssRulesFor('.image-studio-preview-details strong')[0] ?? ''
    const actionsRule = cssRulesFor('.image-studio-preview-actions')[0] ?? ''

    expect(dialogRule).toMatch(/grid-template-rows:\s*auto minmax\(0,\s*1fr\) auto auto/)
    expect(detailsRule).toMatch(/min-height:\s*0/)
    expect(detailsRule).toMatch(/max-height:\s*clamp\(6\.5rem,\s*18vh,\s*11rem\)/)
    expect(detailsRule).toMatch(/overflow-y:\s*auto/)
    expect(detailsRule).toMatch(/overscroll-behavior:\s*contain/)
    expect(promptRule).toMatch(/overflow-wrap:\s*anywhere/)
    expect(actionsRule).toMatch(/position:\s*relative/)
    expect(actionsRule).toMatch(/z-index:\s*1/)
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
