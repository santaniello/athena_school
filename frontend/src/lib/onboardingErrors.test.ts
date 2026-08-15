import { describe, expect, it } from 'vitest'
import { openRouterKeyErrorMessage, profileErrorMessage } from './onboardingErrors'

describe('openRouterKeyErrorMessage', () => {
  it('maps the key-required sentinel to an English message', () => {
    // Given the Go sentinel for a blank key
    const err = new Error('openrouter key is required')

    // When mapping it
    // Then it returns the English message
    expect(openRouterKeyErrorMessage(err)).toBe('Enter your OpenRouter key.')
  })

  it('maps the key-invalid sentinel to an English message', () => {
    // Given the Go sentinel for a rejected key
    const err = new Error('openrouter key is invalid or unauthorized')

    // When mapping it
    // Then it returns the English message
    expect(openRouterKeyErrorMessage(err)).toBe('Invalid or unauthorized key.')
  })

  it('falls back to a generic message for unknown errors', () => {
    // Given an error not in the known map (e.g. a network failure)
    const err = new Error('openrouter: calling key validation endpoint: dial tcp: timeout')

    // When mapping it
    // Then it returns the generic fallback
    expect(openRouterKeyErrorMessage(err)).toBe('An error occurred. Please try again.')
  })
})

describe('profileErrorMessage', () => {
  it('maps each domain validation sentinel to an English message', () => {
    // Given every sentinel returned by profile.UserProfile.Validate
    const cases: Array<[string, string]> = [
      ['name is required', 'Enter your name.'],
      ['assistant name is required', 'Enter what you want to call the assistant.'],
      ['area is required', 'Enter your area of study or work.'],
      [
        'experience level must be beginner, intermediate or advanced',
        'Select a valid experience level.',
      ],
      ['at least one goal is required', 'Add at least one goal.'],
      [
        'study style must be direct, practical_examples or step_by_step',
        'Select a valid study style.',
      ],
      ['assistant language must be pt or en', 'Select a valid assistant language.'],
    ]

    // When mapping each one
    // Then each returns its corresponding English message
    for (const [raw, expected] of cases) {
      expect(profileErrorMessage(new Error(raw))).toBe(expected)
    }
  })

  it('falls back to a generic message for unknown errors', () => {
    // Given an error not in the known map (e.g. a disk failure)
    const err = new Error('onboarding: saving profile: disk full')

    // When mapping it
    // Then it returns the generic fallback
    expect(profileErrorMessage(err)).toBe('An error occurred. Please try again.')
  })
})
