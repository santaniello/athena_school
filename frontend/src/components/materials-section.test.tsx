import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MaterialsSection } from './materials-section'

describe('MaterialsSection', () => {
  it('shows the title and the count in a header button that reports its state', () => {
    render(
      <MaterialsSection title="Quizzes" count={3} open={false} onToggle={vi.fn()}>
        body
      </MaterialsSection>,
    )

    const header = screen.getByRole('button', { name: /Quizzes/ })
    expect(header).toHaveAttribute('aria-expanded', 'false')
    expect(header).toHaveTextContent('3')
  })

  it('hides the body while closed and shows it while open', () => {
    const { rerender } = render(
      <MaterialsSection title="Quizzes" count={0} open={false} onToggle={vi.fn()}>
        the body
      </MaterialsSection>,
    )
    expect(screen.queryByText('the body')).not.toBeInTheDocument()

    rerender(
      <MaterialsSection title="Quizzes" count={0} open onToggle={vi.fn()}>
        the body
      </MaterialsSection>,
    )
    expect(screen.getByText('the body')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Quizzes/ })).toHaveAttribute('aria-expanded', 'true')
  })

  it('calls onToggle when the header is clicked', async () => {
    const onToggle = vi.fn()
    render(
      <MaterialsSection title="Quizzes" count={0} open={false} onToggle={onToggle}>
        body
      </MaterialsSection>,
    )

    await userEvent.click(screen.getByRole('button', { name: /Quizzes/ }))

    expect(onToggle).toHaveBeenCalledOnce()
  })

  it('renders its actions beside the header, outside the toggle button', async () => {
    const onToggle = vi.fn()
    render(
      <MaterialsSection
        title="Sources"
        count={1}
        open
        onToggle={onToggle}
        actions={<button>Add</button>}
      >
        body
      </MaterialsSection>,
    )

    await userEvent.click(screen.getByRole('button', { name: 'Add' }))

    expect(onToggle).not.toHaveBeenCalled()
  })

  it('gives an open body its own scroll area and a closed section no flexible height', () => {
    const { container, rerender } = render(
      <MaterialsSection title="Quizzes" count={0} open onToggle={vi.fn()}>
        body
      </MaterialsSection>,
    )
    expect(screen.getByRole('region', { name: 'Quizzes' })).toHaveClass('overflow-y-auto')
    expect(container.firstElementChild).toHaveClass('flex-1')

    rerender(
      <MaterialsSection title="Quizzes" count={0} open={false} onToggle={vi.fn()}>
        body
      </MaterialsSection>,
    )
    expect(container.firstElementChild).not.toHaveClass('flex-1')
  })
})
