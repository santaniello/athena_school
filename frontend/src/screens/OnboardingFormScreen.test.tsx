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
    assistantLanguage: '',
  }
}

function completeDraft(): ProfileDraft {
  return {
    name: 'Ana',
    assistantName: 'Atena',
    area: 'Engenharia de Software',
    experienceLevel: 'intermediate',
    goals: ['SQL', 'System Design'],
    studyStyle: 'practical_examples',
    assistantLanguage: 'en',
  }
}

describe('OnboardingFormScreen', () => {
  it('disables Continue while required fields are missing', () => {
    // Given an empty draft
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={vi.fn()} onNext={vi.fn()} />)

    // When inspecting the submit button
    // Then it is disabled
    expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled()
  })

  it.each([
    ['name', { ...completeDraft(), name: '' }],
    ['assistantName', { ...completeDraft(), assistantName: '' }],
    ['area', { ...completeDraft(), area: '' }],
    ['experienceLevel', { ...completeDraft(), experienceLevel: '' }],
    ['goals', { ...completeDraft(), goals: [] }],
    ['studyStyle', { ...completeDraft(), studyStyle: '' }],
    ['assistantLanguage', { ...completeDraft(), assistantLanguage: '' }],
  ] as Array<[string, ProfileDraft]>)(
    'keeps Continue disabled when only %s is missing',
    (_field, draft) => {
      // Given every field filled in except one
      render(<OnboardingFormScreen draft={draft} onChange={vi.fn()} onNext={vi.fn()} />)

      // Then Continue stays disabled
      expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled()
    },
  )

  it('enables Continue once every field is filled and calls onNext on submit', async () => {
    // Given a fully filled draft
    const onNext = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={completeDraft()} onChange={vi.fn()} onNext={onNext} />)

    // When submitting the form
    const button = screen.getByRole('button', { name: 'Continue' })
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
    await user.type(screen.getByLabelText('Name'), 'A')

    // Then onChange is called with the updated draft
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), name: 'A' })
  })

  it('selects an experience level through the dropdown', async () => {
    // Given an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={onChange} onNext={vi.fn()} />)

    // When opening the experience level dropdown and picking a level
    await user.click(screen.getByRole('combobox', { name: 'Experience level' }))
    await user.click(await screen.findByRole('option', { name: 'Intermediate' }))

    // Then onChange is called with that level
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), experienceLevel: 'intermediate' })
  })

  it('selects an assistant language through the dropdown', async () => {
    // Given an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={onChange} onNext={vi.fn()} />)

    // When opening the assistant language dropdown and picking a language
    await user.click(screen.getByRole('combobox', { name: 'Assistant language' }))
    await user.click(await screen.findByRole('option', { name: 'Portuguese' }))

    // Then onChange is called with that language
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), assistantLanguage: 'pt' })
  })

  it('selects a study style through the dropdown', async () => {
    // Given an empty draft
    const onChange = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingFormScreen draft={emptyDraft()} onChange={onChange} onNext={vi.fn()} />)

    // When opening the study style dropdown and picking a style
    await user.click(screen.getByRole('combobox', { name: 'Preferred study style' }))
    await user.click(await screen.findByRole('option', { name: 'Detailed step by step' }))

    // Then onChange is called with that style
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), studyStyle: 'step_by_step' })
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
    await user.type(screen.getByLabelText('Goals'), 'SQL{Enter}')

    // Then onChange is called with the goal appended
    expect(onChange).toHaveBeenCalledWith({ ...emptyDraft(), goals: ['SQL'] })
  })
})
