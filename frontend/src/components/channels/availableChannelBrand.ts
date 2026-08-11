import type { GroupPlatform } from '@/types'

export interface AvailableChannelBrand {
  key: GroupPlatform | 'generic'
  label: string
  platform?: GroupPlatform
}

const brands: Record<AvailableChannelBrand['key'], AvailableChannelBrand> = {
  openai: { key: 'openai', label: 'OpenAI', platform: 'openai' },
  anthropic: { key: 'anthropic', label: 'Anthropic', platform: 'anthropic' },
  gemini: { key: 'gemini', label: 'Gemini', platform: 'gemini' },
  antigravity: { key: 'antigravity', label: 'Antigravity', platform: 'antigravity' },
  grok: { key: 'grok', label: 'Grok', platform: 'grok' },
  composite: { key: 'composite', label: 'Composite', platform: 'composite' },
  generic: { key: 'generic', label: 'AI' },
}

export function resolveAvailableChannelBrand(platform: string): AvailableChannelBrand {
  const value = platform.trim().toLowerCase()
  if (value.includes('openai') || value.includes('codex')) return brands.openai
  if (value.includes('anthropic') || value.includes('claude')) return brands.anthropic
  if (value.includes('gemini') || value.includes('google')) return brands.gemini
  if (value.includes('antigravity')) return brands.antigravity
  if (value.includes('grok') || value.includes('xai')) return brands.grok
  if (value.includes('composite')) return brands.composite
  return brands.generic
}
