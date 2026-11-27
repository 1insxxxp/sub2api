import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  getSettings,
  updateSettings,
  preview,
  type PublicGatewayErrorSettings
} from '@/api/admin/publicGatewayErrors'

const client = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn(), post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: client }))

const settings: PublicGatewayErrorSettings = {
  rules: [{
    id: 'capacity', name: 'Capacity', enabled: true, priority: 10,
    upstream_statuses: [429, 503], pattern: '(?i)capacity',
    status: 503, code: 'busy', message: 'Please retry later.'
  }],
  fallback: { status: 502, code: 'upstream_error', message: 'Please retry.' },
  bad_request_fallback: { status: 400, code: 'bad_request', message: 'Check your request.' }
}

beforeEach(() => vi.resetAllMocks())

describe('public gateway error settings API', () => {
  it('loads settings from the dedicated admin endpoint', async () => {
    client.get.mockResolvedValue({ data: settings })
    expect(await getSettings()).toEqual(settings)
    expect(client.get).toHaveBeenCalledWith('/admin/settings/public-gateway-errors')
  })

  it('saves the complete settings and returns the saved settings', async () => {
    client.put.mockResolvedValue({ data: settings })
    expect(await updateSettings(settings)).toEqual(settings)
    expect(client.put).toHaveBeenCalledWith('/admin/settings/public-gateway-errors', settings)
  })

  it.each(['http', 'sse', 'responses', 'gemini'] as const)('previews the draft in %s format', async (format) => {
    const request = { settings, status: 429, diagnostic: 'CAPACITY exhausted', format }
    const response = {
      status: 503, code: 'busy', message: 'Please retry later.',
      rule_id: 'capacity', rule_name: 'Capacity', payload: { error: { code: 'busy' } }
    }
    client.post.mockResolvedValue({ data: response })
    expect(await preview(request)).toEqual(response)
    expect(client.post).toHaveBeenCalledWith('/admin/settings/public-gateway-errors/preview', request)
  })
})
