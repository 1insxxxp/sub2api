import { reactive } from 'vue'
import apiClient from '@/api/client'

export type IpGeoStatus = 'idle' | 'loading' | 'success' | 'error' | 'private'

export interface IpGeoDetail {
  countryCode?: string
  region?: string
  city?: string
  organization?: string
  timezone?: string
  accuracy?: number
  latitude?: string
  longitude?: string
  source?: 'dbip' | 'maxmind'
}

export interface IpGeoEntry {
  status: IpGeoStatus
  label?: string
  detail?: IpGeoDetail
  fetchedAt?: number
}

interface RawGeoResponse {
  ip: string
  private?: boolean
  country_code?: string
  region?: string
  city?: string
  organization?: string
  timezone?: string
  accuracy?: number
  latitude?: string
  longitude?: string
  source?: IpGeoDetail['source']
}

const IDLE_ENTRY: IpGeoEntry = { status: 'idle' }
const CACHE_TTL_MS = 24 * 60 * 60 * 1000
const BATCH_CHUNK_SIZE = 50
const cache = reactive(new Map<string, IpGeoEntry>())
let cacheOwner = ''
let cacheGeneration = 0

// Remove the old persistent customer-IP cache without reading its contents.
try {
  localStorage.removeItem('sub2api:ip-geo-cache:v1')
} catch {
  // Private browsing can disable storage; lookups still work in memory.
}

function currentOwner(): string {
  try {
    const user = JSON.parse(localStorage.getItem('auth_user') || 'null')
    return user?.id == null ? '' : String(user.id)
  } catch {
    return ''
  }
}

function ensureSession(): number {
  const owner = currentOwner()
  if (owner !== cacheOwner) {
    cache.clear()
    cacheOwner = owner
    cacheGeneration++
  }
  return cacheGeneration
}

function isFreshSuccess(entry: IpGeoEntry | undefined): boolean {
  return entry?.status === 'success' && typeof entry.fetchedAt === 'number' && Date.now() - entry.fetchedAt <= CACHE_TTL_MS
}

export function getEntry(ip: string): IpGeoEntry {
  ensureSession()
  const entry = cache.get(ip)
  if (entry?.status === 'success' && !isFreshSuccess(entry)) {
    cache.delete(ip)
    return IDLE_ENTRY
  }
  return entry ?? IDLE_ENTRY
}

export function isPrivateIp(ip: string): boolean {
  const lower = ip.toLowerCase()
  const mapped = lower.match(/^::ffff:(\d+\.\d+\.\d+\.\d+)$/)
  if (mapped) return isPrivateIp(mapped[1])
  const v4 = lower.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/)
  if (v4) {
    const [a, b] = [Number(v4[1]), Number(v4[2])]
    return a === 0 || a === 10 || a === 127 || a >= 224 || (a === 169 && b === 254) ||
      (a === 172 && b >= 16 && b <= 31) || (a === 192 && b === 168)
  }
  if (lower === '::' || lower === '::1') return true
  const firstSegment = lower.split(':', 1)[0]
  return /^fe[89ab][0-9a-f]$/.test(firstSegment) || /^f[cd][0-9a-f]{2}$/.test(firstSegment) || /^ff[0-9a-f]{2}$/.test(firstSegment)
}

export function formatGeoLabel(detail: IpGeoDetail): string {
  return [detail.countryCode, detail.region, detail.city]
    .filter((part): part is string => Boolean(part?.trim()))
    .join(' · ')
}

function toDetail(raw: RawGeoResponse): IpGeoDetail {
  return {
    countryCode: raw.country_code, region: raw.region, city: raw.city,
    organization: raw.organization, timezone: raw.timezone, accuracy: raw.accuracy,
    latitude: raw.latitude, longitude: raw.longitude, source: raw.source,
  }
}

function applyResult(ip: string, raw: RawGeoResponse | undefined): void {
  if (raw?.private) {
    cache.set(ip, { status: 'private' })
  } else if (!raw?.country_code) {
    cache.set(ip, { status: 'error' })
  } else {
    const detail = toDetail(raw)
    cache.set(ip, { status: 'success', label: formatGeoLabel(detail), detail, fetchedAt: Date.now() })
  }
}

async function fetchChunk(ips: string[], generation: number): Promise<boolean> {
  try {
    // Customer IPs stay in the authenticated request body, never a URL or third-party request.
    const { data } = await apiClient.post<RawGeoResponse[]>('/user/ip-geo', { ips })
    if (ensureSession() !== generation) return false
    const byIp = new Map(data.map((item) => [item.ip, item]))
    ips.forEach((ip) => applyResult(ip, byIp.get(ip)))
    return true
  } catch {
    if (ensureSession() === generation) ips.forEach((ip) => cache.set(ip, { status: 'error' }))
    return false
  }
}

export async function fetchOne(ip: string, force = false): Promise<void> {
  const generation = ensureSession()
  if (isPrivateIp(ip)) {
    cache.set(ip, { status: 'private' })
    return
  }
  const existing = getEntry(ip)
  if (!force && (isFreshSuccess(existing) || existing.status === 'loading')) return
  cache.set(ip, { status: 'loading' })
  await fetchChunk([ip], generation)
}

export async function fetchBatch(ips: string[]): Promise<boolean> {
  const generation = ensureSession()
  const targets: string[] = []
  for (const ip of new Set(ips)) {
    if (isPrivateIp(ip)) {
      cache.set(ip, { status: 'private' })
      continue
    }
    const existing = getEntry(ip)
    if (!isFreshSuccess(existing) && existing.status !== 'loading') targets.push(ip)
  }
  targets.forEach((ip) => cache.set(ip, { status: 'loading' }))
  let success = true
  for (let i = 0; i < targets.length; i += BATCH_CHUNK_SIZE) {
    if (ensureSession() !== generation) return false
    if (!await fetchChunk(targets.slice(i, i + BATCH_CHUNK_SIZE), generation)) success = false
  }
  return success
}
