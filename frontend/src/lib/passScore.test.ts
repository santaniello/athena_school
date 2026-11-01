import { beforeEach, describe, expect, it, vi } from 'vitest'
import { GetPassScore, SetPassScore } from '../../wailsjs/go/desktop/App'
import { getPassScore, passScoreErrorMessage, setPassScore } from './passScore'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetPassScore: vi.fn(),
  SetPassScore: vi.fn(),
}))

describe('passScore', () => {
  beforeEach(() => vi.clearAllMocks())

  it('reads the score through the GetPassScore binding', async () => {
    vi.mocked(GetPassScore).mockResolvedValueOnce(65)

    await expect(getPassScore()).resolves.toBe(65)
  })

  it('saves the score through the SetPassScore binding', async () => {
    vi.mocked(SetPassScore).mockResolvedValueOnce()

    await setPassScore(90)

    expect(SetPassScore).toHaveBeenCalledWith(90)
  })

  describe('passScoreErrorMessage', () => {
    it('explains the allowed range for the backend validation error', () => {
      const message = 'minimum passing score must be between 50 and 100'

      expect(passScoreErrorMessage(new Error(message))).toBe(
        'The minimum passing score must be between 50 and 100.',
      )
      expect(passScoreErrorMessage(message)).toBe(
        'The minimum passing score must be between 50 and 100.',
      )
    })

    it('falls back to a generic message for anything else', () => {
      expect(passScoreErrorMessage(new Error('disk on fire'))).toBe(
        'An error occurred. Please try again.',
      )
      expect(passScoreErrorMessage(undefined)).toBe('An error occurred. Please try again.')
    })
  })
})
