// Wails rejects a bound-method promise with the Go error's Error() text —
// either as a plain string or wrapped in an Error, depending on version.
// These are the exact sentinel strings from internal/domain/auth and
// internal/application/auth (see specs/phases/phase-01-desktop-mvp/02-auth-ui.md).
const KNOWN_ERROR_MESSAGES: Record<string, string> = {
  'invalid credentials': 'E-mail ou senha inválidos.',
  'email already exists': 'Já existe uma conta com este e-mail.',
  'account not found': 'Nenhuma conta encontrada com este e-mail.',
}

const FALLBACK_MESSAGE = 'Ocorreu um erro. Tente novamente.'

export function authErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : ''
  return KNOWN_ERROR_MESSAGES[raw] ?? FALLBACK_MESSAGE
}
