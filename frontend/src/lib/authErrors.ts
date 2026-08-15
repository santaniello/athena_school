// Wails rejects a bound-method promise with the Go error's Error() text —
// either as a plain string or wrapped in an Error, depending on version.
// These are the exact sentinel strings from internal/domain/auth and
// internal/application/auth (see specs/phases/phase-01-desktop-mvp/02-auth-ui.md).
const KNOWN_ERROR_MESSAGES: Record<string, string> = {
  'invalid credentials': 'Invalid email or password.',
  'email already exists': 'An account with this email already exists.',
  'account not found': 'No account found with this email.',
}

const FALLBACK_MESSAGE = 'An error occurred. Please try again.'

export function authErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : ''
  return KNOWN_ERROR_MESSAGES[raw] ?? FALLBACK_MESSAGE
}
