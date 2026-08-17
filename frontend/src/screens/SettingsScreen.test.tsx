import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { UpdateProfile } from '../../wailsjs/go/desktop/App'
import SettingsScreen from './SettingsScreen'
import type { ProfileDraft } from '@/lib/profile'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  UpdateProfile: vi.fn(),
  SaveOpenRouterKey: vi.fn(),
}))

function currentProfile(): ProfileDraft {
  return {
    name: 'Ana',
    assistantName: 'Atena',
    area: 'Engenharia de Software',
    experienceLevel: 'intermediate',
    goals: ['SQL', 'System Design'],
    studyStyle: 'practical_examples',
    assistantLanguage: 'en',
  }
}

describe('SettingsScreen', () => {
  it('pre-fills the form with the current profile', () => {
    // Given the currently saved profile
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)

    // Then the fields show the saved values
    expect(screen.getByLabelText('Name')).toHaveValue('Ana')
    expect(screen.getByLabelText('Assistant name')).toHaveValue('Atena')
    expect(screen.getByLabelText('Area')).toHaveValue('Engenharia de Software')
    expect(screen.getByRole('combobox', { name: 'Experience level' })).toHaveTextContent(
      'Intermediate',
    )
  })

  it('saves a profile change and reports the saved draft', async () => {
    // Given a binding that accepts the edit and echoes the saved profile back
    const saved = { ...currentProfile(), assistantName: 'Nova Atena' }
    vi.mocked(UpdateProfile).mockResolvedValueOnce(saved)
    const onProfileUpdated = vi.fn()
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={onProfileUpdated} />)

    // When changing the assistant name and saving
    const assistantNameInput = screen.getByLabelText('Assistant name')
    await user.clear(assistantNameInput)
    await user.type(assistantNameInput, 'Nova Atena')
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    // Then the binding is called with the edit, and onProfileUpdated fires
    // with what the backend actually persisted
    await waitFor(() =>
      expect(UpdateProfile).toHaveBeenCalledWith(
        expect.objectContaining({ assistantName: 'Nova Atena' }),
      ),
    )
    await waitFor(() => expect(onProfileUpdated).toHaveBeenCalledWith(saved))
  })

  it('shows an inline error and does not report an update on a rejected save', async () => {
    // Given a binding that rejects with a domain validation sentinel
    vi.mocked(UpdateProfile).mockRejectedValueOnce(new Error('at least one goal is required'))
    const onProfileUpdated = vi.fn()
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={onProfileUpdated} />)

    // When saving
    await user.click(screen.getByRole('button', { name: 'Save changes' }))

    // Then an inline error is shown and onProfileUpdated never fires
    expect(await screen.findByText('Add at least one goal.')).toBeInTheDocument()
    expect(onProfileUpdated).not.toHaveBeenCalled()
  })

  it('renders the OpenRouter key form', () => {
    // Given the settings screen
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)

    // Then the shared OpenRouter key form is present
    expect(screen.getByLabelText('OpenRouter key')).toBeInTheDocument()
  })
})
