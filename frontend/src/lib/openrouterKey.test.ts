import { describe, expect, it, vi } from 'vitest'
import { SaveOpenRouterKey } from '../../wailsjs/go/desktop/App'
import { saveOpenRouterKey } from './openrouterKey'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveOpenRouterKey: vi.fn(),
}))

describe('saveOpenRouterKey', () => {
  it('delegates to the SaveOpenRouterKey binding', async () => {
    // Given a binding that accepts the key
    vi.mocked(SaveOpenRouterKey).mockResolvedValueOnce()

    // When saving a key
    await saveOpenRouterKey('sk-or-valid')

    // Then the binding is called with that key
    expect(SaveOpenRouterKey).toHaveBeenCalledWith('sk-or-valid')
  })

  it('propagates a rejection from the binding unchanged', async () => {
    // Given a binding that rejects with the invalid-key sentinel
    const err = new Error('openrouter key is invalid or unauthorized')
    vi.mocked(SaveOpenRouterKey).mockRejectedValueOnce(err)

    // When saving the key
    // Then the same error propagates, unmapped
    await expect(saveOpenRouterKey('sk-or-invalid')).rejects.toThrow(err)
  })
})
