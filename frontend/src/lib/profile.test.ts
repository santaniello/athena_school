import { describe, expect, it, vi } from 'vitest'
import { SaveProfile, UpdateProfile } from '../../wailsjs/go/desktop/App'
import { saveUserProfile, updateUserProfile, type ProfileDraft } from './profile'

vi.mock('../../wailsjs/go/desktop/App', () => ({
  SaveProfile: vi.fn(),
  UpdateProfile: vi.fn(),
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

describe('updateUserProfile', () => {
  it('delegates to the UpdateProfile binding and returns the saved draft', async () => {
    // Given a binding that accepts the edit and echoes the saved profile back
    const saved = { ...draft, assistantName: 'Nova Atena' }
    vi.mocked(UpdateProfile).mockResolvedValueOnce(saved)

    // When updating the draft
    const result = await updateUserProfile({ ...draft, assistantName: 'Nova Atena' })

    // Then the binding is called with the equivalent UserProfileInput and
    // its response is returned as the new ProfileDraft
    expect(UpdateProfile).toHaveBeenCalledWith(expect.objectContaining({ assistantName: 'Nova Atena' }))
    expect(result).toEqual(saved)
  })

  it('propagates a rejection from the binding unchanged', async () => {
    // Given a binding that rejects with a domain validation sentinel
    const err = new Error('at least one goal is required')
    vi.mocked(UpdateProfile).mockRejectedValueOnce(err)

    // When updating the draft
    // Then the same error propagates, unmapped
    await expect(updateUserProfile({ ...draft, goals: [] })).rejects.toThrow(err)
  })
})
