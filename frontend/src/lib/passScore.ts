import { GetPassScore, SetPassScore } from '../../wailsjs/go/desktop/App'

// The minimum score a challenge attempt needs to pass, set in Settings. The
// rule itself lives in the backend; see
// specs/phases/phase-03-architecture-lab/06-completion-rules.md.
export function getPassScore(): Promise<number> {
  return GetPassScore()
}

export function setPassScore(score: number): Promise<void> {
  return SetPassScore(score)
}

export const PASS_SCORE_RANGE_MESSAGE = 'The minimum passing score must be between 50 and 100.'

const FALLBACK_MESSAGE = 'An error occurred. Please try again.'

// Wails rejects with the Go error's text, as a string or wrapped in an Error.
// The text matches ErrInvalidPassScore in internal/domain/exercise.
export function passScoreErrorMessage(err: unknown): string {
  const raw = err instanceof Error ? err.message : typeof err === 'string' ? err : ''
  if (raw.includes('minimum passing score must be between')) return PASS_SCORE_RANGE_MESSAGE
  return FALLBACK_MESSAGE
}
