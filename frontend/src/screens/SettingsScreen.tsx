import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { TagInput } from '@/components/tag-input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { OpenRouterKeyForm } from '@/components/openrouter-key-form'
import { hasOpenRouterKey } from '@/lib/openrouterKey'
import { updateUserProfile, type ProfileDraft } from '@/lib/profile'
import { profileErrorMessage } from '@/lib/onboardingErrors'
import { ASSISTANT_LANGUAGES, EXPERIENCE_LEVELS, STUDY_STYLES } from '@/lib/profileOptions'

interface SettingsScreenProps {
  profile: ProfileDraft
  onProfileUpdated: (profile: ProfileDraft) => void
}

// Lets the user edit the OpenRouter key and every profile field without
// re-running onboarding — both share validation/save logic with onboarding
// instead of duplicating it. See
// specs/phases/phase-01-desktop-mvp/08-settings.md.
function SettingsScreen({ profile, onProfileUpdated }: SettingsScreenProps) {
  const [draft, setDraft] = useState(profile)
  const [error, setError] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [hasKey, setHasKey] = useState<boolean | null>(null)

  useEffect(() => {
    void hasOpenRouterKey().then(setHasKey)
  }, [])

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
    <div className="mx-auto flex w-full max-w-lg flex-col gap-8">
      <div className="flex flex-col gap-4">
        <h2 className="font-heading text-sm font-bold tracking-[0.14em] text-foreground uppercase">
          Profile
        </h2>
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
            <Label htmlFor="settings-goals">Goals</Label>
            <TagInput
              id="settings-goals"
              value={draft.goals}
              onChange={(goals) => updateField('goals', goals)}
              placeholder="Type a goal and press Enter"
            />
            <p className="text-xs text-muted-foreground">
              Press Enter or comma after each goal to add it.
            </p>
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

      <div className="flex flex-col gap-4 border-t border-border pt-8">
        <h2 className="font-heading text-sm font-bold tracking-[0.14em] text-foreground uppercase">
          OpenRouter key
        </h2>
        {hasKey !== null && (
          <p className="text-sm text-muted-foreground">
            {hasKey
              ? 'A key is already configured. For security, it is never shown again — enter a new one below to replace it.'
              : 'No key configured yet.'}
          </p>
        )}
        <OpenRouterKeyForm onSaved={() => setHasKey(true)} />
      </div>
    </div>
  )
}

export default SettingsScreen
