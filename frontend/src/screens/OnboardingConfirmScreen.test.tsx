import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SaveProfile } from '../../wailsjs/go/desktop/App'
import OnboardingConfirmScreen from './OnboardingConfirmScreen'
import type { ProfileDraft } from '@/lib/profile'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveProfile: vi.fn(),
}))

// OnboardingConfirmScreen's draft is a controlled prop (the real owner is
// OnboardingScreen). This harness plays that role so typing into an inline
// editor behaves like it would in the actual app, instead of resetting to
// the initial value on every keystroke.
function ControlledHarness({ onChange }: { onChange: (draft: ProfileDraft) => void }) {
  const [draft, setDraft] = useState(completeDraft())
  return (
    <OnboardingConfirmScreen
      draft={draft}
      onChange={(next) => {
        setDraft(next)
        onChange(next)
      }}
      onConfirmed={vi.fn()}
    />
  )
}

function completeDraft(): ProfileDraft {
  return {
    name: 'Ana',
    assistantName: 'Atena',
    area: 'Engenharia de Software',
    experienceLevel: 'intermediate',
    goals: ['SQL', 'System Design'],
    studyStyle: 'Prática com exercícios',
  }
}

describe('OnboardingConfirmScreen', () => {
  it('shows a read-only summary of the draft', () => {
    // Given a filled draft
    render(
      <OnboardingConfirmScreen draft={completeDraft()} onChange={vi.fn()} onConfirmed={vi.fn()} />,
    )

    // When inspecting the rendered summary
    // Then every field value is shown as plain text
    expect(screen.getByText('Ana')).toBeInTheDocument()
    expect(screen.getByText('Atena')).toBeInTheDocument()
    expect(screen.getByText('SQL, System Design')).toBeInTheDocument()
    expect(screen.getByText('Intermediário')).toBeInTheDocument()
  })

  it('shows every field label', () => {
    // Given a filled draft
    render(
      <OnboardingConfirmScreen draft={completeDraft()} onChange={vi.fn()} onConfirmed={vi.fn()} />,
    )

    // Then every field label is shown
    for (const label of [
      'Nome',
      'Nome do assistente',
      'Área',
      'Nível de experiência',
      'Objetivos',
      'Estilo de estudo',
    ]) {
      expect(screen.getByText(label)).toBeInTheDocument()
    }
  })

  it('toggles a row between Editar and Salvar', async () => {
    // Given a filled draft
    const user = userEvent.setup()
    render(
      <OnboardingConfirmScreen draft={completeDraft()} onChange={vi.fn()} onConfirmed={vi.fn()} />,
    )
    const areaRow = within(screen.getByTestId('onboarding-confirm-row-area'))

    // When clicking Editar, then Salvar
    await user.click(areaRow.getByRole('button', { name: 'Editar' }))
    expect(areaRow.getByRole('button', { name: 'Salvar' })).toBeInTheDocument()
    await user.click(areaRow.getByRole('button', { name: 'Salvar' }))

    // Then it is back to the read-only display
    expect(areaRow.getByRole('button', { name: 'Editar' })).toBeInTheDocument()
    expect(areaRow.getByText('Engenharia de Software')).toBeInTheDocument()
  })

  it('edits a text field inline and reports the change through onChange', async () => {
    // Given a filled draft rendered through a controlled harness
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<ControlledHarness onChange={onChange} />)

    // When clicking Editar on the Area row and changing its value
    const areaRow = within(screen.getByTestId('onboarding-confirm-row-area'))
    await user.click(areaRow.getByRole('button', { name: 'Editar' }))
    const input = areaRow.getByLabelText('Área')
    await user.clear(input)
    await user.type(input, 'Design')

    // Then onChange was last called with the fully edited field
    expect(onChange).toHaveBeenLastCalledWith({ ...completeDraft(), area: 'Design' })
  })

  it('edits the goals field inline through the tag input', async () => {
    // Given a filled draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(
      <OnboardingConfirmScreen draft={completeDraft()} onChange={onChange} onConfirmed={vi.fn()} />,
    )
    const goalsRow = within(screen.getByTestId('onboarding-confirm-row-goals'))

    // When clicking Editar on the Objetivos row and adding a goal
    await user.click(goalsRow.getByRole('button', { name: 'Editar' }))
    await user.type(goalsRow.getByLabelText('Objetivos'), 'Java{Enter}')

    // Then onChange is called with the goal appended
    expect(onChange).toHaveBeenCalledWith({
      ...completeDraft(),
      goals: ['SQL', 'System Design', 'Java'],
    })
  })

  it('edits the experience level inline through the dropdown', async () => {
    // Given a filled draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(
      <OnboardingConfirmScreen draft={completeDraft()} onChange={onChange} onConfirmed={vi.fn()} />,
    )
    const levelRow = within(screen.getByTestId('onboarding-confirm-row-experienceLevel'))

    // When clicking Editar on the experience level row and picking a new level
    await user.click(levelRow.getByRole('button', { name: 'Editar' }))
    await user.click(screen.getByRole('combobox', { name: 'Nível de experiência' }))
    await user.click(await screen.findByRole('option', { name: 'Avançado' }))

    // Then onChange is called with the new level
    expect(onChange).toHaveBeenCalledWith({ ...completeDraft(), experienceLevel: 'advanced' })
  })

  it('shows no error alert before any confirmation attempt', () => {
    // Given a freshly rendered confirmation screen
    render(
      <OnboardingConfirmScreen draft={completeDraft()} onChange={vi.fn()} onConfirmed={vi.fn()} />,
    )

    // Then no error alert is shown
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('saves the profile and calls onConfirmed on success', async () => {
    // Given a SaveProfile call that succeeds
    vi.mocked(SaveProfile).mockResolvedValueOnce()
    const onConfirmed = vi.fn()
    const user = userEvent.setup()
    render(
      <OnboardingConfirmScreen
        draft={completeDraft()}
        onChange={vi.fn()}
        onConfirmed={onConfirmed}
      />,
    )

    // When clicking "Confirmar e salvar"
    await user.click(screen.getByRole('button', { name: 'Confirmar e salvar' }))

    // Then SaveProfile is called with the equivalent input and onConfirmed fires
    expect(SaveProfile).toHaveBeenCalledWith(expect.objectContaining(completeDraft()))
    await waitFor(() => expect(onConfirmed).toHaveBeenCalledOnce())
  })

  it('shows an inline error and does not call onConfirmed on failure', async () => {
    // Given a SaveProfile call that rejects
    vi.mocked(SaveProfile).mockRejectedValueOnce(new Error('at least one goal is required'))
    const onConfirmed = vi.fn()
    const user = userEvent.setup()
    render(
      <OnboardingConfirmScreen
        draft={completeDraft()}
        onChange={vi.fn()}
        onConfirmed={onConfirmed}
      />,
    )

    // When clicking "Confirmar e salvar"
    await user.click(screen.getByRole('button', { name: 'Confirmar e salvar' }))

    // Then an inline PT-BR error is shown and onConfirmed never fires
    expect(await screen.findByText('Adicione pelo menos um objetivo.')).toBeInTheDocument()
    expect(onConfirmed).not.toHaveBeenCalled()
  })
})
