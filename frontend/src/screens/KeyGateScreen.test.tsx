import { describe, expect, it, vi } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SaveOpenRouterKey } from '../../wailsjs/go/desktop/App'
import KeyGateScreen from './KeyGateScreen'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveOpenRouterKey: vi.fn(),
}))

describe('KeyGateScreen', () => {
  it('saves the key and calls onSaved', async () => {
    // Given a SaveOpenRouterKey call that succeeds
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce()
    const onSaved = vi.fn()
    const user = userEvent.setup()
    render(<KeyGateScreen onSaved={onSaved} />)

    // When the user submits a key
    await user.type(screen.getByLabelText('Chave da OpenRouter'), 'sk-or-valid')
    await user.click(screen.getByRole('button', { name: 'Conectar' }))

    // Then onSaved fires
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce())
  })
})
