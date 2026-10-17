import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ default: { post: mocks.post } }))

async function lookup() { return import('../ipGeoLookup') }

beforeEach(() => {
  vi.resetModules()
  mocks.post.mockReset()
  localStorage.clear()
})
afterEach(() => { vi.useRealTimers() })

describe('offline IP geolocation', () => {
  it('recognizes private IPv4 and IPv6 addresses, including mapped IPv4', async () => {
    const { isPrivateIp, fetchBatch, getEntry } = await lookup()
    const ips = ['10.0.0.1', '127.0.0.1', '192.168.1.1', '172.16.0.1', '172.31.255.255', '169.254.1.1', '::1', 'fe80::1', 'fe90::1', 'febf::1', 'fc00::1', 'fdff::1', '::ffff:192.168.1.1']
    for (const ip of ips) expect(isPrivateIp(ip), ip).toBe(true)
    await fetchBatch(ips)
    expect(mocks.post).not.toHaveBeenCalled()
    expect(getEntry('::ffff:192.168.1.1').status).toBe('private')
    for (const ip of ['8.8.8.8', '172.32.0.1', 'fec0::1', 'fbff::1', 'fe7f::1']) expect(isPrivateIp(ip), ip).toBe(false)
  })

  it('posts queried IPs only in the authenticated local API body', async () => {
    const { fetchOne, getEntry } = await lookup()
    mocks.post.mockResolvedValue({ data: [{ ip: '8.8.8.8', country_code: 'US', region: 'California', city: 'Mountain View', source: 'dbip' }] })
    await fetchOne('8.8.8.8')
    expect(mocks.post).toHaveBeenCalledWith('/user/ip-geo', { ips: ['8.8.8.8'] })
    expect(getEntry('8.8.8.8')).toMatchObject({ status: 'success', label: 'US · California · Mountain View', detail: { source: 'dbip' } })
    expect(localStorage.getItem('sub2api:ip-geo-cache:v1')).toBeNull()
  })

  it('removes legacy persistent caches without hydrating their customer IPs', async () => {
    localStorage.setItem('sub2api:ip-geo-cache:v1', JSON.stringify({ '8.8.8.8': { label: 'US', fetchedAt: Date.now() } }))
    const { getEntry } = await lookup()
    expect(getEntry('8.8.8.8')).toEqual({ status: 'idle' })
    expect(localStorage.getItem('sub2api:ip-geo-cache:v1')).toBeNull()
  })

  it('caches in memory, supports refresh, and expires stale entries', async () => {
    vi.useFakeTimers()
    const { fetchOne, getEntry } = await lookup()
    mocks.post.mockResolvedValue({ data: [{ ip: '8.8.4.4', country_code: 'US' }] })
    await fetchOne('8.8.4.4')
    await fetchOne('8.8.4.4')
    expect(mocks.post).toHaveBeenCalledTimes(1)
    await fetchOne('8.8.4.4', true)
    expect(mocks.post).toHaveBeenCalledTimes(2)
    vi.advanceTimersByTime(25 * 60 * 60 * 1000)
    expect(getEntry('8.8.4.4')).toEqual({ status: 'idle' })
    await fetchOne('8.8.4.4')
    expect(mocks.post).toHaveBeenCalledTimes(3)
  })

  it('drops the previous user cache when the authenticated user changes', async () => {
    localStorage.setItem('auth_user', JSON.stringify({ id: 1 }))
    const { fetchOne, getEntry } = await lookup()
    mocks.post.mockResolvedValue({ data: [{ ip: '1.1.1.1', country_code: 'AU' }] })
    await fetchOne('1.1.1.1')
    localStorage.setItem('auth_user', JSON.stringify({ id: 2 }))
    expect(getEntry('1.1.1.1')).toEqual({ status: 'idle' })
  })

  it('ignores an in-flight response from a previous authenticated user', async () => {
    localStorage.setItem('auth_user', JSON.stringify({ id: 1 }))
    const { fetchOne, getEntry } = await lookup()
    let resolve!: (value: unknown) => void
    mocks.post.mockReturnValue(new Promise((done) => { resolve = done }))
    const pending = fetchOne('1.0.0.1')
    localStorage.setItem('auth_user', JSON.stringify({ id: 2 }))
    resolve({ data: [{ ip: '1.0.0.1', country_code: 'AU' }] })
    await pending
    expect(getEntry('1.0.0.1')).toEqual({ status: 'idle' })
  })

  it('deduplicates targets, skips private addresses and chunks batches at 50', async () => {
    const { fetchBatch, getEntry } = await lookup()
    const ips = Array.from({ length: 61 }, (_, i) => `203.0.113.${i + 1}`)
    mocks.post.mockImplementation(async (_path, { ips: chunk }: { ips: string[] }) => ({ data: chunk.map(ip => ({ ip, country_code: 'US' })) }))
    await fetchBatch([...ips, ips[0], '10.1.1.1'])
    expect(mocks.post).toHaveBeenCalledTimes(2)
    expect(mocks.post.mock.calls[0][1].ips).toHaveLength(50)
    expect(mocks.post.mock.calls[1][1].ips).toHaveLength(11)
    expect(getEntry('10.1.1.1').status).toBe('private')
    await fetchBatch(ips)
    expect(mocks.post).toHaveBeenCalledTimes(2)
  })

  it('keeps missing/unknown locations as retryable errors', async () => {
    const { fetchBatch, getEntry } = await lookup()
    mocks.post.mockResolvedValue({ data: [{ ip: '203.0.113.80', country_code: 'US' }, { ip: '203.0.113.81' }] })
    expect(await fetchBatch(['203.0.113.80', '203.0.113.81', '203.0.113.82'])).toBe(true)
    expect(getEntry('203.0.113.80').status).toBe('success')
    expect(getEntry('203.0.113.81').status).toBe('error')
    expect(getEntry('203.0.113.82').status).toBe('error')
  })

  it('fails safely when the offline database/API is unavailable, without a third-party fallback', async () => {
    const { fetchBatch, getEntry } = await lookup()
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    mocks.post.mockRejectedValue(new Error('offline geolocation unavailable'))
    expect(await fetchBatch(['203.0.113.90'])).toBe(false)
    expect(getEntry('203.0.113.90').status).toBe('error')
    expect(fetchSpy).not.toHaveBeenCalled()
    fetchSpy.mockRestore()
  })

  it('formats only available location fields', async () => {
    const { formatGeoLabel } = await lookup()
    expect(formatGeoLabel({ countryCode: 'CN', region: 'Guangdong', city: 'Shenzhen' })).toBe('CN · Guangdong · Shenzhen')
    expect(formatGeoLabel({ countryCode: 'US' })).toBe('US')
  })
})
