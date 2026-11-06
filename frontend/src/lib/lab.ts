// Types and pure helpers of the Architecture Lab exam screen. The screen only
// renders what its caller passes in. See
// specs/phases/phase-03-architecture-lab/08-lab-layout-sketch.md.

const SECOND_MS = 1000
const MINUTE_MS = 60 * SECOND_MS

export const WARNING_THRESHOLD_MS = 5 * MINUTE_MS
export const URGENT_THRESHOLD_MS = 1 * MINUTE_MS

export type CountdownTone = 'normal' | 'warning' | 'urgent'

function pad(value: number): string {
  return String(value).padStart(2, '0')
}

// mm:ss under an hour, h:mm:ss from an hour up. A partial second counts as a
// whole one, so the clock reads 00:00 only once the deadline has really passed.
export function formatCountdown(remainingMs: number): string {
  const totalSeconds = Math.max(0, Math.ceil(remainingMs / SECOND_MS))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  const seconds = totalSeconds % 60
  if (hours > 0) return `${hours}:${pad(minutes)}:${pad(seconds)}`
  return `${pad(minutes)}:${pad(seconds)}`
}

export function countdownTone(remainingMs: number): CountdownTone {
  if (remainingMs <= URGENT_THRESHOLD_MS) return 'urgent'
  if (remainingMs <= WARNING_THRESHOLD_MS) return 'warning'
  return 'normal'
}

// The exam gives out at most this many hints.
export const MAX_HINTS = 3

// What the statement panel shows. Never the rubric or any hint.
export interface LabStatement {
  scenario: string
  functionalRequirements: string[]
  nonFunctionalRequirements: string[]
  constraints: string[]
  objectives: string[]
}

// One rubric row while the exam runs: no score, level or percentage.
export interface LabCriterion {
  id: string
  name: string
  description: string
  weight: number
  mandatory: boolean
}

export interface LabHint {
  id: string
  text: string
}
