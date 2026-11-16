# Phase 3.20 — Architecture Lab Language

## Goal

Stop the Architecture Lab from mixing languages. The content of a challenge (title, scenario,
requirements, rubric, hints, defense questions, evaluation) is written by the model in the profile's
assistant language, but every label around it was hardcoded English. A Portuguese profile opened a
challenge and saw a Portuguese title between English headings, and the Start Exam dialog showed
English rules under a Portuguese title.

## Decisions

1. **Where the language follows the profile.** Everything inside the Architecture Lab: the catalog,
   the generation dialog, the Start Exam dialog, the exam screen, the result and history screens,
   and the exercise offer card in the study chat. The menu, navigation, Settings and the other
   screens stay in English.
2. **i18next + react-i18next.** One namespace, `lab`, with two locales:
   `frontend/src/i18n/locales/en.ts` and `pt.ts`. The Portuguese file is typed against the English
   one, so a missing key does not compile. A test also checks the same keys and placeholders.
3. **The language comes from the profile.** `assistantLanguage` (`pt` or `en`) is read by
   `getUserProfile` and written by `saveUserProfile` and `updateUserProfile`, and each one calls
   `setLabLanguage`. Changing the language in Settings applies at once. Anything that is not `pt`,
   an unset value included, is English.
4. **Pure helpers read the active language.** `kindLabel`, `statusLabel`, `difficultyLabel`,
   `formatTimeLimit`, the error message helpers, `formatSessionDate` and `labReturnLabel` call
   `i18n.t`. The label maps (`KIND_LABELS`, ...) became lists of values plus these functions. An
   unknown value from the backend is shown as it came.
5. **"I don't know" keeps its literal.** The button shows a translated label, but it sends
   `DONT_KNOW_ANSWER` (`"I don't know"`), which the domain compares. No change in Go.
6. **`offerLabels` is gone.** The offer card uses the same dictionary. `OfferProfile` no longer
   carries the language.
7. **Existing challenges are left alone.** A challenge keeps the language it was generated in. No
   language column, no migration, no warning when it differs from the profile.

## Out of scope

The menu and other screens, a language field on the challenge, translating `IDontKnowAnswer` in the
domain, and merging the duplicated `languageName` helpers in the Go prompts.
