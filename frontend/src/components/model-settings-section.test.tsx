import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  GetModelSettings,
  ListSelectableModels,
  SaveModelSettings,
} from '../../wailsjs/go/desktop/App'
import { ModelSettingsSection } from './model-settings-section'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  GetModelSettings: vi.fn(),
  SaveModelSettings: vi.fn(),
  ListSelectableModels: vi.fn(),
}))

const DEFAULTS = {
  study: 'anthropic/claude-sonnet-4.5',
  conversationSummary: 'openai/gpt-4o-mini',
  queryEnrichment: 'openai/gpt-4o-mini',
  noteFormatting: 'anthropic/claude-sonnet-4.5',
  embedding: 'openai/text-embedding-3-small',
}
const NONE = {
  study: '',
  conversationSummary: '',
  queryEnrichment: '',
  noteFormatting: '',
  embedding: '',
}
const CHAT_MODELS = [
  'anthropic/claude-sonnet-4.5',
  'anthropic/claude-sonnet-4',
  'openai/gpt-4o-mini',
  'openai/gpt-5',
]
const EMBEDDING_MODELS = ['openai/text-embedding-3-small', 'openai/text-embedding-3-large']

function givenSettings(current: Partial<typeof NONE> = {}) {
  vi.mocked(GetModelSettings).mockResolvedValue({
    current: { ...NONE, ...current },
    defaults: DEFAULTS,
  } as never)
  vi.mocked(ListSelectableModels).mockResolvedValue({
    chat: CHAT_MODELS,
    embedding: EMBEDDING_MODELS,
  } as never)
}

async function renderLoaded(current: Partial<typeof NONE> = {}) {
  givenSettings(current)
  render(<ModelSettingsSection />)
  await screen.findByRole('combobox', { name: 'Study chat model' })
}

async function pick(user: ReturnType<typeof userEvent.setup>, control: string, model: string) {
  await user.click(screen.getByRole('combobox', { name: control }))
  await user.click(screen.getByRole('option', { name: model }))
}

describe('ModelSettingsSection', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('shows one selector per chat feature with its effective model and default', async () => {
    await renderLoaded({ study: 'openai/gpt-5' })

    expect(screen.getByRole('combobox', { name: 'Study chat model' })).toHaveTextContent(
      'openai/gpt-5',
    )
    expect(screen.getAllByText('Default: anthropic/claude-sonnet-4.5')).toHaveLength(2)
    expect(screen.getByRole('combobox', { name: 'Conversation summary model' })).toHaveTextContent(
      'openai/gpt-4o-mini',
    )
    expect(screen.getByRole('combobox', { name: 'Query enrichment model' })).toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'Note formatting (import) model' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Embedding model' })).toHaveTextContent(
      'openai/text-embedding-3-small',
    )
  })

  it('offers reset only for the settings that differ from their default', async () => {
    await renderLoaded({ study: 'openai/gpt-5' })

    expect(screen.getByRole('button', { name: 'Reset study chat model to default' })).toBeEnabled()
    expect(
      screen.getByRole('button', { name: 'Reset conversation summary model to default' }),
    ).toBeDisabled()
  })

  it('saves a chosen chat model immediately and shows it', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    await renderLoaded()

    await pick(user, 'Study chat model', 'openai/gpt-5')

    expect(SaveModelSettings).toHaveBeenCalledWith({ ...NONE, study: 'openai/gpt-5' })
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Study chat model' })).toHaveTextContent(
        'openai/gpt-5',
      ),
    )
    expect(screen.getByRole('button', { name: 'Reset study chat model to default' })).toBeEnabled()
  })

  it('stores picking the default as "use the default" (empty)', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    await renderLoaded({ study: 'openai/gpt-5' })

    await pick(user, 'Study chat model', 'anthropic/claude-sonnet-4.5 (default)')

    expect(SaveModelSettings).toHaveBeenCalledWith(NONE)
  })

  it('keeps the other settings when saving one', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    await renderLoaded({ conversationSummary: 'openai/gpt-5' })

    await pick(user, 'Study chat model', 'openai/gpt-4o-mini')

    expect(SaveModelSettings).toHaveBeenCalledWith({
      ...NONE,
      study: 'openai/gpt-4o-mini',
      conversationSummary: 'openai/gpt-5',
    })
  })

  it('resets a setting to its default without asking for confirmation', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    await renderLoaded({ study: 'openai/gpt-5' })

    await user.click(screen.getByRole('button', { name: 'Reset study chat model to default' }))

    expect(SaveModelSettings).toHaveBeenCalledWith(NONE)
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Study chat model' })).toHaveTextContent(
        'anthropic/claude-sonnet-4.5',
      ),
    )
  })

  it('keeps the previous model and explains when the save is rejected', async () => {
    vi.mocked(SaveModelSettings).mockRejectedValueOnce(
      new Error('settings: model is not in the catalog: "openai/gpt-5"'),
    )
    const user = userEvent.setup()
    await renderLoaded()

    await pick(user, 'Study chat model', 'openai/gpt-5')

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'That model is not available in the OpenRouter catalog.',
    )
    expect(screen.getByRole('combobox', { name: 'Study chat model' })).toHaveTextContent(
      'anthropic/claude-sonnet-4.5',
    )
  })

  it('clears the previous save error once a later save succeeds', async () => {
    vi.mocked(SaveModelSettings)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    await renderLoaded()
    await pick(user, 'Study chat model', 'openai/gpt-5')
    await screen.findByRole('alert')

    await pick(user, 'Study chat model', 'openai/gpt-5')

    await waitFor(() => expect(screen.queryByRole('alert')).not.toBeInTheDocument())
  })

  it('does not change the embedding model until the warning dialog is confirmed', async () => {
    const user = userEvent.setup()
    await renderLoaded()

    await pick(user, 'Embedding model', 'openai/text-embedding-3-large')

    const dialog = screen.getByRole('alertdialog', { name: 'Change the embedding model?' })
    expect(within(dialog).getByText('openai/text-embedding-3-small')).toBeInTheDocument()
    expect(within(dialog).getByText('openai/text-embedding-3-large')).toBeInTheDocument()
    expect(SaveModelSettings).not.toHaveBeenCalled()
  })

  it('keeps the previous embedding model when the dialog is cancelled', async () => {
    const user = userEvent.setup()
    await renderLoaded()
    await pick(user, 'Embedding model', 'openai/text-embedding-3-large')

    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    expect(SaveModelSettings).not.toHaveBeenCalled()
    expect(screen.getByRole('combobox', { name: 'Embedding model' })).toHaveTextContent(
      'openai/text-embedding-3-small',
    )
  })

  it('saves the embedding model on confirm and reminds the user to re-import', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: true })
    const user = userEvent.setup()
    await renderLoaded()
    await pick(user, 'Embedding model', 'openai/text-embedding-3-large')

    await user.click(screen.getByRole('button', { name: 'Change model' }))

    expect(SaveModelSettings).toHaveBeenCalledWith({
      ...NONE,
      embedding: 'openai/text-embedding-3-large',
    })
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument())
    expect(screen.getByRole('combobox', { name: 'Embedding model' })).toHaveTextContent(
      'openai/text-embedding-3-large',
    )
    expect(screen.getByRole('status')).toHaveTextContent(
      'Embedding model changed. Re-import your notes to index them with the new model.',
    )
  })

  it('keeps the dialog open with the error when the embedding save fails', async () => {
    vi.mocked(SaveModelSettings).mockRejectedValueOnce(new Error('boom'))
    const user = userEvent.setup()
    await renderLoaded()
    await pick(user, 'Embedding model', 'openai/text-embedding-3-large')

    await user.click(screen.getByRole('button', { name: 'Change model' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'An error occurred. Please try again.',
    )
    // the modal hides the page from assistive tech, hence { hidden: true }
    expect(
      screen.getByRole('combobox', { name: 'Embedding model', hidden: true }),
    ).toHaveTextContent('openai/text-embedding-3-small')
  })

  it('also asks for confirmation before resetting a custom embedding model', async () => {
    const user = userEvent.setup()
    await renderLoaded({ embedding: 'openai/text-embedding-3-large' })

    await user.click(screen.getByRole('button', { name: 'Reset embedding model to default' }))

    const dialog = screen.getByRole('alertdialog', { name: 'Change the embedding model?' })
    expect(within(dialog).getByText('openai/text-embedding-3-large')).toBeInTheDocument()
    expect(SaveModelSettings).not.toHaveBeenCalled()
  })

  it('does not show the re-import reminder after a chat-model change', async () => {
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    await renderLoaded()

    await pick(user, 'Study chat model', 'openai/gpt-5')

    await waitFor(() => expect(SaveModelSettings).toHaveBeenCalled())
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows a retryable error and no selectors when the settings cannot be loaded', async () => {
    vi.mocked(GetModelSettings).mockRejectedValueOnce(new Error('boom'))
    vi.mocked(ListSelectableModels).mockResolvedValue({ chat: [], embedding: [] } as never)
    const user = userEvent.setup()
    render(<ModelSettingsSection />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the model settings.')
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()

    givenSettings()
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    expect(await screen.findByRole('combobox', { name: 'Study chat model' })).toBeInTheDocument()
  })

  it('disables the selectors and offers a retry when the model catalog is unavailable', async () => {
    vi.mocked(GetModelSettings).mockResolvedValue({ current: NONE, defaults: DEFAULTS } as never)
    vi.mocked(ListSelectableModels).mockRejectedValueOnce(
      new Error('settings: model catalog is unavailable: offline'),
    )
    const user = userEvent.setup()
    render(<ModelSettingsSection />)

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not load the OpenRouter model catalog.',
    )
    expect(screen.getByRole('combobox', { name: 'Study chat model' })).toBeDisabled()
    expect(screen.getByRole('combobox', { name: 'Embedding model' })).toBeDisabled()

    vi.mocked(ListSelectableModels).mockResolvedValue({
      chat: CHAT_MODELS,
      embedding: EMBEDDING_MODELS,
    } as never)
    await user.click(screen.getByRole('button', { name: 'Retry' }))
    await waitFor(() =>
      expect(screen.getByRole('combobox', { name: 'Study chat model' })).toBeEnabled(),
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('still lets the user reset a setting while the catalog is unavailable', async () => {
    vi.mocked(GetModelSettings).mockResolvedValue({
      current: { ...NONE, study: 'openai/gpt-5' },
      defaults: DEFAULTS,
    } as never)
    vi.mocked(ListSelectableModels).mockRejectedValueOnce(new Error('offline'))
    vi.mocked(SaveModelSettings).mockResolvedValueOnce({ embeddingModelChanged: false })
    const user = userEvent.setup()
    render(<ModelSettingsSection />)
    await screen.findByRole('alert')

    await user.click(screen.getByRole('button', { name: 'Reset study chat model to default' }))

    expect(SaveModelSettings).toHaveBeenCalledWith(NONE)
  })

  it('shows the section headings', async () => {
    await renderLoaded()

    expect(screen.getByRole('heading', { name: 'Models' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Advanced' })).toBeInTheDocument()
    expect(
      screen.getByText('Changing the embedding model requires re-importing all of your notes.'),
    ).toBeInTheDocument()
  })
})
