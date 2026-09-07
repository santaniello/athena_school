import {
  forwardRef,
  useCallback,
  useEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react'
import { FolderTree } from 'lucide-react'
import { cn } from '@/lib/utils'
import { listKnowledgeTopics } from '@/lib/knowledge'
import { onIngestDone } from '@/lib/ingest'

interface KnowledgeTopicTreeProps {
  // null means "All topics" — no topic filter applied.
  selectedTopic: string | null
  onSelectTopic: (topic: string | null) => void
}

// KnowledgeTopicTreeHandle lets a caller outside this tree (AppShell, after
// a Knowledge Explorer delete or a topic-changing edit — neither of which
// fires ingest:done) force the same refetch a notes import already
// triggers internally, instead of leaving a since-removed or since-renamed
// topic lingering in the sidebar until the next import or a full restart.
export interface KnowledgeTopicTreeHandle {
  reload: () => void
}

// The Knowledge section's sidebar navigation: a flat topic list plus an
// "All topics" row, mirroring where StudyFolderTree renders under Study in
// app-shell.tsx. Unlike folders, topics are not a separately managed
// entity — they are derived from Items, so there is no create/rename/
// delete here, only selection. Topics reload after a notes import
// completes, since importing can introduce topics that did not exist yet,
// and on demand via the imperative handle above.
const KnowledgeTopicTree = forwardRef<KnowledgeTopicTreeHandle, KnowledgeTopicTreeProps>(
  function KnowledgeTopicTree({ selectedTopic, onSelectTopic }, ref) {
    const [topics, setTopics] = useState<string[]>([])
    const [error, setError] = useState('')
    // Shared across the initial mount, every onIngestDone firing, and every
    // imperative reload() call, so only the most recently *started* call's
    // response (or rejection) is ever applied, regardless of which trigger
    // started it.
    const requestVersionRef = useRef(0)
    const mountedRef = useRef(true)

    const loadTopics = useCallback(() => {
      const version = ++requestVersionRef.current
      listKnowledgeTopics()
        .then((result) => {
          if (mountedRef.current && version === requestVersionRef.current) {
            setError('')
            setTopics(result)
          }
        })
        .catch(() => {
          if (mountedRef.current && version === requestVersionRef.current) {
            setError('Failed to load topics.')
          }
        })
    }, [])

    useImperativeHandle(ref, () => ({ reload: loadTopics }), [loadTopics])

    useEffect(() => {
      mountedRef.current = true
      loadTopics()
      const unsubscribe = onIngestDone(loadTopics)
      return () => {
        mountedRef.current = false
        unsubscribe()
      }
    }, [loadTopics])

    function rowClassName(active: boolean) {
      return cn(
        'flex cursor-pointer items-center gap-1.5 rounded-md py-1 pr-3 pl-10 text-left text-xs hover:bg-accent',
        active && 'bg-secondary',
      )
    }

    return (
      <div className="flex flex-col gap-0.5 py-0.5">
        <button
          type="button"
          className="flex cursor-pointer items-center gap-1.5 rounded-md py-1 pr-3 pl-6 text-left text-xs hover:bg-accent"
          onClick={() => onSelectTopic(null)}
        >
          <FolderTree className="size-3.5 shrink-0 text-primary" aria-hidden="true" />
          <span className="flex-1 truncate font-medium text-foreground">All topics</span>
        </button>
        {topics.map((topic) => {
          const selected = selectedTopic === topic
          return (
            <button
              key={topic}
              type="button"
              className={rowClassName(selected)}
              onClick={() => onSelectTopic(topic)}
            >
              <span
                className={cn(
                  'size-1.5 shrink-0 rounded-full',
                  selected ? 'bg-primary shadow-[0_0_6px_1px_var(--primary)]' : 'bg-muted-foreground',
                )}
                aria-hidden="true"
              />
              <span className="min-w-0 flex-1 truncate text-foreground">{topic}</span>
            </button>
          )
        })}
        {error && <p className="px-3 py-1 text-xs text-destructive">{error}</p>}
      </div>
    )
  },
)

export { KnowledgeTopicTree }
