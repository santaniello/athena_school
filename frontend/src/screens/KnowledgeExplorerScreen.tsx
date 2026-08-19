import { useEffect, useState } from 'react'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { TagInput } from '@/components/tag-input'
import { KnowledgeDeleteDialog } from '@/components/knowledge-delete-dialog'
import { cn } from '@/lib/utils'
import { onIngestDone } from '@/lib/ingest'
import {
  approveKnowledgeItem,
  definitionPreview,
  deleteKnowledgeItem,
  deprecateKnowledgeItem,
  groupByTopic,
  listKnowledgeItems,
  updateKnowledgeItem,
  type KnowledgeItem,
  type KnowledgeItemEdit,
} from '@/lib/knowledge'

interface KnowledgeExplorerScreenProps {
  // null means "All topics".
  selectedTopic: string | null
  // 'review' forces the status filter to draft and hides the picker — the
  // review-queue shortcut described in the Explorer/Review tabs.
  mode: 'explorer' | 'review'
}

const STATUS_LABELS: Record<string, string> = {
  draft: 'Draft',
  approved: 'Approved',
  deprecated: 'Deprecated',
}

const SOURCE_LABELS: Record<string, string> = {
  athena: 'Athena',
  user_note: 'User note',
  imported_doc: 'Imported note',
}

const GENERIC_ERROR = 'An error occurred. Please try again.'

function statusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
}

function sourceLabel(source: string): string {
  return SOURCE_LABELS[source] ?? source
}

function toEdit(item: KnowledgeItem): KnowledgeItemEdit {
  return {
    topic: item.topic,
    concept: item.concept,
    definition: item.definition,
    properties: item.properties,
    tradeOffs: item.tradeOffs,
    relatedConcepts: item.relatedConcepts,
  }
}

function FieldList({ label, values }: { label: string; values: string[] }) {
  if (values.length === 0) return null
  return (
    <div>
      <h3 className="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
        {label}
      </h3>
      <ul className="mt-1 list-disc space-y-0.5 pl-5 text-sm text-foreground">
        {values.map((value) => (
          <li key={value}>{value}</li>
        ))}
      </ul>
    </div>
  )
}

function KnowledgeExplorerScreen({ selectedTopic, mode }: KnowledgeExplorerScreenProps) {
  const [items, setItems] = useState<KnowledgeItem[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [isEditing, setIsEditing] = useState(false)
  const [draft, setDraft] = useState<KnowledgeItemEdit | null>(null)
  const [deletingItem, setDeletingItem] = useState<KnowledgeItem | null>(null)
  const [error, setError] = useState('')

  const effectiveStatus = mode === 'review' ? 'draft' : statusFilter

  useEffect(() => {
    // A topic/status change (or an ingest:done firing mid-flight) can
    // start a new request before an older one resolves; ignore lets a
    // stale response arriving after that lose the race instead of
    // overwriting the current filter's items with the previous one's.
    let ignore = false
    function load() {
      listKnowledgeItems(selectedTopic ?? '', effectiveStatus)
        .then((result) => {
          if (!ignore) setItems(result)
        })
        .catch(() => {
          if (!ignore) setError('Failed to load knowledge items.')
        })
    }
    load()
    const unsubscribe = onIngestDone(load)
    return () => {
      ignore = true
      unsubscribe()
    }
  }, [selectedTopic, effectiveStatus])

  const selectedItem = items.find((item) => item.id === selectedId) ?? null
  const groups = groupByTopic(items)

  function selectItem(item: KnowledgeItem) {
    setSelectedId(item.id)
    setIsEditing(false)
    setError('')
  }

  // A status transition (approve/deprecate) can move updated out of the
  // active status filter — e.g. approving a draft while the Review tab's
  // filter is fixed to draft. Drop it from the list instead of patching it
  // in place so it does not linger under a filter it no longer matches.
  function patchItem(updated: KnowledgeItem) {
    const noLongerMatches = effectiveStatus !== '' && updated.status !== effectiveStatus
    setItems((previous) =>
      noLongerMatches
        ? previous.filter((item) => item.id !== updated.id)
        : previous.map((item) => (item.id === updated.id ? updated : item)),
    )
    if (noLongerMatches) {
      setSelectedId((current) => (current === updated.id ? null : current))
    }
  }

  async function handleApprove(item: KnowledgeItem) {
    try {
      patchItem(await approveKnowledgeItem(item.id))
    } catch {
      setError(GENERIC_ERROR)
    }
  }

  async function handleDeprecate(item: KnowledgeItem) {
    try {
      patchItem(await deprecateKnowledgeItem(item.id))
    } catch {
      setError(GENERIC_ERROR)
    }
  }

  function startEditing(item: KnowledgeItem) {
    setDraft(toEdit(item))
    setIsEditing(true)
  }

  async function handleSaveEdit() {
    if (!selectedItem || !draft) return
    try {
      patchItem(await updateKnowledgeItem(selectedItem.id, draft))
      setIsEditing(false)
    } catch {
      setError(GENERIC_ERROR)
    }
  }

  async function handleConfirmDelete() {
    if (!deletingItem) return
    const id = deletingItem.id
    setDeletingItem(null)
    try {
      await deleteKnowledgeItem(id)
      setItems((previous) => previous.filter((item) => item.id !== id))
      setSelectedId((current) => (current === id ? null : current))
    } catch {
      setError(GENERIC_ERROR)
    }
  }

  return (
    <div className="flex h-full w-full flex-col gap-4">
      {mode === 'explorer' && (
        <div className="flex items-center gap-2">
          <Label htmlFor="knowledge-status-filter" className="text-sm text-muted-foreground">
            Status:
          </Label>
          <Select
            value={statusFilter || 'all'}
            onValueChange={(value) => setStatusFilter(value === 'all' ? '' : value)}
          >
            <SelectTrigger id="knowledge-status-filter" size="sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All</SelectItem>
              <SelectItem value="draft">Draft</SelectItem>
              <SelectItem value="approved">Approved</SelectItem>
              <SelectItem value="deprecated">Deprecated</SelectItem>
            </SelectContent>
          </Select>
        </div>
      )}

      {error && (
        <Alert variant="destructive">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="flex min-h-0 flex-1">
        <div className="thin-scroll w-80 shrink-0 space-y-4 overflow-y-auto border-r pr-4">
          {items.length === 0 ? (
            <p className="text-sm text-muted-foreground">No items found.</p>
          ) : (
            Array.from(groups.entries()).map(([topic, topicItems]) => (
              <div key={topic}>
                <h3 className="mb-1 text-xs font-semibold tracking-wide text-muted-foreground uppercase">
                  {topic}
                </h3>
                <div className="flex flex-col gap-1">
                  {topicItems.map((item) => (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => selectItem(item)}
                      className={cn(
                        'flex flex-col items-start gap-1 rounded-lg border p-2 text-left hover:bg-accent',
                        item.id === selectedId && 'bg-secondary',
                      )}
                    >
                      <span className="flex w-full items-center justify-between gap-2">
                        <span className="truncate font-medium text-foreground">
                          {item.concept}
                        </span>
                        <Badge variant={item.status === 'approved' ? 'default' : 'muted'}>
                          {statusLabel(item.status)}
                        </Badge>
                      </span>
                      <span className="line-clamp-2 text-xs text-muted-foreground">
                        {definitionPreview(item.definition, 100)}
                      </span>
                      <Badge variant="muted">{sourceLabel(item.source)}</Badge>
                    </button>
                  ))}
                </div>
              </div>
            ))
          )}
        </div>

        <div className="thin-scroll flex-1 overflow-y-auto pl-4">
          {!selectedItem ? (
            <p className="text-sm text-muted-foreground">Select an item to see the details.</p>
          ) : isEditing && draft ? (
            <div className="flex max-w-lg flex-col gap-3">
              <div>
                <Label htmlFor="knowledge-edit-topic">Topic</Label>
                <Input
                  id="knowledge-edit-topic"
                  value={draft.topic}
                  onChange={(event) => setDraft({ ...draft, topic: event.target.value })}
                />
              </div>
              <div>
                <Label htmlFor="knowledge-edit-concept">Concept</Label>
                <Input
                  id="knowledge-edit-concept"
                  value={draft.concept}
                  onChange={(event) => setDraft({ ...draft, concept: event.target.value })}
                />
              </div>
              <div>
                <Label htmlFor="knowledge-edit-definition">Definition</Label>
                <Textarea
                  id="knowledge-edit-definition"
                  value={draft.definition}
                  onChange={(event) => setDraft({ ...draft, definition: event.target.value })}
                />
              </div>
              <div>
                <Label htmlFor="knowledge-edit-properties">Properties</Label>
                <TagInput
                  id="knowledge-edit-properties"
                  value={draft.properties}
                  onChange={(properties) => setDraft({ ...draft, properties })}
                />
              </div>
              <div>
                <Label htmlFor="knowledge-edit-tradeoffs">Trade-offs</Label>
                <TagInput
                  id="knowledge-edit-tradeoffs"
                  value={draft.tradeOffs}
                  onChange={(tradeOffs) => setDraft({ ...draft, tradeOffs })}
                />
              </div>
              <div>
                <Label htmlFor="knowledge-edit-related">Related concepts</Label>
                <TagInput
                  id="knowledge-edit-related"
                  value={draft.relatedConcepts}
                  onChange={(relatedConcepts) => setDraft({ ...draft, relatedConcepts })}
                />
              </div>
              <div className="flex gap-2 pt-2">
                <Button onClick={() => void handleSaveEdit()}>Save</Button>
                <Button variant="outline" onClick={() => setIsEditing(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center gap-2">
                <h2 className="font-heading text-lg font-bold text-foreground">
                  {selectedItem.concept}
                </h2>
                <Badge variant={selectedItem.status === 'approved' ? 'default' : 'muted'}>
                  {statusLabel(selectedItem.status)}
                </Badge>
                <Badge variant="muted">{sourceLabel(selectedItem.source)}</Badge>
              </div>
              <p className="text-sm text-muted-foreground">{selectedItem.topic}</p>
              <p className="text-sm whitespace-pre-wrap text-foreground">
                {selectedItem.definition}
              </p>
              <FieldList label="Properties" values={selectedItem.properties} />
              <FieldList label="Trade-offs" values={selectedItem.tradeOffs} />
              <FieldList label="Related concepts" values={selectedItem.relatedConcepts} />

              <div className="flex flex-wrap gap-2 pt-2">
                {selectedItem.status === 'draft' && (
                  <Button onClick={() => void handleApprove(selectedItem)}>Approve</Button>
                )}
                {selectedItem.status === 'approved' && (
                  <Button variant="outline" onClick={() => void handleDeprecate(selectedItem)}>
                    Deprecate
                  </Button>
                )}
                <Button variant="outline" onClick={() => startEditing(selectedItem)}>
                  Edit
                </Button>
                <Button variant="destructive" onClick={() => setDeletingItem(selectedItem)}>
                  Delete
                </Button>
              </div>
            </div>
          )}
        </div>
      </div>

      <KnowledgeDeleteDialog
        item={deletingItem}
        onCancel={() => setDeletingItem(null)}
        onConfirm={() => void handleConfirmDelete()}
      />
    </div>
  )
}

export default KnowledgeExplorerScreen
