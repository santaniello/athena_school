import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { listSessionsForGeneration } from '@/lib/exercise'
import { cancelMindMapGeneration, generateMindMap, type MindMapSummary } from '@/lib/mindmap'
import { setLabLanguage } from '@/i18n'
import { GenerateMindMapDialog } from './generate-mindmap-dialog'

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  listSessionsForGeneration: vi.fn(),
}))
vi.mock('@/lib/mindmap', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/mindmap')>()),
  generateMindMap: vi.fn(),
  cancelMindMapGeneration: vi.fn(),
}))

const SESSIONS = [
  {
    id: 's1',
    topic: 'Distributed Caching',
    folderName: 'Backend',
    startedAt: '2026-11-10T10:00:00Z',
  },
  { id: 's2', topic: 'Go Concurrency', folderName: 'Languages', startedAt: '2026-11-11T10:00:00Z' },
]

const CREATED: MindMapSummary = {
  id: 'map-1',
  sessionId: 's1',
  sessionTitle: 'Distributed Caching',
  title: 'Caching map',
  createdAt: '2026-11-17T09:30:00Z',
  updatedAt: '2026-11-17T09:30:00Z',
  messageId: '',
}

function renderDialog(props: Partial<React.ComponentProps<typeof GenerateMindMapDialog>> = {}) {
  const handlers = { onClose: vi.fn(), onGenerated: vi.fn(), onGeneratedAfterClose: vi.fn() }
  const view = render(<GenerateMindMapDialog open {...handlers} {...props} />)
  return { ...handlers, ...view }
}

describe('GenerateMindMapDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
    vi.mocked(listSessionsForGeneration).mockResolvedValue(SESSIONS)
  })

  it('renders nothing while closed', () => {
    render(
      <GenerateMindMapDialog
        open={false}
        onClose={vi.fn()}
        onGenerated={vi.fn()}
        onGeneratedAfterClose={vi.fn()}
      />,
    )

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(listSessionsForGeneration).not.toHaveBeenCalled()
  })

  it('lists the sessions and enables Generate only once one is chosen', async () => {
    const user = userEvent.setup()
    renderDialog()
    expect(screen.getByText('Loading your study sessions...')).toBeInTheDocument()

    await user.click(await screen.findByRole('radio', { name: /Go Concurrency/ }))

    expect(screen.getByRole('button', { name: 'Generate' })).toBeEnabled()
  })

  it('keeps Generate disabled until a session is chosen', async () => {
    renderDialog()
    await screen.findByRole('radio', { name: /Go Concurrency/ })

    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()
  })

  it('filters the sessions by topic or folder', async () => {
    const user = userEvent.setup()
    renderDialog()
    await screen.findByRole('radio', { name: /Go Concurrency/ })

    await user.type(screen.getByRole('searchbox', { name: 'Search sessions' }), 'backend')

    expect(screen.getByRole('radio', { name: /Distributed Caching/ })).toBeInTheDocument()
    expect(screen.queryByRole('radio', { name: /Go Concurrency/ })).not.toBeInTheDocument()

    await user.clear(screen.getByRole('searchbox', { name: 'Search sessions' }))
    await user.type(screen.getByRole('searchbox', { name: 'Search sessions' }), 'zzz')

    expect(screen.getByText('No session matches your search.')).toBeInTheDocument()
  })

  it('disables Generate when the chosen session was filtered out by the search', async () => {
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Go Concurrency/ }))
    expect(screen.getByRole('button', { name: 'Generate' })).toBeEnabled()

    await user.type(screen.getByRole('searchbox', { name: 'Search sessions' }), 'backend')

    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()
    expect(generateMindMap).not.toHaveBeenCalled()
  })

  it('says there are no study sessions', async () => {
    vi.mocked(listSessionsForGeneration).mockResolvedValue([])
    renderDialog()

    expect(
      await screen.findByText(
        'You have no study sessions yet. Study something first, then come back.',
      ),
    ).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()
  })

  it('says the sessions could not load and retries', async () => {
    const user = userEvent.setup()
    vi.mocked(listSessionsForGeneration).mockRejectedValueOnce(new Error('nope'))
    renderDialog()

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not load your study sessions.',
    )
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('radio', { name: /Go Concurrency/ })).toBeInTheDocument()
  })

  it('generates for the chosen session and hands the map to the caller', async () => {
    const user = userEvent.setup()
    vi.mocked(generateMindMap).mockResolvedValue(CREATED)
    const { onGenerated } = renderDialog()

    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(generateMindMap).toHaveBeenCalledWith('s1')
    expect(onGenerated).toHaveBeenCalledWith(CREATED)
  })

  it('shows progress while generating and locks the choice', async () => {
    const user = userEvent.setup()
    vi.mocked(generateMindMap).mockReturnValue(new Promise(() => {}))
    renderDialog()

    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(screen.getByRole('status')).toHaveTextContent('Drawing your mind map...')
    expect(screen.getByRole('radio', { name: /Go Concurrency/ })).toBeDisabled()
    expect(screen.queryByRole('button', { name: 'Generate' })).not.toBeInTheDocument()
  })

  it('cancels the generation in flight from the Cancel button, and goes back to the choice', async () => {
    const user = userEvent.setup()
    let reject: (reason: unknown) => void = () => {}
    vi.mocked(generateMindMap).mockReturnValue(
      new Promise((_, rejectFn) => {
        reject = rejectFn
      }),
    )
    const { onClose } = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    reject(new Error(JSON.stringify({ code: 'cancelled' })))

    expect(cancelMindMapGeneration).toHaveBeenCalledOnce()
    expect(onClose).not.toHaveBeenCalled()
    expect(await screen.findByRole('button', { name: 'Generate' })).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows the reason a generation failed and offers Try again', async () => {
    const user = userEvent.setup()
    vi.mocked(generateMindMap).mockRejectedValueOnce(
      new Error(JSON.stringify({ code: 'invalid_generation' })),
    )
    renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The generated mind map was not valid. Try again.',
    )
    vi.mocked(generateMindMap).mockResolvedValueOnce(CREATED)
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(generateMindMap).toHaveBeenCalledTimes(2)
  })

  it('cancels a generation nobody waits for when the dialog is dismissed', async () => {
    const user = userEvent.setup()
    vi.mocked(generateMindMap).mockReturnValue(new Promise(() => {}))
    const { onClose } = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    await user.keyboard('{Escape}')

    expect(cancelMindMapGeneration).toHaveBeenCalledOnce()
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('does not cancel anything when it is dismissed with no generation running', async () => {
    const user = userEvent.setup()
    const { onClose } = renderDialog()
    await screen.findByRole('radio', { name: /Distributed Caching/ })

    await user.keyboard('{Escape}')

    expect(cancelMindMapGeneration).not.toHaveBeenCalled()
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('tells the caller about a map that arrives after the dialog closed', async () => {
    const user = userEvent.setup()
    let resolve: (map: MindMapSummary) => void = () => {}
    vi.mocked(generateMindMap).mockReturnValue(
      new Promise((resolveFn) => {
        resolve = resolveFn
      }),
    )
    const { onGenerated, onGeneratedAfterClose, rerender, onClose } = renderDialog()
    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    rerender(
      <GenerateMindMapDialog
        open={false}
        onClose={onClose}
        onGenerated={onGenerated}
        onGeneratedAfterClose={onGeneratedAfterClose}
      />,
    )
    resolve(CREATED)

    await vi.waitFor(() => expect(onGeneratedAfterClose).toHaveBeenCalledWith(CREATED))
    expect(onGenerated).not.toHaveBeenCalled()
  })
})
