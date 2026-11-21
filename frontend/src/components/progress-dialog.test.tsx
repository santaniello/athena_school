import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import { getStudySessionProgress, type StudyProgress, type StudyProgressConcept } from '@/lib/study'
import { setLabLanguage } from '@/i18n'
import { ProgressDialog } from './progress-dialog'

vi.mock('@/lib/study', () => ({
  getStudySessionProgress: vi.fn(),
}))

function concept(overrides: Partial<StudyProgressConcept> = {}): StudyProgressConcept {
  return {
    name: 'mutex',
    status: 'DEMONSTRATED',
    assistanceCount: 0,
    evidenceCount: 2,
    updatedAt: '2026-11-21T12:00:00Z',
    evidence: [],
    ...overrides,
  }
}

function progress(overrides: Partial<StudyProgress> = {}): StudyProgress {
  return { demonstrated: [], explainedOnly: [], difficulties: [], needsReview: [], ...overrides }
}

describe('ProgressDialog', () => {
  it('renders nothing and reads nothing while closed', () => {
    // Given a closed dialog
    // When it renders
    render(<ProgressDialog open={false} sessionId="session-1" onClose={vi.fn()} />)

    // Then no progress is requested
    expect(getStudySessionProgress).not.toHaveBeenCalled()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('shows each group with its concepts, and labels the result as an estimate', async () => {
    // Given a session with one concept in every group
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({
        demonstrated: [concept({ name: 'mutex' })],
        explainedOnly: [concept({ name: 'semaphore', status: 'INTRODUCED' })],
        difficulties: [concept({ name: 'race condition', status: 'PARTIALLY_UNDERSTOOD' })],
        needsReview: [concept({ name: 'deadlock', status: 'NEEDS_REVIEW' })],
      }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then every concept sits under its group, and the dialog says it is the tutor's estimate
    const demonstrated = await screen.findByRole('region', { name: 'You demonstrated' })
    expect(within(demonstrated).getByText('mutex')).toBeInTheDocument()
    const explained = screen.getByRole('region', { name: 'Explained, not yet shown by you' })
    expect(within(explained).getByText('semaphore')).toBeInTheDocument()
    const difficulties = screen.getByRole('region', { name: 'Difficulties' })
    expect(within(difficulties).getByText('race condition')).toBeInTheDocument()
    const review = screen.getByRole('region', { name: 'To review' })
    expect(within(review).getByText('deadlock')).toBeInTheDocument()
    expect(screen.getByText(/estimate by the AI tutor/)).toBeInTheDocument()
    expect(getStudySessionProgress).toHaveBeenCalledWith('session-1')
  })

  it('summarizes how many concepts each group holds, including the empty ones', async () => {
    // Given two demonstrated concepts, one to review and nothing else
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({
        demonstrated: [concept({ name: 'mutex' }), concept({ name: 'channel' })],
        needsReview: [concept({ name: 'deadlock', status: 'NEEDS_REVIEW' })],
      }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then an overview lists the four groups with their counts, in order
    const overview = await screen.findByRole('list', { name: 'Overview' })
    const items = within(overview).getAllByRole('listitem')
    expect(items).toHaveLength(4)
    expect(items[0]).toHaveTextContent(/2\s*You demonstrated/)
    expect(items[1]).toHaveTextContent(/0\s*Explained, not yet shown by you/)
    expect(items[2]).toHaveTextContent(/0\s*Difficulties/)
    expect(items[3]).toHaveTextContent(/1\s*To review/)
  })

  it('shows the number of concepts next to each group title', async () => {
    // Given two demonstrated concepts
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({ demonstrated: [concept({ name: 'mutex' }), concept({ name: 'channel' })] }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the group heading carries its count
    const group = await screen.findByRole('region', { name: 'You demonstrated' })
    expect(within(group).getByRole('heading')).toHaveTextContent(/You demonstrated\s*2/)
  })

  it('omits the groups that have no concept', async () => {
    // Given a session with only a demonstrated concept
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({ demonstrated: [concept()] }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the other groups are not rendered
    await screen.findByRole('region', { name: 'You demonstrated' })
    expect(screen.queryByRole('region', { name: 'Difficulties' })).not.toBeInTheDocument()
    expect(screen.queryByRole('region', { name: 'To review' })).not.toBeInTheDocument()
  })

  it('lists the proofs of a concept with their assessment and whether help was needed', async () => {
    // Given a concept with two proofs
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({
        demonstrated: [
          concept({
            evidence: [
              {
                assessment: 'CORRECT',
                description: 'explained the lock',
                assisted: false,
                createdAt: '2026-11-21T11:00:00Z',
              },
              {
                assessment: 'PARTIALLY_CORRECT',
                description: 'missed the unlock',
                assisted: true,
                createdAt: '2026-11-21T10:00:00Z',
              },
            ],
          }),
        ],
      }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then both proofs show their description and assessment, and only the assisted one says so
    expect(await screen.findByText('explained the lock')).toBeInTheDocument()
    expect(screen.getByText('missed the unlock')).toBeInTheDocument()
    expect(screen.getByText('Correct')).toBeInTheDocument()
    expect(screen.getByText('Partially correct')).toBeInTheDocument()
    expect(screen.getAllByText('With help')).toHaveLength(1)
  })

  it('shows an assessment it does not know as it came, never as another one', async () => {
    // Given a proof with an assessment outside the three
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({
        demonstrated: [
          concept({
            evidence: [
              {
                assessment: 'MAYBE',
                description: '',
                assisted: false,
                createdAt: '2026-11-21T11:00:00Z',
              },
            ],
          }),
        ],
      }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the raw value is shown and none of the three labels is
    expect(await screen.findByText('MAYBE')).toBeInTheDocument()
    expect(screen.queryByText('Incorrect')).not.toBeInTheDocument()
  })

  it('shows an empty message when nothing was observed', async () => {
    // Given a session without observed concepts
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(progress())

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then it explains how concepts appear
    expect(await screen.findByText(/Nothing observed yet/)).toBeInTheDocument()
  })

  it('shows a failure message when the progress cannot be read', async () => {
    // Given a failing read
    vi.mocked(getStudySessionProgress).mockRejectedValueOnce(new Error('db down'))

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the error is shown
    expect(await screen.findByText('Could not load the session progress.')).toBeInTheDocument()
  })

  it('renders concept names and descriptions as plain text, never as markup', async () => {
    // Given untrusted model output carrying HTML
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({
        demonstrated: [
          concept({
            name: '<img src=x alt="tracker">',
            evidence: [
              {
                assessment: 'CORRECT',
                description: '<b>bold</b>',
                assisted: false,
                createdAt: '2026-11-21T11:00:00Z',
              },
            ],
          }),
        ],
      }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then it is text: no image and no bold element is created
    expect(await screen.findByText('<img src=x alt="tracker">')).toBeInTheDocument()
    expect(screen.getByText('<b>bold</b>')).toBeInTheDocument()
    expect(screen.queryByRole('img')).not.toBeInTheDocument()
  })

  it('follows the profile language', async () => {
    // Given the assistant language is Portuguese
    setLabLanguage('pt')
    vi.mocked(getStudySessionProgress).mockResolvedValueOnce(
      progress({ demonstrated: [concept()] }),
    )

    // When the dialog opens
    render(<ProgressDialog open sessionId="session-1" onClose={vi.fn()} />)

    // Then the strings are in Portuguese
    expect(await screen.findByRole('region', { name: 'Você demonstrou' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Progresso da sessão' })).toBeInTheDocument()
  })
})
