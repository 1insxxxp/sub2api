export const GROUP_TAG_PRESETS = ['chat', 'image', 'airp'] as const
export const GROUP_TAG_COLORS = ['#0EA5E9', '#10B981', '#F43F5E', '#8B5CF6', '#F59E0B', '#475569']

export const isGroupTagPreset = (tag: string): tag is typeof GROUP_TAG_PRESETS[number] =>
  GROUP_TAG_PRESETS.some(preset => preset === tag)

export const isGroupTagColor = (color: string) => /^#[0-9a-f]{6}$/i.test(color)

export function groupTagColor(tag: string, color = ''): string {
  if (isGroupTagColor(color)) return color.toUpperCase()
  if (tag === 'image') return '#10B981'
  if (tag === 'airp') return '#F43F5E'
  return '#0EA5E9'
}

export function groupTagStyle(tag: string, color = '') {
  const hex = groupTagColor(tag, color)
  const rgb = [1, 3, 5].map(offset => parseInt(hex.slice(offset, offset + 2), 16))
  const linear = rgb.map(channel => {
    const value = channel / 255
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4
  })
  const luminance = 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2]
  return {
    backgroundColor: `rgba(${rgb.join(', ')}, 0.92)`,
    color: luminance > 0.18 ? '#111827' : '#FFFFFF',
    boxShadow: `0 2px 6px rgba(${rgb.join(', ')}, 0.2)`,
  }
}

export function groupTagSoftStyle(tag: string, color = '') {
  const hex = groupTagColor(tag, color)
  const rgb = [1, 3, 5].map(offset => parseInt(hex.slice(offset, offset + 2), 16))
  return {
    '--tag-background': `rgba(${rgb.join(', ')}, 0.12)`,
    '--tag-border': `rgba(${rgb.join(', ')}, 0.18)`,
    '--tag-edge': `rgba(${rgb.join(', ')}, 0.4)`,
    '--tag-shadow': `rgba(${rgb.join(', ')}, 0.14)`,
    '--tag-hover': `rgba(${rgb.join(', ')}, 0.2)`,
    '--tag-text': `rgb(${rgb.map(channel => Math.round(channel * 0.42)).join(', ')})`,
    '--tag-text-dark': `rgb(${rgb.map(channel => Math.round(channel * 0.45 + 140)).join(', ')})`,
  }
}
