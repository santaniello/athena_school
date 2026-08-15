import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SaveProfile } from '../../wailsjs/go/desktop/App'
import OnboardingScreen from './OnboardingScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveProfile: vi.fn(),
}))

async function fillForm(user: ReturnType<typeof userEvent.setup>) {
  await user.type(screen.getByLabelText('Nome'), 'Ana')
  await user.type(screen.getByLabelText('Como quer chamar o assistente?'), 'Atena')
  await user.type(screen.getByLabelText('Área de atuação ou estudo'), 'Engenharia de Software')
  await user.click(screen.getByRole('combobox', { name: 'Nível de experiência' }))
  await user.click(await screen.findByRole('option', { name: 'Intermediário' }))
  await user.type(screen.getByLabelText('Objetivos'), 'SQL{Enter}')
  await user.type(screen.getByLabelText('Estilo de estudo preferido'), 'Prática')
}

describe('OnboardingScreen', () => {
  it('renders the form step first', () => {
    // Given a freshly rendered onboarding screen
    render(<OnboardingScreen onComplete={vi.fn()} />)

    // Then the form screen (not the confirmation screen) is shown
    expect(screen.getByRole('heading', { name: 'Conte sobre você' })).toBeInTheDocument()
  })

  it('moves to confirmation, saves, and completes onboarding', async () => {
    // Given the onboarding flow starting on the form step
    vi.mocked(SaveProfile).mockResolvedValueOnce()
    const onComplete = vi.fn()
    const user = userEvent.setup()
    render(<OnboardingScreen onComplete={onComplete} />)

    // When filling every field and continuing to the confirmation screen
    await fillForm(user)
    await user.click(screen.getByRole('button', { name: 'Continuar' }))

    // Then the confirmation screen shows the filled-in draft
    expect(screen.getByRole('heading', { name: 'Confirme seu perfil' })).toBeInTheDocument()
    expect(screen.getByText('Ana')).toBeInTheDocument()

    // When confirming and saving
    await user.click(screen.getByRole('button', { name: 'Confirmar e salvar' }))

    // Then the profile is saved and onComplete fires
    expect(SaveProfile).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Ana', experienceLevel: 'intermediate', goals: ['SQL'] }),
    )
    await waitFor(() => expect(onComplete).toHaveBeenCalledOnce())
  })
})
