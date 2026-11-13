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

// True once the board holds at least one cell besides draw.io's two root
// cells (ids 0 and 1). Anything that is not a diagram has no content.
export function diagramHasContent(xml: string): boolean {
  const document = new DOMParser().parseFromString(xml, 'application/xml')
  if (document.querySelector('parsererror')) return false
  return Array.from(document.getElementsByTagName('mxCell')).some((cell) => {
    const id = cell.getAttribute('id')
    return id !== '0' && id !== '1'
  })
}

// What a diagram holds that counts as an answer: each component and
// connection with its label, ends and container. Styles and geometry are left
// out, because draw.io rewrites them when it loads a diagram. A cell wrapped
// in an object element takes its id and label from the wrapper.
function diagramSignature(xml: string): string[] {
  const document = new DOMParser().parseFromString(xml, 'application/xml')
  if (document.querySelector('parsererror')) return []
  const signature: string[] = []
  for (const cell of Array.from(document.getElementsByTagName('mxCell'))) {
    // A layer, for one, is a cell that is neither a component nor a connection.
    if (cell.getAttribute('vertex') !== '1' && cell.getAttribute('edge') !== '1') continue
    const wrapper = cell.parentElement
    const wrapped = wrapper?.tagName === 'object' || wrapper?.tagName === 'UserObject'
    const id = (wrapped ? wrapper.getAttribute('id') : cell.getAttribute('id')) ?? ''
    if (id === '0' || id === '1') continue
    const label = (wrapped ? wrapper.getAttribute('label') : cell.getAttribute('value')) ?? ''
    const kind = cell.getAttribute('edge') === '1' ? 'edge' : 'vertex'
    const ends = `${cell.getAttribute('source') ?? ''}>${cell.getAttribute('target') ?? ''}`
    signature.push([id, kind, label, ends, cell.getAttribute('parent') ?? ''].join('|'))
  }
  return signature.sort()
}

// True when the board differs from the diagram the exam started from, in its
// components or connections.
export function diagramChangedFrom(xml: string, seed: string): boolean {
  return diagramSignature(xml).join('\n') !== diagramSignature(seed).join('\n')
}

// True when the board can be submitted as the learner's answer: it has
// content and, when the exam started from a diagram, differs from it.
export function diagramIsAnswer(xml: string, seed: string): boolean {
  return diagramHasContent(xml) && (seed === '' || diagramChangedFrom(xml, seed))
}

// Where the exam goes back to when it ends: the catalog it was started from or
// the study session whose chat offered it.
export type LabReturn = { kind: 'catalog' } | { kind: 'study'; sessionId: string }

export function labReturnLabel(returnTo: LabReturn): string {
  return returnTo.kind === 'study' ? 'Back to study session' : 'Back to catalog'
}
