import { describe, expect, it, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import OnboardingFormScreen from './OnboardingFormScreen'
import type { ProfileDraft } from '@/lib/profile'

function emptyDraft(): ProfileDraft {
  return {
    name: '',
    assistantName: '',
    area: '',
    experienceLevel: '',
    goals: [],
    studyStyle: '',
  }
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

describe('OnboardingFormScreen', () => {
  it('disables Continuar while required fields are missing', () => {
    // Given an empty draft
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={vi.fn()} onNext={vi.fn()} />)

    // When inspecting the submit button
    // Then it is disabled
    expect(screen.getByRole('button', { name: 'Continuar' })).toBeDisabled()
  })

  it.each([
    ['name', { ...completeDraft(), name: '' }],
    ['assistantName', { ...completeDraft(), assistantName: '' }],
    ['area', { ...completeDraft(), area: '' }],
    ['experienceLevel', { ...completeDraft(), experienceLevel: '' }],
    ['goals', { ...completeDraft(), goals: [] }],
    ['studyStyle', { ...completeDraft(), studyStyle: '' }],
  ] as Array<[string, ProfileDraft]>)(
    'keeps Continuar disabled when only %s is missing',
    (_field, draft) => {
      // Given every field filled in except one
      render(<OnboardingFormScreen draft={draft} onChange={vi.fn()} onNext={vi.fn()} />)

      // Then Continuar stays disabled
      expect(screen.getByRole('button', { name: 'Continuar' })).toBeDisabled()
    },
  )

  it('enables Continuar once every field is filled and calls onNext on submit', async () => {
    // Given a fully filled draft
    const onNext = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={completeDraft()} onChange={vi.fn()} onNext={onNext} />)

    // When submitting the form
    const button = screen.getByRole('button', { name: 'Continuar' })
    expect(button).toBeEnabled()
    await user.click(button)

    // Then onNext fires
    expect(onNext).toHaveBeenCalledOnce()
  })

  it('reports a field change through onChange', async () => {
    // Given an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={onChange} onNext={vi.fn()} />)

    // When typing into the name field
    await user.type(screen.getByLabelText('Nome'), 'A')

    // Then onChange is called with the updated draft
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), name: 'A' })
  })

  it('selects an experience level through the dropdown', async () => {
    // Given an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={onChange} onNext={vi.fn()} />)

    // When opening the experience level dropdown and picking a level
    await user.click(screen.getByRole('combobox', { name: 'Nível de experiência' }))
    await user.click(await screen.findByRole('option', { name: 'Intermediário' }))

    // Then onChange is called with that level
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), experienceLevel: 'intermediate' })
  })

  it('adds a goal through the tag input', async () => {
    // Given a draft with no goals
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(
      <OnboardingFormScreen
        draft={{ ...emptyDraft(), goals: [] }}
        onChange={onChange}
        onNext={vi.fn()}
      />,
    )

    // When typing a goal and pressing Enter in the goals field
    await user.type(screen.getByLabelText('Objetivos'), 'SQL{Enter}')

    // Then onChange is called with the goal appended
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), goals: ['SQL'] })
  })
})
