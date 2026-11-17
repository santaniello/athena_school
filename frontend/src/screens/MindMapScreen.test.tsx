import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { listSessionsForGeneration } from '@/lib/exercise'
import { deleteMindMap, generateMindMap, listMindMaps, type MindMapSummary } from '@/lib/mindmap'
import { setLabLanguage } from '@/i18n'
import MindMapScreen from './MindMapScreen'

vi.mock('@/lib/exercise', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/exercise')>()),
  listSessionsForGeneration: vi.fn(),
}))
vi.mock('@/lib/mindmap', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/mindmap')>()),
  listMindMaps: vi.fn(),
  generateMindMap: vi.fn(),
  deleteMindMap: vi.fn(),
  cancelMindMapGeneration: vi.fn(),
}))

function summary(overrides: Partial<MindMapSummary>): MindMapSummary {
  return {
    id: 'map-1',
    sessionId: 's1',
    sessionTitle: 'Distributed Caching',
    title: 'Caching map',
    createdAt: '2026-11-16T09:30:00Z',
    updatedAt: '2026-11-17T10:30:00Z',
    messageId: '',
    ...overrides,
  }
}

const FIRST = summary({ id: 'map-new', title: 'Go map', sessionTitle: 'Go Concurrency' })
const SECOND = summary({ id: 'map-old' })

function row(title: string) {
  return screen.getByRole('listitem', { name: title })
}

describe('MindMapScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
    vi.mocked(listMindMaps).mockResolvedValue([FIRST, SECOND])
    vi.mocked(listSessionsForGeneration).mockResolvedValue([])
  })

  it('lists the mind maps with their session and last update', async () => {
    render(<MindMapScreen onOpen={vi.fn()} />)

    expect(screen.getByText('Loading your mind maps...')).toBeInTheDocument()
    const first = await screen.findByRole('listitem', { name: 'Go map' })
    expect(within(first).getByText('Go Concurrency')).toBeInTheDocument()
    expect(within(first).getByText(/^Updated /)).toBeInTheDocument()
    expect(within(first).getByRole('button', { name: 'Open' })).toBeInTheDocument()
    expect(row('Caching map')).toBeInTheDocument()
  })

  it('hands the chosen mind map to the shell when Open is clicked', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    render(<MindMapScreen onOpen={onOpen} />)

    await user.click(
      within(await screen.findByRole('listitem', { name: 'Go map' })).getByRole('button', {
        name: 'Open',
      }),
    )

    expect(onOpen).toHaveBeenCalledWith('map-new')
  })

  it('explains how to start when there are no mind maps', async () => {
    vi.mocked(listMindMaps).mockResolvedValue([])
    render(<MindMapScreen onOpen={vi.fn()} />)

    expect(
      await screen.findByText(
        'No mind maps yet. Generate one from a study session you already had.',
      ),
    ).toBeInTheDocument()
  })

  it('says the mind maps could not load and retries', async () => {
    const user = userEvent.setup()
    vi.mocked(listMindMaps).mockRejectedValueOnce(new Error('nope')).mockResolvedValueOnce([SECOND])
    render(<MindMapScreen onOpen={vi.fn()} />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load your mind maps.')
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByRole('listitem', { name: 'Caching map' })).toBeInTheDocument()
  })

  it('shows loading again while a retry runs', async () => {
    const user = userEvent.setup()
    vi.mocked(listMindMaps).mockRejectedValueOnce(new Error('nope'))
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByRole('alert')

    vi.mocked(listMindMaps).mockReturnValueOnce(new Promise(() => {}))
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(screen.getByText('Loading your mind maps...')).toBeInTheDocument()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('asks before deleting, and deletes and reloads when confirmed', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteMindMap).mockResolvedValue(undefined)
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go map' })

    await user.click(within(row('Go map')).getByRole('button', { name: 'Delete' }))
    expect(deleteMindMap).not.toHaveBeenCalled()
    expect(screen.getByText('Delete this mind map?')).toBeInTheDocument()

    vi.mocked(listMindMaps).mockResolvedValue([SECOND])
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }),
    )

    expect(deleteMindMap).toHaveBeenCalledWith('map-new')
    await waitFor(() =>
      expect(screen.queryByRole('listitem', { name: 'Go map' })).not.toBeInTheDocument(),
    )
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('keeps the mind map when the deletion is cancelled', async () => {
    const user = userEvent.setup()
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go map' })

    await user.click(within(row('Go map')).getByRole('button', { name: 'Delete' }))
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Cancel' }),
    )

    expect(deleteMindMap).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('says when a deletion fails, and clears the message when another one starts', async () => {
    const user = userEvent.setup()
    vi.mocked(deleteMindMap).mockRejectedValue(new Error('nope'))
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go map' })

    await user.click(within(row('Go map')).getByRole('button', { name: 'Delete' }))
    await user.click(
      within(screen.getByRole('alertdialog')).getByRole('button', { name: 'Delete' }),
    )
    expect(await screen.findByRole('alert')).toHaveTextContent('Could not delete the mind map.')
    expect(row('Go map')).toBeInTheDocument()

    await user.click(within(row('Caching map')).getByRole('button', { name: 'Delete' }))

    expect(screen.queryByText('Could not delete the mind map.')).not.toBeInTheDocument()
  })

  it('generates a mind map, then reloads the list and highlights the new one', async () => {
    const user = userEvent.setup()
    vi.mocked(listSessionsForGeneration).mockResolvedValue([
      {
        id: 's1',
        topic: 'Distributed Caching',
        folderName: 'Backend',
        startedAt: '2026-11-10T10:00:00Z',
      },
    ])
    const created = summary({ id: 'map-created', title: 'Fresh map' })
    vi.mocked(generateMindMap).mockResolvedValue(created)
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go map' })

    await user.click(screen.getByRole('button', { name: 'Generate mind map with AI' }))
    await user.click(await screen.findByRole('radio', { name: /Distributed Caching/ }))
    vi.mocked(listMindMaps).mockResolvedValue([created, FIRST, SECOND])
    await user.click(screen.getByRole('button', { name: 'Generate' }))

    expect(generateMindMap).toHaveBeenCalledWith('s1')
    const fresh = await screen.findByRole('listitem', { name: 'Fresh map' })
    expect(fresh).toHaveAttribute('aria-current', 'true')
    expect(row('Go map')).not.toHaveAttribute('aria-current')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('closes the generation dialog on request', async () => {
    const user = userEvent.setup()
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByRole('listitem', { name: 'Go map' })

    await user.click(screen.getByRole('button', { name: 'Generate mind map with AI' }))
    await user.click(await screen.findByRole('button', { name: 'Cancel' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('also offers Generate from the empty state', async () => {
    const user = userEvent.setup()
    vi.mocked(listMindMaps).mockResolvedValue([])
    render(<MindMapScreen onOpen={vi.fn()} />)
    await screen.findByText('No mind maps yet. Generate one from a study session you already had.')

    await user.click(screen.getAllByRole('button', { name: 'Generate mind map with AI' })[1])

    expect(await screen.findByRole('dialog')).toBeInTheDocument()
  })

  it('speaks the profile language', async () => {
    setLabLanguage('pt')
    render(<MindMapScreen onOpen={vi.fn()} />)

    expect(await screen.findAllByText(/^Atualizado em /)).not.toHaveLength(0)
    expect(screen.getByRole('button', { name: 'Gerar mapa mental com IA' })).toBeInTheDocument()
    setLabLanguage('en')
  })
})
