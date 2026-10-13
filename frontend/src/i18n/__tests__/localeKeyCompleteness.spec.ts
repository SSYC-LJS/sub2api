import { beforeAll, describe, expect, it } from 'vitest'

import { i18n, loadLocaleMessages } from '@/i18n'
import en from '../locales/en'
import zh from '../locales/zh'

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

  it.each([
    ['en', 'Per-user token usage for the current filters and time range'],
    ['zh', '按当前筛选与时间范围统计每个用户的 Token 用量']
  ] as const)('resolves the reported token ranking subtitle from the %s runtime locale', (locale, expected) => {
    const previousLocale = i18n.global.locale.value
    i18n.global.locale.value = locale

    expect(i18n.global.t('admin.usage.tokenRanking.subtitle')).toBe(expected)

    i18n.global.locale.value = previousLocale
  })
})

type LocaleValue = Record<string, unknown>

function flattenLeafKeys(value: unknown, prefix = ''): string[] {
  if (value === null || typeof value !== 'object' || Array.isArray(value)) {
    return prefix ? [prefix] : []
  }

  return Object.entries(value as LocaleValue).flatMap(([key, child]) => {
    const path = prefix ? `${prefix}.${key}` : key
    return flattenLeafKeys(child, path)
  })
}

function collectStaticSourceKeys(source: string): string[] {
  const keys = new Set<string>()

  // Covers useI18n().t(), the global $t(), and direct i18n.t() calls in both
  // TypeScript and Vue script/template source. Dynamic suffixes are checked by
  // the runtime type of the value and cannot be proven from source text alone.
  const translationCalls = /(?:\bi18n\.t|\$t|\bt)\s*\(\s*(['"])([^'"\r\n]+)\1/g
  for (const match of source.matchAll(translationCalls)) {
    const key = match[2]
    if (!key.endsWith('.')) {
      keys.add(key)
    }
  }

  // Router metadata and i18n-t are key references without a t() call.
  const keyReferences = /(?:keypath|titleKey|descriptionKey)\s*[:=]\s*(['"])([^'"\r\n]+)\1/g
  for (const match of source.matchAll(keyReferences)) {
    keys.add(match[2])
  }

  const i18nTKeypaths = /<i18n-t\b[^>]*\bkeypath\s*=\s*(['"])([^'"\r\n]+)\1/gi
  for (const match of source.matchAll(i18nTKeypaths)) {
    keys.add(match[2])
  }

  return [...keys]
}

function sourceKeys(): string[] {
  const sourceFiles = import.meta.glob('../../**/*.{ts,vue}', {
    query: '?raw',
    import: 'default',
    eager: true
  }) as Record<string, string>

  return Object.entries(sourceFiles)
    .filter(([path]) => !path.includes('/__tests__/') && !/\.(spec|test)\.ts$/.test(path))
    .flatMap(([, source]) => collectStaticSourceKeys(source))
}

function missingKeys(usedKeys: string[], availableKeys: Set<string>): string[] {
  return usedKeys.filter((key) => !availableKeys.has(key)).sort()
}

describe('locale key completeness', () => {
  const enKeys = new Set(flattenLeafKeys(en))
  const zhKeys = new Set(flattenLeafKeys(zh))
  const usedKeys = [...new Set(sourceKeys())].sort()

  it('keeps English and Chinese locale schemas identical', () => {
    expect([...enKeys].filter((key) => !zhKeys.has(key)).sort()).toEqual([])
    expect([...zhKeys].filter((key) => !enKeys.has(key)).sort()).toEqual([])
  })

  it('contains a non-empty message for every locale leaf', () => {
    for (const [locale, messages] of Object.entries({ en, zh })) {
      const emptyKeys = flattenLeafKeys(messages).filter((key) => {
        let current: unknown = messages
        for (const segment of key.split('.')) {
          current = (current as LocaleValue)[segment]
        }
        return typeof current !== 'string' || current.trim() === ''
      })
      expect(emptyKeys, `${locale} has empty or non-string messages`).toEqual([])
    }
  })

  it('contains every statically referenced production key', () => {
    expect(missingKeys(usedKeys, enKeys), 'English locale is missing referenced keys').toEqual([])
    expect(missingKeys(usedKeys, zhKeys), 'Chinese locale is missing referenced keys').toEqual([])
  })
})