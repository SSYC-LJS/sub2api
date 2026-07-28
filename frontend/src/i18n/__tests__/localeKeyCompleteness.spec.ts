import { beforeAll, describe, expect, it } from 'vitest'

import { i18n, loadLocaleMessages } from '@/i18n'

type LocaleMessages = Record<string, unknown>

const sourceModules = import.meta.glob('../../**/*.{ts,vue}', {
  eager: true,
  import: 'default',
  query: '?raw'
}) as Record<string, string>

const ignoredSourcePatterns = [
  '/__tests__/',
  '/i18n/locales/',
  /\.(?:spec|test)\.ts$/
]

function flattenMessageKeys(messages: LocaleMessages, prefix = ''): Set<string> {
  const keys = new Set<string>()

  for (const [name, value] of Object.entries(messages)) {
    const key = prefix ? `${prefix}.${name}` : name

    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      for (const nestedKey of flattenMessageKeys(value as LocaleMessages, key)) {
        keys.add(nestedKey)
      }
    } else {
      keys.add(key)
    }
  }

  return keys
}

function referencedLocaleKeys(): Map<string, Set<string>> {
  const references = new Map<string, Set<string>>()
  const literalTranslationCall = /(?:\$t|\bt)\(\s*(['"`])([A-Za-z0-9_.-]+)\1\s*(?=[,)])/g

  for (const [file, source] of Object.entries(sourceModules)) {
    if (ignoredSourcePatterns.some((pattern) =>
      typeof pattern === 'string' ? file.includes(pattern) : pattern.test(file)
    )) {
      continue
    }

    for (const match of source.matchAll(literalTranslationCall)) {
      const key = match[2]
      const files = references.get(key) ?? new Set<string>()
      files.add(file.replace(/^\.\.\/\.\.\//, 'src/'))
      references.set(key, files)
    }
  }

  return references
}

function missingReferences(messages: LocaleMessages): string[] {
  const availableKeys = flattenMessageKeys(messages)

  return [...referencedLocaleKeys()]
    .filter(([key]) => !availableKeys.has(key))
    .map(([key, files]) => `${key} (${[...files].sort().join(', ')})`)
    .sort()
}

describe('runtime locale key completeness', () => {
  beforeAll(async () => {
    await Promise.all([
      loadLocaleMessages('en'),
      loadLocaleMessages('zh')
    ])
  })

  it.each(['en', 'zh'] as const)('%s contains every statically referenced translation key', (locale) => {
    const messages = i18n.global.getLocaleMessage(locale) as LocaleMessages
    expect(missingReferences(messages)).toEqual([])
  })
})
