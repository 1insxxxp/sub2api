import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, put, post } = vi.hoisted(() => ({
  get: vi.fn(),
  put: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    put,
    post,
  },
}))

import {
  getImageStudioSettings,
  updateImageStudioSettings,
  testImageStudioStorage,
  type ImageStudioSettings,
} from '@/api/admin/settings'

describe('admin image studio settings api', () => {
  beforeEach(() => {
    get.mockReset()
    put.mockReset()
    post.mockReset()
  })

  it('loads, saves, and tests image studio settings through backend endpoints', async () => {
    const settings: ImageStudioSettings = {
      enabled: true,
      allowed_models: ['gpt-image-1'],
      default_model: 'gpt-image-1',
      storage_driver: 'local',
      retention_days: 30,
      max_images_per_user: 100,
      max_reference_image_mb: 20,
      aspect_ratios: [{ ratio: '1:1', size: '1024x1024', billing_tier: '1K' }],
      storage_status: { driver: 'local', status: 'ok', configured: true },
    }
    get.mockResolvedValueOnce({ data: settings })
    put.mockResolvedValueOnce({ data: settings })
    post.mockResolvedValueOnce({ data: { driver: 'local', status: 'ok', configured: true } })

    await getImageStudioSettings()
    await updateImageStudioSettings(settings)
    await testImageStudioStorage()

    expect(get).toHaveBeenCalledWith('/admin/settings/image-studio')
    expect(put).toHaveBeenCalledWith('/admin/settings/image-studio', settings)
    expect(post).toHaveBeenCalledWith('/admin/settings/image-studio/storage/test')
  })
})
