import { beforeEach, describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  listChallengeCatalog,
  listChallengeTopics,
  type Catalog,
  type CatalogItem,
} from '@/lib/exercise'
import ArchitectureLabScreen from './ArchitectureLabScreen'

vi.mock('@/lib/exercise', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/lib/exercise')>()
  return { ...original, listChallengeCatalog: vi.fn(), listChallengeTopics: vi.fn() }
})

// The real dialog is covered by its own tests; this stand-in only exposes
// what the screen hands it.
vi.mock('@/components/generate-challenge-dialog', () => ({
  GenerateChallengeDialog: (props: {
    open: boolean
    defaultDifficulty: string
    onClose: () => void
    onGenerated: (challenge: { id: string }) => void
  }) =>
    props.open ? (
      <div role="dialog" aria-label="Generate dialog">
        <span>default {props.defaultDifficulty}</span>
        <button onClick={props.onClose}>close dialog</button>
        <button onClick={() => props.onGenerated({ id: 'c-new' })}>finish generation</button>
      </div>
    ) : null,
}))

function item(id: string, overrides: Partial<CatalogItem> = {}): CatalogItem {
  return {
    id,
    title: `Challenge ${id}`,
    topic: 'caching',
    difficulty: 'INTERMEDIATE',
    timeLimitMinutes: 45,
    status: 'NOT_STARTED',
    attempts: 0,
    bestDiagramScore: 0,
    ...overrides,
  }
}

function catalog(items: CatalogItem[], counts: Partial<Catalog> = {}): Catalog {
  return { total: items.length, completed: 0, inProgress: 0, items, ...counts }
}

describe('ArchitectureLabScreen', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    vi.mocked(listChallengeTopics).mockResolvedValue(['caching', 'queues'])
  })

  it('shows the counts and one row per challenge', async () => {
    vi.mocked(listChallengeCatalog).mockResolvedValue(
      catalog(
        [item('c1'), item('c2', { status: 'COMPLETED', attempts: 1, bestDiagramScore: 90 })],
        {
          total: 12,
          completed: 3,
          inProgress: 2,
        },
      ),
    )

    render(<ArchitectureLabScreen experienceLevel="intermediate" />)

    expect(await screen.findByText('12 challenges')).toBeInTheDocument()
    expect(screen.getByText('3 completed')).toBeInTheDocument()
    expect(screen.getByText('2 in progress')).toBeInTheDocument()
    expect(screen.getAllByRole('listitem')).toHaveLength(2)
    expect(screen.getByText('Challenge c1')).toBeInTheDocument()
  })

  it('uses the singular for a single challenge', async () => {
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))

    render(<ArchitectureLabScreen experienceLevel="intermediate" />)

    expect(await screen.findByText('1 challenge')).toBeInTheDocument()
  })

  it('shows a loading message until the first catalog arrives', () => {
    vi.mocked(listChallengeCatalog).mockReturnValue(new Promise(() => {}))

    render(<ArchitectureLabScreen experienceLevel="intermediate" />)

    expect(screen.getByText('Loading challenges…')).toBeInTheDocument()
  })

  it('searches by the typed text', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)
    await screen.findByText('Challenge c1')

    await user.type(screen.getByRole('searchbox', { name: 'Search challenges' }), 'cache')

    await waitFor(() =>
      expect(listChallengeCatalog).toHaveBeenLastCalledWith(
        expect.objectContaining({ query: 'cache' }),
      ),
    )
  })

  it('filters by status, difficulty and topic together, and offers the topics present', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)
    await screen.findByText('Challenge c1')

    await user.click(screen.getByRole('combobox', { name: 'Status' }))
    await user.click(await screen.findByRole('option', { name: 'Completed' }))
    await user.click(screen.getByRole('combobox', { name: 'Difficulty' }))
    await user.click(await screen.findByRole('option', { name: 'Advanced' }))
    await user.click(screen.getByRole('combobox', { name: 'Topic' }))
    expect(await screen.findByRole('option', { name: 'queues' })).toBeInTheDocument()
    await user.click(screen.getByRole('option', { name: 'caching' }))

    await waitFor(() =>
      expect(listChallengeCatalog).toHaveBeenLastCalledWith({
        query: '',
        status: 'COMPLETED',
        difficulty: 'ADVANCED',
        topic: 'caching',
      }),
    )
  })

  it('goes back to every status when "All" is chosen again', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)
    await screen.findByText('Challenge c1')
    await user.click(screen.getByRole('combobox', { name: 'Status' }))
    await user.click(await screen.findByRole('option', { name: 'Completed' }))

    await user.click(screen.getByRole('combobox', { name: 'Status' }))
    await user.click(await screen.findByRole('option', { name: 'All statuses' }))

    await waitFor(() =>
      expect(listChallengeCatalog).toHaveBeenLastCalledWith(
        expect.objectContaining({ status: '' }),
      ),
    )
  })

  it('explains how to create a challenge when the catalog is empty, and offers to generate one', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([]))
    render(<ArchitectureLabScreen experienceLevel="beginner" />)

    expect(await screen.findByText('No challenges yet')).toBeInTheDocument()
    expect(screen.getByText(/generate one from a study session/i)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Generate your first challenge' }))

    expect(screen.getByRole('dialog', { name: 'Generate dialog' })).toBeInTheDocument()
    expect(screen.getByText('default BEGINNER')).toBeInTheDocument()
  })

  it('says nothing matches the filters, and clears them on request', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockImplementation(async (filter) =>
      filter.query ? catalog([], { total: 4 }) : catalog([item('c1')], { total: 4 }),
    )
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)
    await screen.findByText('Challenge c1')
    await user.type(screen.getByRole('searchbox', { name: 'Search challenges' }), 'zzz')

    expect(await screen.findByText('No challenges match your filters.')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Clear filters' }))

    expect(await screen.findByText('Challenge c1')).toBeInTheDocument()
    expect(screen.getByRole('searchbox', { name: 'Search challenges' })).toHaveValue('')
  })

  it('shows a retry when the catalog cannot be loaded', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce(catalog([item('c1')]))
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the challenges.')
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('Challenge c1')).toBeInTheDocument()
  })

  it('opens the generation dialog from the header, preselecting the difficulty of the profile', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))
    render(<ArchitectureLabScreen experienceLevel="advanced" />)
    await screen.findByText('Challenge c1')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Generate challenge with AI' }))

    expect(screen.getByText('default ADVANCED')).toBeInTheDocument()
  })

  it('closes the dialog without reloading when it is dismissed', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)
    await screen.findByText('Challenge c1')
    await user.click(screen.getByRole('button', { name: 'Generate challenge with AI' }))

    await user.click(screen.getByRole('button', { name: 'close dialog' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(listChallengeCatalog).toHaveBeenCalledTimes(1)
  })

  it('after a generation, closes the dialog, clears the filters, reloads and highlights the new challenge', async () => {
    const user = userEvent.setup()
    vi.mocked(listChallengeCatalog).mockImplementation(async (filter) =>
      filter.query ? catalog([item('c1')]) : catalog([item('c1'), item('c-new')]),
    )
    render(<ArchitectureLabScreen experienceLevel="intermediate" />)
    await screen.findByText('Challenge c1')
    await user.type(screen.getByRole('searchbox', { name: 'Search challenges' }), 'x')
    await screen.findByText('Challenge c1')
    await user.click(screen.getByRole('button', { name: 'Generate challenge with AI' }))

    await user.click(screen.getByRole('button', { name: 'finish generation' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(await screen.findByText('Challenge c-new')).toBeInTheDocument()
    expect(screen.getByRole('searchbox', { name: 'Search challenges' })).toHaveValue('')
    const rows = screen.getAllByRole('listitem')
    expect(rows.find((row) => row.getAttribute('aria-current') === 'true')).toHaveTextContent(
      'Challenge c-new',
    )
    expect(listChallengeTopics).toHaveBeenCalledTimes(2)
  })

  it('keeps the topic filter usable when the topics cannot be loaded', async () => {
    vi.mocked(listChallengeTopics).mockRejectedValue(new Error('boom'))
    vi.mocked(listChallengeCatalog).mockResolvedValue(catalog([item('c1')]))

    render(<ArchitectureLabScreen experienceLevel="intermediate" />)

    expect(await screen.findByText('Challenge c1')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Topic' })).toBeInTheDocument()
  })
})
