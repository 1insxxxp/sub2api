export interface CallNameEntry {
  key: string
  callName: string
}

export const sourceMappingKey = (sourceGroupId: number, sourceModel: string) =>
  `${sourceGroupId}:${sourceModel.trim().toLocaleLowerCase()}`

const normalizedCallName = (value: string) => value.trim().toLocaleLowerCase()

const sourceSuffix = (sourceName: string) =>
  sourceName.trim().toLocaleLowerCase().replace(/\s+/g, '-').replace(/[^\p{L}\p{N}._-]+/gu, '-').replace(/^-+|-+$/g, '') || 'source'

export const suggestCallName = (sourceModel: string, sourceName: string, existingNames: string[]) => {
  const occupied = new Set(existingNames.map(normalizedCallName))
  if (!occupied.has(normalizedCallName(sourceModel))) return sourceModel

  const base = `${sourceModel}-${sourceSuffix(sourceName)}`
  if (!occupied.has(normalizedCallName(base))) return base
  let index = 2
  while (occupied.has(normalizedCallName(`${base}-${index}`))) index += 1
  return `${base}-${index}`
}

export const validateCallNames = (entries: CallNameEntry[]) => {
  const errors = new Map<string, string>()
  const grouped = new Map<string, string[]>()
  for (const entry of entries) {
    const trimmed = entry.callName.trim()
    if (!trimmed) errors.set(entry.key, '调用名称不能为空')
    else if (trimmed.length > 200) errors.set(entry.key, '调用名称不能超过 200 个字符')
    const normalized = normalizedCallName(trimmed)
    if (normalized) grouped.set(normalized, [...(grouped.get(normalized) || []), entry.key])
  }
  for (const keys of grouped.values()) {
    if (keys.length > 1) keys.forEach(key => errors.set(key, '调用名称不能重复（忽略大小写）'))
  }
  return errors
}
