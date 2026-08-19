import { useMemo, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { saveExtractedKnowledge, type KnowledgeItem } from '@/lib/knowledge'

interface KnowledgeExtractionDialogProps {
  open: boolean
  items: KnowledgeItem[]
  onClose: () => void
}

export function KnowledgeExtractionDialog({
  open,
  items,
  onClose,
}: KnowledgeExtractionDialogProps) {
  const [selected, setSelected] = useState<Set<number>>(
    () => new Set(items.map((_, index) => index)),
  )
  const [saved, setSaved] = useState<Set<number>>(new Set())
  const [isSaving, setIsSaving] = useState(false)
  const [saveError, setSaveError] = useState('')

  const pendingIndices = useMemo(
    () => [...selected].filter((index) => !saved.has(index)).sort((a, b) => a - b),
    [saved, selected],
  )

  function toggle(index: number) {
    setSelected((previous) => {
      const next = new Set(previous)
      if (next.has(index)) next.delete(index)
      else next.add(index)
      return next
    })
  }

  async function handleSave() {
    if (pendingIndices.length === 0) return
    setIsSaving(true)
    setSaveError('')
    try {
      const result = await saveExtractedKnowledge(pendingIndices.map((index) => items[index]))
      const persistedIndices = result.savedIndices
        .map((index) => pendingIndices[index])
        .filter((index): index is number => index !== undefined)
      if (persistedIndices.length > 0) {
        setSaved((previous) => new Set([...previous, ...persistedIndices]))
      }
      if (result.error) {
        setSaveError(result.error)
        return
      }
      onClose()
    } catch (caught) {
      const error = caught instanceof Error ? caught : new Error('Falha ao salvar os rascunhos.')
      setSaveError(error.message)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && onClose()}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Novo conhecimento encontrado</DialogTitle>
          <DialogDescription>
            Revise os conceitos e escolha quais devem ser salvos como rascunho.
          </DialogDescription>
        </DialogHeader>

        {items.length === 0 ? (
          <p className="py-4 text-sm text-muted-foreground">Nenhum conhecimento novo encontrado</p>
        ) : (
          <div className="thin-scroll max-h-[50vh] space-y-3 overflow-y-auto">
            {items.map((item, index) => (
              <label key={item.id || index} className="flex gap-3 rounded-lg border p-3">
                <input
                  type="checkbox"
                  aria-label={`Selecionar ${item.concept}`}
                  checked={selected.has(index)}
                  disabled={saved.has(index) || isSaving}
                  onChange={() => toggle(index)}
                  className="mt-1 size-4 accent-primary"
                />
                <span className="min-w-0">
                  <span className="block font-medium text-foreground">{item.concept}</span>
                  <span className="mt-1 block text-sm text-muted-foreground">
                    {item.definition}
                  </span>
                  {saved.has(index) && (
                    <span className="mt-1 block text-xs text-primary">Salvo</span>
                  )}
                </span>
              </label>
            ))}
          </div>
        )}

        {saveError && (
          <Alert variant="destructive">
            <AlertDescription>{saveError}</AlertDescription>
          </Alert>
        )}

        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={isSaving}>
            Ignorar
          </Button>
          {items.length > 0 && (
            <Button
              onClick={() => void handleSave()}
              disabled={pendingIndices.length === 0 || isSaving}
            >
              {isSaving ? 'Salvando...' : saveError ? 'Tentar novamente' : 'Salvar como rascunhos'}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
