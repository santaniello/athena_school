import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SaveProfile } from '../../wailsjs/go/desktop/App'
import OnboardingScreen from './OnboardingScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveProfile: vi.fn(),
}))

async function fillForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Name'), 'Ana')
  await user.type(screen.getByLabelText('What would you like to call the assistant?'), 'Atena')
  await user.click(screen.getByRole('combobox', { name: 'Assistant language' }))
  await user.click(await screen.findByRole('option', { name: 'English' }))
  await user.type(screen.getByLabelText('Area of study or work'), 'Software Engineering')
  await user.click(screen.getByRole('combobox', { name: 'Experience level' }))
  await user.click(await screen.findByRole('option', { name: 'Intermediate' }))
  await user.type(screen.getByLabelText('Goals'), 'SQL{Enter}')
  await user.click(screen.getByRole('combobox', { name: 'Preferred study style' }))
  await user.click(await screen.findByRole('option', { name: 'Lots of practical examples' }))
}

describe('OnboardingScreen', () => {
  it('renders the form step first', () => {
    // Given a freshly rendered onboarding screen
    render(<OnboardingScreen onComplete={vi.fn()} />)

    // Then the form screen (not the confirmation screen) is shown
    expect(screen.getByRole('heading', { name: 'Tell us about yourself' })).toBeInTheDocument()
  })

  it('moves to confirmation, saves, and completes onboarding', async () => {
    // Given the onboarding flow starting on the form step
    vi.mocked(SaveProfile).mockResolvedValueOnce()
    const onComplete = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingScreen onComplete={onComplete} />)

    // When filling every field and continuing to the confirmation screen
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Continue' }))

    // Then the confirmation screen shows the filled-in draft
    expect(screen.getByRole('heading', { name: 'Confirm your profile' })).toBeInTheDocument()
    expect(screen.getByText('Ana')).toBeInTheDocument()

    // When confirming and saving
    await user.click(screen.getByRole('button', { name: 'Confirm and save' }))

    // Then the profile is saved and onComplete fires
    expect(SaveProfile).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Ana', experienceLevel: 'intermediate', goals: ['SQL'] }),
    )
    await waitFor(() => expect(onComplete).toHaveBeenCalledOnce())
  })
})
