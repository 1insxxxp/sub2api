import type { GroupPlatform, KeyDisplayCategory } from '@/types'

export type KeyGroupProvider = Exclude<KeyDisplayCategory, ''>

export const KEY_GROUP_PROVIDERS = ['anthropic', 'openai', 'domestic', 'other'] as const

// Platform remains the fallback for automatic and legacy display categories.
const PROVIDER_BY_PLATFORM: Record<GroupPlatform, KeyGroupProvider> = {
  anthropic: 'anthropic',
  openai: 'openai',
  kimi: 'domestic',
  zhipu: 'domestic',
  deepseek: 'domestic',
  minimax: 'domestic',
  gemini: 'other',
  grok: 'other',
  antigravity: 'other',
  composite: 'other',
  opencode_go: 'other',
  typesafe: 'other',
  command_code: 'other',
  cline: 'other'
}

export function getKeyGroupProvider(group: {
  platform: GroupPlatform
  key_display_category?: string
}): KeyGroupProvider {
  const category = group.key_display_category
  if (KEY_GROUP_PROVIDERS.some((provider) => provider === category)) {
    return category as KeyGroupProvider
  }
  return PROVIDER_BY_PLATFORM[group.platform] ?? 'other'
}

// Collections use representative provider marks rather than an invented brand logo.
export const KEY_GROUP_PROVIDER_ICONS: Record<KeyGroupProvider, GroupPlatform[]> = {
  anthropic: ['anthropic'],
  openai: ['openai'],
  domestic: ['deepseek', 'kimi'],
  other: ['gemini', 'grok']
}
