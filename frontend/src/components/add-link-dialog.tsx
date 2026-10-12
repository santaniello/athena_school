import { useState } from 'react'
import type { FormEvent } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { isHttpUrl } from '@/lib/utils'

interface AddLinkDialogProps {
  open: boolean
  onCancel: () => void
  // Receives the trimmed URL; the caller starts the link preview.
  onSubmit: (url: string) => void
}

// The form lives in its own component, rendered inside DialogContent, so
// its typed value is discarded every time the dialog closes and a reopened
// dialog never starts with the previous link. See
// specs/phases/phase-02-knowledge-engine/21-add-link-source.md.
function AddLinkForm({ onCancel, onSubmit }: Omit<AddLinkDialogProps, 'open'>) {
  const [url, setUrl] = useState('')
  const valid = isHttpUrl(url)

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (valid) onSubmit(url.trim())
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <DialogHeader>
        <DialogTitle>Add a link</DialogTitle>
        <DialogDescription>
          Athena reads the page, formats it like a note, and lets you review it before it becomes a
          source.
        </DialogDescription>
      </DialogHeader>
      <Input
        type="url"
        aria-label="Link URL"
        placeholder="https://example.com/article"
        value={url}
        onChange={(event) => setUrl(event.target.value)}
        autoFocus
      />
      <DialogFooter>
        <Button type="button" variant="outline" onClick={onCancel}>
          Cancel
        </Button>
        <Button type="submit" disabled={!valid}>
          Continue
        </Button>
      </DialogFooter>
    </form>
  )
}

export function AddLinkDialog({ open, onCancel, onSubmit }: AddLinkDialogProps) {
  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onCancel()}>
      <DialogContent className="sm:max-w-md">
        <AddLinkForm onCancel={onCancel} onSubmit={onSubmit} />
      </DialogContent>
    </Dialog>
  )
}
