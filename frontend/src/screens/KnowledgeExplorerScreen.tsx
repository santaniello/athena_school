import { useEffect, useRef, useState } from 'react'
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
import { IngestProgressDialog } from '@/components/ingest-progress-dialog'
import { cn } from '@/lib/utils'
import { onIngestDone } from '@/lib/ingest'
import {
  approveKnowledgeItem,
  countUnindexedKnowledgeItems,
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
  // True while the knowledge index is retrying — see KnowledgeSectionProps.
  mutationsDisabled: boolean
  // Fired after approving or deleting a draft item — the two actions that
  // change how many drafts are pending review. Lets the sidebar/Review-tab
  // badge (owned by AppShell) stay live without a reload. See
  // specs/phases/phase-02-knowledge-engine/07-knowledge-review.md.
  onKnowledgeChanged?: () => void
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

function KnowledgeExplorerScreen({
  selectedTopic,
  mode,
  mutationsDisabled,
  onKnowledgeChanged,
}: KnowledgeExplorerScreenProps) {
  const [items, setItems] = useState<KnowledgeItem[]>([])
  const [statusFilter, setStatusFilter] = useState('')
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [isEditing, setIsEditing] = useState(false)
  const [draft, setDraft] = useState<KnowledgeItemEdit | null>(null)
  const [deletingItem, setDeletingItem] = useState<KnowledgeItem | null>(null)
  const [error, setError] = useState('')
  const [unindexedCount, setUnindexedCount] = useState(0)
  const [reindexOpen, setReindexOpen] = useState(false)

  const effectiveStatus = mode === 'review' ? 'draft' : statusFilter
  // handleApprove/handleDeprecate are async closures: the render active
  // when the button was clicked is not necessarily the render active when
  // the mutation resolves. A ref, synced after every render, lets
  // patchItem check the filter that is current *now* rather than the one
  // captured back when the closure was created.
  const effectiveStatusRef = useRef(effectiveStatus)
  useEffect(() => {
    effectiveStatusRef.current = effectiveStatus
  })

  useEffect(() => {
    // A topic/status change (or an ingest:done firing mid-flight, which
    // can itself start a second load before the first settles) can leave
    // multiple requests in flight for this effect at once; requestVersion
    // ensures only the most recently *started* request's response is
    // applied, and ignore (set on cleanup) drops any response arriving
    // after a topic/status change moved on entirely.
    let ignore = false
    let requestVersion = 0
    function load() {
      const version = ++requestVersion
      listKnowledgeItems(selectedTopic ?? '', effectiveStatus)
        .then((result) => {
          if (!ignore && version === requestVersion) {
            setError('')
            setItems(result)
          }
        })
        .catch(() => {
          if (!ignore && version === requestVersion) setError('Failed to load knowledge items.')
        })
    }
    load()
    const unsubscribe = onIngestDone(load)
    return () => {
      ignore = true
      unsubscribe()
    }
  }, [selectedTopic, effectiveStatus])

  // On mount only: this is not silent, money-spending auto-indexing — it
  // just counts what a user-triggered "Index now" would need to process,
  // rendered as a dismissible-by-fixing Alert below. A count-fetch failure
  // is not surfaced as a page error; the Alert simply stays hidden.
  useEffect(() => {
    let ignore = false
    countUnindexedKnowledgeItems()
      .then((count) => {
        if (!ignore) setUnindexedCount(count)
      })
      .catch(() => {})
    return () => {
      ignore = true
    }
  }, [])

  // Re-checks after the reindex dialog closes — a run that hits failures
  // (e.g. the OpenRouter key is missing) can still leave items unindexed,
  // so the alert's count must reflect whatever remains rather than
  // assuming a full run always clears it to zero.
  function handleReindexClose() {
    setReindexOpen(false)
    countUnindexedKnowledgeItems()
      .then(setUnindexedCount)
      .catch(() => {})
  }

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
  // Reads the filter via effectiveStatusRef (see above) rather than the
  // effectiveStatus captured when the calling closure started, so a
  // filter change made while the mutation was in flight is respected.
  function patchItem(updated: KnowledgeItem) {
    const currentStatus = effectiveStatusRef.current
    const noLongerMatches = currentStatus !== '' && updated.status !== currentStatus
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
      onKnowledgeChanged?.()
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
    const wasDraft = deletingItem.status === 'draft'
    setDeletingItem(null)
    try {
      await deleteKnowledgeItem(id)
      setItems((previous) => previous.filter((item) => item.id !== id))
      setSelectedId((current) => (current === id ? null : current))
      if (wasDraft) onKnowledgeChanged?.()
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

      {unindexedCount > 0 && (
        <Alert>
          <AlertDescription className="flex flex-wrap items-center justify-between gap-2">
            <span>⚠ {unindexedCount} knowledge items aren&apos;t indexed for search yet.</span>
            <Button size="sm" onClick={() => setReindexOpen(true)} disabled={mutationsDisabled}>
              Index now
            </Button>
          </AlertDescription>
        </Alert>
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
                        <span className="truncate font-medium text-foreground">{item.concept}</span>
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
                <Button onClick={() => void handleSaveEdit()} disabled={mutationsDisabled}>
                  Save
                </Button>
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
                  <Button
                    onClick={() => void handleApprove(selectedItem)}
                    disabled={mutationsDisabled}
                  >
                    Approve
                  </Button>
                )}
                {selectedItem.status === 'approved' && (
                  <Button
                    variant="outline"
                    onClick={() => void handleDeprecate(selectedItem)}
                    disabled={mutationsDisabled}
                  >
                    Deprecate
                  </Button>
                )}
                <Button
                  variant="outline"
                  onClick={() => startEditing(selectedItem)}
                  disabled={mutationsDisabled}
                >
                  Edit
                </Button>
                <Button
                  variant="destructive"
                  onClick={() => setDeletingItem(selectedItem)}
                  disabled={mutationsDisabled}
                >
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

      <IngestProgressDialog open={reindexOpen} kind="reindex" onClose={handleReindexClose} />
    </div>
  )
}

export default KnowledgeExplorerScreen
