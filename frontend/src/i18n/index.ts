import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { en } from './locales/en'
import { pt } from './locales/pt'
import { quizEn } from './locales/quiz-en'
import { quizPt } from './locales/quiz-pt'

// The Architecture Lab speaks the profile's assistant language. The rest of
// the interface is English and does not go through i18next.
export const LAB_NAMESPACE = 'lab'
// The Interactive Quiz has its own namespace and the same language switch.
export const QUIZ_NAMESPACE = 'quiz'
export type LabLanguage = 'en' | 'pt'

// The profile stores 'pt' or 'en'. Anything else, an unset value included,
// is English.
export function toLabLanguage(language: string | undefined): LabLanguage {
  return language === 'pt' ? 'pt' : 'en'
}

void i18n.use(initReactI18next).init({
  resources: {
    en: { [LAB_NAMESPACE]: en, [QUIZ_NAMESPACE]: quizEn },
    pt: { [LAB_NAMESPACE]: pt, [QUIZ_NAMESPACE]: quizPt },
  },
  lng: 'en',
  fallbackLng: 'en',
  ns: [LAB_NAMESPACE, QUIZ_NAMESPACE],
  defaultNS: LAB_NAMESPACE,
  initAsync: false,
  interpolation: { escapeValue: false },
})

// Follows the profile's assistant language.
export function setLabLanguage(language: string | undefined): void {
  const next = toLabLanguage(language)
  if (i18n.language !== next) void i18n.changeLanguage(next)
}

export default i18n
