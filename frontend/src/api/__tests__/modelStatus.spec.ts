import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import axios, { AxiosError, type AxiosAdapter, type InternalAxiosRequestConfig } from 'axios'

const originalAdapter = axios.defaults.adapter

describe('public model status API', () => {
  beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
    localStorage.setItem('auth_token', 'existing-user-session')
    localStorage.setItem('refresh_token', 'existing-refresh-session')
  })

  afterEach(() => {
    axios.defaults.adapter = originalAdapter
    localStorage.clear()
  })

  it('unwraps the public envelope without attaching the current user token', async () => {
    let request!: InternalAxiosRequestConfig
    axios.defaults.adapter = (async config => {
      request = config
      return { data: { code: 0, data: { groups: [] } }, status: 200, statusText: 'OK', headers: {}, config }
    }) satisfies AxiosAdapter
    const { getModelStatus } = await import('../modelStatus')
    const controller = new AbortController()

    await expect(getModelStatus({ signal: controller.signal })).resolves.toEqual({ groups: [] })
    expect(request.url).toBe('/model-status')
    expect(request.headers.get('Authorization')).toBeUndefined()
    expect(request.withCredentials).toBe(false)
    expect(request.signal).toBe(controller.signal)
  })

  it('rejects a failed public request without clearing the logged-in session', async () => {
    const currentURL = window.location.href
    axios.defaults.adapter = (async config => {
      throw new AxiosError('Unauthorized', 'ERR_BAD_REQUEST', config, null, {
        status: 401, statusText: 'Unauthorized', headers: {}, config, data: { code: 401 },
      })
    }) satisfies AxiosAdapter
    const { getModelStatus } = await import('../modelStatus')

    await expect(getModelStatus()).rejects.toMatchObject({ response: { status: 401 } })
    expect(localStorage.getItem('auth_token')).toBe('existing-user-session')
    expect(localStorage.getItem('refresh_token')).toBe('existing-refresh-session')
    expect(window.location.href).toBe(currentURL)
  })

  it('does not present an application error envelope as a successful report', async () => {
    axios.defaults.adapter = (async config => ({
      data: { code: 503, message: 'internal diagnostic' },
      status: 200, statusText: 'OK', headers: {}, config,
    })) satisfies AxiosAdapter
    const { getModelStatus } = await import('../modelStatus')

    await expect(getModelStatus()).rejects.toThrow('Model status is unavailable')
  })
})
