export interface ProfileOption {
  value: string
  label: string
}

// Shared option lists for the profile's enum-like fields, used by the
// onboarding form/confirm screens and the settings screen — see
// specs/phases/phase-01-desktop-mvp/08-settings.md.
export const EXPERIENCE_LEVELS: ProfileOption[] = [
  { value: 'beginner', label: 'Beginner' },
  { value: 'intermediate', label: 'Intermediate' },
  { value: 'advanced', label: 'Advanced' },
]

export const ASSISTANT_LANGUAGES: ProfileOption[] = [
  { value: 'pt', label: 'Portuguese' },
  { value: 'en', label: 'English' },
]

export const STUDY_STYLES: ProfileOption[] = [
  { value: 'direct', label: 'Direct and to the point' },
  { value: 'practical_examples', label: 'Lots of practical examples' },
  { value: 'step_by_step', label: 'Detailed step by step' },
]

export function labelFor(options: ProfileOption[], value: string): string {
  return options.find((option) => option.value === value)?.label ?? value
}
