import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import type { MindMapSummary } from '@/lib/mindmap'
import { setLabLanguage } from '@/i18n'
import { SessionMindMapCard } from './session-mindmap-card'

const MAP: MindMapSummary = {
  id: 'map-1',
  sessionId: 's1',
  sessionTitle: 'Caching',
  title: 'Caching map',
  createdAt: '2026-11-17T09:30:00Z',
  updatedAt: '2026-11-17T10:30:00Z',
  messageId: 'm2',
}

describe('SessionMindMapCard', () => {
  it('shows the title and when the map was last updated', () => {
    setLabLanguage('en')
    render(<SessionMindMapCard mindMap={MAP} onOpen={vi.fn()} />)

    expect(screen.getByText('Caching map')).toBeInTheDocument()
    expect(screen.getByText(/^Updated .*2026/)).toBeInTheDocument()
  })

  it('hands the map to Open', async () => {
    const user = userEvent.setup()
    const onOpen = vi.fn()
    render(<SessionMindMapCard mindMap={MAP} onOpen={onOpen} />)

    await user.click(screen.getByRole('button', { name: 'Open mind map' }))

    expect(onOpen).toHaveBeenCalledWith(MAP)
  })

  it('speaks the profile language', () => {
    setLabLanguage('pt')
    render(<SessionMindMapCard mindMap={MAP} onOpen={vi.fn()} />)

    expect(screen.getByRole('button', { name: 'Abrir mapa mental' })).toBeInTheDocument()
    setLabLanguage('en')
  })
})
