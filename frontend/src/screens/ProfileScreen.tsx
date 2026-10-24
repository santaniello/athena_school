import { useState } from 'react'
import type { FormEvent } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { updateUserProfile, type ProfileDraft } from '@/lib/profile'
import { profileErrorMessage } from '@/lib/onboardingErrors'
import { ASSISTANT_LANGUAGES, EXPERIENCE_LEVELS, STUDY_STYLES } from '@/lib/profileOptions'

interface ProfileScreenProps {
  profile: ProfileDraft
  onProfileUpdated: (profile: ProfileDraft) => void
}

// Lets the user edit every profile field without re-running onboarding —
// shares validation/save logic with onboarding instead of duplicating it.
// Moved out of SettingsScreen into its own sidebar section; see
// specs/phases/phase-01-desktop-mvp/08-settings.md.
function ProfileScreen({ profile, onProfileUpdated }: ProfileScreenProps) {
  const [draft, setDraft] = useState(profile)
  const [error, setError] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  function updateField<K extends keyof ProfileDraft>(field: K, value: ProfileDraft[K]) {
    setDraft((prev) => ({ ...prev, [field]: value }))
  }

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError('')
    setIsSaving(true)
    try {
      const saved = await updateUserProfile(draft)
      setDraft(saved)
      onProfileUpdated(saved)
    } catch (err) {
      setError(profileErrorMessage(err))
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div className="thin-scroll mx-auto flex w-full max-w-lg flex-col gap-8 overflow-y-auto pr-2">
      <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="settings-name">Name</Label>
          <Input
            id="settings-name"
            value={draft.name}
            onChange={(event) => updateField('name', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="settings-assistant-name">Assistant name</Label>
          <Input
            id="settings-assistant-name"
            value={draft.assistantName}
            onChange={(event) => updateField('assistantName', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="settings-assistant-language">Assistant language</Label>
          <Select
            value={draft.assistantLanguage}
            onValueChange={(value) => updateField('assistantLanguage', value)}
          >
            <SelectTrigger id="settings-assistant-language" className="w-full">
              <SelectValue placeholder="Select a language" />
            </SelectTrigger>
            <SelectContent>
              {ASSISTANT_LANGUAGES.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="settings-area">Area</Label>
          <Input
            id="settings-area"
            value={draft.area}
            onChange={(event) => updateField('area', event.target.value)}
            required
          />
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="settings-experience-level">Experience level</Label>
          <Select
            value={draft.experienceLevel}
            onValueChange={(value) => updateField('experienceLevel', value)}
          >
            <SelectTrigger id="settings-experience-level" className="w-full">
              <SelectValue placeholder="Select a level" />
            </SelectTrigger>
            <SelectContent>
              {EXPERIENCE_LEVELS.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="flex flex-col gap-1.5 text-left">
          <Label htmlFor="settings-study-style">Preferred study style</Label>
          <Select
            value={draft.studyStyle}
            onValueChange={(value) => updateField('studyStyle', value)}
          >
            <SelectTrigger id="settings-study-style" className="w-full">
              <SelectValue placeholder="Select a style" />
            </SelectTrigger>
            <SelectContent>
              {STUDY_STYLES.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}

        <Button type="submit" disabled={isSaving}>
          {isSaving ? 'Saving...' : 'Save changes'}
        </Button>
      </form>
    </div>
  )
}

export default ProfileScreen
