import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  AcceptMindMapOffer,
  CancelMindMapGeneration,
  DeclineMindMapOffer,
  DeleteMindMap,
  GenerateMindMap,
  GetMindMap,
  GetPendingMindMapOffer,
  ListMindMaps,
  ListSessionMindMaps,
  RegenerateMindMap,
  SaveMindMapDiagram,
} from '../../wailsjs/go/desktop/App'
import { setLabLanguage } from '@/i18n'
import {
  acceptMindMapOffer,
  cancelMindMapGeneration,
  declineMindMapOffer,
  deleteMindMap,
  formatMindMapDate,
  generateMindMap,
  getMindMap,
  getPendingMindMapOffer,
  listMindMaps,
  listSessionMindMaps,
  mindMapErrorCode,
  mindMapErrorMessage,
  regenerateMindMap,
  saveMindMapDiagram,
} from './mindmap'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  AcceptMindMapOffer: vi.fn(),
  CancelMindMapGeneration: vi.fn(),
  DeclineMindMapOffer: vi.fn(),
  DeleteMindMap: vi.fn(),
  GenerateMindMap: vi.fn(),
  GetMindMap: vi.fn(),
  GetPendingMindMapOffer: vi.fn(),
  ListMindMaps: vi.fn(),
  ListSessionMindMaps: vi.fn(),
  RegenerateMindMap: vi.fn(),
  SaveMindMapDiagram: vi.fn(),
}))

const summary = {
  id: 'map-1',
  sessionId: 'session-1',
  sessionTitle: 'Caching',
  title: 'Caching basics',
  createdAt: '2026-11-17T09:30:00Z',
  updatedAt: '2026-11-17T10:30:00Z',
  messageId: '',
}

describe('mind map bindings', () => {
  beforeEach(() => vi.clearAllMocks())

  it('lists the mind maps and the maps of a session', async () => {
    vi.mocked(ListMindMaps).mockResolvedValue([summary])
    vi.mocked(ListSessionMindMaps).mockResolvedValue([{ ...summary, messageId: 'm1' }])

    expect(await listMindMaps()).toEqual([summary])
    expect(await listSessionMindMaps('session-1')).toEqual([{ ...summary, messageId: 'm1' }])
    expect(ListSessionMindMaps).toHaveBeenCalledWith('session-1')
  })

  it('reads a mind map with its diagram', async () => {
    const full = { ...summary, diagramXml: '<mxGraphModel/>' }
    vi.mocked(GetMindMap).mockResolvedValue(full)

    expect(await getMindMap('map-1')).toEqual(full)
    expect(GetMindMap).toHaveBeenCalledWith('map-1')
  })

  it('generates, regenerates, saves and deletes', async () => {
    const full = { ...summary, diagramXml: '<mxGraphModel/>' }
    vi.mocked(GenerateMindMap).mockResolvedValue(summary)
    vi.mocked(RegenerateMindMap).mockResolvedValue(full)
    vi.mocked(SaveMindMapDiagram).mockResolvedValue(full)
    vi.mocked(DeleteMindMap).mockResolvedValue()

    expect(await generateMindMap('session-1')).toEqual(summary)
    expect(await regenerateMindMap('map-1')).toEqual(full)
    expect(await saveMindMapDiagram('map-1', '<mxGraphModel/>')).toEqual(full)
    await deleteMindMap('map-1')
    expect(GenerateMindMap).toHaveBeenCalledWith('session-1')
    expect(RegenerateMindMap).toHaveBeenCalledWith('map-1')
    expect(SaveMindMapDiagram).toHaveBeenCalledWith('map-1', '<mxGraphModel/>')
    expect(DeleteMindMap).toHaveBeenCalledWith('map-1')
  })

  it('cancels a generation', async () => {
    vi.mocked(CancelMindMapGeneration).mockResolvedValue()

    await cancelMindMapGeneration()

    expect(CancelMindMapGeneration).toHaveBeenCalledOnce()
  })

  it('reads the pending offer of a session, or null', async () => {
    vi.mocked(GetPendingMindMapOffer).mockResolvedValueOnce({
      id: 'o1',
      sessionId: 'session-1',
      messageId: 'm1',
    })
    vi.mocked(GetPendingMindMapOffer).mockResolvedValueOnce(null as never)

    expect(await getPendingMindMapOffer('session-1')).toEqual({
      id: 'o1',
      sessionId: 'session-1',
      messageId: 'm1',
    })
    expect(await getPendingMindMapOffer('session-1')).toBeNull()
  })

  it('accepts and declines an offer', async () => {
    vi.mocked(AcceptMindMapOffer).mockResolvedValue(summary)
    vi.mocked(DeclineMindMapOffer).mockResolvedValue()

    expect(await acceptMindMapOffer('o1')).toEqual(summary)
    await declineMindMapOffer('o1')
    expect(AcceptMindMapOffer).toHaveBeenCalledWith('o1')
    expect(DeclineMindMapOffer).toHaveBeenCalledWith('o1')
  })
})

describe('mind map errors', () => {
  it('reads the code of a coded error, from an Error or a string', () => {
    const raw = JSON.stringify({ code: 'mind_map_not_found', message: 'x' })

    expect(mindMapErrorCode(new Error(raw))).toBe('mind_map_not_found')
    expect(mindMapErrorCode(raw)).toBe('mind_map_not_found')
  })

  it.each([undefined, 42, 'plain text', new Error('boom'), '{"code":5}', '{"nope":1}', 'null'])(
    'reads %s as no code',
    (err) => {
      expect(mindMapErrorCode(err)).toBe('')
    },
  )

  it.each([
    ['invalid_input', 'The mind map request is not valid.'],
    ['source_session_not_found', 'That study session no longer exists.'],
    ['mind_map_not_found', 'That mind map no longer exists.'],
    ['invalid_generation', 'The generated mind map was not valid. Try again.'],
    ['invalid_diagram', 'The diagram could not be saved because it is not valid.'],
    ['diagram_too_large', 'The diagram is too large to save.'],
    ['generation_in_progress', 'A mind map is already being generated.'],
    ['offer_not_found', 'This mind map offer is no longer available.'],
    ['offer_pending', 'A mind map offer is already waiting for an answer.'],
    ['cancelled', 'The mind map generation was cancelled.'],
  ])('writes a message for %s, in the profile language', (code, message) => {
    setLabLanguage('en')
    expect(mindMapErrorMessage(JSON.stringify({ code }))).toBe(message)
    setLabLanguage('pt')
    expect(mindMapErrorMessage(JSON.stringify({ code }))).not.toBe(message)
    expect(mindMapErrorMessage(JSON.stringify({ code }))).not.toBe(
      'Algo deu errado. Tente novamente.',
    )
    setLabLanguage('en')
  })

  it('falls back to a generic message for an unknown code or no code', () => {
    expect(mindMapErrorMessage(JSON.stringify({ code: 'internal' }))).toBe(
      'Something went wrong. Please try again.',
    )
    expect(mindMapErrorMessage('boom')).toBe('Something went wrong. Please try again.')
    setLabLanguage('pt')
    expect(mindMapErrorMessage('boom')).toBe('Algo deu errado. Tente novamente.')
    setLabLanguage('en')
  })
})

describe('formatMindMapDate', () => {
  it('follows the profile language', () => {
    setLabLanguage('en')
    expect(formatMindMapDate('2026-11-17T12:00:00Z')).toContain('2026')
    expect(formatMindMapDate('2026-11-17T12:00:00Z')).toContain('Nov')
    setLabLanguage('pt')
    expect(formatMindMapDate('2026-11-17T12:00:00Z')).toContain('nov')
    setLabLanguage('en')
  })
})
