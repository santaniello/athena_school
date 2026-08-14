import { describe, expect, it } from 'vitest'
import { authErrorMessage } from './authErrors'

describe('authErrorMessage', () => {
  it('maps invalid credentials to a PT-BR message', () => {
    expect(authErrorMessage(new Error('invalid credentials'))).toBe('E-mail ou senha inválidos.')
  })

  it('maps duplicate email to a PT-BR message', () => {
    expect(authErrorMessage(new Error('email already exists'))).toBe(
      'Já existe uma conta com este e-mail.',
    )
  })

  it('maps account not found to a PT-BR message', () => {
    expect(authErrorMessage(new Error('account not found'))).toBe(
      'Nenhuma conta encontrada com este e-mail.',
    )
  })

  it('falls back to a generic PT-BR message for unknown errors', () => {
    expect(authErrorMessage(new Error('boom'))).toBe('Ocorreu um erro. Tente novamente.')
  })

  it('falls back to a generic PT-BR message for non-Error values', () => {
    expect(authErrorMessage('not an error')).toBe('Ocorreu um erro. Tente novamente.')
  })

  it('maps a plain string sentinel the same way as an Error', () => {
    // Wails rejects bound-method promises with a plain string in some versions,
    // not always an Error instance — handle both shapes.
    expect(authErrorMessage('invalid credentials')).toBe('E-mail ou senha inválidos.')
  })
})
