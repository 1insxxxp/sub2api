import { GROUP_TAG_PRESETS, isGroupTagColor } from './groupTag'

export interface ReusableGroupTagOption {
  tag: string
  color: string
  count: number
}

export interface GroupTagSource {
  tag?: string | null
  tag_color?: string | null
}

/**
 * Builds the custom tag choices shown in group forms.
 *
 * Preset tags remain in their dedicated preset list. Custom labels are
 * deduplicated case-insensitively so groups can reuse one label even when
 * older records differ only in casing or whitespace.
 */
export function buildReusableGroupTagOptions(
  groups: ReadonlyArray<GroupTagSource>,
): ReusableGroupTagOption[] {
  const options = new Map<string, ReusableGroupTagOption>()

  for (const group of groups) {
    const tag = group.tag?.trim() ?? ''
    if (!tag || GROUP_TAG_PRESETS.includes(tag as (typeof GROUP_TAG_PRESETS)[number])) continue

    const key = tag.toLocaleLowerCase()
    const color = isGroupTagColor(group.tag_color ?? '') ? group.tag_color!.toUpperCase() : ''
    const existing = options.get(key)
    if (!existing) {
      options.set(key, { tag, color, count: 1 })
      continue
    }

    existing.count += 1
    if (!existing.color && color) existing.color = color
  }

  return [...options.values()].sort((left, right) =>
    right.count - left.count || left.tag.localeCompare(right.tag, 'zh-CN'),
  )
}
