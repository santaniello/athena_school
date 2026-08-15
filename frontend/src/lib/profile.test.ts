import { describe, expect, it, vi } from 'vitest'
import { SaveProfile } from '../../wailsjs/go/desktop/App'
import { saveUserProfile, type ProfileDraft } from './profile'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveProfile: vi.fn(),
}))

const draft: ProfileDraft = {
  name: 'Ana',
  assistantName: 'Atena',
  area: 'Engenharia de Software',
  experienceLevel: 'intermediate',
  goals: ['SQL', 'System Design'],
  studyStyle: 'practical_examples',
  assistantLanguage: 'en',
}

describe('saveUserProfile', () => {
  it('delegates to the SaveProfile binding', async () => {
    // Given a binding that accepts the profile
    vi.mocked(SaveProfile).mockResolvedValueOnce()

    // When saving the draft
    await saveUserProfile(draft)

    // Then the binding is called with the equivalent UserProfileInput
    expect(SaveProfile).toHaveBeenCalledWith(expect.objectContaining(draft))
  })

  it('propagates a rejection from the binding unchanged', async () => {
    // Given a binding that rejects with a domain validation sentinel
    const err = new Error('at least one goal is required')
    vi.mocked(SaveProfile).mockRejectedValueOnce(err)

    // When saving the draft
    // Then the same error propagates, unmapped
    await expect(saveUserProfile({ ...draft, goals: [] })).rejects.toThrow(err)
  })
})
