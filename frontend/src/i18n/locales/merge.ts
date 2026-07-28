export type LocaleMessages = Record<string, unknown>

function isMessageObject(value: unknown): value is LocaleMessages {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

export function mergeLocaleMessages(
  base: LocaleMessages,
  additions: LocaleMessages
): LocaleMessages {
  const merged: LocaleMessages = { ...base }

  for (const [key, value] of Object.entries(additions)) {
    const current = merged[key]
    merged[key] = isMessageObject(current) && isMessageObject(value)
      ? mergeLocaleMessages(current, value)
      : value
  }

  return merged
}
