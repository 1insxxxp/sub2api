import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get, post, deleteMock } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  deleteMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
    delete: deleteMock,
  },
}))

import { imageStudioAPI } from '@/api/images'

describe('image studio user api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    deleteMock.mockReset()
    get.mockResolvedValue({ data: {} })
    post.mockResolvedValue({ data: {} })
    deleteMock.mockResolvedValue({ data: {} })
  })

  it('loads config and paginated image history through user endpoints', async () => {
    await imageStudioAPI.getConfig()
    await imageStudioAPI.getOptions()
    await imageStudioAPI.list({ page: 2, page_size: 12 })

    expect(get).toHaveBeenCalledWith('/user/images/config')
    expect(get).toHaveBeenCalledWith('/user/images/options')
    expect(get).toHaveBeenCalledWith('/user/images', {
      params: { page: 2, page_size: 12 },
    })
  })

  it('sends generation payloads as json', async () => {
    await imageStudioAPI.generate({
      group_id: 9,
      model: 'gpt-image-1',
      prompt: 'A neon blue API gateway',
      aspect_ratio: '16:9',
      quality: '4K',
    })

    expect(post).toHaveBeenCalledWith('/user/images/generate', {
      group_id: 9,
      model: 'gpt-image-1',
      prompt: 'A neon blue API gateway',
      aspect_ratio: '16:9',
      quality: '4K',
    })
  })

  it('sends edit payloads as multipart form data', async () => {
    const file = new File(['fake'], 'reference.png', { type: 'image/png' })

    await imageStudioAPI.edit({
      group_id: 9,
      model: 'gpt-image-1',
      prompt: 'Make it cleaner',
      aspect_ratio: '1:1',
      quality: '2K',
      images: [file],
    })

    expect(post).toHaveBeenCalledTimes(1)
    const [url, formData, config] = post.mock.calls[0]
    expect(url).toBe('/user/images/edit')
    expect(formData).toBeInstanceOf(FormData)
    expect(formData.get('model')).toBe('gpt-image-1')
    expect(formData.get('prompt')).toBe('Make it cleaner')
    expect(formData.get('aspect_ratio')).toBe('1:1')
    expect(formData.get('group_id')).toBe('9')
    expect(formData.get('quality')).toBe('2K')
    expect(formData.getAll('image')).toEqual([file])
    expect(config).toEqual({
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  })

  it('deletes an image by id', async () => {
    await imageStudioAPI.delete(42)

    expect(deleteMock).toHaveBeenCalledWith('/user/images/42')
  })
})
