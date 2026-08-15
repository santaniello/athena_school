import { describe, expect, it } from 'vitest'
import { openRouterKeyErrorMessage, profileErrorMessage } from './onboardingErrors'

describe('openRouterKeyErrorMessage', () => {
  it('maps the key-required sentinel to a PT-BR message', () => {
    // Given the Go sentinel for a blank key
    const err = new Error('openrouter key is required')

    // When mapping it
    // Then it returns the PT-BR message
    expect(openRouterKeyErrorMessage(err)).toBe('Informe sua chave da OpenRouter.')
  })

  it('maps the key-invalid sentinel to a PT-BR message', () => {
    // Given the Go sentinel for a rejected key
    const err = new Error('openrouter key is invalid or unauthorized')

    // When mapping it
    // Then it returns the PT-BR message
    expect(openRouterKeyErrorMessage(err)).toBe('Chave inválida ou não autorizada.')
  })

  it('falls back to a generic message for unknown errors', () => {
    // Given an error not in the known map (e.g. a network failure)
    const err = new Error('openrouter: calling key validation endpoint: dial tcp: timeout')

    // When mapping it
    // Then it returns the generic fallback
    expect(openRouterKeyErrorMessage(err)).toBe('Ocorreu um erro. Tente novamente.')
  })
})

describe('profileErrorMessage', () => {
  it('maps each domain validation sentinel to a PT-BR message', () => {
    // Given every sentinel returned by profile.UserProfile.Validate
    const cases: Array<[string, string]> = [
      ['name is required', 'Informe seu nome.'],
      ['assistant name is required', 'Informe como quer chamar o assistente.'],
      ['area is required', 'Informe sua área de atuação ou estudo.'],
      [
        'experience level must be beginner, intermediate or advanced',
        'Selecione um nível de experiência válido.',
      ],
      ['at least one goal is required', 'Adicione pelo menos um objetivo.'],
      ['study style is required', 'Informe seu estilo de estudo preferido.'],
    ]

    // When mapping each one
    // Then each returns its corresponding PT-BR message
    for (const [raw, expected] of cases) {
      expect(profileErrorMessage(new Error(raw))).toBe(expected)
    }
  })

  it('falls back to a generic message for unknown errors', () => {
    // Given an error not in the known map (e.g. a disk failure)
    const err = new Error('onboarding: saving profile: disk full')

    // When mapping it
    // Then it returns the generic fallback
    expect(profileErrorMessage(err)).toBe('Ocorreu um erro. Tente novamente.')
  })
})
