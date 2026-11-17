import { describe, expect, it } from 'vitest'
import i18n, { setLabLanguage, toLabLanguage } from '@/i18n'
import { en } from './locales/en'
import { pt } from './locales/pt'
import { quizEn } from './locales/quiz-en'
import { quizPt } from './locales/quiz-pt'

// The dotted path of every string of a locale.
function keysOf(tree: object, prefix = ''): string[] {
  return Object.entries(tree).flatMap(([key, value]) =>
    typeof value === 'object' && value !== null
      ? keysOf(value, `${prefix}${key}.`)
      : [`${prefix}${key}`],
  )
}

// A {{placeholder}} the string uses, in order.
function placeholdersOf(text: string): string[] {
  return [...text.matchAll(/{{(\w+)}}/g)].map((match) => match[1]).sort()
}

function leaves(tree: object, prefix = ''): [string, string][] {
  return Object.entries(tree).flatMap(([key, value]): [string, string][] =>
    typeof value === 'object' && value !== null
      ? leaves(value, `${prefix}${key}.`)
      : [[`${prefix}${key}`, String(value)]],
  )
}

describe('locales', () => {
  it('have the same keys in English and Portuguese', () => {
    expect(keysOf(pt).sort()).toEqual(keysOf(en).sort())
  })

  it('use the same placeholders in both languages', () => {
    const english = new Map(leaves(en))
    for (const [key, text] of leaves(pt)) {
      expect(placeholdersOf(text), key).toEqual(placeholdersOf(english.get(key) ?? ''))
    }
  })
})

describe('quiz locales', () => {
  it('have the same keys in English and Portuguese', () => {
    expect(keysOf(quizPt).sort()).toEqual(keysOf(quizEn).sort())
  })

  it('use the same placeholders in both languages', () => {
    const english = new Map(leaves(quizEn))
    for (const [key, text] of leaves(quizPt)) {
      expect(placeholdersOf(text), key).toEqual(placeholdersOf(english.get(key) ?? ''))
    }
  })

  it('follow the profile language through the quiz namespace', () => {
    expect(i18n.t('catalog.title', { ns: 'quiz' })).toBe('Interactive Test')
    setLabLanguage('pt')
    expect(i18n.t('catalog.title', { ns: 'quiz' })).toBe('Teste Interativo')
  })
})

describe('toLabLanguage', () => {
  it('maps the profile language, and anything else to English', () => {
    expect(toLabLanguage('pt')).toBe('pt')
    expect(toLabLanguage('en')).toBe('en')
    expect(toLabLanguage('')).toBe('en')
    expect(toLabLanguage(undefined)).toBe('en')
    expect(toLabLanguage('fr')).toBe('en')
  })
})

describe('setLabLanguage', () => {
  it('switches the language and back', () => {
    setLabLanguage('pt')
    expect(i18n.t('common.cancel')).toBe('Cancelar')

    setLabLanguage('en')
    expect(i18n.t('common.cancel')).toBe('Cancel')
  })

  it('goes back to English when the profile has no language', () => {
    setLabLanguage('pt')

    setLabLanguage(undefined)

    expect(i18n.language).toBe('en')
  })
})
