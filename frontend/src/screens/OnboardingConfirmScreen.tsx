import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { TagInput } from '@/components/tag-input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { AuthLayout } from '@/components/auth-layout'
import { saveUserProfile, type ProfileDraft } from '@/lib/profile'
import { profileErrorMessage } from '@/lib/onboardingErrors'

interface OnboardingConfirmScreenProps {
  draft: ProfileDraft
  onChange: (draft: ProfileDraft) => void
  onConfirmed: () => void
}

const FIELD_LABELS: Record<keyof ProfileDraft, string> = {
  name: 'Nome',
  assistantName: 'Nome do assistente',
  area: 'Área',
  experienceLevel: 'Nível de experiência',
  goals: 'Objetivos',
  studyStyle: 'Estilo de estudo',
}

const EXPERIENCE_LEVEL_LABELS: Record<string, string> = {
  beginner: 'Iniciante',
  intermediate: 'Intermediário',
  advanced: 'Avançado',
}

function displayValue(field: keyof ProfileDraft, draft: ProfileDraft): string {
  if (field === 'goals') return draft.goals.join(', ')
  if (field === 'experienceLevel') {
    return EXPERIENCE_LEVEL_LABELS[draft.experienceLevel] ?? draft.experienceLevel
  }
  return draft[field] as string
}

// Confirmation screen: read-only summary with per-field inline editing, and
// the single SaveProfile call for the whole onboarding flow.
function OnboardingConfirmScreen({ draft, onChange, onConfirmed }: OnboardingConfirmScreenProps) {
  const [editingField, setEditingField] = useState<keyof ProfileDraft | null>(null)
  const [error, setError] = useState('')
  const [isSaving, setIsSaving] = useState(false)

  async function handleConfirm() {
    setError('')
    setIsSaving(true)
    try {
      await saveUserProfile(draft)
      onConfirmed()
    } catch (err) {
      setError(profileErrorMessage(err))
    } finally {
      setIsSaving(false)
    }
  }

  function renderEditor(field: keyof ProfileDraft) {
    if (field === 'goals') {
      return (
        <div className="flex flex-col gap-1.5">
          <TagInput
            value={draft.goals}
            onChange={(goals) => onChange({ ...draft, goals })}
            placeholder="Digite um objetivo e pressione Enter"
            aria-label={FIELD_LABELS.goals}
          />
          <p className="text-xs text-muted-foreground">
            Pressione Enter ou vírgula após cada objetivo para adicioná-lo.
          </p>
        </div>
      )
    }
    if (field === 'experienceLevel') {
      return (
        <Select
          value={draft.experienceLevel}
          onValueChange={(value) => onChange({ ...draft, experienceLevel: value })}
        >
          <SelectTrigger className="w-full" aria-label={FIELD_LABELS.experienceLevel}>
            <SelectValue placeholder="Selecione um nível" />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(EXPERIENCE_LEVEL_LABELS).map(([value, label]) => (
              <SelectItem key={value} value={value}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    }
    return (
      <Input
        value={draft[field] as string}
        onChange={(event) => onChange({ ...draft, [field]: event.target.value })}
        aria-label={FIELD_LABELS[field]}
      />
    )
  }

  return (
    <AuthLayout title="Confirme seu perfil">
      <div className="flex flex-col gap-3 text-left">
        {(Object.keys(FIELD_LABELS) as Array<keyof ProfileDraft>).map((field) => (
          <div
            key={field}
            data-testid={`onboarding-confirm-row-${field}`}
            className="flex flex-col gap-1.5 border-b border-border pb-3"
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-medium text-muted-foreground">
                {FIELD_LABELS[field]}
              </span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setEditingField(editingField === field ? null : field)}
              >
                {editingField === field ? 'Salvar' : 'Editar'}
              </Button>
            </div>
            {editingField === field ? (
              renderEditor(field)
            ) : (
              <span className="text-sm">{displayValue(field, draft)}</span>
            )}
          </div>
        ))}
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <Button type="button" onClick={handleConfirm} disabled={isSaving}>
        {isSaving ? 'Salvando...' : 'Confirmar e salvar'}
      </Button>
    </AuthLayout>
  )
}

export default OnboardingConfirmScreen
