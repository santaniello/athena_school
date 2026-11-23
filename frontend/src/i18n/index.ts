import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import { en } from './locales/en'
import { pt } from './locales/pt'
import { mindmapEn } from './locales/mindmap-en'
import { mindmapPt } from './locales/mindmap-pt'
import { quizEn } from './locales/quiz-en'
import { quizPt } from './locales/quiz-pt'
import { progressEn } from './locales/progress-en'
import { progressPt } from './locales/progress-pt'
import { progressTabEn } from './locales/progress-tab-en'
import { progressTabPt } from './locales/progress-tab-pt'
import { materialsEn } from './locales/materials-en'
import { materialsPt } from './locales/materials-pt'

// The Architecture Lab speaks the profile's assistant language. The rest of
// the interface is English and does not go through i18next.
export const LAB_NAMESPACE = 'lab'
// The Interactive Quiz has its own namespace and the same language switch.
export const QUIZ_NAMESPACE = 'quiz'
// The Mind Map has its own namespace and the same language switch.
export const MINDMAP_NAMESPACE = 'mindmap'
// The Session progress dialog has its own namespace and the same language switch.
export const PROGRESS_NAMESPACE = 'progress'
// The Progress screen has its own namespace and the same language switch.
export const PROGRESS_TAB_NAMESPACE = 'progressTab'
// The Session materials panel has its own namespace and the same language switch.
export const MATERIALS_NAMESPACE = 'materials'
export type LabLanguage = 'en' | 'pt'

// The profile stores 'pt' or 'en'. Anything else, an unset value included,
// is English.
export function toLabLanguage(language: string | undefined): LabLanguage {
  return language === 'pt' ? 'pt' : 'en'
}

void i18n.use(initReactI18next).init({
  resources: {
    en: {
      [LAB_NAMESPACE]: en,
      [QUIZ_NAMESPACE]: quizEn,
      [MINDMAP_NAMESPACE]: mindmapEn,
      [PROGRESS_NAMESPACE]: progressEn,
      [PROGRESS_TAB_NAMESPACE]: progressTabEn,
      [MATERIALS_NAMESPACE]: materialsEn,
    },
    pt: {
      [LAB_NAMESPACE]: pt,
      [QUIZ_NAMESPACE]: quizPt,
      [MINDMAP_NAMESPACE]: mindmapPt,
      [PROGRESS_NAMESPACE]: progressPt,
      [PROGRESS_TAB_NAMESPACE]: progressTabPt,
      [MATERIALS_NAMESPACE]: materialsPt,
    },
  },
  lng: 'en',
  fallbackLng: 'en',
  ns: [
    LAB_NAMESPACE,
    QUIZ_NAMESPACE,
    MINDMAP_NAMESPACE,
    PROGRESS_NAMESPACE,
    PROGRESS_TAB_NAMESPACE,
    MATERIALS_NAMESPACE,
  ],
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
