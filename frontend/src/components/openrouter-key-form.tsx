import { useState } from 'react'
import { saveOpenRouterKey } from '@/lib/openrouterKey'
import { openRouterKeyErrorMessage } from '@/lib/onboardingErrors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface OpenRouterKeyFormProps {
  onSaved: () => void
}

// Reusable masked-key + validate-and-save form. Shared by the onboarding
// key gate and, later, the settings screen (see
// specs/phases/phase-01-desktop-mvp/08-settings.md) — do not duplicate.
function OpenRouterKeyForm({ onSaved }: OpenRouterKeyFormProps) {
  const [key, setKey] = useState('')
  const [error, setError] = useState('')
  const [isValidating, setIsValidating] = useState(false)

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    setError('')
    setIsValidating(true)
    try {
      await saveOpenRouterKey(key)
      onSaved()
    } catch (err) {
      setError(openRouterKeyErrorMessage(err))
    } finally {
      setIsValidating(false)
    }
  }

  return (
    <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
      <div className="flex flex-col gap-1.5 text-left">
        <Label htmlFor="openrouter-key">OpenRouter key</Label>
        <Input
          id="openrouter-key"
          type="password"
          value={key}
          onChange={(event) => setKey(event.target.value)}
          required
        />
      </div>

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <Button type="submit" disabled={isValidating}>
        {isValidating ? 'Validating...' : 'Connect'}
      </Button>
    </form>
  )
}

export { OpenRouterKeyForm }
