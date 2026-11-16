import { GetProfile, SaveProfile, UpdateProfile } from '../../wailsjs/go/desktop/App'
import { desktop } from '../../wailsjs/go/models'
import { setLabLanguage } from '@/i18n'

// Plain-string mirror of UserProfileInput, used to hold the onboarding draft
// while it is being filled in and edited (no CreatedAt: that is stamped by
// the backend on save).
export interface ProfileDraft {
  name: string
  assistantName: string
  area: string
  experienceLevel: string
  studyStyle: string
  assistantLanguage: string
}

// Error mapping happens at the call site (see lib/onboardingErrors.ts), same
// as authErrorMessage.
export async function saveUserProfile(draft: ProfileDraft): Promise<void> {
  await SaveProfile(desktop.UserProfileInput.createFrom(draft))
  setLabLanguage(draft.assistantLanguage)
}

// Reads the saved profile back, for the home screen greeting (see
// specs/phases/phase-01-desktop-mvp/03-home-screen.md).
export async function getUserProfile(): Promise<ProfileDraft> {
  const profile = await GetProfile()
  // The Architecture Lab follows the assistant language of the profile.
  setLabLanguage(profile.assistantLanguage)
  return {
    name: profile.name,
    assistantName: profile.assistantName,
    area: profile.area,
    experienceLevel: profile.experienceLevel,
    studyStyle: profile.studyStyle,
    assistantLanguage: profile.assistantLanguage,
  }
}

// Updates the already-saved profile from the settings screen, preserving
// its original creation date on the backend. Returns the saved (trimmed)
// draft so callers can sync local state without a second GetProfile call.
// See specs/phases/phase-01-desktop-mvp/08-settings.md.
export async function updateUserProfile(draft: ProfileDraft): Promise<ProfileDraft> {
  const saved = await UpdateProfile(desktop.UserProfileInput.createFrom(draft))
  setLabLanguage(saved.assistantLanguage)
  return {
    name: saved.name,
    assistantName: saved.assistantName,
    area: saved.area,
    experienceLevel: saved.experienceLevel,
    studyStyle: saved.studyStyle,
    assistantLanguage: saved.assistantLanguage,
  }
}
