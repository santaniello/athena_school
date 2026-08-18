import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import HomeScreen from './HomeScreen'
import type { ProfileDraft } from '@/lib/profile'

const profile: ProfileDraft = {
  name: 'Felipe',
  assistantName: 'Athena',
  area: 'Software Engineering',
  experienceLevel: 'intermediate',
  goals: ['System Design'],
  studyStyle: 'practical_examples',
  assistantLanguage: 'en',
}

describe('HomeScreen', () => {
  it('renders nothing yet while the profile has not loaded', () => {
    // Given the profile has not resolved yet
    const { container } = render(<HomeScreen profile={null} studyLocked onStartStudy={vi.fn()} />)

    // Then nothing is rendered rather than a greeting with no name
    expect(container).toBeEmptyDOMElement()
  })

  it('greets the user by name using a morning greeting before noon', () => {
    // Given a profile and a time before noon
    render(
      <HomeScreen
        profile={profile}
        studyLocked
        onStartStudy={vi.fn()}
        now={new Date('2026-08-15T09:00:00')}
      />,
    )

    // Then the morning greeting is shown
    expect(screen.getByRole('heading', { name: 'Good morning, Felipe.' })).toBeInTheDocument()
  })

  it('greets the user with an afternoon greeting at exactly noon', () => {
    // Given a profile and a time at the morning/afternoon boundary
    render(
      <HomeScreen
        profile={profile}
        studyLocked
        onStartStudy={vi.fn()}
        now={new Date('2026-08-15T12:00:00')}
      />,
    )

    // Then the afternoon greeting is shown, not the morning one
    expect(screen.getByRole('heading', { name: 'Good afternoon, Felipe.' })).toBeInTheDocument()
  })

  it('greets the user with an afternoon greeting mid-afternoon', () => {
    // Given a profile and a time between noon and 6pm
    render(
      <HomeScreen
        profile={profile}
        studyLocked
        onStartStudy={vi.fn()}
        now={new Date('2026-08-15T14:00:00')}
      />,
    )

    // Then the afternoon greeting is shown
    expect(screen.getByRole('heading', { name: 'Good afternoon, Felipe.' })).toBeInTheDocument()
  })

  it('greets the user with an evening greeting at exactly 6pm', () => {
    // Given a profile and a time at the afternoon/evening boundary
    render(
      <HomeScreen
        profile={profile}
        studyLocked
        onStartStudy={vi.fn()}
        now={new Date('2026-08-15T18:00:00')}
      />,
    )

    // Then the evening greeting is shown, not the afternoon one
    expect(screen.getByRole('heading', { name: 'Good evening, Felipe.' })).toBeInTheDocument()
  })

  it('greets the user with an evening greeting after 6pm', () => {
    // Given a profile and a time after 6pm
    render(
      <HomeScreen
        profile={profile}
        studyLocked
        onStartStudy={vi.fn()}
        now={new Date('2026-08-15T20:00:00')}
      />,
    )

    // Then the evening greeting is shown
    expect(screen.getByRole('heading', { name: 'Good evening, Felipe.' })).toBeInTheDocument()
  })

  it("mentions the assistant's name in the subheading", () => {
    // Given a profile with an assistant name
    render(<HomeScreen profile={profile} studyLocked onStartStudy={vi.fn()} />)

    // Then it appears alongside the greeting
    expect(screen.getByText('Athena is ready when you are.')).toBeInTheDocument()
  })

  it('calls onStartStudy when the CTA is clicked', async () => {
    // Given a rendered home screen
    const onStartStudy = vi.fn()
    const user = userEvent.setup()
    render(<HomeScreen profile={profile} studyLocked={false} onStartStudy={onStartStudy} />)

    // When the CTA is clicked
    await user.click(screen.getByRole('button', { name: 'Start a study session' }))

    // Then it fires
    expect(onStartStudy).toHaveBeenCalledOnce()
  })

  it('shows a locked note under the CTA while Study Mode has not shipped', () => {
    // Given study is still locked
    render(<HomeScreen profile={profile} studyLocked onStartStudy={vi.fn()} />)

    // Then a note explains why
    expect(screen.getByText(/locked until/i)).toBeInTheDocument()
  })

  it('hides the locked note once Study Mode is unlocked', () => {
    // Given study is unlocked
    render(<HomeScreen profile={profile} studyLocked={false} onStartStudy={vi.fn()} />)

    // Then no locked note is shown
    expect(screen.queryByText(/locked until/i)).not.toBeInTheDocument()
  })
})
