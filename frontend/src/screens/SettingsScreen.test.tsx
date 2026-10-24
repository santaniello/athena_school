import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  GetModelSettings,
  HasOpenRouterKey,
  ListSelectableModels,
  ResetLocalData,
} from '../../wailsjs/go/desktop/App'
import SettingsScreen from './SettingsScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveOpenRouterKey: vi.fn(),
  HasOpenRouterKey: vi.fn(),
  ResetLocalData: vi.fn(),
  GetModelSettings: vi.fn(),
  SaveModelSettings: vi.fn(),
  ListSelectableModels: vi.fn(),
}))

describe('SettingsScreen', () => {
  beforeEach(() => {
    vi.mocked(HasOpenRouterKey).mockResolvedValue(false)
    vi.mocked(GetModelSettings).mockResolvedValue({
      current: {
        study: '',
        conversationSummary: '',
        queryEnrichment: '',
        noteFormatting: '',
        embedding: '',
      },
      defaults: {
        study: 'anthropic/claude-sonnet-4.5',
        conversationSummary: 'openai/gpt-4o-mini',
        queryEnrichment: 'openai/gpt-4o-mini',
        noteFormatting: 'anthropic/claude-sonnet-4.5',
        embedding: 'openai/text-embedding-3-small',
      },
    } as never)
    vi.mocked(ListSelectableModels).mockResolvedValue({ chat: [], embedding: [] } as never)
  })

  it('shows that a key is already configured, without ever showing it', async () => {
    // Given a key is already saved
    vi.mocked(HasOpenRouterKey).mockResolvedValue(true)
    render(<SettingsScreen />)

    // Then a status message says so, and the masked field itself stays blank
    expect(await screen.findByText(/key is already configured/i)).toBeInTheDocument()
    expect(screen.getByLabelText('OpenRouter key')).toHaveValue('')
  })

  it('shows that no key is configured yet', async () => {
    // Given no key is saved
    vi.mocked(HasOpenRouterKey).mockResolvedValue(false)
    render(<SettingsScreen />)

    // Then a status message says so
    expect(await screen.findByText(/no key configured yet/i)).toBeInTheDocument()
  })

  it('renders the Models section before the OpenRouter key section', async () => {
    // Given the settings screen
    render(<SettingsScreen />)

    // Then the model selectors are present and come first in the page
    const models = await screen.findByRole('heading', { name: 'Models' })
    const key = screen.getByRole('heading', { name: 'OpenRouter key' })
    expect(await screen.findByRole('combobox', { name: 'Study chat model' })).toBeInTheDocument()
    expect(models.compareDocumentPosition(key) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('renders the OpenRouter key form', () => {
    // Given the settings screen
    render(<SettingsScreen />)

    // Then the shared OpenRouter key form is present
    expect(screen.getByLabelText('OpenRouter key')).toBeInTheDocument()
  })

  it('offers no profile form, which lives in the Profile tab', () => {
    // Given the settings screen
    render(<SettingsScreen />)

    // Then no profile field is present
    expect(screen.queryByLabelText('Assistant name')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Save changes' })).not.toBeInTheDocument()
  })

  it('offers no knowledge extraction setting, since extraction no longer exists', () => {
    // Given the settings screen
    render(<SettingsScreen />)

    // Then there is no extraction section or control
    expect(screen.queryByText('Knowledge extraction')).not.toBeInTheDocument()
    expect(screen.queryByLabelText('Maximum items per extraction')).not.toBeInTheDocument()
  })

  it('opens the reset confirmation dialog from the Danger zone button', async () => {
    // Given the settings screen
    const user = userEvent.setup()
    render(<SettingsScreen />)

    // When clicking "Reset local data"
    await user.click(screen.getByRole('button', { name: 'Reset local data' }))

    // Then the confirmation dialog appears
    expect(await screen.findByText('Reset local data?')).toBeInTheDocument()
  })

  it('calls the reset binding when the dialog is confirmed', async () => {
    // Given the confirmation dialog is open, and the binding never resolves
    // during this assertion
    vi.mocked(ResetLocalData).mockReturnValueOnce(new Promise(() => {}))
    const user = userEvent.setup()
    render(<SettingsScreen />)
    await user.click(screen.getByRole('button', { name: 'Reset local data' }))
    await screen.findByText('Reset local data?')

    // When confirming inside the dialog
    await user.click(screen.getByRole('button', { name: 'Yes, reset local data' }))

    // Then the binding is called and the dialog reports progress
    expect(ResetLocalData).toHaveBeenCalledOnce()
    expect(await screen.findByRole('button', { name: 'Resetting...' })).toBeDisabled()
  })

  it('shows an inline error and re-enables the dialog when the reset binding rejects', async () => {
    // Given a rejected reset
    vi.mocked(ResetLocalData).mockRejectedValueOnce(new Error('sqlite: resetting local data: boom'))
    const user = userEvent.setup()
    render(<SettingsScreen />)
    await user.click(screen.getByRole('button', { name: 'Reset local data' }))
    await screen.findByText('Reset local data?')

    // When confirming
    await user.click(screen.getByRole('button', { name: 'Yes, reset local data' }))

    // Then the error is shown and the dialog's action is usable again
    expect(await screen.findByText('sqlite: resetting local data: boom')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: 'Yes, reset local data' })).toBeEnabled()
  })

  it('closes the reset dialog when Cancel is clicked', async () => {
    // Given the confirmation dialog is open
    const user = userEvent.setup()
    render(<SettingsScreen />)
    await user.click(screen.getByRole('button', { name: 'Reset local data' }))
    await screen.findByText('Reset local data?')

    // When cancelling
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    // Then the dialog closes without calling the binding
    expect(screen.queryByText('Reset local data?')).not.toBeInTheDocument()
    expect(ResetLocalData).not.toHaveBeenCalled()
  })
})
