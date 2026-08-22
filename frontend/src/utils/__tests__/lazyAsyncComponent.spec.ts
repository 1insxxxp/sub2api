import { beforeEach, describe, expect, it, vi } from 'vitest'

const defineAsyncComponentMock = vi.hoisted(() => vi.fn((options) => options))

vi.mock('vue', async () => {
  const actual = await vi.importActual<typeof import('vue')>('vue')
  return {
    ...actual,
    defineAsyncComponent: defineAsyncComponentMock
  }
})

describe('lazyAsyncComponent', () => {
  beforeEach(() => {
    defineAsyncComponentMock.mockClear()
    sessionStorage.clear()
    vi.stubGlobal('location', { reload: vi.fn() })
  })

  it('reloads once when a lazy dialog chunk was replaced by a newer deployment', async () => {
    const { lazyAsyncComponent } = await import('../lazyAsyncComponent')

    const component = lazyAsyncComponent(() => Promise.reject(new Error('Failed to fetch dynamically imported module')))
    const options = component as unknown as {
      loader: () => Promise<unknown>
      onError: (error: Error, retry: () => void, fail: () => void, attempts: number) => void
    }
    const retry = vi.fn()
    const fail = vi.fn()

    options.onError(new Error('Failed to fetch dynamically imported module'), retry, fail, 1)

    expect(location.reload).toHaveBeenCalledTimes(1)
    expect(sessionStorage.getItem('lazy_chunk_reload_attempted')).toMatch(/^\d+$/)
    expect(retry).not.toHaveBeenCalled()
    expect(fail).not.toHaveBeenCalled()
  })

  it('fails instead of entering a reload loop when the lazy chunk still cannot load', async () => {
    const { lazyAsyncComponent } = await import('../lazyAsyncComponent')
    sessionStorage.setItem('lazy_chunk_reload_attempted', Date.now().toString())

    const component = lazyAsyncComponent(() => Promise.reject(new Error('Failed to fetch dynamically imported module')))
    const options = component as unknown as {
      onError: (error: Error, retry: () => void, fail: () => void, attempts: number) => void
    }
    const fail = vi.fn()

    options.onError(new Error('Failed to fetch dynamically imported module'), vi.fn(), fail, 1)

    expect(location.reload).not.toHaveBeenCalled()
    expect(fail).toHaveBeenCalledTimes(1)
  })
})
