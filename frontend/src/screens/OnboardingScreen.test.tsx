import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SaveProfile } from '../../wailsjs/go/desktop/App'
import OnboardingScreen from './OnboardingScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveProfile: vi.fn(),
}))

// Opening a Radix Select makes it pull focus into its listbox. When a text
// input currently holds focus, that blur/focus pair lands outside React's
// act scope under jsdom, and React reports an un-acted update on SelectItem
// (its onFocus sets isFocused). Dropping focus first means Radix's move
// starts from a settled state — a real browser's click does this on its own,
// so this only compensates for jsdom, not for a product bug.
//
// Waiting for the trigger to show the label additionally asserts the pick
// actually took, which clicking an option alone does not.
async function pickOption(
  user: ReturnType<typeof userEvent.setup>,
  comboboxName: string,
  optionName: string,
) {
  ;(document.activeElement as HTMLElement | null)?.blur()
  const combobox = screen.getByRole('combobox', { name: comboboxName })
  await user.click(combobox)
  await user.click(await screen.findByRole('option', { name: optionName }))
  await waitFor(() => expect(combobox).toHaveTextContent(optionName))
}

// Free-text fields are filled by pasting rather than typing: per-character
// typing spends a macrotask and a React render on every keystroke for no
// added coverage, since the controlled inputs see the same change events
// either way. Goals keeps type(), since its TagInput commits on {Enter}.
async function fillForm(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByLabelText('Name'))
  await user.paste('Ana')
  await user.click(screen.getByLabelText('What would you like to call the assistant?'))
  await user.paste('Atena')
  await pickOption(user, 'Assistant language', 'English')
  await user.click(screen.getByLabelText('Area of study or work'))
  await user.paste('Software Engineering')
  await pickOption(user, 'Experience level', 'Intermediate')
  await user.type(screen.getByLabelText('Goals'), 'SQL{Enter}')
  await pickOption(user, 'Preferred study style', 'Lots of practical examples')
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
