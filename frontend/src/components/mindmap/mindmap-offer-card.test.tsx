import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  acceptMindMapOffer,
  cancelMindMapGeneration,
  declineMindMapOffer,
  type MindMapSummary,
} from '@/lib/mindmap'
import { setLabLanguage } from '@/i18n'
import { MindMapOfferCard } from './mindmap-offer-card'

vi.mock('@/lib/mindmap', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/mindmap')>()),
  acceptMindMapOffer: vi.fn(),
  declineMindMapOffer: vi.fn(),
  cancelMindMapGeneration: vi.fn(),
}))

const CREATED: MindMapSummary = {
  id: 'map-1',
  sessionId: 's1',
  sessionTitle: 'Caching',
  title: 'Caching map',
  createdAt: '2026-11-17T09:30:00Z',
  updatedAt: '2026-11-17T09:30:00Z',
  messageId: 'm2',
}

function renderCard() {
  const handlers = { onAccepted: vi.fn(), onDeclined: vi.fn(), onGone: vi.fn() }
  render(<MindMapOfferCard offerId="offer-1" {...handlers} />)
  return handlers
}

describe('MindMapOfferCard', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setLabLanguage('en')
  })

  it('offers to draw the mind map, with Accept and Decline', () => {
    renderCard()

    expect(screen.getByText('Draw a mind map of this session')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it('accepts and hands the new map to the chat', async () => {
    const user = userEvent.setup()
    vi.mocked(acceptMindMapOffer).mockResolvedValue(CREATED)
    const { onAccepted } = renderCard()

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(acceptMindMapOffer).toHaveBeenCalledWith('offer-1')
    expect(onAccepted).toHaveBeenCalledWith(CREATED)
  })

  it('shows progress and locks both buttons while the map is drawn', async () => {
    const user = userEvent.setup()
    vi.mocked(acceptMindMapOffer).mockReturnValue(new Promise(() => {}))
    renderCard()

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(screen.getByText('Drawing your mind map... this can take a moment.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeDisabled()
  })

  it('keeps the offer and says why accepting failed, then works again', async () => {
    const user = userEvent.setup()
    vi.mocked(acceptMindMapOffer).mockRejectedValue(
      new Error(JSON.stringify({ code: 'generation_in_progress' })),
    )
    const { onAccepted } = renderCard()

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'A mind map is already being generated.',
    )
    expect(onAccepted).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Decline' })).toBeEnabled()
  })

  it('clears an old error when the user tries again', async () => {
    const user = userEvent.setup()
    vi.mocked(acceptMindMapOffer).mockRejectedValueOnce(new Error('nope'))
    vi.mocked(acceptMindMapOffer).mockReturnValueOnce(new Promise(() => {}))
    renderCard()
    await user.click(screen.getByRole('button', { name: 'Accept' }))
    await screen.findByRole('alert')

    await user.click(screen.getByRole('button', { name: 'Accept' }))

    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('declines and tells the chat', async () => {
    const user = userEvent.setup()
    vi.mocked(declineMindMapOffer).mockResolvedValue()
    const { onDeclined } = renderCard()

    await user.click(screen.getByRole('button', { name: 'Decline' }))

    expect(declineMindMapOffer).toHaveBeenCalledWith('offer-1')
    expect(onDeclined).toHaveBeenCalledOnce()
  })

  it('keeps the offer when declining fails', async () => {
    const user = userEvent.setup()
    vi.mocked(declineMindMapOffer).mockRejectedValue(new Error('nope'))
    const { onDeclined } = renderCard()

    await user.click(screen.getByRole('button', { name: 'Decline' }))

    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(onDeclined).not.toHaveBeenCalled()
  })

  it.each(['accept', 'decline'] as const)(
    'hands a vanished offer back to the chat when %s finds it gone',
    async (action) => {
      const user = userEvent.setup()
      const gone = new Error(JSON.stringify({ code: 'offer_not_found' }))
      vi.mocked(acceptMindMapOffer).mockRejectedValue(gone)
      vi.mocked(declineMindMapOffer).mockRejectedValue(gone)
      const { onGone } = renderCard()

      await user.click(
        screen.getByRole('button', { name: action === 'accept' ? 'Accept' : 'Decline' }),
      )

      expect(onGone).toHaveBeenCalledOnce()
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    },
  )

  it('offers to cancel only while the map is drawn, and says nothing after a cancel', async () => {
    const user = userEvent.setup()
    let reject: (err: Error) => void = () => {}
    vi.mocked(acceptMindMapOffer).mockReturnValue(
      new Promise((_, rej) => {
        reject = rej
      }),
    )
    vi.mocked(cancelMindMapGeneration).mockResolvedValue()
    renderCard()
    expect(screen.queryByRole('button', { name: 'Cancel' })).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Accept' }))
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(cancelMindMapGeneration).toHaveBeenCalledOnce()
    reject(new Error(JSON.stringify({ code: 'cancelled' })))

    expect(await screen.findByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})
