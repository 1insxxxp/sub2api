export interface AvailableChannelBrand {
  key: 'openai' | 'anthropic' | 'gemini' | 'grok' | 'generic'
  label: string
  accentClass: string
  surfaceClass: string
  borderClass: string
}

const brands: Record<AvailableChannelBrand['key'], AvailableChannelBrand> = {
  openai: { key: 'openai', label: 'OpenAI', accentClass: 'text-emerald-700 dark:text-emerald-300', surfaceClass: 'bg-emerald-100 dark:bg-emerald-500/15', borderClass: 'border-emerald-200 dark:border-emerald-500/30' },
  anthropic: { key: 'anthropic', label: 'Anthropic', accentClass: 'text-orange-700 dark:text-orange-300', surfaceClass: 'bg-orange-100 dark:bg-orange-500/15', borderClass: 'border-orange-200 dark:border-orange-500/30' },
  gemini: { key: 'gemini', label: 'Gemini', accentClass: 'text-blue-700 dark:text-blue-300', surfaceClass: 'bg-blue-100 dark:bg-blue-500/15', borderClass: 'border-blue-200 dark:border-blue-500/30' },
  grok: { key: 'grok', label: 'Grok', accentClass: 'text-slate-800 dark:text-slate-200', surfaceClass: 'bg-slate-100 dark:bg-slate-500/15', borderClass: 'border-slate-200 dark:border-slate-500/30' },
  generic: { key: 'generic', label: 'AI', accentClass: 'text-violet-700 dark:text-violet-300', surfaceClass: 'bg-violet-100 dark:bg-violet-500/15', borderClass: 'border-violet-200 dark:border-violet-500/30' },
}

export function resolveAvailableChannelBrand(platform: string): AvailableChannelBrand {
  const value = platform.trim().toLowerCase()
  if (value.includes('openai') || value.includes('codex')) return brands.openai
  if (value.includes('anthropic') || value.includes('claude')) return brands.anthropic
  if (value.includes('gemini') || value.includes('google')) return brands.gemini
  if (value.includes('grok') || value.includes('xai')) return brands.grok
  return brands.generic
}
