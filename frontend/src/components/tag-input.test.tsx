import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TagInput } from './tag-input'

describe('TagInput', () => {
  it('renders the existing tags', () => {
    // Given a value with two tags
    render(<TagInput value={['SQL', 'System Design']} onChange={vi.fn()} />)

    // When inspecting the rendered tags
    // Then both are shown
    expect(screen.getByText('SQL')).toBeInTheDocument()
    expect(screen.getByText('System Design')).toBeInTheDocument()
  })

  it('adds a tag when Enter is pressed', async () => {
    // Given an empty tag list
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={[]} onChange={onChange} />)

    // When typing a goal and pressing Enter
    await user.type(screen.getByRole('textbox'), 'SQL{Enter}')

    // Then onChange is called with the new tag appended
    expect(onChange).toHaveBeenCalledWith(['SQL'])
  })

  it('adds a tag when a comma is typed', async () => {
    // Given an empty tag list
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={[]} onChange={onChange} />)

    // When typing a goal followed by a comma
    await user.type(screen.getByRole('textbox'), 'Java,')

    // Then onChange is called with the new tag appended
    expect(onChange).toHaveBeenCalledWith(['Java'])
  })

  it('does not add a blank or duplicate tag', async () => {
    // Given a tag list that already contains "SQL"
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={['SQL']} onChange={onChange} />)

    // When pressing Enter with an empty draft, and typing the existing tag again
    await user.type(screen.getByRole('textbox'), '{Enter}')
    await user.type(screen.getByRole('textbox'), 'SQL{Enter}')

    // Then onChange is never called
    expect(onChange).not.toHaveBeenCalled()
  })

  it('removes a tag when its remove control is clicked', async () => {
    // Given a tag list with one entry
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={['SQL']} onChange={onChange} />)

    // When clicking the remove control for that tag
    await user.click(screen.getByRole('button', { name: 'Remove SQL' }))

    // Then onChange is called with the tag removed
    expect(onChange).toHaveBeenCalledWith([])
  })

  it('removes the last tag on Backspace when the draft is empty', async () => {
    // Given a tag list with two entries and an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={['SQL', 'Java']} onChange={onChange} />)

    // When pressing Backspace in the empty input
    await user.type(screen.getByRole('textbox'), '{Backspace}')

    // Then onChange is called with the last tag removed
    expect(onChange).toHaveBeenCalledWith(['SQL'])
  })

  it('trims whitespace from a tag before adding it', async () => {
    // Given an empty tag list
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={[]} onChange={onChange} />)

    // When typing a goal padded with spaces and pressing Enter
    await user.type(screen.getByRole('textbox'), '  SQL  {Enter}')

    // Then onChange is called with the trimmed tag
    expect(onChange).toHaveBeenCalledWith(['SQL'])
  })

  it('does not remove a tag on Backspace while the draft still has text', async () => {
    // Given a tag list with one entry and some text already typed
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={['SQL']} onChange={onChange} />)
    await user.type(screen.getByRole('textbox'), 'ab')

    // When pressing Backspace (deleting a character from the draft, not a tag)
    await user.type(screen.getByRole('textbox'), '{Backspace}')

    // Then onChange is never called
    expect(onChange).not.toHaveBeenCalled()
  })

  it('does nothing on Backspace when there are no tags and the draft is empty', async () => {
    // Given an empty tag list and an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<TagInput value={[]} onChange={onChange} />)

    // When pressing Backspace
    await user.type(screen.getByRole('textbox'), '{Backspace}')

    // Then onChange is never called
    expect(onChange).not.toHaveBeenCalled()
  })

  it('shows the placeholder only while there are no tags yet', () => {
    // Given an empty tag list with a placeholder
    const { rerender } = render(
      <TagInput value={[]} onChange={vi.fn()} placeholder="Digite um objetivo" />,
    )

    // Then the placeholder is shown
    expect(screen.getByPlaceholderText('Digite um objetivo')).toBeInTheDocument()

    // When a tag already exists
    rerender(<TagInput value={['SQL']} onChange={vi.fn()} placeholder="Digite um objetivo" />)

    // Then the placeholder is no longer shown
    expect(screen.queryByPlaceholderText('Digite um objetivo')).not.toBeInTheDocument()
  })
})
