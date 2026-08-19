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
import type { KnowledgeItem } from '@/lib/knowledge'

interface KnowledgeDeleteDialogProps {
  item: KnowledgeItem | null
  onCancel: () => void
  onConfirm: () => void
}

// Delete is irreversible, per the gating table in
// specs/phases/phase-02-knowledge-engine/03-notes-import-and-knowledge-explorer.md.
// For an imported note (Source === 'imported_doc'), the copy gains an extra
// sentence: deleting here does not delete the original file, and
// re-importing the same folder later will not bring this content back
// unless the file is edited first (see the spec's "Domain" section — the
// dedup mtime in ingested_files is untouched by this delete). knowledge.Item
// deliberately carries no file path, so this stays a general explanation
// rather than naming the specific file.
function KnowledgeDeleteDialog({ item, onCancel, onConfirm }: KnowledgeDeleteDialogProps) {
  return (
    <AlertDialog open={item !== null} onOpenChange={(open) => !open && onCancel()}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete &quot;{item?.concept}&quot;?</AlertDialogTitle>
          <AlertDialogDescription>
            This item and all of its indexed chunks will be permanently deleted. This action
            cannot be undone.
            {item?.source === 'imported_doc' && (
              <>
                {' '}
                The original file is not deleted, and reimporting the same folder without
                changing it will not bring this content back — edit the file before reimporting
                if you want to recover it.
              </>
            )}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={onConfirm}>Delete</AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}

export { KnowledgeDeleteDialog }
