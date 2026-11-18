import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { setLabLanguage } from '@/i18n'
import type { MindMapSummary } from '@/lib/mindmap'
import { MindMapRow } from './mindmap-row'

const MAP: MindMapSummary = {
  id: 'map-1',
  sessionId: 's1',
  sessionTitle: 'Distributed Caching',
  title: 'Caching map',
  createdAt: '2026-11-16T09:30:00Z',
  updatedAt: '2026-11-17T10:30:00Z',
  messageId: '',
}

function renderRow(highlighted = false) {
  const onOpen = vi.fn()
  const onDelete = vi.fn()
  render(
    <ul>
      <MindMapRow mindMap={MAP} highlighted={highlighted} onOpen={onOpen} onDelete={onDelete} />
    </ul>,
  )
  return { onOpen, onDelete }
}

describe('MindMapRow', () => {
  it('shows the title, the session and the last update', () => {
    setLabLanguage('en')
    renderRow()

    expect(screen.getByRole('listitem', { name: 'Caching map' })).toBeInTheDocument()
    expect(screen.getByText('Distributed Caching')).toBeInTheDocument()
    expect(screen.getByText(/^Updated .*2026/)).toBeInTheDocument()
  })

  it('marks the mind map that was just generated', () => {
    renderRow(true)

    expect(screen.getByRole('listitem')).toHaveAttribute('aria-current', 'true')
  })

  it('does not mark an ordinary row', () => {
    renderRow(false)

    expect(screen.getByRole('listitem')).not.toHaveAttribute('aria-current')
  })

  it('hands the row to Open and to Delete', async () => {
    const user = userEvent.setup()
    const { onOpen, onDelete } = renderRow()

    await user.click(screen.getByRole('button', { name: 'Open' }))
    await user.click(screen.getByRole('button', { name: 'Delete' }))

    expect(onOpen).toHaveBeenCalledWith(MAP)
    expect(onDelete).toHaveBeenCalledWith(MAP)
  })
})
