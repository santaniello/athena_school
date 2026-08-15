import { describe, expect, it } from 'vitest'
import { authErrorMessage } from './authErrors'

describe('authErrorMessage', () => {
  it('maps invalid credentials to an English message', () => {
    expect(authErrorMessage(new Error('invalid credentials'))).toBe('Invalid email or password.')
  })

  it('maps duplicate email to an English message', () => {
    expect(authErrorMessage(new Error('email already exists'))).toBe(
      'An account with this email already exists.',
    )
  })

  it('maps account not found to an English message', () => {
    expect(authErrorMessage(new Error('account not found'))).toBe(
      'No account found with this email.',
    )
  })

  it('falls back to a generic English message for unknown errors', () => {
    expect(authErrorMessage(new Error('boom'))).toBe('An error occurred. Please try again.')
  })

  it('falls back to a generic English message for non-Error values', () => {
    expect(authErrorMessage('not an error')).toBe('An error occurred. Please try again.')
  })

  it('maps a plain string sentinel the same way as an Error', () => {
    // Wails rejects bound-method promises with a plain string in some versions,
    // not always an Error instance — handle both shapes.
    expect(authErrorMessage('invalid credentials')).toBe('Invalid email or password.')
  })
})
