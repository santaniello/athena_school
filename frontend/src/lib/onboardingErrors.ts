// Wails rejects a bound-method promise with the Go error's Error() text —
// either as a plain string or wrapped in an Error, depending on version.
// These are the exact sentinel strings from internal/domain/config and
// internal/application/onboarding (see specs/phases/phase-01-desktop-mvp/04-onboarding.md).
const FALLBACK_MESSAGE = 'An error occurred. Please try again.'

function rawMessage(err: unknown): string {
  return err instanceof Error ? err.message : typeof err === 'string' ? err : ''
}

const OPENROUTER_KEY_ERROR_MESSAGES: Record<string, string> = {
  'openrouter key is required': 'Enter your OpenRouter key.',
  'openrouter key is invalid or unauthorized': 'Invalid or unauthorized key.',
}

export function openRouterKeyErrorMessage(err: unknown): string {
  return OPENROUTER_KEY_ERROR_MESSAGES[rawMessage(err)] ?? FALLBACK_MESSAGE
}

// These are the exact sentinel strings from internal/domain/profile.
const PROFILE_ERROR_MESSAGES: Record<string, string> = {
  'name is required': 'Enter your name.',
  'assistant name is required': 'Enter what you want to call the assistant.',
  'area is required': 'Enter your area of study or work.',
  'experience level must be beginner, intermediate or advanced': 'Select a valid experience level.',
  'at least one goal is required': 'Add at least one goal.',
  'study style must be direct, practical_examples or step_by_step': 'Select a valid study style.',
  'assistant language must be pt or en': 'Select a valid assistant language.',
}

export function profileErrorMessage(err: unknown): string {
  return PROFILE_ERROR_MESSAGES[rawMessage(err)] ?? FALLBACK_MESSAGE
}
