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
    expect(wrapper.text()).toContain('passion-api-image-9.png')

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

    await wrapper.get('[data-testid="image-studio-delete-trigger-9"]').trigger('click')
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
