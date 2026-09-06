import type { AccountPlatform, GroupPlatform } from '@/types'

export interface PlatformOption<T extends string = string> {
  value: T
  label: string
  platform?: T
}

/**
 * Concrete upstream platforms supported by accounts and request routing.
 * Keep platform selectors derived from this catalog so newly added providers
 * do not silently disappear from list filters.
 */
export const CONCRETE_PLATFORM_OPTIONS = [
  { value: 'anthropic', label: 'Anthropic', platform: 'anthropic' },
  { value: 'openai', label: 'OpenAI', platform: 'openai' },
  { value: 'gemini', label: 'Gemini', platform: 'gemini' },
  { value: 'antigravity', label: 'Antigravity', platform: 'antigravity' },
  { value: 'grok', label: 'Grok', platform: 'grok' },
  { value: 'kimi', label: 'Kimi', platform: 'kimi' },
  { value: 'zhipu', label: 'Zhipu GLM', platform: 'zhipu' },
  { value: 'deepseek', label: 'DeepSeek', platform: 'deepseek' }
] as const satisfies readonly PlatformOption<AccountPlatform>[]

/** Platforms that can own a group. */
export const GROUP_PLATFORM_OPTIONS = [
  ...CONCRETE_PLATFORM_OPTIONS,
  { value: 'composite', label: 'Composite' }
] as const satisfies readonly PlatformOption<GroupPlatform>[]
