import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import MainScreen from './MainScreen'

describe('MainScreen', () => {
  it('renders a welcome heading', () => {
    // Given no account email is known (e.g. an existing local session)
    render(<MainScreen email={undefined} />)

    // Then it still renders the placeholder main screen
    expect(screen.getByRole('heading', { name: /welcome/i })).toBeInTheDocument()
  })

  it('greets the logged-in account by email when known', () => {
    // Given the account email from a fresh login/register
    render(<MainScreen email="user@athena.dev" />)

    // Then it is shown on the screen
    expect(screen.getByText('user@athena.dev')).toBeInTheDocument()
  })
})
