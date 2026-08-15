// Wails rejects a bound-method promise with the Go error's Error() text —
// either as a plain string or wrapped in an Error, depending on version.
// These are the exact sentinel strings from internal/domain/config and
// internal/application/onboarding (see specs/phases/phase-01-desktop-mvp/04-onboarding.md).
const FALLBACK_MESSAGE = 'Ocorreu um erro. Tente novamente.'

function rawMessage(err: unknown): string {
  return err instanceof Error ? err.message : typeof err === 'string' ? err : ''
}

const OPENROUTER_KEY_ERROR_MESSAGES: Record<string, string> = {
  'openrouter key is required': 'Informe sua chave da OpenRouter.',
  'openrouter key is invalid or unauthorized': 'Chave inválida ou não autorizada.',
}

export function openRouterKeyErrorMessage(err: unknown): string {
  return OPENROUTER_KEY_ERROR_MESSAGES[rawMessage(err)] ?? FALLBACK_MESSAGE
}

// These are the exact sentinel strings from internal/domain/profile.
const PROFILE_ERROR_MESSAGES: Record<string, string> = {
  'name is required': 'Informe seu nome.',
  'assistant name is required': 'Informe como quer chamar o assistente.',
  'area is required': 'Informe sua área de atuação ou estudo.',
  'specialty is required': 'Informe seu foco específico.',
  'experience level must be beginner, intermediate or advanced':
    'Selecione um nível de experiência válido.',
  'at least one goal is required': 'Adicione pelo menos um objetivo.',
  'study style is required': 'Informe seu estilo de estudo preferido.',
}

export function profileErrorMessage(err: unknown): string {
  return PROFILE_ERROR_MESSAGES[rawMessage(err)] ?? FALLBACK_MESSAGE
}
