import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HasOpenRouterKey, UpdateProfile } from '../../wailsjs/go/desktop/App'
import SettingsScreen from './SettingsScreen'
import type { ProfileDraft } from '@/lib/profile'
import { getKnowledgeExtractionSettings, updateKnowledgeExtractionSettings } from '@/lib/knowledge'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  UpdateProfile: vi.fn(),
  SaveOpenRouterKey: vi.fn(),
  HasOpenRouterKey: vi.fn(),
}))

vi.mock('@/lib/knowledge', () => ({
  getKnowledgeExtractionSettings: vi.fn(),
  updateKnowledgeExtractionSettings: vi.fn(),
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
  beforeEach(() => {
    vi.mocked(HasOpenRouterKey).mockResolvedValue(false)
    vi.mocked(getKnowledgeExtractionSettings).mockResolvedValue({
      maxKnowledgeExtractionItems: 8,
    })
  })

  it('shows that a key is already configured, without ever showing it', async () => {
    // Given a key is already saved
    vi.mocked(HasOpenRouterKey).mockResolvedValue(true)
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)

    // Then a status message says so, and the masked field itself stays blank
    expect(await screen.findByText(/key is already configured/i)).toBeInTheDocument()
    expect(screen.getByLabelText('OpenRouter key')).toHaveValue('')
  })

  it('shows that no key is configured yet', async () => {
    // Given no key is saved
    vi.mocked(HasOpenRouterKey).mockResolvedValue(false)
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)

    // Then a status message says so
    expect(await screen.findByText(/no key configured yet/i)).toBeInTheDocument()
  })

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

  it('loads and displays the configured knowledge extraction maximum', async () => {
    // Given a configured maximum
    vi.mocked(getKnowledgeExtractionSettings).mockResolvedValueOnce({
      maxKnowledgeExtractionItems: 12,
    })

    // When opening Settings
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)

    // Then the Knowledge Extraction section displays it
    expect(await screen.findByLabelText('Máximo de itens por extração')).toHaveValue(12)
  })

  it('validates the knowledge extraction maximum before saving', async () => {
    // Given the knowledge extraction settings form
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')

    // When entering an out-of-range value and saving
    await user.clear(input)
    await user.type(input, '21')
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then a client-side error is shown and the binding is not called
    expect(await screen.findByText('Informe um valor entre 1 e 20.')).toBeInTheDocument()
    expect(updateKnowledgeExtractionSettings).not.toHaveBeenCalled()
  })

  it('saves a valid knowledge extraction maximum', async () => {
    // Given the knowledge extraction settings form
    vi.mocked(updateKnowledgeExtractionSettings).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')

    // When entering a valid value and saving
    await user.clear(input)
    await user.type(input, '12')
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then the backend receives it and success is shown
    await waitFor(() => expect(updateKnowledgeExtractionSettings).toHaveBeenCalledWith(12))
    expect(await screen.findByText('Configuração salva.')).toBeInTheDocument()
  })
})
