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

  it('shows an inline error when loading knowledge extraction settings fails', async () => {
    // Given the settings binding is unavailable
    vi.mocked(getKnowledgeExtractionSettings).mockRejectedValueOnce(new Error('unavailable'))

    // When opening Settings
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)

    // Then the load failure is reported without crashing the profile form
    expect(await screen.findByText('Não foi possível carregar a configuração.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
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

  it('rejects a knowledge extraction maximum below the minimum', async () => {
    // Given the knowledge extraction settings form
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')

    // When entering a value below the allowed range and saving
    await user.clear(input)
    await user.type(input, '0')
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then validation blocks persistence
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

  it.each([1, 20])('accepts the boundary knowledge extraction maximum %i', async (maximum) => {
    // Given the knowledge extraction settings form
    vi.mocked(updateKnowledgeExtractionSettings).mockResolvedValueOnce()
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')

    // When saving an inclusive boundary value
    await user.clear(input)
    await user.type(input, String(maximum))
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then the boundary is persisted
    await waitFor(() => expect(updateKnowledgeExtractionSettings).toHaveBeenCalledWith(maximum))
  })

  it('locks the knowledge extraction form and reports progress while saving', async () => {
    // Given a settings update that remains in flight
    vi.mocked(updateKnowledgeExtractionSettings).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')
    await user.clear(input)
    await user.type(input, '12')

    // When saving the extraction setting
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then the button exposes and locks its pending state
    expect(screen.getByRole('button', { name: 'Salvando...' })).toBeDisabled()
  })

  it('shows an inline error when saving knowledge extraction settings fails', async () => {
    // Given a rejected settings update
    vi.mocked(updateKnowledgeExtractionSettings).mockRejectedValueOnce(new Error('unavailable'))
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')
    await user.clear(input)
    await user.type(input, '12')

    // When saving the extraction setting
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then the save failure is reported and the action is available again
    expect(await screen.findByText('Não foi possível salvar a configuração.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Salvar configuração de extração' })).toBeEnabled()
  })

  it('clears stale validation and success messages before a new valid save', async () => {
    // Given a previous validation failure
    let resolveSave!: () => void
    vi.mocked(updateKnowledgeExtractionSettings).mockReturnValueOnce(
      new Promise<void>((resolve) => {
        resolveSave = resolve
      }),
    )
    const user = userEvent.setup()
    render(<SettingsScreen profile={currentProfile()} onProfileUpdated={vi.fn()} />)
    const input = await screen.findByLabelText('Máximo de itens por extração')
    await user.clear(input)
    await user.type(input, '21')
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))
    expect(await screen.findByText('Informe um valor entre 1 e 20.')).toBeInTheDocument()

    // When starting a valid save
    await user.clear(input)
    await user.type(input, '12')
    await user.click(screen.getByRole('button', { name: 'Salvar configuração de extração' }))

    // Then stale feedback is cleared before the request resolves
    expect(screen.queryByText('Informe um valor entre 1 e 20.')).not.toBeInTheDocument()
    expect(screen.queryByText('Configuração salva.')).not.toBeInTheDocument()
    resolveSave()
    expect(await screen.findByText('Configuração salva.')).toBeInTheDocument()
  })
})
