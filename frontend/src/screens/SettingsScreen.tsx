import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { OpenRouterKeyForm } from '@/components/openrouter-key-form'
import { ResetLocalDataDialog } from '@/components/reset-local-data-dialog'
import { hasOpenRouterKey } from '@/lib/openrouterKey'
import { ResetLocalData } from '../../wailsjs/go/desktop/App'

// Lets the user edit the OpenRouter key and reset local data. The profile
// fields live in ProfileScreen. See
// specs/phases/phase-01-desktop-mvp/08-settings.md.
function SettingsScreen() {
  const [hasKey, setHasKey] = useState<boolean | null>(null)
  const [isResetDialogOpen, setIsResetDialogOpen] = useState(false)
  const [isResetting, setIsResetting] = useState(false)
  const [resetError, setResetError] = useState('')

  useEffect(() => {
    void hasOpenRouterKey().then(setHasKey)
  }, [])

  async function handleResetLocalData() {
    setResetError('')
    setIsResetting(true)
    try {
      // On success the app reloads (see ResetLocalData in
      // internal/interfaces/desktop/reset.go), so there is nothing to
      // update here — this component is about to be torn down.
      await ResetLocalData()
    } catch (err) {
      setResetError(err instanceof Error ? err.message : 'Failed to reset local data.')
      setIsResetting(false)
    }
  }

  return (
    <div className="thin-scroll mx-auto flex w-full max-w-lg flex-col gap-8 overflow-y-auto pr-2">
      <div className="flex flex-col gap-4">
        <h2 className="font-heading text-lg font-semibold text-foreground">OpenRouter key</h2>
        {hasKey !== null && (
          <p className="text-sm text-muted-foreground">
            {hasKey
              ? 'A key is already configured. For security, it is never shown again — enter a new one below to replace it.'
              : 'No key configured yet.'}
          </p>
        )}
        <OpenRouterKeyForm onSaved={() => setHasKey(true)} />
      </div>

      <div className="flex flex-col gap-4 border-t border-border pt-8">
        <h2 className="font-heading text-lg font-semibold text-destructive">Danger zone</h2>
        <p className="text-sm text-muted-foreground">
          Permanently delete every study session, every folder you created, and your entire
          knowledge base. Your OpenRouter key and profile are not affected.
        </p>
        {resetError && (
          <Alert variant="destructive">
            <AlertDescription>{resetError}</AlertDescription>
          </Alert>
        )}
        <Button
          type="button"
          variant="destructive"
          className="self-start"
          onClick={() => setIsResetDialogOpen(true)}
        >
          Reset local data
        </Button>
      </div>

      <ResetLocalDataDialog
        open={isResetDialogOpen}
        pending={isResetting}
        onCancel={() => setIsResetDialogOpen(false)}
        onConfirm={() => void handleResetLocalData()}
      />
    </div>
  )
}

export default SettingsScreen
