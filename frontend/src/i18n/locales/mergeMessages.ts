type Messages = Record<string, unknown>

function isMessages(value: unknown): value is Messages {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function mergeMessages<T extends Messages, U extends Messages>(base: T, fallback: U): T & U {
  const merged: Messages = { ...base }

  for (const [key, value] of Object.entries(fallback)) {
    const current = merged[key]

    if (isMessages(current) && isMessages(value)) {
      merged[key] = mergeMessages(current, value)
    } else if (current === undefined) {
      merged[key] = value
    }
  }

  return merged as T & U
}
