import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'

interface EmbeddingChangeDialogProps {
  open: boolean
  pending: boolean
  from: string
  to: string
  // Shown inside the dialog: a modal dialog hides the page behind it from
  // assistive tech, so a save error rendered there would go unannounced.
  error?: string
  onCancel: () => void
  onConfirm: () => void
}

// Blocks a change of the embedding model until the user has read what it
// costs. Vectors of different models are never mixed, so after the change the
// existing notes stop being retrieved until they are re-imported. See
// specs/phases/phase-02-knowledge-engine/31-llm-model-param.md, decision 11.
function EmbeddingChangeDialog({
  open,
  pending,
  from,
  to,
  error,
  onCancel,
  onConfirm,
}: EmbeddingChangeDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={(next) => !next && !pending && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Change the embedding model?</AlertDialogTitle>
          <AlertDialogDescription asChild>
            <div className="flex flex-col gap-3">
              <p className="rounded-md bg-muted px-3 py-2 font-mono text-xs break-all text-foreground">
                <span>{from}</span>
                <span aria-hidden="true"> → </span>
                <span>{to}</span>
              </p>
              <ul className="list-disc space-y-1.5 pl-5">
                <li>
                  Retrieval will stop returning your existing notes until every note is re-imported.
                </li>
                <li>Re-importing consumes API credits.</li>
                <li>Until then, answers will not be grounded in your notes.</li>
                <li>Switching back restores the previous index without re-importing.</li>
              </ul>
            </div>
          </AlertDialogDescription>
        </AlertDialogHeader>
        {error && (
          <Alert variant="destructive">
            <AlertDescription>{error}</AlertDescription>
          </Alert>
        )}
        <AlertDialogFooter>
          <AlertDialogCancel disabled={pending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            // AlertDialogAction closes the dialog on click unless prevented;
            // it stays open while saving and closes through the caller.
            onClick={(event) => {
              event.preventDefault()
              onConfirm()
            }}
            disabled={pending}
          >
            {pending ? 'Changing...' : 'Change model'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { EmbeddingChangeDialog }
