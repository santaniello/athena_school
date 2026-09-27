import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { NavItem } from './nav-item'
import { NAVIGATION } from '@/lib/navigation'

const homeItem = NAVIGATION.find((item) => item.id === 'home')!
// challenge stays locked for the whole of Phase 2, unlike knowledge — a
// stable fixture for "locked item" behavior regardless of which section
// unlocks next.
const lockedItem = NAVIGATION.find((item) => item.id === 'challenge')!

describe('NavItem', () => {
  it('calls onSelect with the item id when an unlocked item is clicked', async () => {
    // Given an unlocked nav item
    const onSelect = vi.fn()
    const user = userEvent.setup()
    render(<NavItem item={homeItem} active={false} onSelect={onSelect} />)

    // When it is clicked
    await user.click(screen.getByRole('button', { name: 'Home' }))

    // Then onSelect fires with its id
    expect(onSelect).toHaveBeenCalledWith('home')
  })

  it('marks the active item so it is distinguishable from the rest', () => {
    // Given the active item
    render(<NavItem item={homeItem} active onSelect={vi.fn()} />)

    // Then it is flagged as the current section
    expect(screen.getByRole('button', { name: 'Home' })).toHaveAttribute('aria-current', 'page')
  })

  it('still calls onSelect when a locked item is clicked, never a dead click', async () => {
    // Given a locked nav item
    const onSelect = vi.fn()
    const user = userEvent.setup()
    render(<NavItem item={lockedItem} active={false} onSelect={onSelect} />)

    // When it is clicked
    await user.click(screen.getByRole('button', { name: lockedItem.label }))

    // Then onSelect still fires, so it can route to the coming-soon panel
    expect(onSelect).toHaveBeenCalledWith(lockedItem.id)
  })

  it('shows a "planned for phase" tooltip on a locked item', async () => {
    // Given a locked nav item
    const user = userEvent.setup()
    render(<NavItem item={lockedItem} active={false} onSelect={vi.fn()} />)

    // When hovering it
    await user.hover(screen.getByRole('button', { name: lockedItem.label }))

    // Then a tooltip explains when it ships
    expect(await screen.findByText(`Planned for Phase ${lockedItem.phase}`)).toBeInTheDocument()
  })
})
