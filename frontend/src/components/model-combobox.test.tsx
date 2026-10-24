import { describe, expect, it, vi } from 'vitest'
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ModelCombobox } from './model-combobox'

const OPTIONS = ['anthropic/claude-sonnet-4', 'anthropic/claude-sonnet-4.5', 'openai/gpt-5']

function renderCombobox(overrides: Partial<React.ComponentProps<typeof ModelCombobox>> = {}) {
  const onSelect = vi.fn()
  render(
    <ModelCombobox
      label="Study chat model"
      value="anthropic/claude-sonnet-4.5"
      defaultValue="anthropic/claude-sonnet-4.5"
      options={OPTIONS}
      onSelect={onSelect}
      {...overrides}
    />,
  )
  return { onSelect }
}

describe('ModelCombobox', () => {
  it('shows the current value on a trigger named after the feature, closed by default', () => {
    renderCombobox({ value: 'openai/gpt-5' })

    const trigger = screen.getByRole('combobox', { name: 'Study chat model' })
    expect(trigger).toHaveTextContent('openai/gpt-5')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('marks the default value on the trigger only when it is the one in use', () => {
    renderCombobox()
    expect(screen.getByRole('combobox')).toHaveTextContent('default')
  })

  it('does not mark a non-default value as the default', () => {
    renderCombobox({ value: 'openai/gpt-5' })
    expect(screen.getByRole('combobox')).not.toHaveTextContent('default')
  })

  it('opens a searchable list of every option and flags the current and default ones', async () => {
    const user = userEvent.setup()
    renderCombobox({ value: 'openai/gpt-5' })

    await user.click(screen.getByRole('combobox'))

    expect(screen.getByRole('combobox')).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('textbox', { name: 'Search Study chat model' })).toHaveFocus()
    const options = within(screen.getByRole('listbox')).getAllByRole('option')
    expect(options.map((option) => option.textContent)).toEqual([
      'anthropic/claude-sonnet-4',
      'anthropic/claude-sonnet-4.5 (default)',
      'openai/gpt-5',
    ])
    expect(screen.getByRole('option', { name: 'openai/gpt-5' })).toHaveAttribute(
      'aria-selected',
      'true',
    )
    expect(screen.getByRole('option', { name: 'anthropic/claude-sonnet-4' })).toHaveAttribute(
      'aria-selected',
      'false',
    )
  })

  it('filters the options by a case-insensitive substring', async () => {
    const user = userEvent.setup()
    renderCombobox()
    await user.click(screen.getByRole('combobox'))

    await user.type(screen.getByRole('textbox'), 'SONNET')

    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(2)
    expect(screen.queryByRole('option', { name: 'openai/gpt-5' })).not.toBeInTheDocument()
  })

  it('says so when nothing matches', async () => {
    const user = userEvent.setup()
    renderCombobox()
    await user.click(screen.getByRole('combobox'))

    await user.type(screen.getByRole('textbox'), 'zzz')

    expect(screen.queryByRole('option')).not.toBeInTheDocument()
    expect(screen.getByText('No models match "zzz".')).toBeInTheDocument()
  })

  it('caps the rendered options and tells the user to refine the search', async () => {
    const user = userEvent.setup()
    const many = Array.from({ length: 80 }, (_, i) => `vendor/model-${String(i).padStart(2, '0')}`)
    renderCombobox({ options: many, value: many[0] })
    await user.click(screen.getByRole('combobox'))

    expect(screen.getAllByRole('option')).toHaveLength(50)
    expect(screen.getByText('30 more — refine your search.')).toBeInTheDocument()
  })

  it('reports the chosen model and closes the list', async () => {
    const user = userEvent.setup()
    const { onSelect } = renderCombobox()
    await user.click(screen.getByRole('combobox'))

    await user.click(screen.getByRole('option', { name: 'openai/gpt-5' }))

    expect(onSelect).toHaveBeenCalledExactlyOnceWith('openai/gpt-5')
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('does not report a selection of the model already in use', async () => {
    const user = userEvent.setup()
    const { onSelect } = renderCombobox({ value: 'openai/gpt-5' })
    await user.click(screen.getByRole('combobox'))

    await user.click(screen.getByRole('option', { name: 'openai/gpt-5' }))

    expect(onSelect).not.toHaveBeenCalled()
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('closes on Escape without selecting and clears the search for next time', async () => {
    const user = userEvent.setup()
    const { onSelect } = renderCombobox()
    await user.click(screen.getByRole('combobox'))
    await user.type(screen.getByRole('textbox'), 'gpt')

    await user.keyboard('{Escape}')

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
    expect(onSelect).not.toHaveBeenCalled()
    await user.click(screen.getByRole('combobox'))
    expect(screen.getByRole('textbox')).toHaveValue('')
  })

  it('closes when the trigger is clicked again', async () => {
    const user = userEvent.setup()
    renderCombobox()
    await user.click(screen.getByRole('combobox'))

    await user.click(screen.getByRole('combobox'))

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('closes when focus leaves the control', async () => {
    const user = userEvent.setup()
    renderCombobox()
    await user.click(screen.getByRole('combobox'))

    await user.click(document.body)

    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })

  it('cannot be opened while disabled', async () => {
    const user = userEvent.setup()
    renderCombobox({ disabled: true })

    await user.click(screen.getByRole('combobox'))

    expect(screen.getByRole('combobox')).toBeDisabled()
    expect(screen.queryByRole('listbox')).not.toBeInTheDocument()
  })
})
