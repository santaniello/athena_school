import { useImperativeHandle, type Ref } from 'react'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DiagramEditorHandle } from '@/components/diagram/diagram-editor'
import {
  cancelMindMapGeneration,
  deleteMindMap,
  getMindMap,
  regenerateMindMap,
  saveMindMapDiagram,
  type MindMap,
  type MindMapReturn,
} from '@/lib/mindmap'
import { setLabLanguage } from '@/i18n'
import MindMapViewScreen from './MindMapViewScreen'

vi.mock('@/lib/mindmap', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/mindmap')>()),
  getMindMap: vi.fn(),
  saveMindMapDiagram: vi.fn(),
  regenerateMindMap: vi.fn(),
  deleteMindMap: vi.fn(),
  cancelMindMapGeneration: vi.fn(),
}))

interface EditorProps {
  ref?: Ref<DiagramEditorHandle>
  xml?: string
  onReady: () => void
  onChange: (xml: string) => void
  onConnectionLost: () => void
  onConnectionRestored: () => void
}

// The real editor is covered by its own tests; this stand-in exposes the
// callbacks the screen hands it and answers getXml with xmlOnBoard.
let editor: EditorProps
let xmlOnBoard = ''
// Runs inside getXml, to simulate an edit landing while the diagram is exported.
let onExportEdit: (() => void) | null = null

vi.mock('@/components/diagram/diagram-editor', () => ({
  DiagramEditor: (props: EditorProps) => {
    editor = props
    useImperativeHandle(props.ref, () => ({
      getXml: () => {
        onExportEdit?.()
        return Promise.resolve(xmlOnBoard)
      },
    }))
    return <div data-testid="editor" data-xml={props.xml ?? ''} />
  },
}))

const MAP: MindMap = {
  id: 'map-1',
  sessionId: 's1',
  sessionTitle: 'Distributed Caching',
  title: 'Caching map',
  createdAt: '2026-11-16T09:30:00Z',
  updatedAt: '2026-11-17T10:30:00Z',
  messageId: '',
  diagramXml: '<mxGraphModel>original</mxGraphModel>',
}

interface Handlers {
  onExit: ReturnType<typeof vi.fn>
  onLeaveCancelled: ReturnType<typeof vi.fn>
}

function renderScreen(
  props: { returnTo?: MindMapReturn; leaveRequested?: boolean } = {},
): Handlers & { rerender: (leaveRequested: boolean) => void } {
  const handlers = { onExit: vi.fn(), onLeaveCancelled: vi.fn() }
  const returnTo = props.returnTo ?? { kind: 'menu' }
  const view = render(
    <MindMapViewScreen
      mindMapId="map-1"
      returnTo={returnTo}
      leaveRequested={props.leaveRequested ?? false}
      {...handlers}
    />,
  )
  return {
    ...handlers,
    rerender: (leaveRequested) =>
      view.rerender(
        <MindMapViewScreen
          mindMapId="map-1"
          returnTo={returnTo}
          leaveRequested={leaveRequested}
          {...handlers}
        />,
      ),
  }
}

async function openEditor(props: Parameters<typeof renderScreen>[0] = {}) {
  const user = userEvent.setup()
  const handlers = renderScreen(props)
  await screen.findByTestId('editor')
  act(() => editor.onReady())
  return { user, ...handlers }
}

function edit(xml = '<mxGraphModel>edited</mxGraphModel>') {
  act(() => editor.onChange(xml))
}

describe('MindMapViewScreen', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
    xmlOnBoard = '<mxGraphModel>edited</mxGraphModel>'
    vi.mocked(getMindMap).mockResolvedValue(MAP)
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true)
  })

  afterEach(() => vi.restoreAllMocks())

  it('loads the mind map into the editor, with its title and session', async () => {
    renderScreen()

    expect(screen.getByText('Loading the mind map...')).toBeInTheDocument()
    const board = await screen.findByTestId('editor')
    expect(board).toHaveAttribute('data-xml', MAP.diagramXml)
    expect(getMindMap).toHaveBeenCalledWith('map-1')
    expect(screen.getByRole('heading', { name: 'Caching map' })).toBeInTheDocument()
    expect(screen.getByText('Distributed Caching')).toBeInTheDocument()
    expect(screen.getByText('Preparing the editor...')).toBeInTheDocument()
  })

  it('says the mind map could not load and retries', async () => {
    const user = userEvent.setup()
    vi.mocked(getMindMap).mockRejectedValueOnce(new Error('nope'))
    renderScreen()

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not load the mind map.')
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByTestId('editor')).toBeInTheDocument()
  })

  it('goes back from a failed load', async () => {
    const user = userEvent.setup()
    vi.mocked(getMindMap).mockRejectedValue(new Error('nope'))
    const { onExit } = renderScreen()

    await user.click(await screen.findByRole('button', { name: 'Back to mind maps' }))

    expect(onExit).toHaveBeenCalledOnce()
  })

  it('grants a navigation request at once while nothing is open', async () => {
    vi.mocked(getMindMap).mockReturnValue(new Promise(() => {}))
    const { onExit } = renderScreen({ leaveRequested: true })

    await waitFor(() => expect(onExit).toHaveBeenCalled())
  })

  it('names the way back after where the map was opened from', async () => {
    renderScreen({ returnTo: { kind: 'study', sessionId: 's1' } })

    expect(await screen.findByRole('button', { name: 'Back to study session' })).toBeInTheDocument()
  })

  it('shows the editor once it reports it is ready', async () => {
    await openEditor()

    expect(screen.queryByText('Preparing the editor...')).not.toBeInTheDocument()
  })

  it('keeps Save disabled until the map is edited', async () => {
    await openEditor()

    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
    edit()
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
  })

  it('saves the diagram on the board and clears the unsaved mark', async () => {
    const { user } = await openEditor()
    vi.mocked(saveMindMapDiagram).mockResolvedValue({ ...MAP, diagramXml: xmlOnBoard })
    edit()

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(saveMindMapDiagram).toHaveBeenCalledWith('map-1', '<mxGraphModel>edited</mxGraphModel>')
    await waitFor(() => expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  it('keeps the unsaved mark when the map is edited again while saving', async () => {
    const { user } = await openEditor()
    let finishSave: (saved: MindMap) => void = () => {}
    vi.mocked(saveMindMapDiagram).mockReturnValue(
      new Promise((resolve) => {
        finishSave = resolve
      }),
    )
    edit()
    await user.click(screen.getByRole('button', { name: 'Save' }))

    edit()
    await act(async () => finishSave({ ...MAP, diagramXml: xmlOnBoard }))

    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  it('shows Saving... while the diagram is being saved', async () => {
    const { user } = await openEditor()
    vi.mocked(saveMindMapDiagram).mockReturnValue(new Promise(() => {}))
    edit()

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(screen.getByText('Saving...')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  it('keeps the edits and says why a save failed', async () => {
    const { user } = await openEditor()
    vi.mocked(saveMindMapDiagram).mockRejectedValue(
      new Error(JSON.stringify({ code: 'diagram_too_large' })),
    )
    edit()

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not save the mind map. Your changes are still on screen. The diagram is too large to save.',
    )
    expect(screen.getByText('Unsaved changes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
  })

  it('goes back at once when nothing is unsaved', async () => {
    const { user, onExit } = await openEditor()

    await user.click(screen.getByRole('button', { name: 'Back to mind maps' }))

    expect(onExit).toHaveBeenCalledOnce()
  })

  it('asks before going back with unsaved edits, and stays when told to keep editing', async () => {
    const { user, onExit } = await openEditor()
    edit()

    await user.click(screen.getByRole('button', { name: 'Back to mind maps' }))
    expect(screen.getByText('Leave without saving?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Keep editing' }))

    expect(onExit).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('leaves and discards the edits when confirmed', async () => {
    const { user, onExit } = await openEditor()
    edit()

    await user.click(screen.getByRole('button', { name: 'Back to mind maps' }))
    await user.click(screen.getByRole('button', { name: 'Leave' }))

    expect(onExit).toHaveBeenCalledOnce()
    expect(saveMindMapDiagram).not.toHaveBeenCalled()
  })

  it('grants a navigation request at once when nothing is unsaved', async () => {
    const { rerender, onExit } = await openEditor()

    rerender(true)

    await waitFor(() => expect(onExit).toHaveBeenCalled())
  })

  it('asks about a navigation request with unsaved edits, and tells the shell when cancelled', async () => {
    const { user, rerender, onExit, onLeaveCancelled } = await openEditor()
    edit()

    rerender(true)
    expect(await screen.findByText('Leave without saving?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Keep editing' }))

    expect(onExit).not.toHaveBeenCalled()
    expect(onLeaveCancelled).toHaveBeenCalledOnce()
  })

  it('does not tell the shell about a leave question the user started themselves', async () => {
    const { user, onLeaveCancelled } = await openEditor()
    edit()

    await user.click(screen.getByRole('button', { name: 'Back to mind maps' }))
    await user.click(screen.getByRole('button', { name: 'Keep editing' }))

    expect(onLeaveCancelled).not.toHaveBeenCalled()
  })

  it('asks before updating from the session, then shows the redrawn map', async () => {
    const { user } = await openEditor()
    edit()
    vi.mocked(regenerateMindMap).mockResolvedValue({
      ...MAP,
      title: 'Redrawn',
      diagramXml: '<mxGraphModel>redrawn</mxGraphModel>',
    })

    await user.click(screen.getByRole('button', { name: 'Update from session' }))
    expect(regenerateMindMap).not.toHaveBeenCalled()
    expect(screen.getByText('Update this mind map from the session?')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Update' }))

    expect(regenerateMindMap).toHaveBeenCalledWith('map-1')
    expect(await screen.findByRole('heading', { name: 'Redrawn' })).toBeInTheDocument()
    expect(screen.getByTestId('editor')).toHaveAttribute(
      'data-xml',
      '<mxGraphModel>redrawn</mxGraphModel>',
    )
    expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument()
  })

  it('keeps the map when the update is cancelled', async () => {
    const { user } = await openEditor()

    await user.click(screen.getByRole('button', { name: 'Update from session' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(regenerateMindMap).not.toHaveBeenCalled()
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })

  it('shows progress while updating and cancels the redraw on request', async () => {
    const { user } = await openEditor()
    let reject: (reason: unknown) => void = () => {}
    vi.mocked(regenerateMindMap).mockReturnValue(
      new Promise((_, rejectFn) => {
        reject = rejectFn
      }),
    )
    await user.click(screen.getByRole('button', { name: 'Update from session' }))
    await user.click(screen.getByRole('button', { name: 'Update' }))

    expect(
      screen.getByText('Redrawing from the session... this can take a moment.'),
    ).toBeInTheDocument()
    const buttons = screen.getAllByRole('button', { name: 'Cancel' })
    await user.click(buttons[buttons.length - 1])
    reject(new Error(JSON.stringify({ code: 'cancelled' })))

    expect(cancelMindMapGeneration).toHaveBeenCalledOnce()
    await waitFor(() =>
      expect(
        screen.queryByText('Redrawing from the session... this can take a moment.'),
      ).not.toBeInTheDocument(),
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('says an update failed and that the map was left as it was', async () => {
    const { user } = await openEditor()
    vi.mocked(regenerateMindMap).mockRejectedValue(
      new Error(JSON.stringify({ code: 'invalid_generation' })),
    )

    await user.click(screen.getByRole('button', { name: 'Update from session' }))
    await user.click(screen.getByRole('button', { name: 'Update' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Could not update the mind map. It was left as it was. The generated mind map was not valid. Try again.',
    )
    expect(screen.getByRole('heading', { name: 'Caching map' })).toBeInTheDocument()
  })

  it('asks before deleting, deletes and leaves when confirmed', async () => {
    const { user, onExit } = await openEditor()
    vi.mocked(deleteMindMap).mockResolvedValue(undefined)

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    expect(deleteMindMap).not.toHaveBeenCalled()
    expect(screen.getByText('Delete this mind map?')).toBeInTheDocument()
    await user.click(screen.getAllByRole('button', { name: 'Delete' }).pop()!)

    expect(deleteMindMap).toHaveBeenCalledWith('map-1')
    await waitFor(() => expect(onExit).toHaveBeenCalledOnce())
  })

  it('says when a deletion fails and stays on the map', async () => {
    const { user, onExit } = await openEditor()
    vi.mocked(deleteMindMap).mockRejectedValue(new Error('nope'))

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.click(screen.getAllByRole('button', { name: 'Delete' }).pop()!)

    expect(await screen.findByRole('alert')).toHaveTextContent('Could not delete the mind map.')
    expect(onExit).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDefined()
  })

  it('leaves when the map is already gone on delete', async () => {
    const { user, onExit } = await openEditor()
    vi.mocked(deleteMindMap).mockRejectedValue(
      new Error(JSON.stringify({ code: 'mind_map_not_found' })),
    )

    await user.click(screen.getByRole('button', { name: 'Delete' }))
    await user.click(screen.getAllByRole('button', { name: 'Delete' }).pop()!)

    await waitFor(() => expect(onExit).toHaveBeenCalledOnce())
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('clears the unsaved mark when an edit arrived while the diagram was being exported', async () => {
    const { user } = await openEditor()
    vi.mocked(saveMindMapDiagram).mockResolvedValue({ ...MAP, diagramXml: xmlOnBoard })
    edit()
    // An edit event lands during the export, so it is already in the exported XML.
    const board = editor
    onExportEdit = () => act(() => board.onChange('<mxGraphModel>late</mxGraphModel>'))

    await user.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument())
    onExportEdit = null
  })

  it('shows Saved briefly after a save', async () => {
    const { user } = await openEditor()
    vi.mocked(saveMindMapDiagram).mockResolvedValue({ ...MAP, diagramXml: xmlOnBoard })
    edit()

    await user.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByText('Saved')).toBeInTheDocument()
  })

  it('waits for a save in flight before granting a navigation request', async () => {
    const { user, onExit, rerender } = await openEditor()
    let finishSave: (saved: MindMap) => void = () => {}
    vi.mocked(saveMindMapDiagram).mockReturnValue(
      new Promise((resolve) => {
        finishSave = resolve
      }),
    )
    edit()
    await user.click(screen.getByRole('button', { name: 'Save' }))

    rerender(true)
    expect(onExit).not.toHaveBeenCalled()
    await act(async () => finishSave({ ...MAP, diagramXml: xmlOnBoard }))

    await waitFor(() => expect(onExit).toHaveBeenCalledOnce())
  })

  it('cancels a redraw in flight when a navigation request arrives, then leaves', async () => {
    const { user, onExit, rerender } = await openEditor()
    let reject: (reason: unknown) => void = () => {}
    vi.mocked(regenerateMindMap).mockReturnValue(
      new Promise((_, rejectFn) => {
        reject = rejectFn
      }),
    )
    await user.click(screen.getByRole('button', { name: 'Update from session' }))
    await user.click(screen.getByRole('button', { name: 'Update' }))

    rerender(true)
    expect(cancelMindMapGeneration).toHaveBeenCalledOnce()
    expect(onExit).not.toHaveBeenCalled()
    await act(async () => reject(new Error(JSON.stringify({ code: 'cancelled' }))))

    await waitFor(() => expect(onExit).toHaveBeenCalledOnce())
  })

  it('warns while the connection to the editor is lost and clears it when restored', async () => {
    await openEditor()

    act(() => editor.onConnectionLost())
    expect(screen.getByText('The editor needs an internet connection.')).toBeInTheDocument()
    act(() => editor.onConnectionRestored())

    expect(screen.queryByText('The editor needs an internet connection.')).not.toBeInTheDocument()
  })

  it('says the editor needs the internet when the browser is offline, and tries again', async () => {
    const user = userEvent.setup()
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(false)
    renderScreen()

    expect(await screen.findByText('The editor needs an internet connection.')).toBeInTheDocument()
    expect(screen.queryByTestId('editor')).not.toBeInTheDocument()
    vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true)
    await user.click(screen.getByRole('button', { name: 'Try again' }))

    expect(await screen.findByTestId('editor')).toBeInTheDocument()
  })

  it('keeps loading and says it is slow when the editor is late but the browser is online', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      renderScreen()
      await screen.findByTestId('editor')

      await act(() => vi.advanceTimersByTimeAsync(15000))

      expect(screen.getByText(/taking longer than usual/)).toBeInTheDocument()
      expect(screen.getByTestId('editor')).toBeInTheDocument()
      expect(screen.queryByText('The editor needs an internet connection.')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not run the load timer once the editor is ready', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      renderScreen()
      await screen.findByTestId('editor')
      act(() => editor.onReady())

      await act(() => vi.advanceTimersByTimeAsync(20000))

      expect(screen.queryByText('The editor needs an internet connection.')).not.toBeInTheDocument()
    } finally {
      vi.useRealTimers()
    }
  })

  it('speaks the profile language', async () => {
    setLabLanguage('pt')
    renderScreen()

    expect(
      await screen.findByRole('button', { name: 'Voltar aos mapas mentais' }),
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Atualizar pela sessão' })).toBeInTheDocument()
  })
})
